# Issue #10 — Financial Analytics, Conditional SQL Aggregations & Continuous Timeline

## 1. Learning Objectives
Sau khi hoàn thành bài học này, bạn sẽ nắm vững:
- Kỹ thuật tổng hợp dữ liệu nâng cao trong SQL bằng biểu thức điều kiện: `SUM(CASE WHEN type = 'INCOME' THEN amount ELSE 0 END)`.
- Cách phòng tránh lỗi nhân đôi dòng (Row Multiplication / Cartesian Product Trap) khi JOIN nhiều bảng quan hệ một - nhiều.
- Thuật toán lấp đầy khoảng trống dữ liệu (Zero-Filling Gap Elimination) để tạo biểu đồ xu hướng (Trend Chart) liên tục $N$ tháng không bị đứt gãy.
- Kỹ thuật tính toán tỷ lệ phần trăm (Percentage Share) và làm tròn 2 chữ số thập phân an toàn với `math.Round`.
- Lý do tách riêng `AnalyticsRepository` thay vì nhồi nhét vào `TransactionRepository` (Single Responsibility Principle).

---

## 2. What Problem Does This Issue Solve?
Người dùng không chỉ muốn xem danh sách từng giao dịch riêng lẻ, họ cần cái nhìn tổng thể về sức khỏe tài chính:
1. **Báo cáo tài chính tháng (Monthly Summary)**: Tổng thu nhập bao nhiêu? Tổng chi tiêu bao nhiêu? Tiết kiệm ròng (`net_savings = income - expense`) là âm hay dương? Tiến độ tiêu ngân sách đã đạt bao nhiêu %?
2. **Cơ cấu chi tiêu (Expense Distribution)**: Tiền của tôi chủ yếu chảy vào đâu? Ăn uống chiếm bao nhiêu %, Nhà ở chiếm bao nhiêu %? Các khoản chưa phân loại (Uncategorized) chiếm bao nhiêu %?
3. **Biểu đồ xu hướng tài chính (Financial Trend)**: Thu nhập và chi tiêu biến động ra sao qua các tháng (ví dụ 6 tháng gần nhất)?
4. **Vấn đề tháng không có giao dịch (Data Gaps)**: Nếu tháng 1 người dùng không tiêu đồng nào, câu lệnh `GROUP BY` trong SQL sẽ bỏ qua tháng 1. Nếu Backend trả về dữ liệu thiếu tháng 1, biểu đồ đường (Line Chart) trên ứng dụng React/Mobile sẽ bị vẽ lệch trục thời gian!

---

## 3. Prerequisite Go Knowledge

### 3.1. Go `math.Round` và làm tròn số thập phân
- Hàm `math.Round(x)` làm tròn số thực đến số nguyên gần nhất.
- Để làm tròn đến **2 chữ số thập phân** (ví dụ: `33.33333% -> 33.33%`), công thức toán học chuẩn trong Go là:
```go
percentage = math.Round((value / total * 100) * 100) / 100
```
- **Ví dụ**:
  - `(100 / 300 * 100) = 33.333333...`
  - Nhân 100 -> `3333.3333...`
  - `math.Round` -> `3333.0`
  - Chia 100 -> `33.33`
- **Kết nối FinTrack**: Sử dụng trong `GetCategoryExpenses` và `GetMonthlySummary`.

### 3.2. Go Map kết hợp Ordered Slice để bảo toàn thứ tự
- **Bản chất của Go Map**: Map trong Go không đảm bảo thứ tự (Unordered). Khi lặp qua map bằng `for k, v := range myMap`, thứ tự các phần tử là ngẫu nhiên giữa các lần chạy.
- Để vừa tra cứu nhanh theo key $O(1)$, vừa đảm bảo danh sách trả về đúng thứ tự thời gian tăng dần (từ tháng cũ nhất đến tháng mới nhất), Go sử dụng mô hình kết hợp:
  1. Slice `orderedMonths []string`: Lưu danh sách các tháng theo thứ tự niên đại.
  2. Map `monthMap map[string]*Point`: Cho phép cập nhật số liệu của từng tháng khi duyệt kết quả SQL.
  3. Cuối cùng, duyệt slice `orderedMonths` để lấy dữ liệu từ `monthMap` xuất ra JSON.

---

## 4. Architecture

Kiến trúc tầng Analytics phục vụ màn hình Dashboard:

```text
HTTP Request: GET /api/v1/analytics/summary?month=2026-03
       │
  (AuthMiddleware) ──► userID: "u111"
       │
       ▼
[Handler: AnalyticsHandler.GetMonthlySummary]
  • Đọc query param: month="2026-03"
       │
       ▼
[Service: AnalyticsService.GetMonthlySummary]
  • Parse định dạng "2006-01" bằng time.Parse
  • Trích xuất year=2026, month=3
       │
       ▼
[Repository: PostgresAnalyticsRepository.GetMonthlySummary]
  • TRUY VẤN 1: Tổng hợp thu nhập & chi tiêu từ bảng transactions:
      SUM(CASE WHEN type = 'INCOME' ...)
      SUM(CASE WHEN type = 'EXPENSE' ...)
  • TRUY VẤN 2: Tổng hợp hạn mức ngân sách & chi tiêu thực tế từ bảng budgets:
      SUM(b.amount), SUM(t.amount)
  • Tính toán trong Go:
      netSavings = totalIncome - totalExpense
      remaining = totalBudget - totalSpent
      percentage = spent / budget * 100
       │
       ▼
[PostgreSQL Database]
  • Thực thi 2 câu query tối ưu trong 1 round-trip kết nối
```

---

## 5. File-by-File Analysis

### 5.1. `internal/domain/analytics.go`
- **Structs Analysis**:
  - `BudgetStatusOverview`: Tiến độ tiêu ngân sách (`TotalBudget`, `TotalSpent`, `Remaining`, `Percentage`).
  - `MonthlySummary`: Báo cáo tài chính tháng tổng thể (`TotalIncome`, `TotalExpense`, `NetSavings`, `BudgetStatus`).
  - `CategoryExpenseShare`: Tỷ trọng chi tiêu theo từng danh mục. Chú ý: `CategoryID *uuid.UUID` có thể là `nil` (đối với các khoản chi chưa phân loại "Uncategorized").
  - `MonthlyTrendPoint`: Một điểm dữ liệu trên biểu đồ xu hướng tháng (`Month`, `Income`, `Expense`, `NetSavings`).
- **Interview Importance**: ⭐ SHOULD KNOW.

### 5.2. `internal/repository/analytics_repository.go`
- **Interface Contract**:
  - `GetMonthlySummary(ctx context.Context, userID uuid.UUID, year int, month int) (*domain.MonthlySummary, error)`
  - `GetCategoryExpenses(ctx context.Context, userID uuid.UUID, year int, month int) ([]domain.CategoryExpenseShare, error)`
  - `GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) ([]domain.MonthlyTrendPoint, error)`
- **Tại sao tách riêng `AnalyticsRepository`?**:
  - `TransactionRepository` phục vụ các tác vụ OLTP (Online Transaction Processing: thêm, sửa, xóa bản ghi đơn lẻ).
  - `AnalyticsRepository` phục vụ các tác vụ OLAP dạng nhẹ (tổng hợp, gom nhóm, thống kê số liệu).
  - Tách rời giúp code tuân thủ Single Responsibility Principle (SRP) và dễ dàng chuyển hướng sang đọc Database Read-Replica trong tương lai.
- **Interview Importance**: 🔥 MUST KNOW. Kiến trúc tách biệt OLTP và OLAP/Reporting.

### 5.3. `internal/repository/postgres_analytics_repository.go`
- **Responsibility**: Cài đặt các câu lệnh SQL tổng hợp phức tạp.
- **Phân tích chi tiết câu lệnh 1: Tính Thu & Chi trong tháng**:
  ```sql
  SELECT 
      COALESCE(SUM(CASE WHEN type = 'INCOME' THEN amount ELSE 0 END), 0) AS total_income,
      COALESCE(SUM(CASE WHEN type = 'EXPENSE' THEN amount ELSE 0 END), 0) AS total_expense
  FROM transactions
  WHERE user_id = $1 AND transaction_date >= $2 AND transaction_date < $3
  ```
  - Thay vì chạy 2 câu lệnh riêng biệt (`WHERE type = 'INCOME'` và `WHERE type = 'EXPENSE'`), kỹ thuật **Conditional Aggregation** cho phép quét bảng `transactions` đúng **1 lần duy nhất** để tính đồng thời cả Thu và Chi.
- **Phân tích chi tiết câu lệnh 2: Báo cáo cơ cấu chi tiêu (`GetCategoryExpenses`)**:
  ```sql
  SELECT 
      c.id AS category_id,
      COALESCE(c.name, 'Uncategorized') AS category_name,
      SUM(t.amount) AS amount
  FROM transactions t
  LEFT JOIN categories c ON c.id = t.category_id AND c.user_id = t.user_id
  WHERE t.user_id = $1 
    AND t.type = 'EXPENSE'
    AND t.transaction_date >= $2 
    AND t.transaction_date < $3
  GROUP BY c.id, c.name
  ORDER BY amount DESC, category_name ASC
  ```
  - Dùng `LEFT JOIN` để các giao dịch có `category_id IS NULL` vẫn được gom nhóm thành công với tên hiển thị là `'Uncategorized'`.
  - Sắp xếp `amount DESC`: Danh mục tiêu tốn nhiều tiền nhất luôn hiển thị lên đầu tiên.
