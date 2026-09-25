# Issue #7 — Transaction CRUD, Money Representation (VND int64) & Nullable Pointers

## 1. Learning Objectives
Sau khi hoàn thành bài học này, bạn sẽ nắm vững:
- Cách mô hình hóa tiền tệ chính xác trong hệ thống tài chính: Tại sao lưu VND bằng `int64` / SQL `BIGINT`, và tại sao TUYỆT ĐỐI KHÔNG dùng `float64`.
- Quản lý dữ liệu lịch (Calendar Date) dạng `YYYY-MM-DD` bằng chuỗi thay vì timestamp để chống lỗi lệch múi giờ (Timezone Shift Bug).
- Kỹ thuật sử dụng con trỏ Go (`*uuid.UUID`, `*string`) để xử lý các cột có thể nhận giá trị `NULL` trong PostgreSQL.
- Thiết kế cơ sở dữ liệu quan hệ với chiến lược Foreign Key: Phân biệt `ON DELETE CASCADE` (xóa User thì xóa sạch Transaction) vs `ON DELETE SET NULL` (xóa Category thì Transaction vẫn giữ nguyên, chỉ set category về `NULL`).
- Quy tắc xác thực tính tương thích loại giao dịch (Cross-entity Validation): Giao dịch `INCOME` không được phép gắn danh mục `EXPENSE`.
- Viết Unit Test và Integration Test cho nghiệp vụ tài chính.

---

## 2. What Problem Does This Issue Solve?
Giao dịch (Transaction) là trái tim của FinTrack. Tuy nhiên, lập trình backend tài chính thường gặp 3 lỗi chí mạng:
1. **Lỗi làm tròn dấu phẩy động (Floating-point Rounding Error)**: Dùng kiểu số thực (`float32`, `float64`) để tính toán tiền bạc khiến tài khoản người dùng bị lệch 1 đồng hoặc xuất hiện những số lẻ kỳ quái như `150000.00000000003 VND`.
2. **Lỗi lệch ngày do múi giờ (Timezone Drift)**: Người dùng ở Việt Nam (UTC+7) ghi chi tiêu lúc 23:30 ngày 15/03. Nếu server lưu dạng UTC timestamp (`2026-03-15 16:30:00Z`), khi hiển thị trên giao diện hoặc query thống kê, giao dịch có nguy cơ bị nhảy sang ngày hôm trước hoặc hôm sau.
3. **Mất dữ liệu giao dịch khi xóa danh mục**: Nếu cấu hình khóa ngoại `ON DELETE CASCADE` cho Category, khi người dùng xóa danh mục "Ăn trưa", toàn bộ 500 giao dịch ăn trưa của cả năm sẽ bị xóa sạch khỏi lịch sử tài chính! Cần cơ chế giữ lại giao dịch và chuyển sang trạng thái "Chưa phân loại" (`ON DELETE SET NULL`).

---

## 3. Prerequisite Go Knowledge

### 3.1. Tại sao KHÔNG bao giờ dùng `float64` cho tiền tệ?
- Máy tính biểu diễn số thực theo chuẩn nhị phân **IEEE-754**.
- Nhiều phân số thập phân quen thuộc của con người không thể biểu diễn chính xác tuyệt đối dưới dạng nhị phân hữu hạn (giống như $1/3 = 0.3333...$ trong hệ thập phân).
- **Ví dụ kinh điển**:
```go
package main
import "fmt"

func main() {
    a := 0.1
    b := 0.2
    fmt.Println(a + b) // In ra: 0.30000000000000004
}
```
- Nếu một hệ thống ngân hàng hoặc ví điện tử cộng dồn hàng triệu giao dịch dạng `float64`, sai số tích lũy sẽ làm thâm hụt hàng triệu đồng!
- **Giải pháp của FinTrack**: Đồng Việt Nam (VND) không có đơn vị xu lẻ (nhỏ nhất là 1 đồng). Do đó, FinTrack lưu toàn bộ số tiền dưới dạng số nguyên `int64` (trong Go) và cột `BIGINT` (trong PostgreSQL). 150.000 VND được lưu chính xác là số nguyên `150000`. Phép cộng trừ số nguyên luôn luôn chính xác 100%.

### 3.2. Go Pointers (`*T`) và Database `NULL`
- Trong Go, các kiểu nguyên thủy (`int64`, `string`, `uuid.UUID`) luôn có giá trị mặc định là Zero Value (`0`, `""`, `uuid.Nil`), chúng không thể nhận giá trị `nil`.
- Nếu database trả về `NULL` vào một biến `string` thông thường khi gọi `Scan()`, driver `database/sql` sẽ báo lỗi: `sql: Scan error on column index X: unsupported Scan, storing driver.Value type <nil> into type *string`.
- **Cách giải quyết**: Khai báo trường là con trỏ:
  - `CategoryID *uuid.UUID`: Nếu cột trong DB là `NULL`, biến Go sẽ nhận giá trị `nil`. Nếu cột có giá trị, biến sẽ trỏ tới UUID đó.
  - `Description *string`: Xử lý ghi chú giao dịch có thể có hoặc không.

