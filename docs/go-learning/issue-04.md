# Issue #4 — User Registration & Atomic Default Category Seeding

## 1. Learning Objectives
Sau khi hoàn thành bài học này, bạn sẽ nắm vững:
- Cách xây dựng tính năng đăng ký tài khoản (User Registration) chuẩn kiến trúc phân tầng trong Go.
- Cơ chế băm mật khẩu an toàn với thư viện `golang.org/x/crypto/bcrypt` và giới hạn độ dài 72 bytes.
- Kỹ thuật điều phối Database Transaction (`tx.BeginTx`, `tx.Commit`, `tx.Rollback`) bằng Go standard library `database/sql`.
- Quản lý transaction an toàn bằng `defer func() { _ = tx.Rollback() }()`.
- Cách ánh xạ lỗi Database (PostgreSQL Error Code `23505` Unique Violation) sang Domain Error `domain.ErrConflict` bằng `errors.As`.
- Cách định nghĩa Custom Error Type `domain.ValidationError` và kiểm tra lỗi bằng `errors.As`.
- Viết Unit Test cho Service tầng nghiệp vụ sử dụng Fake Repository (Mocking in-memory).

---

## 2. What Problem Does This Issue Solve?
Trong một ứng dụng quản lý tài chính cá nhân như FinTrack:
1. **Bảo mật danh tính**: Người dùng cần tài khoản với email duy nhất và mật khẩu được băm bảo mật, không bao giờ lưu plain text.
2. **Khởi tạo dữ liệu người dùng (Onboarding Experience)**: Khi vừa đăng ký, người dùng không thể ghi chép chi tiêu nếu chưa có danh mục (categories). Nếu bắt họ tự gõ từng danh mục ("Food & Dining", "Transportation", "Salary"...), trải nghiệm ban đầu sẽ rất tệ.
3. **Đảm bảo tính toàn vẹn (Data Integrity - Atomicity)**: Việc tạo `User` và tạo 6 `DefaultCategories` phải là một khối thống nhất (atomic). Nếu insert user thành công nhưng insert categories bị crash/lỗi mạng, hệ thống không được phép để lại một user "què cụt" không có danh mục. Toàn bộ thao tác phải được gói trong **Database Transaction**: Tất cả thành công, hoặc không gì cả (All-or-Nothing).

---

## 3. Prerequisite Go Knowledge

### 3.1. Struct và Struct Tags (`json:"..."`)
- **Khái niệm**: Trong Go không có class, `struct` là kiểu dữ liệu tổng hợp (composite type) gom nhiều trường (fields) lại với nhau. Struct tag là metadata nằm sau kiểu dữ liệu trong dấu backtick `` ` ``, chỉ định cách thư viện ngoài (như `encoding/json`) serialize/deserialize.
- **Ví dụ cơ bản**:
```go
type User struct {
    Name     string `json:"name"`
    Password string `json:"-"` // Dấu "-" nghĩa là không bao giờ xuất trường này ra JSON
}
```
- **Kết nối FinTrack**: Trong `internal/domain/user.go`, `PasswordHash` có tag `json:"-"` để đảm bảo dù có vô tình return struct `User` qua HTTP, mã hash cũng không bao giờ lộ ra ngoài API.

### 3.2. Pointers (`*`) và Nullable / Optional Values
- **Khái niệm**: Go biến mặc định truyền theo dạng copy giá trị (pass-by-value). Con trỏ `*T` lưu địa chỉ ô nhớ của kiểu `T`. Nếu một trường có thể nhận giá trị `NULL` trong database hoặc `null`/bỏ qua trong JSON, Go dùng con trỏ `*string` thay vì `string`. Giá trị rỗng của con trỏ là `nil`.
- **Ví dụ cơ bản**:
```go
var name *string = nil // Đại diện cho NULL
val := "Duc Hoang"
name = &val            // name trỏ tới địa chỉ của chuỗi "Duc Hoang"
```
- **Kết nối FinTrack**: `DisplayName *string` trong `domain.User` cho phép người dùng không nhập tên hiển thị (khi đó giá trị là `nil`, database lưu `NULL`).

### 3.3. Slice (`make` và `len`)
- **Khái niệm**: Slice là mảng động (dynamic array). Hàm `make([]T, len, cap)` cấp phát bộ nhớ trước cho slice, giúp tối ưu hiệu năng tránh việc re-allocate bộ nhớ liên tục như khi dùng `append`.
- **Ví dụ cơ bản**:
```go
items := make([]int, 3) // slice gồm 3 phần tử [0, 0, 0]
items[0] = 10
```
- **Kết nối FinTrack**: Trong `internal/service/auth_service.go`, hàm `Register` dùng `categories := make([]domain.Category, len(domain.DefaultCategories))` để cấp phát đúng 6 ô nhớ cho 6 danh mục mặc định.

### 3.4. Defer Statement
- **Khái niệm**: `defer` trì hoãn việc thực thi một hàm cho đến khi hàm bao ngoài (enclosing function) trả về (return). Các lệnh `defer` chạy theo thứ tự LIFO (Last In First Out).
- **Ví dụ cơ bản**:
```go
func doSomething() {
    println("1")
    defer println("3")
    println("2")
} // Output: 1 -> 2 -> 3
```
- **Kết nối FinTrack**: Quản lý `tx.Rollback()` và `stmt.Close()` ngay sau khi mở kết nối / khởi tạo statement.

---

## 4. Architecture

Quá trình đăng ký người dùng đi qua 5 tầng nghiêm ngặt:

```text
HTTP Client (POST /api/v1/auth/register)
       │
       ▼