- **Thuật toán Zero-Filling trong `GetMonthlyTrend`**:
  ```go
  monthMap := make(map[string]*domain.MonthlyTrendPoint)
  orderedMonths := make([]string, 0, months)
  cur := startMonth
  for !cur.After(currentMonthStart) {
      mStr := cur.Format("2006-01")
      orderedMonths = append(orderedMonths, mStr)
      monthMap[mStr] = &domain.MonthlyTrendPoint{Month: mStr, Income: 0, Expense: 0, NetSavings: 0}
      cur = cur.AddDate(0, 1, 0)
  }
  // Duyệt rows từ SQL và ghi đè số thực tế vào map...
  ```
  - Đảm bảo nếu một tháng không có giao dịch, nó vẫn xuất hiện trong kết quả với giá trị `0` thay vì biến mất.
- **Interview Importance**: 🔥 MUST KNOW. Conditional Sum, Row Multiplication Prevention, và Zero-Filling.

### 5.4. `internal/service/analytics_service.go`
- **Validation Logic**:
  - Xác thực chuỗi tháng `monthStr` bằng `time.Parse("2006-01", monthStr)`.
  - Giới hạn số tháng xem biểu đồ xu hướng: `months` mặc định là 6, và nằm trong khoảng [1, 24] (không cho phép query vượt quá 2 năm gây quá tải).
- **Interview Importance**: ⭐ SHOULD KNOW.

### 5.5. `internal/handler/analytics_handler.go`
- **Responsibility**: Khai báo 3 endpoint:
  1. `GET /api/v1/analytics/summary?month=YYYY-MM`
  2. `GET /api/v1/analytics/categories?month=YYYY-MM`
  3. `GET /api/v1/analytics/trend?months=6`
- **Interview Importance**: ⭐ SHOULD KNOW.

---

## 6. Concrete Runtime Flow

Chi tiết luồng chạy của `GET /api/v1/analytics/trend?months=3`:

```text
Giả sử hôm nay là ngày 25/03/2026:
1. AnalyticsHandler:
   - userID = GetUserID(c).
   - c.DefaultQuery("months", "6") -> parse thành months = 3.
   - Gọi analyticsService.GetMonthlyTrend(ctx, userID, 3).

2. AnalyticsService:
   - Kiểm tra months = 3 nằm trong [1, 24] -> Hợp lệ.
   - Gọi analyticsRepo.GetMonthlyTrend(ctx, userID, 3).

3. PostgresAnalyticsRepository.GetMonthlyTrend:
   a. Tính toán mốc thời gian:
      - Tháng hiện tại: "2026-03"
      - Khởi tạo timeline 3 tháng liên tiếp:
        ["2026-01", "2026-02", "2026-03"]
      - Nạp sẵn vào monthMap với Income = 0, Expense = 0.
   b. Thực thi SQL:
      - Quét transactions từ 2026-01-01 đến 2026-04-01.
      - Giả sử kết quả trả về từ DB:
        • "2026-01": Income = 20M, Expense = 15M
        • "2026-03": Income = 25M, Expense = 18M
        (Lưu ý: Tháng 2 người dùng đi du lịch, không phát sinh giao dịch nào nên DB không trả về dòng 2026-02!)
   c. Cập nhật vào monthMap:
      - "2026-01" -> Cập nhật số thực tế.
      - "2026-02" -> Giữ nguyên Income = 0, Expense = 0!
      - "2026-03" -> Cập nhật số thực tế.
   d. Duyệt orderedMonths để build mảng kết quả:
      - Trả về đúng 3 phần tử liên tục: [Tháng 1, Tháng 2 (giá trị 0), Tháng 3].

4. Client nhận JSON:
   - Frontend nhận đủ 3 điểm thời gian và vẽ biểu đồ đường trơn tru, không bị méo lệch trục hoành!
```

---

## 7. Important Go Concepts

### 7.1. Chống lỗi chia cho 0 (`Zero Division Error`)
- Khi tính tỷ lệ phần trăm ngân sách:
  ```go
  var percentage float64
  if totalBudget > 0 {
      percentage = math.Round((float64(totalSpent)/float64(totalBudget)*100)*100) / 100
  }
  ```
- Nếu người dùng chưa đặt ngân sách (`totalBudget == 0`), trong Go phép chia `float64 / 0` sẽ sinh ra giá trị đặc biệt `+Inf` (Dương vô cùng) hoặc `NaN` (Not a Number).
- Khi thư viện JSON encode số `+Inf` hoặc `NaN`, nó sẽ báo lỗi runtime: `json: unsupported value: +Inf`. Luôn kiểm tra mẫu số khác 0 trước khi thực hiện phép chia!

### 7.2. Ép kiểu rõ ràng khi tính phần trăm
- Trong Go, `totalSpent` và `totalBudget` là `int64`.
- Nếu bạn viết `totalSpent / totalBudget`, Go sẽ thực hiện **phép chia số nguyên (Integer Division)**: `50000 / 100000 = 0` (bị mất toàn bộ phần thập phân!).
- Bắt buộc phải ép kiểu sang `float64` trước khi chia: `float64(totalSpent) / float64(totalBudget)`.

