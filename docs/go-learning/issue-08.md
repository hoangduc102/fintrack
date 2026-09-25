# Issue #8 — Transaction History, Dynamic SQL, Offset Pagination & Deterministic Sorting

## 1. Learning Objectives
Sau khi hoàn thành bài học này, bạn sẽ nắm vững:
- Kỹ thuật xây dựng câu truy vấn SQL động (Dynamic SQL Query Construction) an toàn trong Go mà không bị tấn công SQL Injection.
- Quản lý chỉ mục tham số động (`$1, $2, $3...`) và slice đối số `args []interface{}`.
- Cơ chế phân trang chuẩn Offset-based Pagination: Công thức tính `offset = (page - 1) * limit` và công thức chia trần nguyên `(total + limit - 1) / limit`.
- Kỹ thuật sắp xếp đơn định 3 tầng (3-tier Deterministic Sorting) để loại bỏ hoàn toàn hiện tượng trùng hoặc nhảy cóc dữ liệu khi phân trang.
- Xác thực loại trừ lẫn nhau (Mutual Exclusion Validation) giữa `category_id` và `uncategorized=true`.
- Tách biệt câu lệnh `COUNT(*)` và câu lệnh lấy dữ liệu `SELECT ... LIMIT ... OFFSET`.

---

## 2. What Problem Does This Issue Solve?
Sau một thời gian sử dụng, một người dùng có thể có hàng nghìn bản ghi giao dịch. Hệ thống phải giải quyết các bài toán sau:
1. **Lọc đa tiêu chí linh hoạt (Dynamic Filtering)**: Người dùng muốn:
   - Xem giao dịch trong một khoảng thời gian (`start_date`, `end_date`).
   - Lọc theo loại (`type=INCOME` hoặc `type=EXPENSE`).
   - Lọc theo một danh mục cụ thể (`category_id=...`).
   - Lọc tất cả các giao dịch *chưa phân loại* (`uncategorized=true`, tức là `category_id IS NULL`).
   - Hoặc kết hợp tất cả các tiêu chí trên cùng một lúc!
2. **Quá tải bộ nhớ (Memory Exhaustion / DoS)**: Nếu trả về toàn bộ 10.000 giao dịch trong 1 request, RAM server sẽ bị phình to và ứng dụng di động phía client sẽ bị đơ lag. Cần cơ chế phân trang có giới hạn (`limit <= 100`).
3. **Lỗi phân trang nhảy cóc (Pagination Hopping Bug)**: Khi sắp xếp theo ngày (`ORDER BY transaction_date DESC`), nếu người dùng có 10 giao dịch diễn ra trong cùng một ngày, giữa trang 1 và trang 2 các giao dịch có thể bị lặp lại hoặc biến mất do PostgreSQL không đảm bảo thứ tự của các dòng có cùng giá trị ngày.

---

## 3. Prerequisite Go Knowledge

### 3.1. Variadic Arguments và Ellipsis (`...`)
- **Khái niệm**: Trong Go, một hàm có thể nhận số lượng tham số tùy ý bằng cú pháp `args ...any`. Khi bạn đã có sẵn một slice và muốn truyền từng phần tử của nó vào hàm variadic, bạn dùng cú pháp bung slice: `args...`.
- **Ví dụ cơ bản**:
```go
func Query(query string, args ...any) { ... }

params := []any{"user-id-1", 10, 0}
Query("SELECT ...", params...) // Bung slice thành các tham số rời rạc
```
- **Kết nối FinTrack**: `r.db.QueryContext(ctx, itemsQuery, itemsArgs...)` cho phép truyền slice các tham số động vào hàm `QueryContext` của `database/sql`.

### 3.2. Integer Ceiling Division (Chia trần số nguyên)
- **Khái niệm**: Để tính tổng số trang (`totalPages`) từ tổng số bản ghi (`total`) và kích thước trang (`limit`), ta cần phép chia làm tròn lên (ceiling division). Thay vì ép kiểu sang số thực `math.Ceil(float64(total) / float64(limit))` (chậm và có nguy cơ sai số float), Go idiom sử dụng toán học số nguyên thuần túy:
```go
totalPages := (totalCount + limit - 1) / limit
```
- **Ví dụ kiểm chứng**:
  - `total = 10, limit = 5` -> `(10 + 4) / 5 = 14 / 5 = 2` trang.
  - `total = 11, limit = 5` -> `(11 + 4) / 5 = 15 / 5 = 3` trang.
- **Kết nối FinTrack**: Được sử dụng trực tiếp trong `TransactionService.ListTransactions`.

