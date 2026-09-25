# Issue #9 — Monthly Budgets, Upsert (`ON CONFLICT`), `LEFT JOIN` Aggregation & Date Boundaries

## 1. Learning Objectives
Sau khi hoàn thành bài học này, bạn sẽ nắm vững:
- Kỹ thuật Upsert (Insert-or-Update) nguyên tử trong PostgreSQL sử dụng `ON CONFLICT (...) DO UPDATE` và bảng ảo `EXCLUDED`.
- Thiết kế tính toán số tiền đã chi tiêu động (Dynamic Spent Calculation) bằng `LEFT JOIN`, `GROUP BY`, `COALESCE(SUM(...), 0)` thay vì lưu trường dư thừa (denormalization).
- Thuật toán xác định biên ngày tháng chính xác bằng khoảng nửa mở `[start, next_start)` kết hợp `time.Date` và `start.AddDate(0, 1, 0)` để tự động xử lý năm nhuận và tháng có 28/29/30/31 ngày.
- Lý do tại sao `budgets` sử dụng `ON DELETE CASCADE` cho Category trong khi `transactions` lại sử dụng `ON DELETE SET NULL`.
- Nghiệp vụ ràng buộc: Ngân sách chỉ được phép áp dụng cho danh mục CHI TIÊU (`EXPENSE`), tuyệt đối không áp dụng cho danh mục THU NHẬP (`INCOME`).

---

## 2. What Problem Does This Issue Solve?
Trong quản lý tài chính, người dùng cần đặt hạn mức chi tiêu cho từng danh mục theo từng tháng (ví dụ: "Tháng 03/2026, ngân sách ăn uống là 5.000.000 VND"):
1. **Tránh lỗi trùng lặp khi đặt lại hạn mức (Idempotent Configuration)**: Nếu người dùng đã đặt ngân sách 5 triệu cho tháng 3, sau đó họ muốn nâng lên 6 triệu, API không được báo lỗi trùng khóa `409 Conflict`, mà phải tự động cập nhật số tiền mới (Upsert).
2. **Sai số khi lưu trường `spent` tĩnh**: Nếu mỗi khi có giao dịch mới, ta lại cộng tiền vào cột `spent` trong bảng `budgets`, thì khi người dùng sửa ngày giao dịch, đổi danh mục, hoặc xóa giao dịch, trường `spent` sẽ lập tức bị sai lệch (data drift / sync bugs). FinTrack giải quyết triệt để vấn đề này bằng cách **tính toán `spent` động ngay trong câu truy vấn SQL**.
3. **Bài toán số ngày trong tháng**: Tháng 2 có 28 hoặc 29 ngày (năm nhuận), các tháng khác có 30 hoặc 31 ngày. Việc hardcode ngày cuối tháng (`2026-02-28`) rất dễ gây bug bỏ sót giao dịch ngày 29/02.

---

## 3. Prerequisite Go Knowledge

### 3.1. Go `time.Date` và `AddDate(years, months, days)`
- **Khái niệm**: Go cung cấp hàm `time.Date(year, month, day, hour, min, sec, nsec, loc)` để khởi tạo một mốc thời gian chính xác.
- Phương thức `t.AddDate(0, 1, 0)` cộng đúng 1 tháng theo lịch và tự động tính toán số ngày chính xác cho mọi tháng, bao gồm cả năm nhuận.
- **Ví dụ cơ bản**:
```go
start := time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC)
nextStart := start.AddDate(0, 1, 0) // Tự động thành: 2024-03-01 (năm nhuận 2024 có 29 ngày)
```
- **Kết nối FinTrack**: Trong `ListByMonth` và `GetSpent`, ta dùng `start.Format("2006-01-02")` và `nextStart.Format("2006-01-02")` để tạo hai mốc chặn đầu và chặn đuôi cho truy vấn SQL.

### 3.2. Khoảng nửa mở `[start, next_start)` trong SQL
- Thay vì dùng `BETWEEN '2026-03-01' AND '2026-03-31'` (dễ sai khi tính đến giờ phút giây hoặc tháng có 30 ngày), ta dùng:
```sql
transaction_date >= '2026-03-01' AND transaction_date < '2026-04-01'
```
- Bất kỳ ngày nào trong tháng 3 đều thỏa mãn: lớn hơn hoặc bằng ngày mùng 1 tháng 3 VÀ nhỏ hơn ngày mùng 1 tháng 4.

---

## 4. Architecture

Kiến trúc nghiệp vụ Ngân sách và tính toán số tiền đã chi:

```text
HTTP Request: PUT /api/v1/budgets
Body: { "category_id": "c111", "year": 2026, "month": 3, "amount": 5000000 }
       │
  (AuthMiddleware) ──► userID: "u111"
       │
       ▼
[Handler: BudgetHandler.Upsert]
  • Bind JSON sang CreateBudgetRequest
       │
       ▼
[Service: BudgetService.UpsertBudget]
  • Validate: amount > 0, 2000 <= year <= 2100, 1 <= month <= 12
  • Kiểm tra danh mục qua catRepo.FindByID(ctx, catID, userID)
  • BẮT BUỘC: cat.Type == CategoryTypeExpense (Không cho đặt ngân sách cho danh mục Thu nhập)
  • Gọi budgetRepo.Upsert(ctx, budget)
  • Gọi budgetRepo.GetSpent(ctx, userID, catID, year, month) để tính spent hiện tại
       │
       ▼
[Repository: PostgresBudgetRepository.Upsert]
  • INSERT INTO budgets (...)
  • ON CONFLICT (user_id, category_id, year, month)
  • DO UPDATE SET amount = EXCLUDED.amount, updated_at = EXCLUDED.updated_at
  • RETURNING id, created_at, updated_at
       │
       ▼
[PostgreSQL Database]
  • Cập nhật nếu đã có, tạo mới nếu chưa có trong đúng 1 atomic operation.
```

---

## 5. File-by-File Analysis

### 5.1. `internal/domain/budget.go`
- **Struct Analysis**:
  ```go
  type Budget struct {
      ID           uuid.UUID `json:"id"`
      UserID       uuid.UUID `json:"user_id"`
      CategoryID   uuid.UUID `json:"category_id"`
      CategoryName string    `json:"category_name,omitempty"` // Lấy từ JOIN categories
      Amount       int64     `json:"amount"`                  // Hạn mức tối đa
      Spent        int64     `json:"spent"`                   // Số tiền đã chi (tính toán động)
      Year         int       `json:"year"`
      Month        int       `json:"month"`
      CreatedAt    time.Time `json:"created_at"`
      UpdatedAt    time.Time `json:"updated_at"`
  }
  ```
- **Điểm đặc biệt**: Trường `Spent` và `CategoryName` không phải là các cột trong bảng `budgets`! Chúng là **Computed Fields** (trường được tính toán động khi truy vấn).
- **Interview Importance**: 🔥 MUST KNOW. Phân biệt Stored Columns vs Computed Fields.

### 5.2. `migrations/000003_create_budgets.up.sql`
- **DDL & Constraints Analysis**:
  ```sql
  CREATE TABLE IF NOT EXISTS budgets (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
      amount BIGINT NOT NULL CHECK (amount > 0),
      year INT NOT NULL CHECK (year >= 2000 AND year <= 2100),
      month INT NOT NULL CHECK (month >= 1 AND month <= 12),
      created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      CONSTRAINT uq_budgets_user_category_year_month UNIQUE (user_id, category_id, year, month)
  );
  ```
- **Tại sao ở đây dùng `ON DELETE CASCADE` cho Category?**:
  - Đối với `transactions`, khi xóa danh mục, ta muốn giữ lại tiền bạc nên dùng `ON DELETE SET NULL`.
  - Nhưng đối với `budgets`, ngân sách là hạn mức đặt *riêng cho danh mục đó*. Nếu danh mục "Cà phê" bị xóa, thì "Ngân sách cà phê" không còn bất kỳ ý nghĩa tồn tại nào! Do đó, `ON DELETE CASCADE` là lựa chọn chính xác về mặt mô hình hóa dữ liệu (Domain Modeling).
- **Unique Constraint 4 cột**: `uq_budgets_user_category_year_month` là điều kiện tiên quyết để câu lệnh `ON CONFLICT` hoạt động.
- **Interview Importance**: 🔥 MUST KNOW. Tại sao Transaction dùng SET NULL mà Budget dùng CASCADE?

### 5.3. `internal/repository/postgres_budget_repository.go`
- **Kỹ thuật Upsert với `EXCLUDED`**:
  ```sql
  INSERT INTO budgets (id, user_id, category_id, amount, year, month, created_at, updated_at)
  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
  ON CONFLICT (user_id, category_id, year, month)
  DO UPDATE SET
      amount = EXCLUDED.amount,
      updated_at = EXCLUDED.updated_at
  RETURNING id, created_at, updated_at
  ```
  - `EXCLUDED` là một bảng ảo đặc biệt do PostgreSQL cung cấp trong mệnh đề `ON CONFLICT`. Nó chứa các giá trị mới mà bạn vừa định chèn vào (tương ứng với `$4` và `$8`).