---

## 8. Database / SQL Concepts

### 8.1. Cạm bẫy nhân đôi dòng (Row Multiplication / Cartesian Product Trap)
- Đây là lỗi nghiêm trọng nhất khi viết query tổng hợp dữ liệu từ nhiều bảng quan hệ 1-N:
- **Tình huống**: Bảng `budgets` có 1 dòng cho danh mục "Ăn uống" (Hạn mức 5 triệu). Trong tháng đó, bảng `transactions` có 10 giao dịch ăn uống.
- **Nếu bạn viết 1 câu query JOIN gộp tất cả**:
  ```sql
  -- CỰC KỲ NGUY HIỂM! BUG NHÂN ĐÔI DÒNG!
  SELECT SUM(b.amount), SUM(t.amount)
  FROM budgets b
  LEFT JOIN transactions t ON t.category_id = b.category_id;
  ```
- Phép JOIN sẽ tạo ra **10 dòng kết quả**. Cột `b.amount` (5 triệu) sẽ bị lặp lại 10 lần!
- Kết quả `SUM(b.amount)` sẽ thành **50 triệu** (gấp 10 lần thực tế!).
- **Giải pháp của FinTrack**: Tách thành 2 câu query riêng biệt:
  1. Câu query 1 tính tổng thu chi trực tiếp trên bảng `transactions`.
  2. Câu query 2 tính ngân sách và chi tiêu theo danh mục ngân sách.
  - Tách rời đảm bảo kết quả luôn chính xác 100%.

### 8.2. Kỹ thuật `CASE WHEN` bên trong hàm `SUM()`
- Biểu thức `SUM(CASE WHEN type = 'INCOME' THEN amount ELSE 0 END)` tương đương với:
  - Nếu dòng đó là `INCOME`: lấy giá trị `amount`.
  - Nếu dòng đó là `EXPENSE`: coi như giá trị là `0`.
  - Cộng tất cả lại.
- Giúp chuyển đổi dòng thành cột (Pivot Row-to-Column) mà không cần dùng hàm `PIVOT` phức tạp.

---

## 9. Error Handling

### Xử lý lỗi định dạng tham số Query `month`:
- Người dùng có thể truyền `?month=2026-3` (thiếu số 0) hoặc `?month=invalid`.
- Tầng Service parse bằng `time.Parse("2006-01", monthStr)`. Nếu lỗi, trả về `ValidationError`:
  ```json
  {
    "error": {
      "code": "VALIDATION_ERROR",
      "message": "month must be in YYYY-MM format",
      "fields": {
        "month": "month must be in YYYY-MM format"
      }
    }
  }
  ```

---

## 10. Testing

### 10.1. Kiểm thử Zero-Filling cho các tháng không có dữ liệu
Trong `internal/repository/postgres_analytics_repository_test.go`:
- Tạo giao dịch vào tháng 1 và tháng 3. Không tạo bất kỳ giao dịch nào trong tháng 2.
- Gọi `GetMonthlyTrend(ctx, userID, 3)`.
- Khẳng định kết quả trả về đúng 3 phần tử.
- Khẳng định phần tử thứ 2 mang `Month = "YYYY-02"` với `Income = 0`, `Expense = 0`, `NetSavings = 0`.

### 10.2. Kiểm thử chống chia cho 0
- Gọi `GetMonthlySummary` cho một tháng hoàn toàn chưa có ngân sách (`totalBudget = 0`).
- Khẳng định API trả về `percentage = 0.0` và không bị lỗi panic hoặc JSON serialization error.

---

## 11. Beginner Mistakes

### 5 Sai lầm phổ biến của người mới:
1. **Lấy toàn bộ transactions về Go rồi dùng vòng lặp `for` để tính tổng**: Kéo hàng chục nghìn bản ghi qua mạng về RAM server để tính toán, làm chậm hệ thống gấp hàng trăm lần so với việc để database tính toán bằng `SUM()`.
2. **Chia số nguyên `int64 / int64` khi tính phần trăm**: Khiến kết quả luôn bằng `0` nếu tử số nhỏ hơn mẫu số.
3. **Không kiểm tra mẫu số bằng 0 trước khi chia**: Dẫn đến giá trị `+Inf` làm sập hàm serialize JSON.
4. **Không xử lý các tháng bị khuyết dữ liệu (Missing Gaps)**: Khiến biểu đồ Frontend bị mất mốc thời gian.
5. **Gộp bảng budgets và transactions vào chung 1 câu lệnh SUM**: Dính lỗi nhân đôi số liệu ngân sách (Row Multiplication Trap).