[Handler: AuthHandler.Register]
  • Bind JSON sang RegisterRequest
  • Validation cơ bản (Format HTTP)
       │
       ▼
[Service: AuthService.Register]
  • Chuẩn hóa email (lowercase, trim)
  • Validate email cú pháp & password độ dài (8-72 bytes)
  • Kiểm tra email trùng lặp (Application-level check)
  • Băm mật khẩu (bcrypt)
  • Chuẩn bị danh sách default categories
       │
       ▼
[Repository: PostgresUserRepository.CreateWithCategories]
  • Mở Database Transaction (tx.BeginTx)
  • INSERT INTO users ... RETURNING id, created_at
  • Chuẩn bị Prepared Statement: INSERT INTO categories ...
  • Lặp qua categories và ExecContext
  • Commit Transaction (tx.Commit)
       │
       ▼
[PostgreSQL Database]
  • Lưu bản ghi users và 6 bản ghi categories đồng thời.
```

---

## 5. File-by-File Analysis

### 5.1. `internal/domain/user.go`
- **Responsibility**: Khai báo struct cốt lõi đại diện cho thực thể `User`.
- **Important Syntax**:
  - `ID uuid.UUID`: Sử dụng kiểu UUID v4 chuẩn quốc tế.
  - `PasswordHash string ` `json:"-"`: Tuyệt đối ẩn hash khi parse JSON.
  - `DisplayName *string ` `json:"display_name,omitempty"`: Cho phép null, nếu rỗng thì bỏ qua field khi encode JSON.
- **Interview Importance**: 🔥 MUST KNOW. Hỏi về cách xử lý optional fields và bảo mật dữ liệu nhạy cảm.

### 5.2. `internal/domain/category.go`
- **Responsibility**: Định nghĩa thực thể danh mục giao dịch và danh sách 6 danh mục mẫu `DefaultCategories`.
- **Important Syntax**:
  - `type CategoryType string`: Định nghĩa type riêng thay vì dùng string nguyên thủy.
  - `const ( CategoryTypeIncome CategoryType = "INCOME" ... )`: Giả lập enum trong Go.
  - `var DefaultCategories = []struct{ Name string; Type CategoryType }{ ... }`: Anonymous struct slice chứa dữ liệu hạt giống (seed).
- **Interview Importance**: ⭐ SHOULD KNOW. Cách định nghĩa Type-safe Enum trong Go.

### 5.3. `internal/domain/errors.go`
- **Responsibility**: Centralized sentinel errors và custom validation error type.
- **Important Syntax**:
```go
var (
    ErrNotFound   = errors.New("not found")
    ErrConflict   = errors.New("conflict")
    ErrBadRequest = errors.New("bad request")
)