- **Kỹ thuật `LEFT JOIN` và `COALESCE` trong `ListByMonth`**:
  ```sql
  SELECT 
      b.id, b.user_id, b.category_id, c.name AS category_name,
      b.amount, b.year, b.month,
      COALESCE(SUM(t.amount), 0) AS spent,
      b.created_at, b.updated_at
  FROM budgets b
  JOIN categories c ON c.id = b.category_id
  LEFT JOIN transactions t ON t.category_id = b.category_id
      AND t.user_id = b.user_id
      AND t.type = 'EXPENSE'
      AND t.transaction_date >= $4
      AND t.transaction_date < $5
  WHERE b.user_id = $1 AND b.year = $2 AND b.month = $3
  GROUP BY b.id, b.user_id, b.category_id, c.name, b.amount, b.year, b.month, b.created_at, b.updated_at
  ORDER BY c.name ASC, b.id ASC
  ```
  - **Tại sao phải `LEFT JOIN`?**: Nếu một ngân sách vừa tạo chưa có bất kỳ chi tiêu nào trong tháng, `INNER JOIN` sẽ loại bỏ ngân sách đó khỏi kết quả! `LEFT JOIN` giữ lại ngân sách và trả về dòng với các cột của `transactions` là `NULL`.
  - **`COALESCE(SUM(t.amount), 0)`**: Hàm `SUM()` trên các giá trị `NULL` sẽ trả về `NULL`. `COALESCE` thay thế `NULL` bằng số `0`, giúp scan trực tiếp vào kiểu `int64` trong Go mà không bị lỗi.
- **Interview Importance**: 🔥 MUST KNOW. Kỹ thuật `LEFT JOIN`, `COALESCE`, và `EXCLUDED` trong PostgreSQL.

### 5.4. `internal/service/budget_service.go`
- **Validation Rules**:
  - `year` trong khoảng [2000, 2100].
  - `month` trong khoảng [1, 12].
  - Kiểm tra `cat.Type != domain.CategoryTypeExpense`:
    ```go
    if cat.Type != domain.CategoryTypeExpense {
        return nil, domain.ValidationError{
            Field: "category_id",
            Message: "budgets can only be set on expense categories",
        }
    }
    ```
    Không thể đặt ngân sách cho tiền Lương hay Thưởng (INCOME)!
- **Interview Importance**: ⭐ SHOULD KNOW. Business domain boundary checks.

---

## 6. Concrete Runtime Flow

Chi tiết luồng chạy của một request `PUT /api/v1/budgets`:

```text
1. Client gửi PUT /api/v1/budgets:
   Body: { "category_id": "c111", "amount": 3000000, "year": 2026, "month": 3 }
   Header: Authorization: Bearer <token-u111>

2. AuthMiddleware:
   - Xác thực token -> userID: "u111".

3. BudgetHandler.Upsert:
   - Bind JSON -> UpsertBudgetRequest.
   - Gọi budgetService.UpsertBudget(ctx, input).

4. BudgetService.UpsertBudget:
   - Kiểm tra: amount = 3000000 > 0 (Hợp lệ).
   - Kiểm tra: 2026 nằm trong [2000, 2100], month = 3 nằm trong [1, 12] (Hợp lệ).
   - Gọi catRepo.FindByID(ctx, "c111", "u111"):
     • Tìm thấy danh mục "Tiền ăn" thuộc về user "u111".
     • Kiểm tra: cat.Type == "EXPENSE" (Hợp lệ).
   - Gọi budgetRepo.Upsert(ctx, budget):
     • PostgreSQL kiểm tra constraint UNIQUE(user_id, category_id, year, month).
     • NẾU ĐÃ CÓ: Cập nhật amount thành 3000000, cập nhật updated_at.
     • NẾU CHƯA CÓ: Chèn dòng mới.
     • Trả về ID, created_at, updated_at.
   - Gọi budgetRepo.GetSpent(ctx, "u111", "c111", 2026, 3):
     • monthStart = "2026-03-01", nextMonthStart = "2026-04-01".
     • SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE ...
     • Trả về spent = 1250000 VND.
   - Gán budget.Spent = 1250000 và budget.CategoryName = "Tiền ăn".

5. Phản hồi HTTP:
   - Handler trả về HTTP 200 OK với đầy đủ thông tin:
     {
       "id": "b111-...",
       "category_id": "c111-...",
       "category_name": "Tiền ăn",
       "amount": 3000000,
       "spent": 1250000,
       "year": 2026,
       "month": 3
     }
```

---

## 7. Important Go Concepts

### 7.1. Chuyển đổi số nguyên sang kiểu `time.Month`
- Trong Go, `time.Month` là một custom type định nghĩa từ `int`:
  ```go
  type Month int
  const (
      January Month = 1 + iota
      February
      // ...
  )
  ```
- Khi truyền biến `month int` vào hàm `time.Date`, bạn cần ép kiểu tường minh:
  ```go
  time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
  ```

### 7.2. Safe Integer Scanning với SQL `COALESCE`
- Nếu bạn scan kết quả của một hàm tổng hợp (`SUM`) trực tiếp vào biến `var spent int64`:
  - Khi không có dòng nào khớp, SQL trả về `NULL`.
  - Go driver sẽ báo lỗi: `cannot scan NULL into *int64`.
  - Hàm `COALESCE(SUM(...), 0)` ở tầng SQL chuyển `NULL` thành `0`, giúp lệnh `Scan(&spent)` luôn an toàn 100%.

---

## 8. Database / SQL Concepts