### 3 Nguy cơ sinh bug thực tế:
1. **Lỗi timezone khi gom nhóm theo tháng bằng SQL**: Dùng `TO_CHAR(transaction_date, 'YYYY-MM')` trên cột timestamp có timezone có thể khiến giao dịch lúc 01:00 sáng ngày mùng 1 bị tính nhầm sang tháng trước.
2. **Bỏ sót các giao dịch chưa phân loại trong báo cáo danh mục**: Dùng `INNER JOIN categories` thay vì `LEFT JOIN`, khiến toàn bộ các khoản chi không gắn danh mục bị biến mất khỏi báo cáo.
3. **Làm tròn số thực sai cách**: Dùng `fmt.Sprintf("%.2f")` chuyển sang chuỗi rồi ép kiểu ngược lại số thực vừa chậm vừa dễ sinh lỗi định dạng dấu chấm/phẩy.

### 2 Cái bẫy phỏng vấn (Interview Traps):
1. *"Khi nào nên tính toán tổng hợp dữ liệu ở tầng Database (SQL) và khi nào nên tính ở tầng Backend (Go)?"* -> **Bẫy**: Database cực mạnh trong việc quét hàng triệu dòng, lọc và cộng dồn số liệu qua Index (chuyển 100.000 dòng thành 1 con số duy nhất gửi qua mạng). Tuy nhiên, các logic nghiệp vụ phức tạp, điều kiện rẽ nhánh đặc thù hoặc các phép toán nặng về CPU nên được thực hiện ở Backend để tránh làm nghẽn CPU của Database server.
2. *"Hiện tượng Row Multiplication khi JOIN là gì và làm sao để phòng tránh?"* -> **Bẫy**: Khi JOIN 1-N, các dòng ở bảng 1 bị nhân bản lên theo số dòng tương ứng ở bảng N. Nếu `SUM()` trên cột của bảng 1, kết quả sẽ bị phóng đại. Cách khắc phục: Tách thành các subquery hoặc câu query riêng biệt.

---

## 12. Code Reading Exercises

### Bài tập 1: Dự đoán kết quả
Đoạn code Go sau in ra kết quả gì?
```go
var spent int64 = 50
var budget int64 = 100
pct := spent / budget * 100
fmt.Println(pct)
```
*Gợi ý suy nghĩ*: `50 / 100` trong phép chia số nguyên của Go bằng mấy?

### Bài tập 2: Tìm bug SQL
Tìm lỗi nghiêm trọng trong câu query báo cáo sau:
```sql
SELECT c.name, SUM(t.amount)
FROM categories c
JOIN transactions t ON t.category_id = c.id
WHERE t.user_id = $1
GROUP BY c.name;
```
*Gợi ý suy nghĩ*: Câu query này có thống kê được các khoản chi tiêu mà người dùng không gắn danh mục (`category_id IS NULL`) không?

---

## 13. Active Recall

### Câu hỏi ôn tập
1. Cú pháp `SUM(CASE WHEN ... THEN ... ELSE ... END)` trong SQL có tác dụng gì?
2. Tại sao việc chia cho số 0 trong kiểu `float64` của Go lại gây lỗi khi encode JSON?
3. Hiện tượng Row Multiplication (nhân đôi dòng) xảy ra khi nào trong SQL JOIN?
4. Tại sao cần thuật toán Zero-Filling cho biểu đồ xu hướng (Monthly Trend)?
5. Tại sao `AnalyticsRepository` lại tách riêng khỏi `TransactionRepository`?

---

### Model Answers

#### 1. Tác dụng của Conditional Sum
- **English**: It aggregates subsets of data based on dynamic conditions in a single table pass, such as computing total income and total expense simultaneously without separate queries.
- **Vietnamese**: Nó tổng hợp các tập con dữ liệu dựa trên điều kiện linh hoạt trong đúng 1 lần quét bảng, ví dụ như tính toán đồng thời cả tổng thu và tổng chi mà không cần chạy 2 query riêng biệt.

#### 2. Lỗi chia 0 với JSON trong Go
- **English**: `float64` division by zero produces `+Inf` or `NaN`. Standard Go `encoding/json` cannot serialize these non-finite numbers into valid JSON, resulting in a runtime serialization error.
- **Vietnamese**: Phép chia số thực cho 0 sinh ra giá trị `+Inf` hoặc `NaN`. Thư viện `encoding/json` của Go không thể serialize các giá trị vô cực này thành JSON hợp lệ, dẫn đến lỗi runtime khi phản hồi HTTP.

#### 3. Hiện tượng Row Multiplication
- **English**: It occurs in 1-to-many JOINs where rows from the parent table are duplicated for every matching child row. Calling `SUM()` on parent columns aggregates the duplicate values, artificially inflating the totals.
- **Vietnamese**: Xảy ra trong các phép JOIN quan hệ 1-N, khi các dòng của bảng cha bị nhân bản cho mỗi dòng con tương ứng. Gọi `SUM()` trên cột của bảng cha sẽ cộng dồn các giá trị lặp này, làm sai lệch phóng đại kết quả.