### 3.3. Parsing Calendar Date với mốc thời gian mẫu Go `2006-01-02`
- Khác với các ngôn ngữ khác dùng định dạng `%Y-%m-%d`, Go dùng một thời điểm mẫu duy nhất trong lịch sử: **Giờ 15, ngày 02, tháng 01, năm 2006 (MST -0700)**:
  - Năm: `2006`
  - Tháng: `01`
  - Ngày: `02`
- **Ví dụ**:
```go
_, err := time.Parse("2006-01-02", "2026-03-25")
if err != nil {
    // Không đúng định dạng YYYY-MM-DD
}
```

---

## 4. Architecture

Kiến trúc xử lý Giao dịch và xác thực chéo danh mục:

```text
HTTP Request (POST /api/v1/transactions)
       │
  (AuthMiddleware) ──► Trích xuất userID
       │
       ▼
[Handler: TransactionHandler.Create]
  • Bind JSON sang CreateTransactionRequest
       │
       ▼
[Service: TransactionService.CreateTransaction]
  • Kiểm tra amount > 0
  • Kiểm tra type IN ('INCOME', 'EXPENSE')
  • Parse định dạng ngày YYYY-MM-DD bằng time.Parse
  • NẾU CÓ CategoryID:
      - Gọi categoryRepo.FindByID(ctx, catID, userID)
      - Kiểm tra cat.Type == txType (Ví dụ: Danh mục EXPENSE không được gán cho INCOME)
       │
       ▼
[Repository: PostgresTransactionRepository.Create]
  • INSERT INTO transactions (id, user_id, amount, type, category_id, transaction_date, ...)
  • RETURNING id, created_at, updated_at
       │
       ▼
[PostgreSQL Database]
  • Check constraint: amount > 0
  • Check constraint: type IN ('INCOME', 'EXPENSE')
  • Foreign key: user_id (CASCADE), category_id (SET NULL)
```

---

## 5. File-by-File Analysis

### 5.1. `internal/domain/transaction.go`
- **Responsibility**: Định nghĩa struct `Transaction` cốt lõi.
- **Fields Analysis**:
  ```go
  type Transaction struct {
      ID              uuid.UUID       `json:"id"`
      UserID          uuid.UUID       `json:"user_id"`
      Amount          int64           `json:"amount"`           // Tiền VND tính bằng số nguyên
      Type            TransactionType `json:"type"`             // "INCOME" hoặc "EXPENSE"
      CategoryID      *uuid.UUID      `json:"category_id"`      // Con trỏ: có thể null
      TransactionDate string          `json:"transaction_date"` // Chuỗi "YYYY-MM-DD"
      Description     *string         `json:"description"`      // Con trỏ: có thể null
      CreatedAt       time.Time       `json:"created_at"`
      UpdatedAt       time.Time       `json:"updated_at"`
  }
  ```
- **Interview Importance**: 🔥 MUST KNOW. Giải thích tại sao dùng `int64` cho tiền và con trỏ cho `category_id`.

### 5.2. `migrations/000002_create_transactions.up.sql`
- **Responsibility**: Khởi tạo schema bảng `transactions` và các chỉ mục (indexes).
- **DDL & Constraints**:
  - `amount BIGINT NOT NULL CHECK (amount > 0)`: Chặn số âm hoặc 0 ở mức database.
  - `type VARCHAR(10) NOT NULL CHECK (type IN ('INCOME', 'EXPENSE'))`: Ràng buộc toàn vẹn miền giá trị.
  - `category_id UUID REFERENCES categories(id) ON DELETE SET NULL`: Khi xóa danh mục, không xóa giao dịch.
  - `user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`: Khi xóa tài khoản, xóa sạch mọi dữ liệu liên quan.
  - `transaction_date DATE NOT NULL`: Lưu ngày theo chuẩn SQL DATE (không kèm giờ, phút, giây).
  - Index `idx_transactions_user_date ON transactions (user_id, transaction_date DESC)`: Tối ưu cho màn hình xem lịch sử chi tiêu từ gần nhất đến xa nhất.
  - Index `idx_transactions_user_category ON transactions (user_id, category_id)`: Tối ưu cho việc lọc theo danh mục hoặc tính ngân sách.
- **Interview Importance**: 🔥 MUST KNOW. Phân biệt `CASCADE` vs `SET NULL` và chiến lược index composite.