### 8.1. Cơ chế hoạt động của `ON CONFLICT (...) DO UPDATE`
1. Khi câu lệnh `INSERT` được gửi tới PostgreSQL, engine kiểm tra trên chỉ mục duy nhất (Unique Index) tương ứng với danh sách cột trong mệnh đề `ON CONFLICT (user_id, category_id, year, month)`.
2. Nếu không có xung đột, bản ghi mới được chèn bình thường.
3. Nếu phát hiện vi phạm Unique, PostgreSQL hủy bỏ thao tác chèn và ngay lập tức chuyển sang thực thi mệnh đề `DO UPDATE`.
4. Toàn bộ quá trình này diễn ra **nguyên tử (Atomic)** trong nội bộ database engine, không có khoảng cách thời gian giữa bước kiểm tra và bước ghi (loại bỏ hoàn toàn Race Condition so với việc viết code: `SELECT` kiểm tra trước rồi mới `INSERT` hoặc `UPDATE`).

### 8.2. Pseudo-table `EXCLUDED`
- `EXCLUDED` đại diện cho các giá trị mà câu lệnh `INSERT` dự định đưa vào nhưng bị chặn lại do xung đột.
- `DO UPDATE SET amount = EXCLUDED.amount` nghĩa là: *"Lấy giá trị amount mới mà client vừa gửi lên để ghi đè vào cột amount của dòng đang có sẵn trong database"*.

---

## 9. Error Handling

### Xử lý lỗi nghiệp vụ vs Lỗi cơ sở dữ liệu:
- Khi người dùng gửi một `category_id` không tồn tại, tầng Service kiểm tra bằng `catRepo.FindByID` và trả về `ValidationError{Field: "category_id"}`.
- Nếu không kiểm tra ở Service mà để SQL tự báo lỗi, PostgreSQL sẽ bắn ra lỗi khóa ngoại `23503` (foreign_key_violation). Bắt lỗi sớm ở tầng Service giúp ứng dụng trả về thông báo rõ ràng: `"category not found or belongs to another user"` thay vì lỗi database thô.

---

## 10. Testing

### 10.1. Kiểm thử tính toán Spent động (Dynamic Spent Calculation Test)
Trong `internal/service/budget_service_test.go`:
- Tạo ngân sách cho tháng 3/2026: Amount = 1.000.000 VND.
- Tạo 2 giao dịch chi tiêu trong tháng 3: 200.000 VND và 300.000 VND.
- Tạo 1 giao dịch thu nhập (INCOME) trong tháng 3: 500.000 VND.
- Tạo 1 giao dịch chi tiêu vào tháng 4 (ngày 01/04/2026): 400.000 VND.
- Gọi `ListBudgetsByMonth`:
  - Khẳng định `Spent` chỉ bằng **500.000 VND** (200k + 300k).
  - Giao dịch INCOME không được cộng vào spent.
  - Giao dịch tháng 4 không bị tính lẹm vào tháng 3.

### 10.2. Kiểm thử Upsert ghi đè hạn mức
- Gọi `UpsertBudget` lần 1 với Amount = 2.000.000 VND.
- Gọi `UpsertBudget` lần 2 với Amount = 3.500.000 VND.
- Khẳng định tổng số bản ghi trong database vẫn chỉ là 1, và `amount` đã được cập nhật thành 3.500.000 VND.

---

## 11. Beginner Mistakes

### 5 Sai lầm phổ biến của người mới:
1. **Lưu cột `spent` tĩnh trong bảng `budgets`**: Dẫn tới việc số liệu bị lệch khi người dùng sửa hoặc xóa giao dịch cũ.
2. **Dùng `BETWEEN` thay vì khoảng nửa mở `[start, next_start)`**: Khiến việc tính toán ngày cuối tháng bị phụ thuộc vào số ngày 28/29/30/31 của từng tháng cụ thể.
3. **Dùng `INNER JOIN` khi lấy danh sách ngân sách kèm chi tiêu**: Khiến những danh mục chưa phát sinh chi tiêu nào trong tháng bị biến mất khỏi màn hình ngân sách của người dùng.
4. **Quên bọc `COALESCE` ngoài `SUM()`**: Dẫn tới lỗi scan NULL vào biến số nguyên trong Go khi danh mục chưa có giao dịch nào.
5. **Cấu hình `ON DELETE SET NULL` cho Category trong bảng `budgets`**: Cột `category_id` trong `budgets` là `NOT NULL` và là thành phần của Khóa duy nhất. Nếu cấu hình SET NULL, PostgreSQL sẽ báo lỗi khi xóa category. Phải dùng `ON DELETE CASCADE`.