#### 4. Tại sao cần Zero-Filling?
- **English**: SQL `GROUP BY` only returns rows for months that have recorded transactions. Zero-filling populates missing chronological months with 0s, ensuring continuous timelines for charting libraries.
- **Vietnamese**: Mệnh đề `GROUP BY` trong SQL chỉ trả về dòng cho các tháng thực sự có giao dịch. Thuật toán Zero-filling điền số 0 vào các tháng bị khuyết, đảm bảo dòng thời gian liên tục cho các thư viện vẽ biểu đồ.

#### 5. Lý do tách `AnalyticsRepository`
- **English**: Separation of Concerns. `TransactionRepository` handles transactional CRUD operations (OLTP), while `AnalyticsRepository` handles read-heavy aggregation reporting (OLAP), enabling independent optimization and read-replica scaling.
- **Vietnamese**: Phân tách trách nhiệm (Separation of Concerns). `TransactionRepository` xử lý các thao tác CRUD giao dịch nghiệp vụ (OLTP), trong khi `AnalyticsRepository` xử lý các tác vụ đọc tổng hợp nặng (OLAP), giúp tối ưu hóa độc lập và dễ dàng tách sang Database Read-Replica.

---

## 14. Fresher Go Backend Interview Questions

### Question 1
- **English**: How does Conditional Aggregation with `CASE WHEN` optimize reporting queries?
- **Vietnamese**: Kỹ thuật tổng hợp có điều kiện với `CASE WHEN` tối ưu hóa các câu truy vấn báo cáo như thế nào?
- **Model Answer (EN)**: It allows computing multiple aggregate metrics across different criteria (e.g. total income and total expense) in a single sequential scan of the table, avoiding multiple expensive database round-trips or full-table scans.
- **Model Answer (VI)**: Nó cho phép tính toán nhiều chỉ số tổng hợp theo các tiêu chí khác nhau (như tổng thu nhập và tổng chi tiêu) chỉ trong đúng 1 lần quét bảng tuần tự, tránh việc phải chạy nhiều câu truy vấn lặp đi lặp lại tốn kém tài nguyên.
- **Explanation**: Tối ưu hóa I/O cơ sở dữ liệu.
- **Follow-up**: Can you combine `COUNT` and `SUM` inside the same query? (Answer: Yes, SQL allows arbitrary combinations of aggregate functions in a single SELECT).

### Question 2
- **English**: How do you round floating-point numbers to 2 decimal places in Go?
- **Vietnamese**: Bạn làm tròn số thực đến 2 chữ số thập phân trong Go như thế nào?
- **Model Answer (EN)**: Multiply the number by 100, apply `math.Round()`, and then divide by 100: `math.Round(val * 100) / 100`.
- **Model Answer (VI)**: Nhân số đó với 100, áp dụng hàm `math.Round()`, rồi chia lại cho 100: `math.Round(val * 100) / 100`.
- **Explanation**: Kỹ thuật làm tròn số thực tiêu chuẩn trong Go mà không phụ thuộc vào chuỗi string.
- **Follow-up**: Why not use `strconv.FormatFloat`? (Answer: Converting to string and back to float allocates memory and is slower than pure mathematical operations).

### Question 3
- **English**: What is the Cartesian Product problem in SQL aggregations and how did FinTrack prevent it?
- **Vietnamese**: Vấn đề tích Cartesian (Cartesian Product) trong các phép tổng hợp SQL là gì và FinTrack đã phòng ngừa nó ra sao?
- **Model Answer (EN)**: Joining multiple 1-to-N child tables causes parent rows to duplicate, artificially inflating `SUM` calculations. FinTrack prevents this by executing separate queries for transaction totals and budget totals instead of a single massive join.
- **Model Answer (VI)**: Việc JOIN nhiều bảng con quan hệ 1-N làm các dòng của bảng cha bị nhân bản, khiến phép tính `SUM` bị phóng đại sai lệch. FinTrack ngăn chặn điều này bằng cách tách thành các câu query riêng biệt cho tổng giao dịch và tổng ngân sách thay vì dùng 1 câu lệnh JOIN khổng lồ.
- **Explanation**: Kiến thức sâu sắc về cơ sở dữ liệu quan hệ.
- **Follow-up**: Is executing two lightweight queries always slower than one massive complex query? (Answer: No, two targeted index-driven queries are often much faster and more predictable than a giant Cartesian join).