### 3.3. `strings.Join` để nối mệnh đề SQL
- Thay vì cộng chuỗi bằng dấu `+` khiến code rối rắm và khó kiểm soát dấu phẩy hoặc chữ `AND`, ta gom các điều kiện vào một slice `[]string` rồi dùng `strings.Join(whereClauses, " AND ")`.

---

## 4. Architecture

Kiến trúc xử lý lọc và phân trang động:

```text
HTTP Request: GET /api/v1/transactions?start_date=2026-03-01&type=EXPENSE&page=2&limit=20
       │
  (AuthMiddleware) ──► userID: "u111"
       │
       ▼
[Handler: TransactionHandler.List]
  • Đọc query params: page, limit, start_date, end_date, type, category_id, uncategorized
       │
       ▼
[Service: TransactionService.ListTransactions]
  • Default page = 1, default limit = 20, max limit = 100
  • Validate: category_id và uncategorized không được đồng thời xuất hiện
  • Validate: start_date <= end_date
  • Tính toán offset = (page - 1) * limit
       │
       ▼
[Repository: PostgresTransactionRepository.List]
  • Bắt đầu với: whereClauses = ["user_id = $1"], args = [userID], argIdx = 2
  • NẾU có StartDate: whereClauses += "transaction_date >= $2", args += "2026-03-01", argIdx++
  • NẾU có Type: whereClauses += "type = $3", args += "EXPENSE", argIdx++
  • Nối điều kiện: whereSQL = strings.Join(whereClauses, " AND ")
  • BƯỚC 1: QueryRowContext: SELECT COUNT(*) WHERE whereSQL -> totalCount
  • BƯỚC 2: QueryContext: SELECT ... WHERE whereSQL ORDER BY ... LIMIT $4 OFFSET $5
       │
       ▼
[PostgreSQL Database]
  • Thực thi query sử dụng index idx_transactions_user_date
```

---

## 5. File-by-File Analysis

### 5.1. `internal/repository/transaction_repository.go`
- **Structs Analysis**:
  ```go
  type ListTransactionsFilter struct {
      UserID        uuid.UUID
      StartDate     *string
      EndDate       *string
      Type          *domain.TransactionType
      CategoryID    *uuid.UUID
      Uncategorized bool
      Limit         int
      Offset        int
  }

  type ListTransactionsResult struct {
      Transactions []domain.Transaction
      TotalCount   int
  }
  ```
- **Why structured this way?**: Thay vì một method nhận 8 tham số rời rạc (`List(ctx, userID, startDate, endDate...)`), gom vào một struct filter giúp:
  1. Dễ dàng mở rộng thêm các tiêu chí lọc trong tương lai mà không làm vỡ interface.
  2. Tránh nhầm lẫn thứ tự giữa các tham số cùng kiểu (như `Limit` và `Offset`).
- **Interview Importance**: 🔥 MUST KNOW. Parameter Object Pattern.

### 5.2. `internal/repository/postgres_transaction_repository.go` (`List` Method)
- **Responsibility**: Xây dựng SQL động và quản lý tham số an toàn.
- **Dynamic Parameter Construction**:
  ```go
  whereClauses := []string{"user_id = $1"}
  args := []interface{}{filter.UserID}
  argIdx := 2

  if filter.StartDate != nil && *filter.StartDate != "" {
      whereClauses = append(whereClauses, fmt.Sprintf("transaction_date >= $%d", argIdx))
      args = append(args, *filter.StartDate)
      argIdx++
  }
  // Tương tự cho EndDate, Type, CategoryID...
  ```
- **Deterministic Sorting Logic**:
  ```sql
  ORDER BY transaction_date DESC, created_at DESC, id DESC
  LIMIT $%d OFFSET $%d
  ```
  1. Tiêu chí 1: `transaction_date DESC` (Ngày giao dịch mới nhất lên đầu).
  2. Tiêu chí 2: `created_at DESC` (Nếu cùng ngày, giao dịch nhập sau lên đầu).
  3. Tiêu chí 3: `id DESC` (Khóa chính UUID duy nhất tuyệt đối, đảm bảo thứ tự cố định 100% kể cả khi được tạo cùng microsecond).
- **Interview Importance**: 🔥 MUST KNOW. Kỹ thuật đánh số biến `$1, $2, $3` và lý do cần Deterministic Sorting.

### 5.3. `internal/service/transaction_service.go` (`ListTransactions`)
- **Responsibility**: Chứa các quy tắc phòng thủ và tính toán phân trang.
- **Defensive Limits**:
  - `limit` tối đa là 100 (`if limit > 100 { return ValidationError }`). Chống tấn công vắt kiệt RAM.
  - Phân tích ngày bằng `time.Parse` và kiểm tra `startDate.After(*endDate)`.
  - Kiểm tra loại trừ: `if input.CategoryID != nil && input.Uncategorized { return ValidationError }`.