type ValidationError struct {
    Field   string
    Message string
}
func (e ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
```
- **Why structured this way?**: Bất kỳ struct nào cài đặt method `Error() string` đều thỏa mãn interface `error` của Go. Tầng Service có thể trả về lỗi chi tiết theo từng field, tầng Handler dùng `errors.As` để bóc tách và trả về JSON có cấu trúc `fields: { email: "..." }`.
- **Interview Importance**: 🔥 MUST KNOW. Cơ chế error interface trong Go.

### 5.4. `internal/repository/user_repository.go`
- **Responsibility**: Interface định nghĩa hợp đồng lưu trữ User. Tầng Service chỉ phụ thuộc vào Interface này, hoàn toàn không biết gì về PostgreSQL.
- **Methods**:
  - `CreateWithCategories(ctx context.Context, user *domain.User, categories []domain.Category) error`
  - `FindByEmail(ctx context.Context, email string) (*domain.User, error)`
  - `FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)`
- **Interview Importance**: 🔥 MUST KNOW. Dependency Inversion Principle (DIP).

### 5.5. `internal/repository/postgres_user_repository.go`
- **Responsibility**: Cài đặt interface `UserRepository` tương tác trực tiếp với PostgreSQL qua `*sql.DB`.
- **Important Syntax & Logic**:
```go
func (r *PostgresUserRepository) CreateWithCategories(ctx context.Context, user *domain.User, categories []domain.Category) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer func() {
        _ = tx.Rollback()
    }()
    // ... insert user và categories ...
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    return nil
}
```
- **Postgres Error Mapping**:
```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23505" {
    return fmt.Errorf("%w: email %q already exists", domain.ErrConflict, user.Email)
}
```
- **Prepared Statement**: `tx.PrepareContext(ctx, queryCategory)` biên dịch câu SQL 1 lần và tái sử dụng cho 6 danh mục, giúp tối ưu hiệu năng mạng và DB server.
- **Interview Importance**: 🔥 MUST KNOW. Quản lý transaction, `defer tx.Rollback()`, và Postgres Error Code `23505`.

### 5.6. `internal/service/auth_service.go`
- **Responsibility**: Chứa toàn bộ nghiệp vụ đăng ký và đăng nhập.
- **Important Logic**:
  - Email normalization: `strings.ToLower(strings.TrimSpace(input.Email))`.
  - Email regex/parser: Sử dụng `net/mail.ParseAddress` kết hợp kiểm tra ký tự `@` và domain `.`.
  - Password rules: Độ dài từ 8 đến 72 bytes.
  - Bcrypt hashing: `bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)`.
- **Interview Importance**: 🔥 MUST KNOW. Tại sao bcrypt giới hạn 72 bytes? (Bcrypt thuật toán nguyên bản bỏ qua toàn bộ byte từ byte 73 trở đi).

### 5.7. `internal/handler/auth_handler.go`
- **Responsibility**: Tiếp nhận HTTP request, parse JSON, gọi Service, mapping Domain Error thành HTTP Status Code.
- **Mapping Logic**:
  - `domain.ValidationError` -> `400 Bad Request` với object chi tiết lỗi.
  - `domain.ErrConflict` -> `409 Conflict`.
  - Other errors -> `500 Internal Server Error` (không để lộ chi tiết database cho client).
- **Response Formatting**: Chuyển thời gian thành chuỗi ISO 8601 qua `CreatedAt.Format(time.RFC3339)`.
- **Interview Importance**: ⭐ SHOULD KNOW. Error translation tại handler boundary.

---

## 6. Concrete Runtime Flow

Chi tiết luồng chạy của một request `POST /api/v1/auth/register`:

```text
1. Client gửi JSON: {"email": "User@FinTrack.io ", "password": "securePassword123", "display_name": "Duc Hoang"}
2. AuthHandler.Register:
   a. c.ShouldBindJSON(&req) giải mã JSON thành RegisterRequest.
   b. Gọi h.authService.Register(ctx, input).
3. AuthService.Register:
   a. Chuẩn hóa email -> "user@fintrack.io".
   b. Kiểm tra cú pháp email -> Hợp lệ.
   c. Kiểm tra độ dài mật khẩu: 17 bytes (nằm trong [8, 72]) -> Hợp lệ.
   d. s.userRepo.FindByEmail -> Trả về domain.ErrNotFound (nghĩa là email chưa ai đăng ký).
   e. bcrypt.GenerateFromPassword -> Sinh chuỗi hash $2a$10$...
   f. Khởi tạo 6 categories từ domain.DefaultCategories.
   g. Gọi s.userRepo.CreateWithCategories(ctx, user, categories).
4. PostgresUserRepository.CreateWithCategories:
   a. r.db.BeginTx(ctx, nil) -> Lấy 1 connection từ pool, gửi lệnh BEGIN.
   b. tx.QueryRowContext thực thi INSERT INTO users ... RETURNING id, created_at, updated_at.
   c. Scan các giá trị trả về vào con trỏ user.
   d. tx.PrepareContext chuẩn bị câu lệnh INSERT INTO categories.
   e. Lặp 6 lần: ExecContext với catID mới và user.ID vừa được gán.
   f. tx.Commit() gửi lệnh COMMIT đến PostgreSQL.
   g. defer tx.Rollback() được gọi: Do transaction đã COMMIT thành công, Rollback() trả về sql.ErrTxDone và bị bỏ qua an toàn.
5. AuthHandler.Register:
   a. Nhận đối tượng user đã có đầy đủ ID và Timestamps.
   b. Trả về HTTP 201 Created cùng UserResponse.