### 5.3. `internal/repository/transaction_repository.go`
- **Responsibility**: Interface định nghĩa các phương thức CRUD giao dịch:
  - `Create(ctx context.Context, tx *domain.Transaction) error`
  - `CreateBatch(ctx context.Context, txs []domain.Transaction) error` (Dùng cho import CSV ở Issue #11)
  - `FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Transaction, error)`
  - `List(ctx context.Context, filter ListTransactionsFilter) (*ListTransactionsResult, error)`
  - `Update(ctx context.Context, tx *domain.Transaction) error`
  - `Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error`
- **Interview Importance**: ⭐ SHOULD KNOW.

### 5.4. `internal/repository/postgres_transaction_repository.go`
- **Responsibility**: Cài đặt câu lệnh SQL chi tiết.
- **Important Syntax**:
  - `TO_CHAR(transaction_date, 'YYYY-MM-DD')`: Chuyển đổi cột kiểu `DATE` của PostgreSQL thành chuỗi chuẩn ISO để scan thẳng vào trường `tx.TransactionDate` dạng `string` trong Go mà không bị driver đổi múi giờ.
  - Hỗ trợ tham số con trỏ `tx.CategoryID`: Nếu `tx.CategoryID == nil`, driver `database/sql` tự động truyền giá trị `NULL` vào cột database.
- **Interview Importance**: 🔥 MUST KNOW. Kỹ thuật `TO_CHAR` định dạng ngày trong SQL.

### 5.5. `internal/service/transaction_service.go`
- **Responsibility**: Xử lý logic nghiệp vụ và xác thực chéo.
- **Key Method: `validateTransaction`**:
  ```go
  if categoryID != nil && *categoryID != uuid.Nil {
      cat, err := s.categoryRepo.FindByID(ctx, *categoryID, userID)
      if err != nil {
          return domain.ValidationError{Field: "category_id", Message: "category not found or not owned by user"}
      }
      if cat.Type != domain.CategoryType(txType) {
          return domain.ValidationError{Field: "category_id", Message: "category type does not match transaction type"}
      }
  }
  ```
  - Kiểm tra xem danh mục có tồn tại và thuộc về user không.
  - Kiểm tra kiểu: Không cho phép giao dịch chi tiêu (`EXPENSE`) gán danh mục thu nhập (`INCOME`).
- **Interview Importance**: 🔥 MUST KNOW. Cross-resource type validation.

### 5.6. `internal/handler/transaction_handler.go`
- **Responsibility**: Tiếp nhận HTTP request cho giao dịch, parse JSON payload, gọi Service, trả về HTTP status code phù hợp (201 Created cho Create, 200 OK cho Read/Update, 204 No Content cho Delete).
- **Interview Importance**: ⭐ SHOULD KNOW.

---

## 6. Concrete Runtime Flow

Chi tiết luồng chạy của một request `POST /api/v1/transactions`:

```text
1. Client gửi POST /api/v1/transactions
   Body: {
     "amount": 50000,
     "type": "EXPENSE",
     "category_id": "c111-c111-c111",
     "transaction_date": "2026-03-25",
     "description": "Ăn trưa bún bò"
   }
   Header: Authorization: Bearer <token-user-1>

2. AuthMiddleware:
   - Xác thực token -> userID: "u111-u111-u111".
   - c.Set("user_id", "u111-u111-u111").

3. TransactionHandler.Create:
   - userID = GetUserID(c).
   - c.ShouldBindJSON(&req).
   - Gọi transactionService.CreateTransaction(ctx, input).

4. TransactionService.CreateTransaction:
   - validateTransaction():
     • amount = 50000 > 0 -> Hợp lệ.
     • type = "EXPENSE" -> Hợp lệ.
     • time.Parse("2006-01-02", "2026-03-25") -> Hợp lệ.
     • Có category_id: Gọi categoryRepo.FindByID(ctx, "c111-...", "u111-...").
     • Tìm thấy Category "Ăn uống" có Type = "EXPENSE".
     • So sánh: cat.Type == txType ("EXPENSE" == "EXPENSE") -> Hợp lệ!
   - Gọi txRepo.Create(ctx, tx).

5. PostgresTransactionRepository.Create:
   - Thực thi câu lệnh SQL INSERT INTO transactions ... RETURNING id, created_at, updated_at.
   - Database kiểm tra các ràng buộc CHECK và Khóa ngoại. Thành công.
   - Scan ID và timestamps mới sinh vào struct tx.

6. Phản hồi HTTP:
   - Handler nhận tx hoàn chỉnh, format sang JSON và trả về HTTP 201 Created.
```

---

## 7. Important Go Concepts

### 7.1. Dereferencing Nil Pointers (Nguy cơ số 1 gây Panic)
- Khi một biến là con trỏ `*T`, nếu nó mang giá trị `nil`, việc cố tình truy cập giá trị bên trong (`*ptr` hoặc `ptr.Field`) sẽ gây ra runtime panic: `panic: runtime error: invalid memory address or nil pointer dereference`.
- **Nguyên tắc bất di bất dịch**: Luôn kiểm tra con trỏ khác `nil` trước khi dereference:
  ```go
  if categoryID != nil && *categoryID != uuid.Nil {
      // An toàn khi truy cập *categoryID
  }
  ```

### 7.2. So sánh Enum thông qua Type Conversion
- Trong Go, `domain.TransactionType` và `domain.CategoryType` là hai kiểu dữ liệu khác nhau dù cùng có kiểu cơ sở là `string`.
- Bạn không thể so sánh trực tiếp: `if cat.Type == txType` (trình biên dịch Go sẽ báo lỗi: `mismatched types CategoryType and TransactionType`).
- **Cách viết chuẩn**: Ép kiểu rõ ràng:
  ```go
  if cat.Type != domain.CategoryType(txType) { ... }
  ```

---

## 8. Database / SQL Concepts

### 8.1. Phân biệt `ON DELETE CASCADE` và `ON DELETE SET NULL`
| Bảng tham chiếu | Ràng buộc khóa ngoại | Hành vi khi bản ghi cha bị xóa | Lý do áp dụng trong FinTrack |
| :--- | :--- | :--- | :--- |
| `users` | `ON DELETE CASCADE` | Xóa User -> Xóa sạch tất cả giao dịch | Xóa tài khoản thì toàn bộ dữ liệu cá nhân phải bị hủy bỏ hoàn toàn (tuân thủ luật bảo vệ dữ liệu cá nhân GDPR). |
| `categories` | `ON DELETE SET NULL` | Xóa Category -> Giữ giao dịch, set `category_id = NULL` | Xóa một danh mục không được phép làm biến mất lịch sử tiền bạc của người dùng. Giao dịch chuyển thành "Chưa phân loại". |

### 8.2. Chiến lược Index Composite cho Giao dịch
```sql
CREATE INDEX idx_transactions_user_date ON transactions (user_id, transaction_date DESC);
```
- Tại sao đặt `user_id` ở đầu và `transaction_date DESC` ở sau?
- **Nguyên lý B-Tree Index**: Database sẽ lọc nhanh chóng tất cả các giao dịch của một `user_id` cụ thể, sau đó dữ liệu của user đó đã được sắp xếp sẵn theo thứ tự ngày giảm dần. Khi người dùng mở app và xem trang 1 các giao dịch mới nhất, database chỉ cần đọc trực tiếp từ Index mà không cần tốn CPU để thực hiện thuật toán Sort trong RAM.

---

## 9. Error Handling

### Xử lý lỗi khóa ngoại (Foreign Key Violation - PostgreSQL Code `23503`)
- Nếu một client cố tình gửi `category_id` là một UUID ngẫu nhiên không tồn tại trong bảng `categories`, PostgreSQL sẽ chặn lại với mã lỗi SQLSTATE `23503` (foreign_key_violation).
- Tuy nhiên trong FinTrack, tầng Service chủ động kiểm tra bằng `categoryRepo.FindByID` trước khi ghi vào database. Điều này cho phép trả về lỗi thân thiện:
  ```json
  {
    "error": {
      "code": "VALIDATION_ERROR",
      "message": "category not found or not owned by user",
      "fields": {
        "category_id": "category not found or not owned by user"
      }
    }
  }
  ```

---

## 10. Testing

### 10.1. Kiểm thử xác thực chéo loại giao dịch (Cross-type validation test)
Trong `internal/service/transaction_service_test.go`:
- Giả lập (mock) `categoryRepo` chứa danh mục "Lương" (Type: `INCOME`).
- Gọi `CreateTransaction` với `Type = EXPENSE` và `CategoryID = "id-lương"`.
- Khẳng định trả về lỗi `ValidationError` trên trường `category_id` với nội dung `"category type does not match transaction type"`.

### 10.2. Kiểm thử tạo giao dịch không có danh mục (Uncategorized)
- Gọi `CreateTransaction` với `CategoryID = nil`.
- Khẳng định giao dịch được tạo thành công với `category_id` mang giá trị `nil` trong Go và `NULL` trong database.

---

## 11. Beginner Mistakes

### 5 Sai lầm phổ biến của người mới:
1. **Dùng `float64` để lưu tiền tệ**: Dẫn tới sai số làm tròn số thực trong các phép tính lũy kế.
2. **Dùng `time.Time` chứa cả giờ/phút/giây để lưu ngày giao dịch**: Gây ra lỗi lệch ngày khi người dùng ở các múi giờ khác nhau truy cập hệ thống.
3. **Cấu hình `ON DELETE CASCADE` trên cột `category_id`**: Khiến người dùng mất sạch toàn bộ lịch sử chi tiêu khi lỡ tay xóa một danh mục.
4. **Quên kiểm tra `nil` trước khi giải mã con trỏ UUID**: Viết `*req.CategoryID` trực tiếp gây ra panic sập server khi client gửi request không có trường `category_id`.
5. **Không kiểm tra số tiền âm hoặc bằng 0**: Chấp nhận giao dịch với `amount = 0` hoặc số âm khiến các công thức tính toán tổng thu nhập và chi tiêu bị đảo lộn.

### 3 Nguy cơ sinh bug thực tế:
1. **Lệch type giữa Transaction và Category**: Cho phép gán danh mục "Tiền nhà" (`EXPENSE`) cho một khoản thu nhập (`INCOME`), dẫn đến báo cáo chi tiêu danh mục bị tính toán sai hoàn toàn.
2. **Lỗ hổng gán trộm danh mục của người khác**: Chỉ kiểm tra `category_id` tồn tại trong database mà không kiểm tra `user_id` sở hữu danh mục đó, cho phép User A gán danh mục riêng tư của User B vào giao dịch của mình.
3. **Lỗi format ngày khi parse SQL**: Không dùng `TO_CHAR` khiến driver Postgres tự chuyển cột `DATE` thành UTC timestamp ở thời điểm `00:00:00`, sau đó chuyển đổi múi giờ làm lùi ngày về hôm trước.

### 2 Cái bẫy phỏng vấn (Interview Traps):
1. *"Tại sao trong FinTrack em dùng `int64` cho tiền mà không dùng `float64` hoặc `string`?"* -> **Bẫy**: `float64` bị lỗi sai số chuẩn IEEE-754. `string` thì không tính toán cộng trừ nhân chia trực tiếp được. `int64` (hoặc định dạng số nguyên nhỏ nhất của tiền tệ, ví dụ cent cho USD và đồng cho VND) là tiêu chuẩn vàng trong kỹ thuật phần mềm tài chính.
2. *"Tại sao khi xóa Category ta lại dùng `ON DELETE SET NULL` thay vì cấm xóa (`RESTRICT`) hoặc xóa theo (`CASCADE`)?"* -> **Bẫy**: `CASCADE` làm mất dữ liệu lịch sử tài chính của người dùng. `RESTRICT` gây khó chịu vì người dùng không thể dọn dẹp danh mục cũ nếu nó đã từng có giao dịch. `SET NULL` là giải pháp cân bằng hoàn hảo nhất: danh mục biến mất nhưng tiền bạc và lịch sử vẫn nguyên vẹn.

---

## 12. Code Reading Exercises

### Bài tập 1: Dự đoán kết quả
Đoạn code sau có an toàn không khi thực thi?
```go
type Request struct {
    CategoryID *string `json:"category_id"`
}

func Process(req Request) {
    if len(*req.CategoryID) > 0 {
        fmt.Println("Category is present")
    }
}
```
*Gợi ý suy nghĩ*: Điều gì xảy ra nếu client gửi JSON `{}` (không có trường `category_id`)?

### Bài tập 2: Tìm bug logic tài chính
Đoạn code sau có lỗi gì khi tính số dư?
```go
var balance float64 = 0
for _, amount := range transactions {
    balance += float64(amount) * 0.0001
}
```

---

## 13. Active Recall

### Câu hỏi ôn tập
1. Chuẩn IEEE-754 gây ra vấn đề gì khi lập trình các ứng dụng tài chính bằng `float64`?
2. Tại sao FinTrack lưu ngày giao dịch dưới dạng chuỗi `YYYY-MM-DD` thay vì `time.Time` có timezone?
3. Sự khác nhau giữa hành vi của `ON DELETE CASCADE` và `ON DELETE SET NULL` trong SQL là gì?
4. Tại sao trường `CategoryID` trong struct `Transaction` phải là con trỏ `*uuid.UUID`?
5. Tại sao cần kiểm tra xem `cat.Type` có bằng `tx.Type` trước khi tạo giao dịch không?

---

### Model Answers

#### 1. Vấn đề của IEEE-754 với `float64`
- **English**: Binary floating-point representation cannot precisely represent many decimal fractions (such as 0.1), resulting in rounding inaccuracies (e.g. 0.1 + 0.2 = 0.30000000000000004) that corrupt financial balances over time.
- **Vietnamese**: Biểu diễn dấu phẩy động nhị phân không thể đại diện chính xác nhiều phân số thập phân (như 0.1), dẫn đến sai số làm tròn tích lũy làm sai lệch số dư tài chính theo thời gian.

#### 2. Tại sao lưu `YYYY-MM-DD`?
- **English**: A transaction date is a calendar date representing a specific day in the user's local timeline. Storing it as a pure `YYYY-MM-DD` calendar string prevents timezone conversions on server clusters from shifting transactions into the previous or next day.
- **Vietnamese**: Ngày giao dịch là một mốc lịch cố định theo lịch địa phương của người dùng. Lưu dưới dạng chuỗi lịch thuần túy `YYYY-MM-DD` giúp ngăn chặn việc chuyển đổi múi giờ trên server làm dịch chuyển giao dịch sang ngày hôm trước hoặc hôm sau.

#### 3. Phân biệt `CASCADE` và `SET NULL`
- **English**: `CASCADE` automatically deletes child rows when the referenced parent row is deleted. `SET NULL` preserves child rows and sets their foreign key column to `NULL`.
- **Vietnamese**: `CASCADE` tự động xóa luôn các dòng con khi dòng cha bị xóa. `SET NULL` giữ lại các dòng con và đặt giá trị của cột khóa ngoại về `NULL`.

#### 4. Tại sao `CategoryID` là con trỏ `*uuid.UUID`?
- **English**: Because transactions can be uncategorized, mapping to a database `NULL`. A pointer allows representing this `NULL` as `nil` in Go, whereas a plain `uuid.UUID` cannot be `nil`.
- **Vietnamese**: Vì giao dịch có thể không thuộc danh mục nào, tương ứng với giá trị `NULL` trong cơ sở dữ liệu. Con trỏ cho phép biểu diễn `NULL` này dưới dạng `nil` trong Go, trong khi biến `uuid.UUID` thông thường không thể nhận giá trị `nil`.

#### 5. Tại sao phải so khớp `cat.Type` và `tx.Type`?
- **English**: To maintain domain integrity. A user must not categorize an `EXPENSE` transaction under an `INCOME` category (or vice versa), which would distort category-based expense reporting and budget calculations.
- **Vietnamese**: Để duy trì tính toàn vẹn nghiệp vụ. Người dùng không được phép gắn một giao dịch CHI TIÊU vào một danh mục THU NHẬP (hoặc ngược lại), điều này sẽ làm sai lệch toàn bộ báo cáo chi tiêu và ngân sách.

---

## 14. Fresher Go Backend Interview Questions

### Question 1
- **English**: Why does FinTrack use `int64` instead of `float64` for money amounts?
- **Vietnamese**: Tại sao FinTrack lại sử dụng `int64` thay vì `float64` cho số tiền giao dịch?
- **Model Answer (EN)**: Floating-point types in computing follow the IEEE-754 standard, which suffers from precision and rounding errors during decimal arithmetic. Storing currency as integers in the smallest currency unit (such as VND without subunit, or cents for USD) ensures 100% exact mathematical operations.
- **Model Answer (VI)**: Kiểu dấu phẩy động trong máy tính tuân theo chuẩn IEEE-754, gặp lỗi sai số làm tròn khi tính toán số thập phân. Lưu tiền tệ bằng số nguyên ở đơn vị nhỏ nhất (như VND không có đơn vị phụ, hoặc cent đối với USD) đảm bảo độ chính xác tuyệt đối 100% trong mọi phép tính toán.
- **Explanation**: Tiêu chuẩn thiết kế phần mềm tài chính ngân hàng.
- **Follow-up**: How would you represent currencies with fractional units, such as USD or EUR? (Answer: Store the amount in cents as an integer, e.g., $10.50 is stored as 1050 cents).

### Question 2
- **English**: How do you represent nullable database columns in Go structs?
- **Vietnamese**: Bạn biểu diễn các cột có thể nhận giá trị NULL trong cơ sở dữ liệu như thế nào trong struct Go?
- **Model Answer (EN)**: There are two main approaches: using pointers (e.g. `*uuid.UUID`, `*string`) where `nil` maps to SQL `NULL`, or using `database/sql` wrapper types like `sql.NullString`, `sql.NullInt64`. FinTrack prefers pointers because they serialize cleanly to JSON as `null` or can be omitted.
- **Model Answer (VI)**: Có hai cách chính: sử dụng con trỏ (như `*uuid.UUID`, `*string`) với giá trị `nil` tương ứng với SQL `NULL`, hoặc dùng các struct bọc sẵn như `sql.NullString`, `sql.NullInt64`. FinTrack ưu tiên dùng con trỏ vì chúng serialize sang JSON thành `null` rất tự nhiên và sạch đẹp.
- **Explanation**: Xử lý dữ liệu nullable trong Go.
- **Follow-up**: What is the risk of using pointers for nullable fields? (Answer: Dereferencing a nil pointer without checking will cause a runtime panic).

### Question 3
- **English**: Why did you choose `ON DELETE SET NULL` for the category foreign key in transactions?
- **Vietnamese**: Tại sao bạn chọn `ON DELETE SET NULL` cho khóa ngoại danh mục trong bảng transactions?
- **Model Answer (EN)**: If we used `ON DELETE CASCADE`, deleting a category would permanently erase all historical financial transactions associated with it, destroying the user's expense history. `SET NULL` keeps financial records intact while marking the category as uncategorized.
- **Model Answer (VI)**: Nếu dùng `ON DELETE CASCADE`, việc xóa một danh mục sẽ xóa vĩnh viễn toàn bộ các giao dịch tài chính lịch sử gắn với nó, làm mất dữ liệu của người dùng. `SET NULL` giữ nguyên vẹn các bản ghi giao dịch và chỉ đánh dấu danh mục là chưa phân loại.
- **Explanation**: Tư duy thiết kế cơ sở dữ liệu hướng người dùng (Data Preservation).
- **Follow-up**: What would happen if we used `ON DELETE RESTRICT` instead? (Answer: The database would reject the category deletion until the user manually deletes or reassigns all existing transactions).

### Question 4
- **English**: How do you validate calendar dates in Go without being affected by timezone shifts?
- **Vietnamese**: Làm thế nào để xác thực ngày lịch trong Go mà không bị ảnh hưởng bởi việc lệch múi giờ?
- **Model Answer (EN)**: We validate the format using `time.Parse("2006-01-02", dateStr)` and store it directly as a pure ISO date string (`YYYY-MM-DD`) in Go and a SQL `DATE` column in PostgreSQL, avoiding converting it into a timezone-aware timestamp.
- **Model Answer (VI)**: Chúng ta xác thực định dạng bằng `time.Parse("2006-01-02", dateStr)` và lưu trữ trực tiếp dưới dạng chuỗi ngày ISO thuần túy (`YYYY-MM-DD`) trong Go và cột SQL `DATE` trong PostgreSQL, tránh việc biến nó thành một timestamp có gắn múi giờ.
- **Explanation**: Kỹ thuật xử lý ngày tháng độc lập với múi giờ (Timezone-agnostic Calendar Date).
- **Follow-up**: What is the significance of the reference date `2006-01-02` in Go? (Answer: It represents month 1, day 2, hour 3, minute 4, second 5, year 2006 in MST -0700).

### Question 5
- **English**: Why do we use `TO_CHAR(transaction_date, 'YYYY-MM-DD')` when querying PostgreSQL?
- **Vietnamese**: Tại sao chúng ta lại dùng `TO_CHAR(transaction_date, 'YYYY-MM-DD')` khi truy vấn PostgreSQL?
- **Model Answer (EN)**: When querying a PostgreSQL `DATE` column, the Go driver defaults to scanning it into a `time.Time` object in local or UTC time at midnight (00:00:00). Using `TO_CHAR` ensures PostgreSQL formats the date directly as a string, allowing safe scanning directly into our Go `string` field without timezone distortion.
- **Model Answer (VI)**: Khi truy vấn cột `DATE` của PostgreSQL, driver của Go mặc định sẽ scan nó thành đối tượng `time.Time` tại nửa đêm (00:00:00). Dùng `TO_CHAR` đảm bảo PostgreSQL format ngày thành chuỗi ký tự ngay từ database, cho phép scan an toàn vào biến `string` trong Go mà không sợ bị lệch múi giờ.
- **Explanation**: Tương tác chuyên sâu giữa PostgreSQL và driver Go `database/sql`.
- **Follow-up**: Can you scan `TO_CHAR` output into a `*string`? (Answer: Yes, standard Scan handles string pointers).

### Question 6
- **English**: How does FinTrack ensure that a user cannot assign another user's category to their transaction?
- **Vietnamese**: FinTrack làm thế nào để đảm bảo một user không thể gán danh mục của user khác vào giao dịch của mình?
- **Model Answer (EN)**: During transaction validation in `TransactionService`, when a `category_id` is provided, we query `categoryRepo.FindByID(ctx, *categoryID, userID)` passing the authenticated `userID`. If the category belongs to someone else, the query returns `ErrNotFound`, and we reject the request with a validation error.
- **Model Answer (VI)**: Trong quá trình validate ở `TransactionService`, khi có `category_id`, ta gọi `categoryRepo.FindByID(ctx, *categoryID, userID)` kèm theo `userID` đã xác thực. Nếu danh mục thuộc về người khác, hàm sẽ trả về `ErrNotFound` và hệ thống sẽ từ chối request với lỗi validation.
- **Explanation**: Bảo mật kiểm soát truy cập chéo thực thể (Cross-Entity Ownership Validation).
- **Follow-up**: What HTTP status code is returned in this scenario? (Answer: 400 Bad Request with a structured field validation error).

### Question 7
- **English**: What is a composite database index and how does `idx_transactions_user_date` optimize queries?
- **Vietnamese**: Composite database index là gì và chỉ mục `idx_transactions_user_date` tối ưu truy vấn như thế nào?
- **Model Answer (EN)**: A composite index covers multiple columns in a specific order. `(user_id, transaction_date DESC)` allows PostgreSQL to quickly isolate all transactions of a user and immediately read them in reverse chronological order without an expensive in-memory sort operation.
- **Model Answer (VI)**: Composite index là chỉ mục bao gồm nhiều cột theo một thứ tự xác định. `(user_id, transaction_date DESC)` cho phép PostgreSQL nhanh chóng cô lập toàn bộ giao dịch của một user và đọc ngay lập tức theo thứ tự thời gian giảm dần mà không cần tốn chi phí sắp xếp dữ liệu trong RAM.
- **Explanation**: Tối ưu hóa hiệu năng cơ sở dữ liệu.
- **Follow-up**: Can this index be used to query by `transaction_date` alone without `user_id`? (Answer: No, B-tree composite indexes must match from the leftmost prefix).

### Question 8
- **English**: How does the CHECK constraint `amount > 0` at the database level complement application validation?
- **Vietnamese**: Ràng buộc CHECK `amount > 0` ở tầng database bổ trợ cho validation ở tầng ứng dụng như thế nào?
- **Model Answer (EN)**: Application validation catches mistakes early and provides user-friendly error messages. Database constraints act as the ultimate fail-safe defense-in-depth, preventing bad data from entering the database even if there is a bug, bypass, or migration script mistake in the application layer.
- **Model Answer (VI)**: Validation tầng ứng dụng giúp phát hiện lỗi sớm và phản hồi thông báo thân thiện cho người dùng. Ràng buộc ở tầng database đóng vai trò là chốt chặn phòng thủ chiều sâu (Defense-in-Depth) cuối cùng, ngăn chặn dữ liệu rác lọt vào DB kể cả khi có bug ở tầng code hoặc khi chạy script can thiệp trực tiếp.
- **Explanation**: Nguyên lý an toàn Defense-in-Depth.
- **Follow-up**: Does PostgreSQL enforce CHECK constraints during bulk COPY or UPDATE operations? (Answer: Yes, CHECK constraints are always enforced unless explicitly disabled).

### Question 9
- **English**: What happens in Go when you convert `domain.TransactionType` to `domain.CategoryType`?
- **Vietnamese**: Điều gì xảy ra trong Go khi bạn chuyển đổi `domain.TransactionType` sang `domain.CategoryType`?
- **Model Answer (EN)**: Since both types share the same underlying type (`string`), type conversion `domain.CategoryType(txType)` is a zero-cost compiler-checked cast that allows comparing their values.
- **Model Answer (VI)**: Vì cả hai kiểu dữ liệu đều có kiểu cơ sở ngầm là `string`, việc ép kiểu `domain.CategoryType(txType)` là một phép chuyển đổi kiểu an toàn, chi phí thực thi bằng 0 (zero-cost) được trình biên dịch kiểm tra để cho phép so sánh giá trị giữa hai kiểu.
- **Explanation**: Hệ thống kiểu dữ liệu tĩnh mạnh (Strong Static Type System) của Go.
- **Follow-up**: Can you cast an `int` to `CategoryType` directly? (Answer: No, compiler forbids casting between non-convertible types).

### Question 10
- **English**: Why does `TransactionHandler` format dates using `time.RFC3339` for timestamps but leave `transaction_date` as a raw string?
- **Vietnamese**: Tại sao `TransactionHandler` format thời gian bằng `time.RFC3339` cho các timestamp nhưng giữ nguyên chuỗi thô cho `transaction_date`?
- **Model Answer (EN)**: `created_at` and `updated_at` represent exact moments in universal time (`TIMESTAMPTZ`), requiring the full ISO-8601/RFC3339 format with UTC offsets. `transaction_date` is a pure calendar date (`YYYY-MM-DD`) without hours or timezone concepts, so returning it as a plain 10-character string preserves its calendar nature.
- **Model Answer (VI)**: `created_at` và `updated_at` đại diện cho các thời khắc chính xác trong thời gian thực (`TIMESTAMPTZ`), cần định dạng đầy đủ ISO-8601/RFC3339 kèm múi giờ UTC. `transaction_date` là một ngày lịch thuần túy (`YYYY-MM-DD`) không có khái niệm giờ giấc hay múi giờ, do đó trả về chuỗi 10 ký tự thuần túy giữ đúng bản chất ngày lịch của nó.
- **Explanation**: Phân biệt giữa Timestamp (thời điểm) và Calendar Date (ngày lịch).
- **Follow-up**: What is the standard Go layout string for RFC3339? (Answer: `time.RFC3339`).

---

## 15. Knowledge Connection
- **Kế thừa từ Issue #6**: Sử dụng các danh mục đã tạo từ Issue #6 để gán vào `category_id`.
- **Chuẩn bị cho Issue #8 (Filtering & Pagination)**: Số lượng giao dịch được tạo ra ở Issue này sẽ được lọc đa tiêu chí (theo ngày, loại, danh mục) và phân trang ở Issue #8.
- **Chuẩn bị cho Issue #9 (Budgets) & #10 (Analytics)**: Toàn bộ số tiền chi tiêu `amount` từ các giao dịch có kiểu `EXPENSE` ở Issue này sẽ được dùng làm dữ liệu đầu vào để tính toán hạn mức ngân sách và biểu đồ tài chính.

---

## 16. Learning Checkpoint

### I Must Understand
1. Lý do tuyệt đối không dùng số thực `float64` để lưu trữ và tính toán tiền bạc.
2. Cách dùng số nguyên `int64` / SQL `BIGINT` cho tiền tệ (VND).
3. Cách phân biệt `ON DELETE CASCADE` (xóa sạch) và `ON DELETE SET NULL` (giữ lại và set null).
4. Cách xử lý con trỏ Go (`*uuid.UUID`, `*string`) cho các trường dữ liệu có thể mang giá trị `NULL`.
5. Mốc định dạng ngày mẫu kinh điển của Go: `"2006-01-02"`.

### I Should Be Able To Code
1. Định nghĩa struct Go chứa các trường optional/nullable bằng con trỏ.
2. Viết hàm kiểm tra hợp lệ chuỗi ngày lịch bằng `time.Parse("2006-01-02", dateStr)`.
3. Viết câu lệnh SQL `UPDATE` và `RETURNING` an toàn với điều kiện `user_id`.

### I Should Be Able To Explain In An Interview
1. Chuẩn IEEE-754 ảnh hưởng đến tính toán tiền tệ như thế nào và cách khắc phục?
2. Tại sao lại dùng `ON DELETE SET NULL` cho Category thay vì `CASCADE`?
3. Tại sao ngày giao dịch nên lưu dạng chuỗi `YYYY-MM-DD` thay vì timestamp có giờ phút?

### Advanced Topics I Do Not Need Yet
- Xử lý đa tiền tệ (Multi-currency conversion) với tỷ giá hối đoái biến động theo thời gian thực.
- Thư viện `shopspring/decimal` (thư viện số học thập phân có độ chính xác tùy biến dùng cho tiền tệ có nhiều số lẻ).
- Cơ chế lập lịch giao dịch định kỳ (Recurring Transactions: tiền thuê nhà lặp lại hàng tháng).