### 3 Nguy cơ sinh bug thực tế:
1. **Cộng cả giao dịch `INCOME` vào tiền chi tiêu ngân sách**: Quên điều kiện `t.type = 'EXPENSE'` trong câu lệnh JOIN, làm giảm số tiền đã chi một cách vô lý.
2. **Quên điều kiện `t.user_id = b.user_id` trong mệnh đề `LEFT JOIN`**: Cho phép giao dịch của người khác bị tính nhầm vào ngân sách của mình nếu vô tình trùng `category_id`.
3. **Race Condition khi cập nhật ngân sách bằng code**: Chạy lệnh `SELECT` kiểm tra rồi mới chạy `INSERT` hoặc `UPDATE` thay vì dùng câu lệnh đơn nguyên tử `ON CONFLICT`.

### 2 Cái bẫy phỏng vấn (Interview Traps):
1. *"Tại sao trong FinTrack em không lưu sẵn trường `spent` vào bảng `budgets`, mỗi lần thêm giao dịch thì cộng thêm vào, vừa nhanh vừa đỡ phải JOIN?"* -> **Bẫy**: Đây là câu hỏi kinh điển về Denormalization vs Consistency. Lưu tĩnh trường `spent` tạo ra rủi ro Data Inconsistency cực lớn: Nếu giao dịch bị xóa, bị đổi ngày, đổi loại, đổi số tiền, hệ thống sẽ phải viết hàng loạt trigger hoặc event phức tạp để đồng bộ. Tính toán động bằng SQL `LEFT JOIN` luôn đảm bảo tính nhất quán tuyệt đối (Single Source of Truth).
2. *"Bảng ảo `EXCLUDED` trong PostgreSQL có tác dụng gì?"* -> **Bẫy**: `EXCLUDED` không phải là bảng chứa các dòng bị loại bỏ/lỗi! Nó chứa chính các giá trị mới mà câu lệnh INSERT đang cố gắng đưa vào database.

---

## 12. Code Reading Exercises

### Bài tập 1: Dự đoán kết quả
Câu lệnh SQL sau có lấy được ngân sách của danh mục chưa có giao dịch nào không?
```sql
SELECT b.id, SUM(t.amount) as spent
FROM budgets b
JOIN transactions t ON t.category_id = b.category_id
WHERE b.user_id = $1
GROUP BY b.id;
```
*Gợi ý suy nghĩ*: Nếu bảng `transactions` không có dòng nào khớp với `category_id`, kết quả của phép `JOIN` (INNER JOIN) sẽ là gì?

### Bài tập 2: Tìm bug ngày tháng
Đoạn code sau có tính đúng ngày cuối cùng của tháng 2 năm 2024 (năm nhuận) không?
```go
endOfMonth := fmt.Sprintf("%04d-%02d-28", year, month)
```

---

## 13. Active Recall

### Câu hỏi ôn tập
1. Cú pháp `ON CONFLICT (...) DO UPDATE` trong PostgreSQL giải quyết vấn đề gì?
2. Bảng giả lập `EXCLUDED` trong câu lệnh Upsert chứa dữ liệu gì?
3. Tại sao trong bảng `budgets`, khóa ngoại `category_id` lại dùng `ON DELETE CASCADE` thay vì `ON DELETE SET NULL` như trong `transactions`?
4. Tại sao cần dùng `LEFT JOIN` kết hợp `COALESCE(SUM(...), 0)` khi truy vấn danh sách ngân sách kèm số tiền đã chi?
5. Tại sao khoảng nửa mở `[start, next_start)` lại ưu việt hơn toán tử `BETWEEN` khi truy vấn theo tháng?

---

### Model Answers

#### 1. Tác dụng của `ON CONFLICT DO UPDATE`
- **English**: It provides an atomic Upsert (Insert-or-Update) operation, eliminating race conditions between concurrent inserts and updates on unique constraint keys.
- **Vietnamese**: Cung cấp thao tác Upsert (chèn hoặc cập nhật) nguyên tử, loại bỏ hoàn toàn hiện tượng tranh chấp (race condition) giữa các thao tác chèn và sửa trên các khóa có ràng buộc duy nhất.

#### 2. Dữ liệu trong `EXCLUDED`
- **English**: `EXCLUDED` represents the row proposed for insertion that conflicted with an existing row, providing access to the newly submitted values.
- **Vietnamese**: `EXCLUDED` đại diện cho dòng dữ liệu mới được gửi lên để chèn nhưng bị xung đột với dòng đã có sẵn, cho phép truy cập các giá trị mới được gửi lên.

#### 3. Tại sao `budgets` dùng CASCADE?
- **English**: A budget represents a limit defined specifically for a category. If that category is deleted, the budget has no logical purpose and must be removed. In contrast, transactions represent historical money spent that must be preserved even if the category is deleted.
- **Vietnamese**: Ngân sách là hạn mức được định nghĩa riêng cho một danh mục. Nếu danh mục đó bị xóa, ngân sách không còn mục đích tồn tại và phải bị xóa theo. Ngược lại, giao dịch đại diện cho lịch sử tiền tệ đã chi tiêu và bắt buộc phải được giữ lại.