- **Interview Importance**: 🔥 MUST KNOW. DoS prevention và Business validation.

### 5.4. `internal/handler/transaction_handler.go` (`List`)
- **Responsibility**: Parse các query parameters từ URL, chuyển đổi kiểu dữ liệu (string sang int, string sang boolean) và đóng gói JSON trả về:
  ```json
  {
    "items": [...],
    "pagination": {
      "page": 2,
      "limit": 20,
      "total_items": 45,
      "total_pages": 3
    }
  }
  ```
- **Interview Importance**: ⭐ SHOULD KNOW. Envelope Pagination Response Pattern.

---

## 6. Concrete Runtime Flow

Chi tiết luồng chạy của một request lọc: `GET /api/v1/transactions?type=EXPENSE&uncategorized=true&page=1&limit=10`:

```text
1. Request đến Handler:
   - AuthMiddleware xác thực -> userID = "u111".
   - c.Query("type") -> "EXPENSE".
   - c.Query("uncategorized") -> "true" (parse sang bool: true).
   - c.Query("page") -> "1", c.Query("limit") -> "10".

2. TransactionService:
   - Validate: Không có category_id kèm uncategorized -> Hợp lệ.
   - Validate: type = "EXPENSE" nằm trong ENUM -> Hợp lệ.
   - Tính offset = (1 - 1) * 10 = 0.
   - Chuyển tiếp vào txRepo.List(ctx, filter).

3. PostgresTransactionRepository:
   - Khởi tạo:
     whereClauses = ["user_id = $1"]
     args = ["u111"]
     argIdx = 2
   - Thêm Type:
     whereClauses += "type = $2"
     args += "EXPENSE"
     argIdx = 3
   - Thêm Uncategorized:
     whereClauses += "category_id IS NULL" (Không tốn biến tham số $!)
   - whereSQL trở thành:
     "user_id = $1 AND type = $2 AND category_id IS NULL"
   - Thực thi COUNT:
     QueryRowContext("SELECT COUNT(*) FROM transactions WHERE user_id = $1 AND type = $2 AND category_id IS NULL", "u111", "EXPENSE")
     Scan -> totalCount = 15.
   - Ghép câu lệnh ITEMS:
     "... LIMIT $3 OFFSET $4"
     itemsArgs = ["u111", "EXPENSE", 10, 0]
   - QueryContext và Scan từng dòng vào slice Transactions.

4. Tính toán phản hồi:
   - totalPages = (15 + 10 - 1) / 10 = 24 / 10 = 2 trang.
   - Handler trả về JSON danh sách 10 items kèm pagination metadata.
```

---

## 7. Important Go Concepts

### 7.1. Cắt ghép Slice an toàn với `append`
- Trong hàm `List`:
  ```go
  itemsArgs := append(args, limit, offset)
  ```
- Lưu ý: Không dùng `args = append(args, limit, offset)` nếu bạn còn muốn tái sử dụng slice `args` cho việc khác, vì `append` có thể sửa đổi mảng ngầm bên dưới nếu dung lượng (capacity) còn trống. Tạo một slice mới `itemsArgs` là cách viết phòng thủ an toàn.

### 7.2. Chuẩn hóa chuỗi Boolean trong HTTP Query
- Người dùng có thể truyền `?uncategorized=true`, `?uncategorized=1`, `?uncategorized=TRUE`.
- Trong Go, sử dụng `strconv.ParseBool(c.Query("uncategorized"))` sẽ tự động xử lý chính xác tất cả các biến thể trên (`1, t, T, TRUE, true, True` đều là `true`).

---

## 8. Database / SQL Concepts

### 8.1. Tại sao `user_id = $1` LUÔN LUÔN là điều kiện đầu tiên?
1. **An toàn bảo mật (Zero Trust)**: Mảng `whereClauses` được khởi tạo cứng:
   ```go
   whereClauses := []string{"user_id = $1"}
   ```
   Điều này đảm bảo cho dù người dùng không truyền bất kỳ bộ lọc nào (`StartDate == nil`, `Type == nil`), câu query sinh ra **vẫn luôn có `WHERE user_id = $1`**. Không bao giờ xảy ra tình trạng câu query bị rỗng điều kiện `WHERE` dẫn đến việc quét toàn bộ dữ liệu của tất cả người dùng trong hệ thống!
2. **Tối ưu B-Tree Index**: Tận dụng cột đứng đầu của Composite Index `(user_id, transaction_date DESC)`.