```

---

## 7. Important Go Concepts

### 7.1. Con trỏ Receiver `(r *PostgresUserRepository)` vs Value Receiver
- Trong Go, methods có thể gắn với con trỏ receiver `(r *Type)` hoặc value receiver `(r Type)`.
- Với Repository hoặc Service (chứa `db *sql.DB`, config, interface), luôn sử dụng **Pointer Receiver** để:
  1. Tránh copy toàn bộ struct mỗi lần gọi method.
  2. Đảm bảo struct chia sẻ chung một trạng thái kết nối.

### 7.2. Error Wrapping `%w` và `errors.Is` / `errors.As`
- Trước Go 1.13, việc kiểm tra lỗi gốc rất phức tạp vì bị bọc chuỗi string.
- Từ Go 1.13:
  - Dùng `fmt.Errorf("...: %w", err)` với động từ `%w` để bọc lỗi gốc (wrap error).
  - `errors.Is(err, target)`: Kiểm tra xem cây lỗi (error chain) có chứa sentinel error cụ thể không (như `domain.ErrConflict`).
  - `errors.As(err, &target)`: Tìm trong error chain xem có lỗi nào mang kiểu cụ thể không (như `*pgconn.PgError` hoặc `domain.ValidationError`) và gán giá trị sang biến đích.

### 7.3. Slice Truncation / Allocation với `make`
- Khác với JavaScript `array.push()`, trong Go khi biết trước kích thước mảng (ở đây là 6 categories), khởi tạo bằng `make([]T, len)` có độ phức tạp $O(1)$ về cấp phát bộ nhớ.

---

## 8. Database / SQL Concepts

### 8.1. Transaction Lifecycle trong `database/sql`
```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer func() {
    _ = tx.Rollback() // Cực kỳ quan trọng: Nếu panic hoặc return sớm do lỗi, DB connection sẽ được rollback và giải phóng về pool!
}()

// Thao tác DB qua tx (tx.QueryRowContext, tx.ExecContext...)

if err := tx.Commit(); err != nil {
    return err // Nếu commit lỗi, defer rollback vẫn dọn dẹp
}
```

### 8.2. Tại sao `defer tx.Rollback()` không phá hỏng `tx.Commit()`?
- Trong chuẩn `database/sql`, nếu `tx.Commit()` đã thực thi thành công, trạng thái transaction chuyển thành "done".
- Lệnh gọi `tx.Rollback()` phía sau trong hàm `defer` sẽ trả về lỗi `sql.ErrTxDone` và không gây bất kỳ tác động tiêu cực nào tới database.
- Đây là idiom chuẩn mực số 1 của Go khi xử lý transaction.

### 8.3. Mã lỗi PostgreSQL `23505` (unique_violation)
- Khi hai request cùng đăng ký một email cùng lúc, cả hai đều có thể vượt qua bước kiểm tra ở tầng ứng dụng (`FindByEmail`).
- Database constraint `UNIQUE (email)` là lá chắn cuối cùng. Khi bị vi phạm, PostgreSQL trả về mã chuẩn SQLSTATE `23505`. Driver `pgx` ánh xạ thành struct `*pgconn.PgError`.

---

## 9. Error Handling

### Quy tắc phân tầng lỗi trong FinTrack:
1. **Tầng Database (Repository)**: Bắt lỗi Postgres thô (`*pgconn.PgError` code `23505`), bọc lại bằng Domain Error: `fmt.Errorf("%w: email %q already exists", domain.ErrConflict, user.Email)`.
2. **Tầng Nghiệp vụ (Service)**: Kiểm tra các quy tắc nghiệp vụ, trả về `domain.ValidationError` hoặc các sentinel errors (`domain.ErrConflict`).
3. **Tầng Vận chuyển (Handler)**: Bóc tách lỗi bằng `errors.As` / `errors.Is` để quyết định HTTP Status Code (400, 409, 500). Tuyệt đối không để lộ chi tiết SQL ra ngoài response của client.

---

## 10. Testing

### 10.1. Fake Repository Pattern
Trong `internal/service/auth_service_test.go`, chúng ta không kết nối PostgreSQL thật mà tạo `FakeUserRepository`:
```go
type fakeUserRepo struct {
    users map[string]*domain.User
}