### Question 4
- **English**: Why does Go's `map` not guarantee iteration order, and how do you ensure deterministic JSON output?
- **Vietnamese**: Tại sao kiểu `map` trong Go không đảm bảo thứ tự duyệt và làm sao để đảm bảo kết quả JSON trả về có thứ tự cố định?
- **Model Answer (EN)**: Go intentionally randomizes map iteration order to prevent code from relying on implementation-specific hash bucket layouts. To produce deterministic sequential output, maintain a separate slice of keys in the desired order and iterate through the slice to look up values in the map.
- **Model Answer (VI)**: Go cố tình làm ngẫu nhiên hóa thứ tự duyệt map để lập trình viên không bị phụ thuộc ngầm vào thứ tự sắp xếp bucket băm bên dưới. Để đảm bảo kết quả tuần tự cố định, ta duy trì một slice riêng chứa các key theo thứ tự mong muốn và duyệt slice đó để lấy giá trị từ map.
- **Explanation**: Thiết kế ngôn ngữ Go cơ bản (Go Map Internals).
- **Follow-up**: Does JSON specification require keys in objects to be ordered? (Answer: The JSON specification treats object keys as unordered, but array elements must preserve order).

### Question 5
- **English**: How does FinTrack ensure that uncategorized expenses are included in category expense shares?
- **Vietnamese**: FinTrack làm thế nào để đảm bảo các khoản chi chưa phân loại vẫn xuất hiện trong cơ cấu chi tiêu danh mục?
- **Model Answer (EN)**: By performing a `LEFT JOIN categories c ON c.id = t.category_id` and using `COALESCE(c.name, 'Uncategorized')`. Uncategorized transactions have `t.category_id IS NULL`, which results in `c.name` being NULL, safely converted to the label `'Uncategorized'`.
- **Model Answer (VI)**: Bằng cách thực hiện `LEFT JOIN categories c ON c.id = t.category_id` và sử dụng hàm `COALESCE(c.name, 'Uncategorized')`. Các giao dịch chưa phân loại có `t.category_id IS NULL`, dẫn đến `c.name` mang giá trị NULL và được hàm COALESCE gán nhãn thành `'Uncategorized'`.
- **Explanation**: Xử lý dữ liệu ngoại lệ một cách thanh lịch trong SQL.
- **Follow-up**: What is the value of `CategoryID` in the Go struct for uncategorized rows? (Answer: `nil`).

### Question 6
- **English**: Why does `AnalyticsService` validate that `months` is between 1 and 24?
- **Vietnamese**: Tại sao `AnalyticsService` lại xác thực số tháng `months` phải nằm trong khoảng từ 1 đến 24?
- **Model Answer (EN)**: It enforces a reasonable operational window for dashboard trend analytics. Allowing unbounded queries (e.g. 1000 months) could trigger prolonged table scans, consuming excessive memory and CPU on the database server.
- **Model Answer (VI)**: Nhằm thực thi giới hạn vận hành hợp lý cho báo cáo xu hướng. Việc cho phép người dùng query số tháng tùy ý không giới hạn (như 1000 tháng) có thể gây ra việc quét bảng kéo dài, làm cạn kiệt RAM và CPU của máy chủ cơ sở dữ liệu.
- **Explanation**: Giới hạn tài nguyên và bảo vệ hệ thống.
- **Follow-up**: What is the default value of `months` if omitted? (Answer: 6 months).

### Question 7
- **English**: What is the difference between OLTP and OLAP workloads?
- **Vietnamese**: Sự khác nhau giữa khối lượng công việc OLTP và OLAP là gì?
- **Model Answer (EN)**: OLTP (Online Transaction Processing) involves high-frequency, low-latency CRUD operations on individual records. OLAP (Online Analytical Processing) involves complex aggregations and scans across large historical datasets for reporting and business intelligence.
- **Model Answer (VI)**: OLTP (Xử lý giao dịch trực tuyến) bao gồm các thao tác CRUD tần suất cao, độ trễ thấp trên từng bản ghi đơn lẻ. OLAP (Xử lý phân tích trực tuyến) bao gồm các phép tổng hợp và quét phức tạp trên các tập dữ liệu lịch sử lớn phục vụ báo cáo và phân tích kinh doanh.
- **Explanation**: Kiến trúc dữ liệu doanh nghiệp.
- **Follow-up**: At high scale, how do you prevent OLAP queries from slowing down OLTP transactions? (Answer: Direct analytical queries to a read-replica database or dedicated data warehouse).

### Question 8
- **English**: How does `TO_CHAR(transaction_date, 'YYYY-MM')` in PostgreSQL enable grouping by month?
- **Vietnamese**: Hàm `TO_CHAR(transaction_date, 'YYYY-MM')` trong PostgreSQL giúp gom nhóm theo tháng như thế nào?
- **Model Answer (EN)**: It truncates and formats any calendar date into a uniform 7-character string representing the year and month (e.g. `'2026-03'`), allowing the `GROUP BY` clause to aggregate all transactions falling within the same calendar month together.
- **Model Answer (VI)**: Nó định dạng và cắt ngắn ngày lịch thành chuỗi 7 ký tự đại diện cho năm và tháng (ví dụ: `'2026-03'`), cho phép mệnh đề `GROUP BY` gom tất cả các giao dịch diễn ra trong cùng một tháng lại với nhau.
- **Explanation**: Gom nhóm dữ liệu thời gian trong SQL.
- **Follow-up**: What is an alternative PostgreSQL function for truncating dates? (Answer: `DATE_TRUNC('month', transaction_date)`).