### 8.2. Bản chất của SQL `OFFSET` và giới hạn của nó
- Câu lệnh `LIMIT 20 OFFSET 1000` hoạt động như thế nào trong cơ sở dữ liệu?
- Database phải đọc và duyệt qua **1.020 bản ghi**, sau đó vứt bỏ 1.000 bản ghi đầu tiên và chỉ trả về 20 bản ghi cuối cùng!
- Khi `OFFSET` quá lớn (ví dụ trang thứ 10.000), truy vấn sẽ bị chậm. Tuy nhiên, đối với ứng dụng quản lý tài chính cá nhân nơi một người dùng thường chỉ xem lịch sử trong vòng vài trang đầu (1 - 10 trang), `OFFSET` pagination là giải pháp đơn giản, hiệu quả và cho phép người dùng nhảy đến trang bất kỳ (Random Page Access).

---

## 9. Error Handling

### Xử lý lỗi mâu thuẫn bộ lọc (Filter Conflict)
- Nếu client truyền cả `category_id=uuid` VÀ `uncategorized=true`:
  - Về mặt toán học SQL: `category_id = 'uuid' AND category_id IS NULL` sẽ luôn luôn trả về 0 kết quả (mệnh đề mâu thuẫn).
  - Thay vì âm thầm chạy câu query vô nghĩa tốn tài nguyên DB, tầng Service chủ động chặn lại và trả về lỗi 400 Bad Request rõ ràng:
    `"cannot combine category_id and uncategorized"`.

---

## 10. Testing

### 10.1. Kiểm thử phân trang đầy đủ (Pagination Test)
Trong `internal/service/transaction_service_test.go`:
- Tạo 25 giao dịch giả lập.
- Query trang 1 với `limit = 10` -> Nhận 10 items, `total_pages = 3`, `total_items = 25`.
- Query trang 2 với `limit = 10` -> Nhận 10 items tiếp theo.
- Query trang 3 với `limit = 10` -> Nhận 5 items cuối cùng.
- Query trang 4 với `limit = 10` -> Nhận 0 items, `total_pages = 3`.

### 10.2. Kiểm thử SQL Injection với tham số lọc
- Truyền `StartDate = "2026-01-01; DROP TABLE transactions; --"`.
- Service bắt lỗi định dạng ngày và từ chối ngay lập tức trước khi chạm tới tầng Database.

---

## 11. Beginner Mistakes

### 5 Sai lầm phổ biến của người mới:
1. **Nối chuỗi trực tiếp tham số người dùng vào SQL**: Viết `whereClauses = append(whereClauses, "type = '" + filter.Type + "'")` mở toang cánh cửa cho hacker tấn công SQL Injection.
2. **Không giới hạn `limit` tối đa**: Cho phép client truyền `?limit=1000000` làm cạn kiệt RAM server.
3. **Tính `totalPages` bằng `float64`**: Viết `int(math.Ceil(float64(total) / float64(limit)))` vừa dài dòng, vừa tốn chi phí ép kiểu và dễ dính lỗi làm tròn số thực.
4. **Chỉ sắp xếp theo `transaction_date`**: Dẫn đến lỗi xáo trộn thứ tự các bản ghi trong cùng một ngày giữa các trang.
5. **Đánh sai thứ tự tham số `$1, $2`**: Tăng biến đếm `argIdx` không đồng bộ với việc append vào slice `args`, khiến câu lệnh SQL truyền nhầm dữ liệu cột này sang cột khác.

### 3 Nguy cơ sinh bug thực tế:
1. **Lỗi `argIdx` khi có điều kiện không cần tham số**: Điều kiện `"category_id IS NULL"` không cần tham số giữ chỗ `$`. Nếu lập trình viên vô tình gọi `argIdx++`, các tham số phía sau sẽ bị lệch số thứ tự (ví dụ: `$1, $2, $4` thiếu `$3`), khiến PostgreSQL báo lỗi cú pháp.
2. **Lỗi `start_date > end_date`**: Không kiểm tra ngày bắt đầu lớn hơn ngày kết thúc khiến câu query luôn trả về mảng rỗng mà không báo cho người dùng biết họ đã nhập sai khoảng thời gian.
3. **Không đóng `rows.Close()` trong câu query danh sách**: Gây treo connection pool.

### 2 Cái bẫy phỏng vấn (Interview Traps):
1. *"Khi phân trang với `OFFSET`, điều gì xảy ra nếu có một giao dịch mới được chèn vào trong lúc người dùng đang chuyển từ trang 1 sang trang 2?"* -> **Bẫy**: Bản ghi mới chèn vào đầu bảng sẽ đẩy toàn bộ dữ liệu lùi xuống 1 vị trí. Khi người dùng load trang 2, bản ghi cuối cùng của trang 1 sẽ bị lặp lại ở đầu trang 2. (Cách khắc phục triệt để trong các hệ thống feed lớn là dùng Keyset/Cursor-based Pagination).
2. *"Tại sao trong câu lệnh COUNT(*) ta không cần mệnh đề `ORDER BY`?"* -> **Bẫy**: `ORDER BY` trong `COUNT(*)` là hoàn toàn vô nghĩa và làm chậm câu truy vấn một cách lãng phí vì database chỉ cần đếm số lượng dòng, không cần biết dòng nào đứng trước dòng nào.