func (f *fakeUserRepo) CreateWithCategories(ctx context.Context, u *domain.User, cats []domain.Category) error {
    if _, exists := f.users[u.Email]; exists {
        return domain.ErrConflict
    }
    f.users[u.Email] = u
    return nil
}
```
- **Tại sao dùng Fake Repository?**:
  - Test chạy siêu nhanh (dưới 10ms), không cần cài đặt Docker/Postgres trong môi trường CI của Unit Test.
  - Dễ dàng giả lập các trường hợp lỗi biên (database connection drop, email duplicate...).

### 10.2. Lệnh chạy test
```powershell
go test -v -race ./internal/service/...
```
- Cờ `-race`: Kích hoạt Go Data Race Detector, đảm bảo mã nguồn an toàn khi chạy đa luồng.

---

## 11. Beginner Mistakes

### 5 Sai lầm phổ biến của người mới:
1. **Quên `defer tx.Rollback()`**: Nếu câu lệnh insert thứ 2 bị lỗi, hàm return sớm mà không rollback, connection đó bị treo transaction và không bao giờ được trả về pool, gây rò rỉ kết nối (connection leak).
2. **Dùng `len(password)` thay vì kiểm tra byte slice với mật khẩu Unicode**: Bcrypt chỉ nhận tối đa 72 bytes. Một ký tự tiếng Việt hoặc Emoji có thể tốn 3-4 bytes UTF-8. Nếu chuỗi có 25 ký tự nhưng dài 75 bytes, bcrypt sẽ âm thầm cắt ngắn mật khẩu nếu không chủ động giới hạn.
3. **So sánh lỗi bằng `err == domain.ErrConflict` thay vì `errors.Is`**: Nếu lỗi đã được wrap qua `fmt.Errorf("%w: ...", err)`, phép so sánh trực tiếp `==` sẽ trả về `false`.
4. **Không chuẩn hóa Email (`strings.ToLower`)**: Người dùng đăng ký `John@example.com` nhưng đăng nhập bằng `john@example.com` sẽ bị báo lỗi không tìm thấy tài khoản.
5. **Thực thi SQL vòng lặp mà không dùng `PrepareContext`**: Chạy 6 câu lệnh `tx.ExecContext` riêng lẻ khiến DB phải parse và lập execution plan 6 lần thay vì tái sử dụng 1 prepared statement.

### 3 Nguy cơ sinh bug thực tế:
1. **Race condition khi kiểm tra Email**: Chỉ kiểm tra bằng `FindByEmail` ở tầng Service mà không có constraint `UNIQUE` dưới DB sẽ dẫn đến lỗi trùng lặp khi 2 request đến cùng mili-giây.
2. **Quên gọi `RETURNING id`**: Sau khi insert, struct `User` trong bộ nhớ vẫn có `ID` rỗng nếu không đọc giá trị tự sinh từ database.
3. **Lưu plain text password khi gặp lỗi hash**: Nếu không kiểm tra `if err != nil` ngay sau `bcrypt.GenerateFromPassword`, hệ thống có thể lưu hash rỗng hoặc lỗi.

### 2 Cái bẫy phỏng vấn (Interview Traps):
1. *"Tại sao em không hash password bằng SHA-256 hoặc MD5 cho nhanh?"* -> **Bẫy**: SHA-256 quá nhanh, hacker có thể tính hàng tỷ hash mỗi giây bằng GPU để brute-force. Phải dùng thuật toán adaptive slow hashing như Bcrypt hoặc Argon2.
2. *"Hàm `defer tx.Rollback()` luôn chạy kể cả khi `tx.Commit()` thành công, vậy database có bị rollback nhầm không?"* -> **Bẫy**: Không, vì `database/sql` kiểm tra nếu tx đã commit thì Rollback() sẽ không làm gì cả và trả về `sql.ErrTxDone`.

---

## 12. Code Reading Exercises

### Bài tập 1: Dự đoán kết quả
Đoạn code sau có vấn đề gì khi thực thi?
```go
func RegisterUser(db *sql.DB, email, pass string) error {
    tx, _ := db.Begin()
    _, err := tx.Exec("INSERT INTO users(email) VALUES($1)", email)
    if err != nil {
        return err
    }
    tx.Commit()
    return nil
}
```
*Gợi ý suy nghĩ*: Nếu câu lệnh `tx.Exec` thất bại, trạng thái của `tx` trong connection pool sẽ ra sao?

### Bài tập 2: Tìm bug tiềm ẩn
Đoạn code validation sau có bắt được mật khẩu dài quá giới hạn của Bcrypt không?
```go
func ValidatePassword(p string) bool {
    return len(p) >= 8 && len(p) <= 72
}
```
*Gợi ý suy nghĩ*: `len(p)` trong Go đếm số bytes hay số ký tự rune? Điều gì xảy ra nếu người dùng nhập emoji?

---

## 13. Active Recall

### Câu hỏi ôn tập
1. Tại sao thao tác tạo User và tạo Default Categories bắt buộc phải nằm trong cùng một Database Transaction?
2. Trong Go, tại sao cần bọc `tx.Rollback()` trong một lệnh `defer` ngay sau `db.BeginTx`?
3. Khi nào nên dùng `errors.Is` và khi nào nên dùng `errors.As`?
4. Tag `json:"-"` trên field `PasswordHash` có ý nghĩa gì?
5. Tại sao không nên tin tưởng 100% vào bước kiểm tra `FindByEmail` ở tầng Service để chống trùng lặp email?

---

### Model Answers

#### 1. Tại sao tạo User và Default Categories phải cùng Transaction?
- **English**: Creating a user and their default categories must be atomic to ensure data integrity. If creating categories fails halfway due to a network glitch or crash, we must not leave an incomplete user account with missing categories in the database.
- **Vietnamese**: Đảm bảo tính toàn vẹn dữ liệu (Atomicity). Nếu tạo categories thất bại giữa chừng do lỗi mạng hoặc sập nguồn, hệ thống không được để lại một user dở dang không có danh mục trong database.

#### 2. Tại sao cần `defer tx.Rollback()`?
- **English**: Placing `tx.Rollback()` in a defer statement guarantees that if the function returns early due to an error or panic before `tx.Commit()` is called, the transaction is cleanly aborted and the connection is returned to the pool, preventing connection leaks.
- **Vietnamese**: Đảm bảo rằng nếu hàm kết thúc sớm vì gặp lỗi hoặc panic trước khi đến lệnh `tx.Commit()`, transaction sẽ tự động được hủy bỏ và giải phóng kết nối về pool, ngăn chặn rò rỉ kết nối.

#### 3. Phân biệt `errors.Is` và `errors.As`
- **English**: `errors.Is` checks if any error in the error tree matches a specific sentinel value (e.g. `domain.ErrConflict`). `errors.As` checks if any error matches a specific type and extracts its contents into a pointer target (e.g. extracting `*pgconn.PgError`).
- **Vietnamese**: `errors.Is` kiểm tra xem cây lỗi có chứa một sentinel error cụ thể nào đó không. `errors.As` kiểm tra xem lỗi có thuộc một kiểu struct cụ thể nào không và trích xuất dữ liệu của struct đó sang biến con trỏ.

#### 4. Ý nghĩa tag `json:"-"`
- **English**: It instructs the `encoding/json` encoder to completely omit this field during serialization, ensuring hashed passwords are never leaked in HTTP API responses.
- **Vietnamese**: Hướng dẫn bộ serialize JSON bỏ qua hoàn toàn trường này, đảm bảo mã hash mật khẩu không bao giờ bị lộ ra ngoài qua HTTP API.

#### 5. Tại sao không chỉ dựa vào `FindByEmail`?
- **English**: Because of concurrency and race conditions. Two simultaneous requests with the same email can both execute `FindByEmail` at the exact same millisecond and see that the email doesn't exist yet. The unique constraint at the database level is the only true source of truth.
- **Vietnamese**: Do hiện tượng tranh chấp đồng thời (race condition). Hai request gửi cùng email trong cùng một mili-giây đều có thể vượt qua bước `FindByEmail`. Ràng buộc `UNIQUE` ở tầng database mới là chốt chặn đảm bảo an toàn tuyệt đối.

---

## 14. Fresher Go Backend Interview Questions

### Question 1
- **English**: How does Go's `database/sql` handle database transactions, and how do you prevent transaction leaks?
- **Vietnamese**: Gói `database/sql` của Go xử lý transaction như thế nào và làm sao để chống rò rỉ transaction?
- **Model Answer (EN)**: You initiate a transaction with `db.BeginTx(ctx, opts)`. To prevent leaks, immediately defer `tx.Rollback()`. If `tx.Commit()` succeeds, rollback becomes a safe no-op. If an error occurs midway, defer ensures the transaction rolls back.
- **Model Answer (VI)**: Ta bắt đầu transaction bằng `db.BeginTx(ctx, opts)`. Để chống rò rỉ, ta lập tức gọi `defer tx.Rollback()`. Nếu `tx.Commit()` thành công, lệnh rollback trở thành vô hại. Nếu có lỗi giữa chừng, defer đảm bảo kết nối được rollback và đóng lại.
- **Explanation**: Tránh việc giữ connection trong pool bị treo vô thời hạn.
- **Follow-up**: What does `tx.Rollback()` return if `tx.Commit()` was already called? (Answer: `sql.ErrTxDone`).

### Question 2
- **English**: Why does FinTrack use `bcrypt` instead of standard hashing like SHA-256 for passwords?
- **Vietnamese**: Tại sao FinTrack sử dụng `bcrypt` thay vì các hàm băm tiêu chuẩn như SHA-256 cho mật khẩu?
- **Model Answer (EN)**: SHA-256 is designed to be fast, which allows attackers to compute billions of guesses per second with GPUs. Bcrypt is deliberately slow and includes an adjustable work factor (salt + cost) to resist brute-force and rainbow table attacks.
- **Model Answer (VI)**: SHA-256 được thiết kế để chạy cực nhanh, cho phép hacker thử hàng tỷ mật khẩu mỗi giây bằng GPU. Bcrypt cố tình chạy chậm và có chi phí tính toán điều chỉnh được (cost factor) cùng muối (salt) tự sinh để chống tấn công brute-force.
- **Explanation**: Thuật toán băm mật khẩu phải là slow hash.
- **Follow-up**: What is the maximum password length supported by Bcrypt? (Answer: 72 bytes).

### Question 3
- **English**: How do you detect and handle PostgreSQL duplicate key errors in Go?
- **Vietnamese**: Làm thế nào để phát hiện và xử lý lỗi trùng khóa (duplicate key) của PostgreSQL trong Go?
- **Model Answer (EN)**: We inspect the returned error using `errors.As(err, &pgErr)` where `pgErr` is `*pgconn.PgError`. If `pgErr.Code == "23505"`, it signifies a unique violation, which we map to `domain.ErrConflict`.
- **Model Answer (VI)**: Ta kiểm tra lỗi trả về bằng `errors.As(err, &pgErr)` với `pgErr` là `*pgconn.PgError`. Nếu `pgErr.Code == "23505"`, đó là vi phạm unique constraint, và ta ánh xạ thành `domain.ErrConflict`.
- **Explanation**: Tránh parse chuỗi string không ổn định của thông báo lỗi.
- **Follow-up**: Why shouldn't you check `strings.Contains(err.Error(), "duplicate key")`? (Answer: Error strings can change across database versions or localization settings).

### Question 4
- **English**: What is the difference between `PrepareContext` and executing SQL queries directly?
- **Vietnamese**: Sự khác nhau giữa `PrepareContext` và việc thực thi câu lệnh SQL trực tiếp là gì?
- **Model Answer (EN)**: `PrepareContext` compiles the SQL statement once on the database server. When executing multiple inserts in a loop (like default categories), it saves compilation and execution plan overhead on each iteration.
- **Model Answer (VI)**: `PrepareContext` biên dịch câu lệnh SQL một lần trên database server. Khi thực hiện nhiều câu lệnh lặp lại (như 6 danh mục mặc định), nó giúp tiết kiệm chi phí biên dịch và tạo execution plan ở mỗi lần lặp.
- **Explanation**: Tối ưu hiệu năng khi ghi dữ liệu hàng loạt nhỏ.
- **Follow-up**: Do you need to close a prepared statement created on a transaction? (Answer: Yes, always `defer stmt.Close()`).

### Question 5
- **English**: Why does the `User` struct use a pointer `*string` for `DisplayName`?
- **Vietnamese**: Tại sao struct `User` lại dùng con trỏ `*string` cho trường `DisplayName`?
- **Model Answer (EN)**: A pointer allows distinguishing between an empty string `""` and an absent/null value `nil`. In SQL, this maps cleanly to `NULL` versus an empty text column.
- **Model Answer (VI)**: Con trỏ cho phép phân biệt giữa một chuỗi rỗng `""` và giá trị không tồn tại `nil` (NULL). Trong SQL, điều này ánh xạ chính xác sang `NULL` thay vì lưu chuỗi rỗng.
- **Explanation**: Go không có kiểu `null` cho primitive types, con trỏ mang giá trị `nil` đại diện cho null.
- **Follow-up**: What is the risk of using pointers for struct fields? (Answer: Dereferencing a nil pointer causes a runtime panic).

### Question 6
- **English**: How does Go's `net/mail.ParseAddress` help in email validation?
- **Vietnamese**: Hàm `net/mail.ParseAddress` của Go hỗ trợ xác thực email như thế nào?
- **Model Answer (EN)**: It parses standard RFC 5322 address formats. However, because it also accepts formats like `Name <email@domain.com>`, FinTrack additionally ensures that `addr.Address == normalizedEmail` and verifies the presence of `@` and `.` characters.
- **Model Answer (VI)**: Nó kiểm tra định dạng email theo chuẩn RFC 5322. Tuy nhiên vì nó chấp nhận cả dạng `Tên <email@domain.com>`, FinTrack cần kiểm tra thêm `addr.Address == normalizedEmail` và sự tồn tại của ký tự `@` và `.`.
- **Explanation**: Kết hợp thư viện chuẩn với các điều kiện chặn biên.
- **Follow-up**: Why not use a huge complex regular expression? (Answer: Complex regexes can cause ReDoS vulnerabilities and are notoriously difficult to maintain).

### Question 7
- **English**: What is a Custom Error type in Go and how does FinTrack use it?
- **Vietnamese**: Custom Error type trong Go là gì và FinTrack sử dụng nó như thế nào?
- **Model Answer (EN)**: Any type that implements the `Error() string` method is an error. FinTrack defines `type ValidationError struct { Field string; Message string }` to return structured field-level errors to the client.
- **Model Answer (VI)**: Bất kỳ kiểu dữ liệu nào cài đặt method `Error() string` đều là một error. FinTrack định nghĩa struct `ValidationError` để trả về thông tin lỗi chi tiết theo từng trường cho client.
- **Explanation**: Tận dụng tính đa hình của Go interface.
- **Follow-up**: How does the HTTP handler extract the `Field` property from an interface error? (Answer: Using `errors.As(err, &valErr)`).

### Question 8
- **English**: Why does `AuthService` accept `repository.UserRepository` interface instead of the concrete struct?
- **Vietnamese**: Tại sao `AuthService` lại nhận interface `repository.UserRepository` thay vì struct cụ thể?
- **Model Answer (EN)**: This adheres to the Dependency Inversion Principle. It decouples business logic from the database implementation and allows unit testing with in-memory fake repositories without a real PostgreSQL instance.
- **Model Answer (VI)**: Tuân thủ nguyên lý Dependency Inversion. Nó tách rời logic nghiệp vụ khỏi database cụ thể, cho phép viết unit test với fake repository trong bộ nhớ mà không cần chạy PostgreSQL thật.
- **Explanation**: Nguyên tắc thiết kế Clean Architecture / Hexagonal Architecture.
- **Follow-up**: Where should Go interfaces ideally be defined: consumer or producer? (Answer: Consumer package defines the interface it requires).

### Question 9
- **English**: What does `time.Now().UTC()` achieve and why should timestamps be in UTC?
- **Vietnamese**: `time.Now().UTC()` làm gì và tại sao timestamp nên luôn ở múi giờ UTC?
- **Model Answer (EN)**: It ensures all timestamps recorded by the application are normalized to Coordinated Universal Time, preventing discrepancies caused by different server locales or daylight saving time changes.
- **Model Answer (VI)**: Đảm bảo toàn bộ mốc thời gian lưu trong hệ thống được quy về giờ chuẩn quốc tế UTC, tránh lỗi sai lệch giờ giữa các server ở múi giờ khác nhau hoặc do quy ước giờ mùa hè (DST).
- **Explanation**: Thực hành chuẩn cho mọi hệ thống backend production.
- **Follow-up**: When should UTC timestamps be converted to local time? (Answer: At the presentation/frontend layer based on the user's timezone).

### Question 10
- **English**: How does the HTTP handler prevent exposing internal database errors to end users?
- **Vietnamese**: Tầng HTTP handler làm thế nào để ngăn chặn việc để lộ chi tiết lỗi database cho người dùng cuối?
- **Model Answer (EN)**: It explicitly checks for known domain errors (`ErrNotFound`, `ErrConflict`, `ValidationError`). Any unrecognized error falls back to a generic `500 Internal Server Error` with message `"failed to register user"`, while raw details are logged internally.
- **Model Answer (VI)**: Nó kiểm tra rõ ràng các lỗi domain đã biết. Bất kỳ lỗi lạ nào không xác định sẽ rơi vào nhánh mặc định `500 Internal Server Error` với thông báo chung chung, trong khi chi tiết lỗi thô chỉ được ghi vào log nội bộ.
- **Explanation**: Phòng thủ bảo mật (Information Disclosure Prevention).
- **Follow-up**: What is the danger of returning `err.Error()` directly in a 500 response? (Answer: Attackers can learn database table structures, connection strings, or system paths).

---

## 15. Knowledge Connection
- **Kế thừa từ Issue #2 & #3**: Sử dụng cấu hình `Config`, router Gin, và connection pool `*sql.DB` đã khởi tạo từ hai issue trước.
- **Chuẩn bị cho Issue #5**: Người dùng vừa đăng ký ở Issue #4 sẽ dùng email và mật khẩu này để đăng nhập (Login) nhận JWT token trong Issue #5.
- **Chuẩn bị cho Issue #6 & #7**: Các danh mục được tự động sinh trong transaction này (`DefaultCategories`) sẽ lập tức sẵn sàng để gán vào các giao dịch thu/chi trong Issue #7.

---

## 16. Learning Checkpoint

### I Must Understand
1. Bản chất của Database Transaction và lý do dùng `defer tx.Rollback()`.
2. Cách băm mật khẩu bằng Bcrypt và lý do không dùng thuật toán băm nhanh.
3. Cách phân biệt `errors.Is` (so sánh giá trị) và `errors.As` (ép kiểu struct).
4. Mã lỗi PostgreSQL `23505` biểu thị vi phạm unique constraint.
5. Cách dùng struct tag `json:"-"` để bảo mật dữ liệu nhạy cảm.

### I Should Be Able To Code
1. Viết một hàm thực hiện transaction chèn 2 bảng có `defer tx.Rollback()` và `tx.Commit()`.
2. Định nghĩa một struct Custom Error thỏa mãn interface `error`.
3. Viết unit test sử dụng Fake Repository trong bộ nhớ.

### I Should Be Able To Explain In An Interview
1. Tại sao cần kiểm tra unique cả ở tầng ứng dụng lẫn tầng database?
2. Giới hạn 72 bytes của Bcrypt xuất phát từ đâu và cách phòng tránh lỗi khi người dùng nhập chuỗi dài?
3. Tại sao tầng Service chỉ nên phụ thuộc vào Interface của Repository?

### Advanced Topics I Do Not Need Yet
- Thuật toán Argon2id (thay thế Bcrypt trong các hệ thống đòi hỏi chống tấn công bằng mạch ASIC chuyên dụng).
- Cơ chế Two-Phase Commit (2PC) trong hệ thống phân tán (Distributed Transactions).
- Email verification flow (gửi mail kích hoạt tài khoản qua SMTP/Queue).