### Question 9
- **English**: Why is `NetSavings` computed in Go code rather than in SQL?
- **Vietnamese**: Tại sao `NetSavings` được tính bằng code Go thay vì tính trong câu lệnh SQL?
- **Model Answer (EN)**: Since `totalIncome` and `totalExpense` are already retrieved by the query, computing `netSavings = totalIncome - totalExpense` in Go is a trivial $O(1)$ CPU subtraction, keeping the SQL query simpler and easier to maintain.
- **Model Answer (VI)**: Vì `totalIncome` và `totalExpense` đã được câu query lấy về, việc tính `netSavings = totalIncome - totalExpense` trong Go là một phép trừ số nguyên $O(1)$ siêu nhanh, giúp câu lệnh SQL đơn giản và dễ bảo trì hơn.
- **Explanation**: Phân bổ hợp lý giữa xử lý ở Database vs Backend.
- **Follow-up**: Can `NetSavings` be negative? (Answer: Yes, when expenses exceed income, indicating a financial deficit).

### Question 10
- **English**: What is the danger of returning unrounded float values in JSON APIs?
- **Vietnamese**: Nguy cơ của việc trả về các số thực chưa được làm tròn trong JSON API là gì?
- **Model Answer (EN)**: Unrounded floating-point calculations can produce ugly decimal strings like `33.333333333333336` or cause minor discrepancies across different client platforms and programming languages. Rounding to 2 decimal places provides clean, predictable API contracts.
- **Model Answer (VI)**: Các phép tính số thực chưa làm tròn có thể sinh ra những chuỗi số lẻ dài như `33.333333333333336` hoặc gây ra những sai lệch nhỏ giữa các nền tảng client khác nhau. Làm tròn 2 chữ số thập phân mang lại cấu trúc dữ liệu JSON sạch sẽ và dễ đoán.
- **Explanation**: Thiết kế API chuyên nghiệp (API Polish).
- **Follow-up**: What function in Go converts a float to a fixed decimal string directly? (Answer: `fmt.Sprintf("%.2f", val)`).

---

## 15. Knowledge Connection
- **Kế thừa từ Issue #7, #8, #9**: Tận dụng toàn bộ dữ liệu giao dịch từ Issue #7, kỹ thuật lọc theo ngày từ Issue #8, và dữ liệu hạn mức ngân sách từ Issue #9.
- **Chuẩn bị cho Issue #11 (Concurrent Batch Import)**: Khi người dùng sử dụng tính năng Import file CSV dung lượng lớn ở Issue #11, toàn bộ số liệu báo cáo, tổng kết tháng và cơ cấu chi tiêu ở Issue #10 này sẽ tự động cập nhật ngay lập tức.

---

## 16. Learning Checkpoint

### I Must Understand
1. Cách viết câu lệnh Conditional Aggregation với `SUM(CASE WHEN ...)`.
2. Cách tránh lỗi nhân đôi dòng (Row Multiplication Trap) khi JOIN các bảng quan hệ 1-N.
3. Thuật toán Zero-Filling bằng Go Map và Ordered Slice để tạo dòng thời gian liên tục.
4. Cách làm tròn số thực đến 2 chữ số thập phân bằng `math.Round`.
5. Sự khác biệt giữa khối lượng công việc OLTP và OLAP.

### I Should Be Able To Code
1. Viết câu query SQL tổng hợp nhiều chỉ số đồng thời bằng `CASE WHEN`.
2. Viết thuật toán Zero-Filling cho chuỗi dữ liệu thời gian trong Go.
3. Viết phép tính phần trăm an toàn có kiểm tra chống chia cho 0.

### I Should Be Able To Explain In An Interview
1. Hiện tượng Row Multiplication trong SQL là gì và làm sao để xử lý?
2. Tại sao Go Map lại không đảm bảo thứ tự duyệt và cách khắc phục?
3. Tại sao nên tách riêng `AnalyticsRepository` thay vì gộp chung vào `TransactionRepository`?

### Advanced Topics I Do Not Need Yet
- Materialized Views trong PostgreSQL (bảng tổng hợp được tính sẵn và làm mới định kỳ).
- Window Functions nâng cao (`LAG`, `LEAD`, `ROW_NUMBER`) để tính tỷ lệ tăng trưởng so với tháng trước (Month-over-Month Growth).
- Tích hợp công cụ trực quan hóa dữ liệu như Grafana hoặc Apache Superset.