---

## 12. Code Reading Exercises

### Bài tập 1: Dự đoán kết quả
Đoạn code xây dựng SQL động sau có lỗi gì?
```go
clauses := []string{"1=1"}
args := []interface{}{}
if name != "" {
    clauses = append(clauses, "name = $1")
    args = append(args, name)
}
if age > 0 {
    clauses = append(clauses, "age = $2")
    args = append(args, age)
}
```
*Gợi ý suy nghĩ*: Điều gì xảy ra nếu `name == ""` nhưng `age > 0`? Câu lệnh SQL sinh ra sẽ là gì và PostgreSQL sẽ phản ứng ra sao?

### Bài tập 2: Tối ưu công thức toán
Viết lại biểu thức sau bằng toán số nguyên thuần túy trong Go mà không dùng gói `math`:
```go
pages := int(math.Ceil(float64(totalItems) / float64(pageSize)))
```

---

## 13. Active Recall

### Câu hỏi ôn tập
1. Tại sao việc quản lý chỉ mục tham số động (`argIdx`) trong PostgreSQL đòi hỏi sự cẩn trọng cao hơn so với MySQL (dùng dấu `?`)?
2. Công thức chia trần nguyên (Integer Ceiling Division) trong Go được viết như thế nào?
3. Tại sao FinTrack phải sắp xếp theo cả 3 tiêu chí: `transaction_date DESC, created_at DESC, id DESC`?
4. Điều gì xảy ra nếu client truyền `uncategorized=true` và `category_id=...` cùng lúc?
5. Tại sao câu query `COUNT(*)` không được chứa `LIMIT` và `OFFSET`?

---

### Model Answers

#### 1. Quản lý chỉ mục tham số trong PostgreSQL
- **English**: PostgreSQL uses numbered positional parameters (`$1, $2, $3`), unlike MySQL which uses unnumbered `?`. The placeholder numbers must exactly match their corresponding 1-based index in the arguments slice passed to the driver, requiring careful tracking of `argIdx`.
- **Vietnamese**: PostgreSQL sử dụng các tham số có đánh số vị trí cụ thể (`$1, $2, $3`), khác với MySQL chỉ dùng dấu `?`. Số hiệu của ký tự giữ chỗ bắt buộc phải khớp chính xác với chỉ mục trong mảng đối số truyền vào, đòi hỏi phải theo dõi biến `argIdx` cực kỳ cẩn thận.

#### 2. Công thức chia trần nguyên
- **English**: `totalPages = (totalItems + pageSize - 1) / pageSize`.
- **Vietnamese**: `totalPages = (totalItems + pageSize - 1) / pageSize`.

#### 3. Tại sao sắp xếp 3 tầng?
- **English**: Multiple transactions can occur on the same calendar date, and potentially at the same millisecond timestamp. Adding the strictly unique primary key `id DESC` as the final tie-breaker guarantees deterministic ordering, preventing records from jumping between pages.
- **Vietnamese**: Nhiều giao dịch có thể diễn ra trong cùng một ngày, thậm chí cùng một mili-giây. Thêm khóa chính duy nhất tuyệt đối `id DESC` làm chốt chặn cuối cùng đảm bảo thứ tự sắp xếp đơn định 100%, ngăn các dòng dữ liệu bị nhảy cóc giữa các trang.

#### 4. Kết hợp `uncategorized` và `category_id`
- **English**: It is rejected with a 400 Validation Error because they are mutually exclusive criteria (`category_id = $1 AND category_id IS NULL` is contradictory).
- **Vietnamese**: Bị từ chối với lỗi 400 Validation Error vì đây là hai tiêu chí loại trừ lẫn nhau (vừa đòi bằng một ID cụ thể vừa đòi phải là NULL là điều mâu thuẫn).

#### 5. Tại sao COUNT(*) không chứa LIMIT/OFFSET?
- **English**: `COUNT(*)` calculates the total number of all matching records in the entire dataset. Including `LIMIT` or `OFFSET` would distort the count, returning only the page size instead of the true total.
- **Vietnamese**: `COUNT(*)` dùng để tính tổng số lượng bản ghi thỏa mãn điều kiện trên toàn bộ tập dữ liệu. Nếu cho `LIMIT` hoặc `OFFSET` vào, nó sẽ chỉ đếm số bản ghi trên trang hiện tại thay vì tổng số thực tế.

