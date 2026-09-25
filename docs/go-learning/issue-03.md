# Issue #3 — PostgreSQL, Connection Pooling, Migrations & Initial Schema

---

## 1. Learning Objectives (Mục tiêu học tập)

Sau khi hoàn thành bài học này, bạn sẽ:
- Hiểu kiến trúc gói chuẩn `database/sql` trong Go và cơ chế hoạt động của Connection Pool.
- Nắm vững vai trò của Driver `pgx` và ý nghĩa của việc import ẩn với dấu gạch dưới `_`.
- Cấu hình chuẩn xác 4 tham số vàng của Connection Pool: `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`, `ConnMaxIdleTime`.
- Hiểu tại sao hàm `sql.Open` không thực sự mở kết nối và lý do bắt buộc phải gọi `PingContext`.
- Làm chủ công cụ `golang-migrate`, tư duy thiết kế schema với DDL lũy kế (Idempotent DDL), và quản lý container cơ sở dữ liệu với Docker Compose.

---

## 2. What Problem Does This Issue Solve? (Vấn đề thực tế)

Trong bất kỳ hệ thống quản lý tài chính nào, cơ sở dữ liệu là nơi lưu trữ thiêng liêng nhất của dữ liệu.
1. **Quá tải kết nối (Connection Exhaustion)**: Nếu mỗi request HTTP lại mở một kết nối TCP mới tới PostgreSQL, server sẽ nhanh chóng sập vì cạn kiệt tài nguyên (lỗi `FATAL: sorry, too many clients already`).
2. **Kiểm tra kết nối ảo tưởng**: Trong Go, hàm `sql.Open()` chỉ xác thực chuỗi kết nối (DSN) chứ **chưa hề bắt tay mạng (TCP Handshake)** với PostgreSQL! Nếu database chết, `sql.Open()` vẫn trả về thành công mà không có lỗi.
3. **Mất đồng bộ Schema (Schema Drift)**: Nếu mỗi lập trình viên tự tạo bảng bằng tay trong GUI (DBeaver, pgAdmin), dự án sẽ vỡ vụn khi deploy lên production. Ta cần các file migration có thể tự động chạy tăng tiến (Up) hoặc khôi phục (Down) có kiểm soát phiên bản.

Issue #3 xây dựng lớp giao tiếp cơ sở dữ liệu an toàn, hiệu năng cao và có thể dự đoán được cho FinTrack.

---

## 3. Prerequisite Go Knowledge (Kiến thức Go tiên quyết)

### A. Blank Import (`_ "package/path"`)
- Trong Go, nếu bạn import một package mà không sử dụng bất kỳ hàm hay biến nào của nó trong code, trình biên dịch (Go Compiler) sẽ **báo lỗi ngay lập tức**.
- Tuy nhiên, nhiều thư viện cần đăng ký (register) bản thân vào hệ thống lúc khởi động thông qua hàm `init()`.
- Để cho phép import package chỉ nhằm mục đích kích hoạt hàm `init()`, Go cung cấp cú pháp **Blank Import** bằng dấu gạch dưới `_`.

*Ví dụ nhỏ:*
```go
import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib" // Đăng ký driver "pgx" vào sql package
)
```

### B. Thời gian trong Go (`time.Duration`)
- Go có kiểu định danh thời gian mạnh `time.Duration`.
- Nó là số nguyên đại diện cho số nanosecond (`ns`).
- Bạn có thể viết phép nhân trực quan: `15 * time.Minute`, `5 * time.Second`.

---

## 4. Architecture (Kiến trúc kết nối cơ sở dữ liệu)

```text
HTTP Request
     │
     ▼
database/sql (Pool Manager của Go)
  ├── Kết nối 1 (Đang bận chạy query)
  ├── Kết nối 2 (Đang rảnh - Idle)
  └── Kết nối N (Tối đa MaxOpenConns = 25)
     │
     ▼ (Driver pgx)
Mạng TCP / Socket
     │
     ▼
PostgreSQL Container (Port 5432)
  ├── Table: users
  └── Table: categories
```

- **`sql.DB` KHÔNG PHẢI là 1 kết nối đơn lẻ!** `sql.DB` là một **Connection Pool** (bộ điều phối hồ kết nối). Nó tự động mở kết nối khi có query, cho mượn kết nối, và thu hồi kết nối về trạng thái Idle khi query xong.

---

## 5. File-by-File Analysis (Phân tích chi tiết từng file)

### File 1: `internal/database/database.go`