#### 4. Tại sao dùng `LEFT JOIN` và `COALESCE`?
- **English**: `LEFT JOIN` ensures budgets with zero transactions are not omitted from the query results. `COALESCE(SUM(...), 0)` converts SQL `NULL` sums into integer `0`, preventing driver scan errors into Go `int64`.
- **Vietnamese**: `LEFT JOIN` đảm bảo những ngân sách chưa có giao dịch nào không bị biến mất khỏi kết quả truy vấn. `COALESCE(SUM(...), 0)` chuyển giá trị `NULL` thành số `0`, ngăn lỗi khi scan vào biến `int64` trong Go.

#### 5. Ưu thế của khoảng nửa mở `[start, next_start)`
- **English**: It is month-length agnostic. By taking the 1st of the month and adding 1 month with `AddDate(0, 1, 0)`, it automatically accounts for 28, 29, 30, and 31-day months, avoiding hardcoded last-day calculations.
- **Vietnamese**: Không phụ thuộc vào độ dài số ngày của tháng. Bằng cách lấy ngày mùng 1 của tháng hiện tại và cộng thêm 1 tháng bằng `AddDate(0, 1, 0)`, nó tự động xử lý chính xác các tháng có 28, 29, 30 hay 31 ngày mà không cần hardcode ngày cuối tháng.

---

## 14. Fresher Go Backend Interview Questions

### Question 1
- **English**: What is an Upsert in PostgreSQL and how do you implement it in Go?
- **Vietnamese**: Upsert trong PostgreSQL là gì và bạn cài đặt nó như thế nào trong Go?
- **Model Answer (EN)**: An Upsert atomically inserts a new row or updates an existing row if a unique conflict occurs. In PostgreSQL, this is achieved using `INSERT INTO ... ON CONFLICT (columns) DO UPDATE SET ... RETURNING ...` and executed via `QueryRowContext`.
- **Model Answer (VI)**: Upsert là thao tác nguyên tử cho phép chèn một dòng mới hoặc cập nhật dòng đã có nếu xảy ra xung đột khóa duy nhất. Trong PostgreSQL, điều này được thực hiện qua câu lệnh `INSERT INTO ... ON CONFLICT (columns) DO UPDATE SET ... RETURNING ...` và thực thi bằng `QueryRowContext`.
- **Explanation**: Khái niệm cốt lõi khi thiết kế API idempotent.
- **Follow-up**: What prerequisite must exist on the table for `ON CONFLICT` to work? (Answer: A UNIQUE constraint or UNIQUE index on the specified conflict target columns).

### Question 2
- **English**: Why does FinTrack calculate `spent` dynamically via SQL rather than storing it as a column in the `budgets` table?
- **Vietnamese**: Tại sao FinTrack tính toán `spent` động qua SQL thay vì lưu sẵn một cột trong bảng `budgets`?
- **Model Answer (EN)**: To maintain single-source-of-truth consistency. Storing a static `spent` column would require complex synchronization triggers or event handlers whenever transactions are added, edited, deleted, or back-dated. Dynamic calculation guarantees 100% accuracy at all times.
- **Model Answer (VI)**: Để duy trì tính nhất quán theo nguyên tắc Single Source of Truth. Lưu một cột `spent` tĩnh đòi hỏi các trigger hoặc cơ chế event phức tạp để đồng bộ mỗi khi giao dịch được thêm, sửa, xóa hoặc đổi ngày trong quá khứ. Tính toán động đảm bảo số liệu luôn chính xác 100%.
- **Explanation**: Hiểu về Normalization vs Denormalization và Data Integrity.
- **Follow-up**: At what scale would computing spent dynamically become a bottleneck? (Answer: When transactions reach tens of millions, at which point read-optimized caching with Redis or materialized views is introduced).

### Question 3
- **English**: What is the difference between `INNER JOIN` and `LEFT JOIN` in SQL?
- **Vietnamese**: Sự khác nhau giữa `INNER JOIN` và `LEFT JOIN` trong SQL là gì?
- **Model Answer (EN)**: `INNER JOIN` returns only rows that have matching records in both tables. `LEFT JOIN` returns all rows from the left table, along with matched columns from the right table, or `NULL` if no match exists.
- **Model Answer (VI)**: `INNER JOIN` chỉ trả về các dòng có bản ghi khớp ở cả hai bảng. `LEFT JOIN` trả về toàn bộ các dòng từ bảng bên trái, kèm theo các cột từ bảng bên phải nếu khớp, hoặc điền giá trị `NULL` nếu không có bản ghi nào khớp.
- **Explanation**: Kiến thức SQL cơ sở bắt buộc đối với Backend Engineer.
- **Follow-up**: Why would using INNER JOIN break the budget listing feature? (Answer: Budgets with zero transactions in that month would be omitted completely).