---

## 14. Fresher Go Backend Interview Questions

### Question 1
- **English**: How do you safely construct dynamic SQL queries with variable conditions in Go?
- **Vietnamese**: Bạn xây dựng câu truy vấn SQL động với các điều kiện thay đổi một cách an toàn trong Go như thế nào?
- **Model Answer (EN)**: Maintain a slice of WHERE clause strings and a corresponding slice of argument values. Incrementally append parameterized placeholders (`fmt.Sprintf("col = $%d", argIdx)`) and their values, then join the clauses with `strings.Join(clauses, " AND ")`. Never interpolate user input directly into SQL strings.
- **Model Answer (VI)**: Duy trì một slice chứa các chuỗi điều kiện WHERE và một slice chứa các giá trị đối số tương ứng. Thêm dần các ký tự giữ chỗ có tham số hóa (`fmt.Sprintf("col = $%d", argIdx)`) và giá trị của chúng, sau đó nối các mệnh đề lại bằng `strings.Join(clauses, " AND ")`. Tuyệt đối không bao giờ nối trực tiếp biến của người dùng vào chuỗi SQL.
- **Explanation**: Kỹ thuật xây dựng câu lệnh SQL động chuẩn mực.
- **Follow-up**: Have you used any query builder libraries in Go? (Answer: Libraries like `squirrel` or ORMs like `GORM`, but writing clean standard library code avoids external dependencies).

### Question 2
- **English**: What is Deterministic Sorting in database pagination and why is it critical?
- **Vietnamese**: Sắp xếp đơn định (Deterministic Sorting) trong phân trang cơ sở dữ liệu là gì và tại sao nó lại quan trọng?
- **Model Answer (EN)**: Deterministic sorting ensures that query results are returned in the exact same deterministic order across multiple executions. Without unique tie-breaking columns (like the primary key `id`), rows with identical sort values can shift positions between requests, causing missing or duplicate records when paging.
- **Model Answer (VI)**: Sắp xếp đơn định đảm bảo kết quả truy vấn luôn được trả về theo một thứ tự duy nhất cố định qua các lần thực thi khác nhau. Nếu không có cột chốt chặn duy nhất (như khóa chính `id`), các dòng có giá trị sắp xếp giống nhau có thể bị tráo đổi vị trí giữa các request, gây lặp hoặc sót bản ghi khi chuyển trang.
- **Explanation**: Tránh bug hiển thị dữ liệu phân trang.
- **Follow-up**: What is the tie-breaker column in FinTrack? (Answer: The primary key `id DESC`).

### Question 3
- **English**: What is the trade-off between Offset-based pagination and Cursor-based pagination?
- **Vietnamese**: Đánh đổi giữa phân trang Offset-based và Cursor-based là gì?
- **Model Answer (EN)**: Offset pagination allows direct navigation to arbitrary pages (e.g. jumping to page 5) and is easy to implement, but suffers from performance degradation at high offsets ($O(N)$ row skips) and pagination drift under high write loads. Cursor pagination is extremely fast ($O(1)$ index seek) and stable against concurrent writes, but cannot jump to arbitrary pages.
- **Model Answer (VI)**: Phân trang Offset cho phép nhảy đến trang bất kỳ (ví dụ: nhảy tới trang 5) và rất dễ cài đặt, nhưng bị giảm hiệu năng khi offset lớn và dễ bị trôi dữ liệu khi có nhiều bản ghi mới được ghi liên tục. Phân trang Cursor chạy cực nhanh và ổn định trước các thao tác ghi đồng thời, nhưng không thể nhảy đến một trang bất kỳ.
- **Explanation**: Hiểu sâu về các chiến lược phân trang trong thiết kế hệ thống.
- **Follow-up**: Why is Offset pagination suitable for FinTrack? (Answer: Users typically view recent transactions within the first few pages, where offset overhead is negligible).

### Question 4
- **English**: Why does `TransactionService` cap the query `limit` at 100?
- **Vietnamese**: Tại sao `TransactionService` lại giới hạn `limit` tối đa là 100?
- **Model Answer (EN)**: To protect against Denial-of-Service (DoS) and memory exhaustion. If an attacker or buggy client requests `?limit=1000000`, the server would attempt to allocate memory for millions of struct objects, potentially triggering the Go runtime Out-Of-Memory (OOM) killer.
- **Model Answer (VI)**: Để chống tấn công từ chối dịch vụ (DoS) và cạn kiệt bộ nhớ RAM. Nếu kẻ tấn công hoặc client bị lỗi gửi request `?limit=1000000`, server sẽ cố gắng cấp phát bộ nhớ cho hàng triệu đối tượng struct, dễ dẫn tới việc tiến trình Go bị hệ điều hành tiêu diệt do hết RAM (OOM killer).
- **Explanation**: Thiết kế hệ thống phòng thủ (Defensive API Design).
- **Follow-up**: What status code is returned if `limit > 100`? (Answer: 400 Bad Request with a ValidationError).