Xem trực tiếp tại: [internal/database/database.go](file:///d:/Go/pet-projects/fin-track/internal/database/database.go)

```go
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func DefaultConfig(url string) Config {
	return Config{
		URL:             url,
		MaxOpenConns:    25,
		MaxIdleConns:    25,
		ConnMaxLifetime: 15 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

func OpenWithConfig(ctx context.Context, cfg Config) (*sql.DB, error) {
	if cfg.URL == "" {
		return nil, errors.New("database URL cannot be empty")
	}

	db, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
```

#### Trách nhiệm của file
- Mở kết nối tới PostgreSQL sử dụng driver `pgx`.
- Áp dụng 4 thiết lập connection pool mặc định tối ưu.
- Thực hiện `PingContext` để đảm bảo cơ sở dữ liệu thực sự sống trước khi trả về con trỏ `*sql.DB`.

#### 4 Tham số Connection Pool vàng giải thích chi tiết:
1. `MaxOpenConns (25)`: Số lượng kết nối tối đa mở đồng thời tới database. Tránh làm sập PostgreSQL khi lượng request tăng đột biến.
2. `MaxIdleConns (25)`: Số lượng kết nối rảnh rỗi được giữ lại trong pool để tái sử dụng ngay lập tức mà không phải tốn chi phí bắt tay TCP lại.
3. `ConnMaxLifetime (15 phút)`: Tuổi thọ tối đa của 1 kết nối. Sau 15 phút, kết nối sẽ được đóng và thay mới để tránh các lỗi mạng âm ỉ hoặc rò rỉ bộ nhớ ở server database.
4. `ConnMaxIdleTime (5 phút)`: Nếu một kết nối rảnh rỗi quá 5 phút mà không ai dùng, nó sẽ được giải phóng để trả tài nguyên cho PostgreSQL.

#### Đánh giá phỏng vấn
- 🔥 **MUST KNOW**: Tại sao sau khi gọi `sql.Open()` bắt buộc phải gọi `db.PingContext(ctx)`?
- ⭐ **SHOULD KNOW**: `_ "github.com/jackc/pgx/v5/stdlib"` hoạt động như thế nào?

---

### File 2: `migrations/000001_init_schema.up.sql`

Xem trực tiếp tại: [migrations/000001_init_schema.up.sql](file:///d:/Go/pet-projects/fin-track/migrations/000001_init_schema.up.sql)

```sql
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(50) NOT NULL,
    type VARCHAR(10) NOT NULL CHECK (type IN ('INCOME', 'EXPENSE')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_categories_user_type_name ON categories (user_id, type, LOWER(name));
CREATE INDEX IF NOT EXISTS idx_categories_user_id ON categories (user_id);
```

#### Giải thích thiết kế Schema:
- **`UUID` làm Primary Key**: `gen_random_uuid()` tạo mã định danh ngẫu nhiên phiên bản v4, bảo mật hơn số nguyên tự tăng (`SERIAL` dễ bị đoán số lượng bản ghi), hỗ trợ phân tán dữ liệu tốt hơn.
- **`ON DELETE CASCADE`**: Nếu một người dùng bị xóa (`users`), toàn bộ danh mục của họ (`categories`) cũng sẽ tự động bị xóa theo.
- **Ràng buộc kiểm tra `CHECK (type IN ('INCOME', 'EXPENSE'))`**: Khóa cứng dữ liệu ở tầng cơ sở dữ liệu, ngăn chặn lỗi logic ở backend lưu sai loại danh mục.
- **Unique Index không phân biệt hoa thường**:
  `uq_categories_user_type_name ON categories (user_id, type, LOWER(name))`
  Đảm bảo trong cùng 1 tài khoản và cùng 1 loại (chi tiêu), người dùng không thể tạo 2 danh mục trùng tên như `"Food"` và `"food"`.
- **Index `idx_categories_user_id`**: Tăng tốc tối đa câu lệnh tìm kiếm danh mục theo người dùng (`WHERE user_id = $1`).

---

## 6. Concrete Runtime Flow (Khởi động kết nối DB trong `main.go`)

```mermaid
sequenceDiagram
    autonumber
    participant Main as cmd/server/main.go
    participant DB as internal/database
    participant SQL as database/sql Pool
    participant PG as PostgreSQL Engine

    Main->>DB: Open(ctx, cfg.DatabaseURL)
    DB->>SQL: sql.Open("pgx", url)
    Note over SQL: Chỉ kiểm tra cú pháp URL, CHƯA mở mạng!
    DB->>SQL: SetMaxOpenConns, SetMaxIdleConns, etc.
    DB->>SQL: PingContext(ctx)
    SQL->>PG: Mở kết nối TCP & Handshake
    PG-->>SQL: Sẵn sàng (Ready for query)
    SQL-->>DB: nil (Ping thành công)
    DB-->>Main: *sql.DB
    Main->>Main: opts = append(opts, server.WithDB(db))
```

---

## 7. Important Go Concepts (Khái niệm Go cốt lõi)

### Error Wrapping với `%w`
Trong `database.go`:
```go
return nil, fmt.Errorf("failed to ping database: %w", err)
```
- Toán tử `%w` (Wrap) gói lỗi gốc `err` bên trong thông điệp mới.
- Người gọi ở tầng trên có thể dùng `errors.Is` hoặc `errors.As` để "mở gói" và kiểm tra mã lỗi gốc từ PostgreSQL mà không bị mất dấu vết vết ngăn xếp (Stack trace).

---

## 8. Database / SQL Concepts

### Connection Pool Lifecycle
- Khi một hàm gọi `db.QueryContext(...)`: Pool kiểm tra xem có kết nối nào đang rảnh (Idle) không.
  - Nếu có: Lấy ra dùng ngay.
  - Nếu không và số kết nối chưa chạm trần `MaxOpenConns`: Mở kết nối TCP mới.
  - Nếu đã chạm trần `MaxOpenConns`: Câu lệnh bị chặn (Block) và chờ một kết nối khác được trả về.
- Khi hàm duyệt xong và gọi `rows.Close()`: Kết nối không bị ngắt, mà được trả về Pool làm kết nối Idle cho request tiếp theo.

---

## 9. Error Handling (Xử lý lỗi)

- Nếu `DATABASE_URL` rỗng: Trả về lỗi kiểm tra tính hợp lệ `errors.New("database URL cannot be empty")`.
- Nếu `PingContext` thất bại (sai mật khẩu, database chưa bật): Luôn gọi `_ = db.Close()` để dọn dẹp các socket rác trước khi trả về lỗi cho `main.go`.

---

## 10. Testing (Kiểm thử)

Xem file [internal/database/database_test.go](file:///d:/Go/pet-projects/fin-track/internal/database/database_test.go):
- **`TestOpen_EmptyURL`**: Kiểm tra việc truyền URL rỗng lập tức trả về lỗi.
- **`TestOpen_UnreachableHost`**: Truyền URL tới một địa chỉ ma `postgres://user:pass@127.0.0.1:9999/fintrack` kèm timeout ngắn 200ms để xác minh rằng `PingContext` thực sự phát hiện được mạng chết và không bị treo vĩnh viễn.

---

## 11. Beginner Mistakes (Sai lầm phổ biến)

1. **Tin tưởng vào `sql.Open` mà không gọi `PingContext`**: Server khởi động thành công giả tạo, đến khi người dùng gửi request đầu tiên mới phát hiện database bị chết.
2. **Đặt `MaxIdleConns > MaxOpenConns`**: Trong Go, nếu bạn đặt `MaxIdleConns` lớn hơn `MaxOpenConns`, Go sẽ tự động ép `MaxIdleConns` giảm xuống bằng `MaxOpenConns`.
3. **Đặt `MaxIdleConns = 0`**: Mọi request đến đều phải mở kết nối TCP mới, làm giảm hiệu năng server gấp 10 lần.
4. **Không đặt `ConnMaxLifetime`**: Các kết nối TCP để quá lâu có thể bị Firewall hoặc Cloud NAT âm thầm ngắt kết nối (Stale Connection), gây lỗi `connection reset by peer`.
5. **Quên Blank Import driver**: Nếu không có dòng `_ "github.com/jackc/pgx/v5/stdlib"`, khi gọi `sql.Open("pgx", ...)` chương trình sẽ panic với lỗi: `sql: unknown driver "pgx" (forgotten import?)`.

---

## 12. Code Reading Exercises (Bài tập đọc code)

### Bài tập 1: Dự đoán hành vi
```go
db, err := sql.Open("pgx", "postgres://wrong_user:wrong_pass@localhost:5432/db")
if err != nil {
    log.Fatal("Lỗi rồi!")
}
fmt.Println("Thành công!")
```
*Câu hỏi*: Khi chạy đoạn code trên với thông tin đăng nhập sai, dòng chữ nào sẽ được in ra? Tại sao?

---

## 13. Active Recall (Câu hỏi tự kiểm tra)

1. Tại sao ta phải dùng `_ "github.com/jackc/pgx/v5/stdlib"` thay vì import thông thường?
2. `sql.DB` trong Go là một kết nối hay một hồ kết nối (Pool)?
3. Điều gì xảy ra nếu tất cả 25 kết nối trong pool đều đang bận và có một query thứ 26 ập đến?
4. Tại sao bảng `categories` lại cần cả hai loại index: Unique Index trên `(user_id, type, LOWER(name))` và Normal Index trên `(user_id)`?
5. Ràng buộc `ON DELETE CASCADE` ở bảng `categories` có tác dụng gì?

---

### Model Answers (Đáp án mẫu)

1. **English**: Blank import triggers the package's `init()` function, registering the "pgx" driver into Go's standard `database/sql` driver registry without importing any symbols directly.
   **Tiếng Việt**: Blank import kích hoạt hàm `init()` của package đó, giúp đăng ký driver "pgx" vào bộ quản lý driver của `database/sql` mà không cần gọi trực tiếp hàm nào.
2. **English**: `sql.DB` is a thread-safe connection pool, not a single physical connection.
   **Tiếng Việt**: `sql.DB` là một pool quản lý nhiều kết nối an toàn đa luồng, không phải một kết nối đơn lẻ.
3. **English**: The 26th query blocks and waits until one of the active connections finishes its work and is returned to the pool, or until the context expires.
   **Tiếng Việt**: Query thứ 26 sẽ bị tạm dừng và chờ cho đến khi một trong 25 kết nối kia chạy xong và trả về pool, hoặc đến khi context bị timeout.
4. **English**: The unique index enforces business uniqueness (no duplicate names per user and type). The normal index on `user_id` accelerates queries that filter only by user without type or name.
   **Tiếng Việt**: Unique index bắt buộc tính duy nhất (không trùng tên trong 1 loại). Index trên `user_id` tăng tốc cho các câu truy vấn chỉ lọc theo `user_id` mà không cần quan tâm đến tên hay loại.
5. **English**: When a user row is deleted from the `users` table, all associated categories belonging to that user are automatically deleted by the database engine.
   **Tiếng Việt**: Khi một người dùng bị xóa khỏi bảng `users`, toàn bộ danh mục thuộc về người dùng đó sẽ tự động bị xóa sạch bởi cơ sở dữ liệu.

---

## 14. Fresher Go Backend Interview Questions (Bộ câu hỏi phỏng vấn)

### Câu 1
- **English**: Why does `sql.Open` not return an error when the database server is completely offline?
- **Vietnamese**: Tại sao `sql.Open` không báo lỗi khi database server bị sập hoàn toàn?
- **Model Answer (EN)**: `sql.Open` only validates the driver name and the format of the connection string. It does not establish a network connection. A connection is only established lazily when needed, or immediately by calling `db.PingContext()`.
- **Model Answer (VI)**: `sql.Open` chỉ kiểm tra tính hợp lệ của tên driver và định dạng của chuỗi kết nối. Nó chưa hề tạo kết nối mạng. Kết nối chỉ thực sự được mở khi có nhu cầu (lazy connection), hoặc được kích hoạt ngay bằng lệnh `db.PingContext()`.

### Câu 2
- **English**: How should you size `SetMaxOpenConns` and `SetMaxIdleConns` in a Go backend?
- **Vietnamese**: Bạn nên thiết lập `SetMaxOpenConns` và `SetMaxIdleConns` như thế nào trong Go backend?
- **Model Answer (EN)**: `MaxOpenConns` should be tuned based on PostgreSQL's `max_connections` limit and the number of app replicas. `MaxIdleConns` should ideally equal `MaxOpenConns` to prevent constantly opening and closing connections under sustained traffic.
- **Model Answer (VI)**: `MaxOpenConns` cần căn cứ vào giới hạn `max_connections` của PostgreSQL và số lượng server chạy song song. `MaxIdleConns` lý tưởng nhất nên đặt bằng `MaxOpenConns` để tránh việc liên tục mở và đóng kết nối TCP dưới tải cao.

---

## 15. Knowledge Connection (Kết nối kiến thức)

- **Issue #3** hoàn thiện việc kết nối và tạo bảng cơ sở dữ liệu (`users` và `categories`).
- Trong **Issue #4**, chúng ta sẽ xây dựng tính năng đăng ký tài khoản (`User Registration`), sử dụng struct `User`, gọi thư viện `bcrypt` để băm mật khẩu và dùng `BeginTx` để mở transaction chèn danh mục mặc định.

---

## 16. Learning Checkpoint (Trạm kiểm soát)

### I Must Understand (Bắt buộc phải hiểu)
- [x] Bản chất của `sql.DB` là Connection Pool.
- [x] Tác dụng của `_` (Blank Import) đối với database driver.
- [x] Tại sao phải luôn gọi `PingContext` sau `sql.Open`.
- [x] Ý nghĩa của 4 tham số cấu hình pool.
- [x] Cơ chế Unique Index và Foreign Key `ON DELETE CASCADE`.