### Question 4
- **English**: How does Go's `time.AddDate` handle leap years and differing month lengths?
- **Vietnamese**: Hàm `time.AddDate` của Go xử lý năm nhuận và các tháng có độ dài khác nhau như thế nào?
- **Model Answer (EN)**: `time.AddDate(years, months, days)` normalizes calendar dates according to the Gregorian calendar rules. Adding 1 month to `2024-02-01` produces `2024-03-01`, seamlessly spanning the 29 days of February in leap year 2024.
- **Model Answer (VI)**: Hàm `time.AddDate(years, months, days)` tự động chuẩn hóa ngày lịch theo các quy tắc của lịch Gregory. Cộng 1 tháng vào `2024-02-01` sẽ cho ra `2024-03-01`, tự động bao quát trọn vẹn 29 ngày của tháng 2 trong năm nhuận 2024.
- **Explanation**: Tận dụng thư viện chuẩn của Go để loại bỏ các lỗi tính toán thời gian thủ công.
- **Follow-up**: Does `time.Add(30 * 24 * time.Hour)` achieve the same result? (Answer: No, adding fixed hours does not account for variable month lengths or daylight saving changes).

### Question 5
- **English**: What is the purpose of `COALESCE` in SQL aggregations?
- **Vietnamese**: Mục đích của hàm `COALESCE` trong các phép tổng hợp SQL là gì?
- **Model Answer (EN)**: `COALESCE` returns the first non-null argument in its list. In aggregations like `COALESCE(SUM(amount), 0)`, if no rows matched the condition, `SUM` evaluates to `NULL`; `COALESCE` safely converts this to `0`.
- **Model Answer (VI)**: `COALESCE` trả về đối số đầu tiên không mang giá trị `NULL` trong danh sách của nó. Trong các phép tổng hợp như `COALESCE(SUM(amount), 0)`, nếu không có dòng nào khớp điều kiện, hàm `SUM` trả về `NULL`; `COALESCE` sẽ chuyển giá trị đó về số `0` một cách an toàn.
- **Explanation**: Tránh lỗi driver khi scan NULL vào kiểu số nguyên nguyên thủy trong Go.
- **Follow-up**: How many arguments can `COALESCE` accept? (Answer: Any number of arguments, returning the first non-null one).

### Question 6
- **English**: Why does FinTrack forbid setting budgets on `INCOME` categories?
- **Vietnamese**: Tại sao FinTrack lại cấm thiết lập ngân sách cho các danh mục `INCOME`?
- **Model Answer (EN)**: By definition, a budget represents a spending cap or limit. Income categories represent incoming earnings (e.g. Salary, Freelance). Placing a spending ceiling on income is semantically invalid within the domain model.
- **Model Answer (VI)**: Về mặt định nghĩa nghiệp vụ, ngân sách là một hạn mức trần nhằm kiểm soát chi tiêu. Danh mục thu nhập đại diện cho nguồn tiền thu vào (như Lương, Thưởng). Đặt trần chi tiêu cho nguồn thu là điều vô nghĩa và sai lệch về mặt mô hình nghiệp vụ (Domain Model).
- **Explanation**: Bảo vệ tính đúng đắn của Domain Logic.
- **Follow-up**: Where is this rule enforced: database or service layer? (Answer: Enforced in the Service layer during input validation).

### Question 7
- **English**: Why does `FindByID` in `PostgresBudgetRepository` use an `INNER JOIN` with `categories`?
- **Vietnamese**: Tại sao hàm `FindByID` trong `PostgresBudgetRepository` lại dùng `INNER JOIN` với bảng `categories`?
- **Model Answer (EN)**: Because every budget has a mandatory `NOT NULL` foreign key referencing an existing category (`category_id UUID NOT NULL`). Every budget is guaranteed to have a corresponding category, making `INNER JOIN` safe and optimal for fetching `category_name`.
- **Model Answer (VI)**: Vì mọi ngân sách đều có một khóa ngoại bắt buộc `NOT NULL` trỏ tới một danh mục đang tồn tại (`category_id UUID NOT NULL`). Mọi ngân sách đều chắc chắn có một danh mục tương ứng, do đó `INNER JOIN` là an toàn tuyệt đối và tối ưu nhất để lấy `category_name`.
- **Explanation**: Tận dụng tính toàn vẹn của khóa ngoại để chọn đúng loại JOIN.
- **Follow-up**: What if the category was deleted? (Answer: Because of `ON DELETE CASCADE`, the budget would have been deleted as well).

### Question 8
- **English**: Why do we check `user_id` inside the `LEFT JOIN` condition for transactions?
- **Vietnamese**: Tại sao chúng ta lại kiểm tra `t.user_id = b.user_id` ngay bên trong điều kiện của `LEFT JOIN`?
- **Model Answer (EN)**: Putting `t.user_id = b.user_id` directly in the `LEFT JOIN ... ON` clause ensures that transactions belonging to other users are excluded before the join occurs, maintaining strict tenant isolation during the aggregation.
- **Model Answer (VI)**: Đặt `t.user_id = b.user_id` trực tiếp trong mệnh đề `LEFT JOIN ... ON` đảm bảo rằng các giao dịch của người dùng khác bị loại trừ ngay trước khi phép join diễn ra, duy trì sự cô lập người dùng nghiêm ngặt trong quá trình tính tổng.
- **Explanation**: Tối ưu hóa điều kiện JOIN trong cơ sở dữ liệu đa người dùng.
- **Follow-up**: What would happen if `t.user_id = $1` was placed in the `WHERE` clause instead? (Answer: It would convert the LEFT JOIN into an INNER JOIN, excluding budgets that have no transactions).