### Question 5
- **English**: How does FinTrack implement filtering for uncategorized transactions?
- **Vietnamese**: FinTrack cài đặt tính năng lọc các giao dịch chưa phân loại như thế nào?
- **Model Answer (EN)**: When `uncategorized=true` is passed, the repository appends `"category_id IS NULL"` to the WHERE clauses without appending an argument to the `args` slice or incrementing `argIdx`.
- **Model Answer (VI)**: Khi cờ `uncategorized=true` được truyền lên, repository sẽ thêm mệnh đề `"category_id IS NULL"` vào danh sách điều kiện WHERE mà không cần thêm phần tử vào slice `args` hay tăng biến `argIdx`.
- **Explanation**: Xử lý điều kiện SQL không có tham số giữ chỗ.
- **Follow-up**: Can you use `category_id = NULL` in SQL? (Answer: No, in SQL comparisons with NULL must use `IS NULL` or `IS NOT NULL`).

### Question 6
- **English**: Why do we execute two separate SQL queries for paginated listings (`COUNT` and `SELECT`)?
- **Vietnamese**: Tại sao chúng ta lại thực thi hai câu truy vấn SQL riêng biệt khi phân trang (`COUNT` và `SELECT`)?
- **Model Answer (EN)**: The `COUNT(*)` query computes the total number of matching items needed for pagination metadata (`total_pages`, `total_items`). The `SELECT` query retrieves only the slice of rows for the requested page using `LIMIT` and `OFFSET`.
- **Model Answer (VI)**: Câu query `COUNT(*)` tính toán tổng số lượng bản ghi thỏa mãn điều kiện để cung cấp metadata phân trang (`total_pages`, `total_items`). Câu query `SELECT` chỉ lấy ra tập hợp các dòng của trang được yêu cầu thông qua `LIMIT` và `OFFSET`.
- **Explanation**: Chuẩn thiết kế Pagination Envelope trong REST API.
- **Follow-up**: Could we use window function `COUNT(*) OVER()` instead? (Answer: Yes, but it forces scanning all matching rows even when limiting, which can be slower on large datasets).

### Question 7
- **English**: How does the variadic `args...` syntax work when passing arguments to `QueryContext`?
- **Vietnamese**: Cú pháp variadic `args...` hoạt động như thế nào khi truyền tham số vào `QueryContext`?
- **Model Answer (EN)**: `args...` unpacks a slice of `[]interface{}` into individual positional arguments accepted by `QueryContext(ctx, query, args ...any)`.
- **Model Answer (VI)**: Cú pháp `args...` thực hiện việc bung (unpack) một slice kiểu `[]interface{}` thành các tham số vị trí rời rạc theo đúng chữ ký của hàm `QueryContext(ctx, query, args ...any)`.
- **Explanation**: Cú pháp Go cơ bản nhưng quan trọng khi gọi thư viện chuẩn.
- **Follow-up**: Can you pass a slice of concrete type (e.g. `[]string`) using `...` into `...any`? (Answer: No, Go requires explicitly allocating an `[]any` slice).

### Question 8
- **English**: Why does FinTrack validate that `startDate <= endDate` at the Service layer?
- **Vietnamese**: Tại sao FinTrack lại xác thực `startDate <= endDate` ở tầng Service?
- **Model Answer (EN)**: If `startDate > endDate`, the SQL query `transaction_date >= start AND transaction_date <= end` would mathematically return 0 rows. Rejecting it early at the service layer prevents wasted database execution and alerts the client to their inverted input parameters.
- **Model Answer (VI)**: Nếu `startDate > endDate`, câu query SQL về mặt logic toán học sẽ luôn trả về 0 dòng. Chặn lại từ sớm ở tầng Service giúp tránh lãng phí tài nguyên database và thông báo rõ ràng cho client biết họ đã nhập ngược tham số.
- **Explanation**: Tiết kiệm tài nguyên và nâng cao trải nghiệm lập trình viên (Developer Experience).
- **Follow-up**: What package is used to parse the date strings? (Answer: Standard library `time` with `time.Parse`).

### Question 9
- **English**: How does the index `(user_id, transaction_date DESC)` accelerate both COUNT and SELECT queries?
- **Vietnamese**: Chỉ mục `(user_id, transaction_date DESC)` tăng tốc cho cả câu lệnh COUNT lẫn SELECT như thế nào?
- **Model Answer (EN)**: It allows an Index Scan filtering by `user_id` and date range directly in the B-Tree leaf pages without scanning the heap table. For `SELECT`, the index already stores the rows in the exact descending order requested, eliminating an explicit sort operation.
- **Model Answer (VI)**: Nó cho phép thực hiện Index Scan để lọc theo `user_id` và khoảng ngày trực tiếp trên các trang lá của cây B-Tree mà không cần quét toàn bộ bảng heap. Với câu `SELECT`, chỉ mục đã lưu sẵn các dòng theo đúng thứ tự giảm dần, giúp loại bỏ hoàn toàn thao tác sort tốn kém.
- **Explanation**: Tương tác giữa B-Tree Index và SQL Engine.
- **Follow-up**: What EXPLAIN command shows if an index was used? (Answer: `EXPLAIN ANALYZE <query>`).

### Question 10
- **English**: Why is `whereClauses` initialized with `user_id = $1` instead of an empty slice?
- **Vietnamese**: Tại sao `whereClauses` được khởi tạo sẵn với `user_id = $1` thay vì một slice rỗng?
- **Model Answer (EN)**: Initializing with `user_id = $1` enforces hard multi-tenant isolation by default. Even if all optional filters are omitted, the query is guaranteed to include the tenant boundary, preventing catastrophic accidental full-table leaks.
- **Model Answer (VI)**: Khởi tạo sẵn với `user_id = $1` giúp thực thi triệt để nguyên tắc cô lập người dùng theo mặc định. Kể cả khi tất cả các bộ lọc tùy chọn đều bị bỏ qua, câu truy vấn vẫn được đảm bảo luôn bị giới hạn trong phạm vi của user đó, ngăn ngừa sự cố lộ toàn bộ dữ liệu hệ thống.
- **Explanation**: Nguyên tắc Secure By Default trong kỹ thuật phần mềm.
- **Follow-up**: What is the initial value of `argIdx`? (Answer: 2, because `$1` is already occupied by `user_id`).

---

## 15. Knowledge Connection
- **Kế thừa từ Issue #7**: Lấy danh sách các giao dịch được tạo ra từ Issue #7 với định dạng `int64` cho tiền tệ và chuỗi `YYYY-MM-DD` cho ngày.
- **Chuẩn bị cho Issue #9 (Budget Tracking)**: Khi người dùng muốn xem các giao dịch chi tiêu cấu thành nên ngân sách của một danh mục, hệ thống sẽ sử dụng bộ lọc `category_id` và khoảng ngày `start_date` / `end_date` đã xây dựng ở Issue #8 này.
- **Chuẩn bị cho Issue #10 (Analytics)**: Tận dụng các nguyên lý lọc theo khoảng thời gian để tổng hợp số liệu thu chi theo tháng.

---

## 16. Learning Checkpoint

### I Must Understand
1. Cách xây dựng câu truy vấn SQL động an toàn với `whereClauses` và `args []interface{}`.
2. Tại sao câu truy vấn luôn phải bắt đầu bằng `user_id = $1`.
3. Công thức chia trần số nguyên `(total + limit - 1) / limit` để tính số trang.
4. Tầm quan trọng của Deterministic Sorting 3 tầng (`transaction_date DESC, created_at DESC, id DESC`).
5. Giới hạn `limit <= 100` để phòng chống tấn công DoS / rò rỉ bộ nhớ.

### I Should Be Able To Code
1. Viết một hàm xây dựng câu lệnh SQL có điều kiện tham số động và biến đếm `argIdx`.
2. Viết logic tính toán phân trang Offset chuẩn trong Go.
3. Viết validation kiểm tra loại trừ lẫn nhau giữa hai trường tham số.

### I Should Be Able To Explain In An Interview
1. Tại sao không được nối chuỗi biến của người dùng trực tiếp vào câu lệnh SQL?
2. Ưu và nhược điểm của Offset-based Pagination so với Cursor-based Pagination.
3. Hiện tượng lặp hoặc sót bản ghi khi phân trang (Pagination Hopping) là gì và cách giải quyết?

### Advanced Topics I Do Not Need Yet
- Cursor-based / Keyset Pagination với toán tử so sánh tuple: `WHERE (transaction_date, id) < ($last_date, $last_id)`.
- Full-text Search trong PostgreSQL bằng `tsvector` và `tsquery` để tìm kiếm giao dịch theo từ khóa trong `description`.
- Phân vùng bảng (Table Partitioning) theo năm/tháng cho bảng `transactions` khi dữ liệu đạt hàng chục triệu dòng.