### Question 9
- **English**: How does `PostgresBudgetRepository.Delete` verify that the budget belonged to the caller?
- **Vietnamese**: Hàm `PostgresBudgetRepository.Delete` xác minh ngân sách thuộc về người gọi như thế nào?
- **Model Answer (EN)**: It executes `DELETE FROM budgets WHERE id = $1 AND user_id = $2`. If `res.RowsAffected() == 0`, it returns `domain.ErrNotFound`, preventing any user from deleting another user's budget.
- **Model Answer (VI)**: Nó thực thi lệnh `DELETE FROM budgets WHERE id = $1 AND user_id = $2`. Nếu `res.RowsAffected() == 0`, hàm trả về `domain.ErrNotFound`, ngăn chặn người dùng xóa ngân sách của người khác.
- **Explanation**: Triển khai Tenant Isolation nhất quán xuyên suốt các thực thể.
- **Follow-up**: What HTTP status code is returned on successful deletion? (Answer: HTTP 204 No Content).

### Question 10
- **English**: What is the composite index `idx_budgets_user_year_month` used for?
- **Vietnamese**: Chỉ mục tổng hợp `idx_budgets_user_year_month` được dùng để làm gì?
- **Model Answer (EN)**: It optimizes the monthly listing query (`WHERE b.user_id = $1 AND b.year = $2 AND b.month = $3`), allowing PostgreSQL to locate a user's budgets for a specific month in a single index lookup.
- **Model Answer (VI)**: Nó tối ưu hóa câu truy vấn xem danh sách ngân sách theo tháng (`WHERE b.user_id = $1 AND b.year = $2 AND b.month = $3`), cho phép PostgreSQL tìm ra tất cả ngân sách của user trong tháng đó chỉ qua một lần tra cứu index duy nhất.
- **Explanation**: Thiết kế chỉ mục tương ứng với mẫu truy vấn (Query Pattern Indexing).
- **Follow-up**: Is this index redundant with the UNIQUE constraint? (Answer: The unique constraint includes `category_id`; this index specifically accelerates listing all categories for a month).

---

## 15. Knowledge Connection
- **Kế thừa từ Issue #6 & #7**: Ngân sách phụ thuộc vào danh mục `categories` (Issue #6) và tính toán số tiền đã tiêu từ bảng `transactions` (Issue #7).
- **Chuẩn bị cho Issue #10 (Analytics & Financial Summary)**: Trong màn hình tổng quan tài chính ở Issue #10, chúng ta sẽ so sánh tổng ngân sách đã đặt (`total_budget`) với tổng chi tiêu thực tế (`total_spent`) để tính tỷ lệ tiết kiệm ròng (net savings) của người dùng.

---

## 16. Learning Checkpoint

### I Must Understand
1. Cách viết câu lệnh Upsert với `ON CONFLICT (...) DO UPDATE` và bảng ảo `EXCLUDED`.
2. Kỹ thuật tính toán `spent` động bằng `LEFT JOIN` và `COALESCE(SUM(...), 0)`.
3. Cách tính khoảng thời gian nửa mở `[start, next_start)` bằng `time.Date` và `AddDate(0, 1, 0)`.
4. Sự khác nhau giữa `ON DELETE CASCADE` của Budget và `ON DELETE SET NULL` của Transaction.
5. Quy tắc nghiệp vụ: Ngân sách chỉ áp dụng cho danh mục `EXPENSE`.

### I Should Be Able To Code
1. Viết câu truy vấn Upsert trong PostgreSQL bằng `database/sql`.
2. Viết câu query `LEFT JOIN` kết hợp `GROUP BY` và `COALESCE` để tính tổng dữ liệu con.
3. Viết hàm sinh chuỗi ngày chặn đầu chặn đuôi cho một tháng bất kỳ.

### I Should Be Able To Explain In An Interview
1. Tại sao nên tính toán `spent` động thay vì lưu tĩnh trong database?
2. Sự khác nhau giữa `INNER JOIN` và `LEFT JOIN` trong ngữ cảnh báo cáo ngân sách.
3. Bảng ảo `EXCLUDED` trong PostgreSQL hoạt động như thế nào?

### Advanced Topics I Do Not Need Yet
- Ngân sách luân chuyển (Rollover Budgets: số tiền thừa tháng này tự động cộng dồn vào ngân sách tháng sau).
- Cảnh báo vượt ngân sách thời gian thực qua Webhook hoặc Push Notification.
- Ngân sách phân cấp theo cây danh mục cha - con (Parent-Child Category Budgets).
