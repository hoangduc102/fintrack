# Issue #6 — Category CRUD, Multi-Tenant Ownership & Conflict Handling

## 1. Learning Objectives
Sau khi hoàn thành bài học này, bạn sẽ nắm vững:
- Cách xây dựng trọn vẹn bộ API CRUD (Create, Read, Update, Delete) cho thực thể `Category` trong Go.
- Nguyên tắc cô lập dữ liệu người dùng (Tenant Isolation / Ownership Enforcement) ở tầng SQL Repository.
- Kỹ thuật truy vấn danh sách (`QueryContext`) kết hợp vòng lặp `rows.Next()`, `rows.Scan()` và kiểm tra lỗi trễ `rows.Err()`.
- Kỹ thuật xóa bản ghi và xác minh sự tồn tại bằng `ExecContext` và `res.RowsAffected()`.
- Xử lý tham số truy vấn tùy chọn (Optional Query Parameters `?type=INCOME|EXPENSE`) bằng con trỏ Go (`*domain.CategoryType`).
- Ánh xạ ràng buộc phức hợp Unique Composite Constraint `(user_id, name, type)` sang HTTP 409 Conflict.
- Sử dụng chuẩn HTTP Status `204 No Content` cho thao tác DELETE thành công.

---

## 2. What Problem Does This Issue Solve?
Trong FinTrack, mỗi người dùng có nhu cầu quản lý chi tiêu khác nhau:
1. **Tự do tùy biến**: Người dùng cần quyền tự tạo thêm danh mục (ví dụ: "Cà phê", "Tiền trọ", "Học phí"), đổi tên danh mục cũ, hoặc xóa những danh mục không còn sử dụng.
2. **Ngăn chặn rò rỉ và can thiệp chéo dữ liệu (IDOR / Tenant Breach)**: Đây là lỗi bảo mật kinh điển! Kẻ tấn công có tài khoản A không bao giờ được phép xem, sửa hoặc xóa danh mục của tài khoản B, kể cả khi họ đoán được UUID của danh mục đó.
3. **Chống trùng tên danh mục trong cùng một nhóm**: Trong cùng một tài khoản, người dùng không thể tạo 2 danh mục "Ăn uống" cùng loại "EXPENSE". Tuy nhiên, một danh mục "Thưởng" ở nhóm "INCOME" và một danh mục "Thưởng" ở nhóm "EXPENSE" (ví dụ: chi thưởng cho nhân viên) thì hoàn toàn hợp lệ.

---

## 3. Prerequisite Go Knowledge

### 3.1. Go Interfaces và Polymorphism
- **Khái niệm**: Interface trong Go là tập hợp các chữ ký phương thức (method signatures). Go sử dụng **Implicit Interface Implementation** (Duck typing: *"Nếu nó bơi như vịt, kêu như vịt thì nó là vịt"*). Struct không cần từ khóa `implements`. Chỉ cần struct cài đặt đủ các hàm đã khai báo trong interface, nó sẽ tự động thỏa mãn interface đó.
- **Ví dụ cơ bản**:
```go
type Greeter interface {
    Greet() string
}

type Human struct{}
func (h Human) Greet() string { return "Hello" } // Tự động thỏa mãn Greeter!
```
- **Kết nối FinTrack**: `PostgresCategoryRepository` thỏa mãn interface `CategoryRepository` được định nghĩa trong `internal/repository/category_repository.go`.

### 3.2. Pointers as Optional Function Arguments
- **Khái niệm**: Nếu một tham số có thể có hoặc không (optional), cách tự nhiên nhất trong Go là dùng con trỏ `*T`. Nếu client không truyền, giá trị là `nil`. Nếu có truyền, con trỏ trỏ tới vùng nhớ chứa giá trị.
- **Ví dụ cơ bản**:
```go
func Filter(categoryType *string) {
    if categoryType != nil {
        fmt.Println("Filtering by:", *categoryType)
    }
}
```
- **Kết nối FinTrack**: Trong `FindByUserAndType(..., categoryType *domain.CategoryType)`, nếu `categoryType == nil` nghĩa là lấy tất cả danh mục của user.

### 3.3. Slice of Structs vs Slice of Pointers
- **Khái niệm**: `[]domain.Category` lưu trữ trực tiếp các struct liên tiếp trong bộ nhớ (contiguous memory), rất thân thiện với CPU cache. Khi duyệt bằng `for i := range categories`, ta dùng `&categories[i]` để lấy địa chỉ truyền vào hàm format.
- **Kết nối FinTrack**: Hàm `toCategoryResponse(&categories[i])` trong handler giúp chuyển đổi struct domain sang struct HTTP response mà không copy thừa thãi.

---

## 4. Architecture

Mô hình CRUD Category với nguyên tắc kiểm tra quyền sở hữu tại mỗi tầng:

```text
HTTP Request (GET / PUT / DELETE / POST)
       │
  (AuthMiddleware) ──► Đã xác thực token, gán userID vào gin.Context
       │
       ▼
[Handler: CategoryHandler]
  • Lấy userID qua GetUserID(c)
  • Parse URL Path UUID (cho PUT/DELETE)
  • Bind Request JSON (cho POST/PUT)
       │
       ▼
[Service: CategoryService]
  • Validate Name (TrimSpace, không rỗng, <= 50 ký tự Unicode)
  • Validate Type (phải là INCOME hoặc EXPENSE)
  • Truyền (ctx, catID, userID) xuống Repository
       │
       ▼
[Repository: PostgresCategoryRepository]
  • BẮT BUỘC chèn điều kiện: WHERE user_id = $userID
  • Ngăn chặn hoàn toàn truy cập dữ liệu của user khác
       │
       ▼
[PostgreSQL Database]
  • categories table (CHECK user_id, UNIQUE(user_id, name, type))
```

---

## 5. File-by-File Analysis

### 5.1. `internal/domain/category.go`
- **Responsibility**: Định nghĩa thực thể `Category` và enum `CategoryType`.
- **Fields**:
  - `ID uuid.UUID`: Khóa chính định danh duy nhất.
  - `UserID uuid.UUID`: Khóa ngoại trỏ về `users.id`, xác định chủ sở hữu.
  - `Name string`: Tên danh mục (đã được giới hạn <= 50 ký tự ở service).
  - `Type CategoryType`: Loại thu (`INCOME`) hoặc chi (`EXPENSE`).
- **Interview Importance**: ⭐ SHOULD KNOW.

### 5.2. `internal/repository/category_repository.go`
- **Responsibility**: Khai báo hợp đồng lưu trữ Category.
- **Interface Contract**:
  - `Create(ctx context.Context, category *domain.Category) error`
  - `FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Category, error)`
  - `FindByUserAndType(ctx context.Context, userID uuid.UUID, categoryType *domain.CategoryType) ([]domain.Category, error)`
  - `Update(ctx context.Context, category *domain.Category) error`
  - `Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error`
- **Chỉ điểm kiến trúc quan trọng**: Mọi phương thức đọc/sửa/xóa đều nhận `userID uuid.UUID` làm tham số bắt buộc.
- **Interview Importance**: 🔥 MUST KNOW. Thiết kế bảo mật chống lỗi IDOR ngay từ tầng Repository.

### 5.3. `internal/repository/postgres_category_repository.go`
- **Responsibility**: Cài đặt câu lệnh SQL tương tác trực tiếp với PostgreSQL.
- **Important Syntax & Logic**:
  1. **Tạo mới với RETURNING**:
     ```sql
     INSERT INTO categories (id, user_id, name, type, created_at, updated_at)
     VALUES ($1, $2, $3, $4, $5, $6)
     RETURNING id, created_at, updated_at
     ```
  2. **Quét danh sách với `sql.Rows`**:
     ```go
     defer rows.Close()
     categories := make([]domain.Category, 0)
     for rows.Next() {
         var cat domain.Category
         if err := rows.Scan(&cat.ID, &cat.UserID, &cat.Name, &cat.Type, &cat.CreatedAt, &cat.UpdatedAt); err != nil {
             return nil, err
         }
         categories = append(categories, cat)
     }
     if err := rows.Err(); err != nil {
         return nil, err
     }
     ```
     *Lưu ý*: `rows.Err()` kiểm tra các lỗi mạng xảy ra trong quá trình lặp mà `rows.Next()` gặp phải khi stream dữ liệu từ database.
  3. **Kiểm tra xóa thành công**:
     ```go
     res, err := r.db.ExecContext(ctx, query, id, userID)
     // ...
     rowsAffected, err := res.RowsAffected()
     if rowsAffected == 0 {
         return fmt.Errorf("%w: category not found", domain.ErrNotFound)
     }
     ```
     Nếu `rowsAffected == 0`, hoặc ID không tồn tại, hoặc ID tồn tại nhưng thuộc về user khác! Cả hai trường hợp đều trả về `domain.ErrNotFound` để không làm lộ thông tin cho kẻ tấn công.
- **Interview Importance**: 🔥 MUST KNOW. Kỹ thuật duyệt `sql.Rows`, xử lý `rows.Err()`, và `res.RowsAffected()`.

### 5.4. `internal/service/category_service.go`
- **Responsibility**: Thực thi kiểm tra quy tắc nghiệp vụ (Validation rules).
- **Validation Rules**:
  - `strings.TrimSpace(name)`: Cắt bỏ khoảng trắng vô nghĩa ở hai đầu.
  - `len([]rune(trimmedName)) > 50`: Đếm số ký tự Unicode thực tế (chữ có dấu như tiếng Việt) thay vì chỉ đếm byte thô.
  - Kiểm tra `Type` bắt buộc phải là `INCOME` hoặc `EXPENSE`.
  - Trong `UpdateCategory`, gọi `FindByID` trước để xác minh bản ghi tồn tại và thuộc sở hữu của user trước khi thực hiện cập nhật.
- **Interview Importance**: 🔥 MUST KNOW. Cách đếm độ dài chuỗi Unicode an toàn bằng `[]rune`.

### 5.5. `internal/handler/category_handler.go`
- **Responsibility**: Tiếp nhận request, bóc tách `c.Param("id")`, parse UUID, gọi service, trả về response JSON chuẩn RESTful.
- **HTTP Status Mapping**:
  - `Create`: 201 Created
  - `List`: 200 OK
  - `Update`: 200 OK
  - `Delete`: 204 No Content (`c.Status(http.StatusNoContent)`)
  - Vi phạm unique: 409 Conflict
  - Không tìm thấy hoặc không có quyền: 404 Not Found
- **Interview Importance**: ⭐ SHOULD KNOW. Chuẩn RESTful HTTP status codes.

---

## 6. Concrete Runtime Flow

Chi tiết luồng chạy của một request `DELETE /api/v1/categories/550e8400-e29b-41d4-a716-446655440000`:

```text
1. Client gửi DELETE /api/v1/categories/<uuid> kèm Header "Authorization: Bearer <token-user-A>".
2. AuthMiddleware:
   - Validate token hợp lệ -> Lấy được userID của user-A: "aaa-aaa-aaa".
   - c.Set("user_id", "aaa-aaa-aaa").
   - c.Next().
3. CategoryHandler.Delete:
   - Lấy userID = GetUserID(c) -> "aaa-aaa-aaa".
   - uuid.Parse(c.Param("id")) -> Lấy được catID: "550e8400-...".
   - Gọi categoryService.DeleteCategory(ctx, catID, userID).
4. CategoryService.DeleteCategory:
   - Chuyển tiếp lệnh xuống repository: categoryRepo.Delete(ctx, catID, userID).
5. PostgresCategoryRepository.Delete:
   - Thực thi SQL: DELETE FROM categories WHERE id = $1 AND user_id = $2;
   - Truyền tham số: $1 = catID, $2 = userID-A.
   - res.RowsAffected():
     • Nếu user-A đúng là chủ nhân của danh mục: Database xóa 1 dòng, rowsAffected = 1 -> Return nil.
     • Nếu danh mục thuộc về user-B: Câu lệnh WHERE không khớp dòng nào, rowsAffected = 0.
       -> Trả về domain.ErrNotFound.
6. Handler phản hồi:
   - Nếu thành công: c.Status(http.StatusNoContent) -> Trả về HTTP 204 không có body.
   - Nếu rowsAffected == 0: Trả về HTTP 404 Not Found {"error": {"code":"NOT_FOUND","message":"category not found"}}.
```

---

## 7. Important Go Concepts

### 7.1. Chuỗi ký tự Unicode và `[]rune`
- Trong Go, kiểu `string` thực chất là một chuỗi các bytes (read-only slice of bytes), được mã hóa bằng UTF-8.
- Ký tự tiếng Anh (`'A'`, `'b'`) tốn 1 byte. Nhưng ký tự tiếng Việt (`'á'`, `'ế'`, `'đ'`) tốn 2 hoặc 3 bytes!
- Nếu bạn dùng `len("Ăn uống")`, Go sẽ trả về **10** (bytes), trong khi con người chỉ nhìn thấy **8** ký tự.
- **Kỹ thuật chuẩn**: Chuyển chuỗi sang slice các rune: `len([]rune(str))` để đếm chính xác số lượng ký tự hiển thị.

### 7.2. Lặp qua `sql.Rows` và kiểm tra `rows.Err()`
- Đây là lỗi phỏng vấn kinh điển!
```go
for rows.Next() {
    // scan...
}
if err := rows.Err(); err != nil {
    return nil, err
}
```
- Khi vòng lặp `for rows.Next()` kết thúc, nó có thể kết thúc vì 2 lý do:
  1. Đã duyệt hết tất cả các dòng dữ liệu (bình thường).
  2. Kết nối mạng bị đứt giữa chừng trong lúc server DB đang gửi các dòng dữ liệu tiếp theo!
- `rows.Next()` chỉ trả về `false` khi gặp lỗi, nó không trả về lỗi trực tiếp. Lỗi thực sự được lưu lại bên trong struct `rows`, và ta **bắt buộc** phải gọi `rows.Err()` sau vòng lặp để kiểm tra xem quá trình đọc có trọn vẹn hay không.

### 7.3. `defer rows.Close()`
- Luôn gọi `defer rows.Close()` ngay sau khi kiểm tra `err == nil` của `QueryContext`.
- Nếu quên đóng rows, kết nối DB đó sẽ bị giữ chặt và không bao giờ được trả về pool cho đến khi Garbage Collector thu hồi, gây tê liệt server khi lượng truy cập tăng.

---

## 8. Database / SQL Concepts

### 8.1. Multi-Tenant Scoping (Cô lập người dùng)
- Trong một ứng dụng SaaS/Multi-tenant chia sẻ chung database (Shared Database, Shared Schema), mọi câu truy vấn `SELECT`, `UPDATE`, `DELETE` **bắt buộc phải có `WHERE user_id = $x`**.
- Không bao giờ viết:
  ```sql
  -- CỰC KỲ NGUY HIỂM! Lỗ hổng IDOR!
  DELETE FROM categories WHERE id = $1;
  ```
- Luôn luôn viết:
  ```sql
  -- AN TOÀN TUYỆT ĐỐI
  DELETE FROM categories WHERE id = $1 AND user_id = $2;
  ```

### 8.2. Deterministic Sorting (Sắp xếp đơn định)
Trong `FindByUserAndType`:
```sql
ORDER BY type ASC, name ASC, id ASC
```
- Tại sao cần thêm `id ASC` ở cuối?
- Nếu hai danh mục có cùng loại và cùng tên (trong trường hợp khác bảng), thứ tự trả về giữa các lần query có thể bị xáo trộn tùy thuộc vào execution plan của database engine. Bổ sung khóa chính duy nhất (`id ASC`) đảm bảo kết quả trả về luôn luôn giữ nguyên 100% thứ tự qua các lần gọi API (Deterministic Ordering).

---

## 9. Error Handling

### Tại sao lại trả về 404 khi truy cập tài nguyên của người khác?
- Giả sử User A có danh mục ID `111`. User B cố tình gửi request `DELETE /api/v1/categories/111`.
- Nếu hệ thống trả về **403 Forbidden**: User B biết chắc chắn rằng danh mục `111` có tồn tại trên đời, chỉ là họ không có quyền sửa. Điều này giúp kẻ tấn công dò quét được ID tài nguyên của hệ thống.
- Nếu hệ thống trả về **404 Not Found**: User B hoàn toàn không biết danh mục `111` có thực sự tồn tại hay không. Dữ liệu của User A được ẩn giấu an toàn 100%.

---

## 10. Testing

### 10.1. Kiểm thử phân quyền dữ liệu (Tenant Isolation Test)
Trong `internal/handler/category_handler_test.go`:
- Tạo Category thuộc về User A.
- Đăng nhập với User B, gửi `PUT /api/v1/categories/<id-của-user-A>`.
- Khẳng định API trả về `404 Not Found` và dữ liệu của User A không hề bị thay đổi.

### 10.2. Kiểm thử chống trùng lặp (Conflict Test)
- Gửi tạo danh mục "Ăn uống" (EXPENSE).
- Gửi tiếp request thứ 2 tạo "Ăn uống" (EXPENSE).
- Khẳng định API trả về `409 Conflict`.
- Gửi tiếp request thứ 3 tạo "Ăn uống" nhưng loại (INCOME).
- Khẳng định API trả về `201 Created` thành công.

---

## 11. Beginner Mistakes

### 5 Sai lầm phổ biến của người mới:
1. **Dùng `len(name)` để kiểm tra độ dài tên danh mục**: Tên tiếng Việt như "Ăn uống ngoài quán" sẽ bị đếm sai độ dài, làm người dùng bị báo lỗi quá ký tự dù thực tế họ gõ chưa tới 50 chữ.
2. **Quên `defer rows.Close()`**: Gây rò rỉ connection pool nghiêm trọng.
3. **Quên gọi `rows.Err()` sau vòng lặp `for rows.Next()`**: Bỏ lọt lỗi đứt cáp mạng giữa chừng khi đọc dữ liệu lớn.
4. **Không kiểm tra `res.RowsAffected()` khi DELETE**: Lệnh delete trong SQL nếu không tìm thấy dòng nào vẫn thực thi thành công với 0 dòng bị xóa (`err == nil`). Nếu lập trình viên chỉ check `err != nil`, hệ thống sẽ trả về 204 No Content gây hiểu lầm là đã xóa thành công một bản ghi không tồn tại.
5. **Cập nhật cả `type` của category khi Update**: Thay đổi type từ EXPENSE sang INCOME của một category đã gắn với hàng nghìn giao dịch trong quá khứ sẽ làm sai lệch toàn bộ báo cáo tài chính của người dùng. Chỉ cho phép đổi tên (`name`).

### 3 Nguy cơ sinh bug thực tế:
1. **Lỗ hổng IDOR (Insecure Direct Object References)**: Quên truyền `user_id` vào câu lệnh SQL UPDATE/DELETE, cho phép người dùng bất kỳ sửa đổi dữ liệu của người khác.
2. **SQL Injection do cộng chuỗi query**: Ghép chuỗi `query += " WHERE user_id = " + userID` thay vì truyền biến tham số `$1, $2`.
3. **Mất dấu vết cập nhật**: Quên cập nhật trường `updated_at = time.Now().UTC()` trong câu lệnh UPDATE.

### 2 Cái bẫy phỏng vấn (Interview Traps):
1. *"Tại sao trong Go sau vòng lặp `for rows.Next()` lại bắt buộc phải gọi thêm `rows.Err()`?"* -> **Bẫy**: Rất nhiều ứng viên tưởng rằng `rows.Next()` khi gặp lỗi sẽ panic hoặc trả về error. Thực chất nó chỉ return `false`, lập trình viên phải gọi `rows.Err()` để lấy lỗi ngầm đó ra.
2. *"Khi xóa một tài nguyên mà client không có quyền sở hữu, em trả về 403 Forbidden hay 404 Not Found?"* -> **Bẫy**: Trả về 403 làm lộ sự tồn tại của tài nguyên (Resource Enumeration). Câu trả lời của senior luôn là **404 Not Found**.

---

## 12. Code Reading Exercises

### Bài tập 1: Dự đoán kết quả
Đoạn code sau có lỗi gì nếu mạng DB bị chập chờn?
```go
rows, err := db.Query("SELECT name FROM categories")
if err != nil { return err }
defer rows.Close()

var names []string
for rows.Next() {
    var name string
    rows.Scan(&name)
    names = append(names, name)
}
return names, nil
```
*Gợi ý suy nghĩ*: Nếu quá trình stream dữ liệu bị gián đoạn ở dòng thứ 10/100, hàm này trả về gì? Có báo lỗi cho tầng trên biết không?

### Bài tập 2: Tìm bug bảo mật
Tìm lỗ hổng nghiêm trọng trong hàm sau:
```go
func (r *Repo) UpdateCategoryName(ctx context.Context, id uuid.UUID, newName string) error {
    _, err := r.db.ExecContext(ctx, "UPDATE categories SET name = $1 WHERE id = $2", newName, id)
    return err
}
```

---

## 13. Active Recall

### Câu hỏi ôn tập
1. Duck typing trong Go hoạt động như thế nào qua interface?
2. Tại sao `rows.Err()` bắt buộc phải được gọi sau vòng lặp `rows.Next()`?
3. Khi thực hiện lệnh `DELETE` trong SQL, hàm nào trong Go cho biết có bản ghi nào thực sự bị xóa hay không?
4. Tại sao FinTrack chỉ cho phép người dùng đổi tên Category (`name`) mà không cho phép đổi loại (`type`)?
5. Sự khác biệt giữa `len(str)` và `len([]rune(str))` trong Go là gì?

---

### Model Answers

#### 1. Duck typing trong Go
- **English**: In Go, interface implementation is implicit. A concrete type implements an interface simply by implementing all of its declared methods, without any explicit `implements` keyword.
- **Vietnamese**: Trong Go, việc cài đặt interface là ngầm định. Một kiểu cụ thể thỏa mãn một interface chỉ đơn giản bằng cách cài đặt tất cả các method của interface đó mà không cần dùng từ khóa `implements`.

#### 2. Tại sao gọi `rows.Err()`?
- **English**: `rows.Next()` returns `false` both when all rows have been successfully read and when an error occurs during iteration. Calling `rows.Err()` is the only way to detect mid-stream iteration errors.
- **Vietnamese**: `rows.Next()` trả về `false` trong cả hai trường hợp: đọc hết dữ liệu hoặc gặp lỗi giữa chừng khi duyệt. Gọi `rows.Err()` là cách duy nhất để phát hiện các lỗi phát sinh trong quá trình stream dữ liệu.

#### 3. Kiểm tra số bản ghi bị xóa
- **English**: By calling `res.RowsAffected()` on the `sql.Result` returned by `ExecContext`.
- **Vietnamese**: Bằng cách gọi phương thức `res.RowsAffected()` trên đối tượng `sql.Result` được trả về bởi `ExecContext`.

#### 4. Tại sao không cho đổi `type`?
- **English**: Changing a category type from EXPENSE to INCOME would retroactively invalidate all historical transaction calculations, budgets, and financial analytics associated with that category.
- **Vietnamese**: Đổi loại danh mục từ CHI TIÊU sang THU NHẬP sẽ làm sai lệch hồi tố toàn bộ các tính toán giao dịch lịch sử, ngân sách và báo cáo tài chính gắn liền với danh mục đó.

#### 5. Phân biệt `len(str)` và `len([]rune(str))`
- **English**: `len(str)` returns the number of raw bytes (UTF-8 bytes), whereas `len([]rune(str))` returns the actual number of Unicode code points (characters).
- **Vietnamese**: `len(str)` trả về số lượng bytes thô (bytes UTF-8), trong khi `len([]rune(str))` trả về số lượng ký tự Unicode thực tế hiển thị cho người dùng.

---

## 14. Fresher Go Backend Interview Questions

### Question 1
- **English**: How do you implement Tenant Isolation in a relational database when using Go?
- **Vietnamese**: Bạn thực hiện cô lập người dùng (Tenant Isolation) trong cơ sở dữ liệu quan hệ như thế nào khi dùng Go?
- **Model Answer (EN)**: Always include the authenticated `user_id` extracted from the JWT context into the `WHERE` clause of every `SELECT`, `UPDATE`, and `DELETE` query at the repository layer.
- **Model Answer (VI)**: Luôn đưa `user_id` đã được xác thực từ JWT context vào mệnh đề `WHERE` của tất cả các câu lệnh `SELECT`, `UPDATE`, và `DELETE` ở tầng repository.
- **Explanation**: Ngăn chặn hoàn toàn lỗi bảo mật IDOR.
- **Follow-up**: What if you need admin users who can view all tenants? (Answer: Implement role checks in the service layer or separate admin repositories).

### Question 2
- **English**: What is the difference between `QueryContext` and `ExecContext` in `database/sql`?
- **Vietnamese**: Sự khác nhau giữa `QueryContext` và `ExecContext` trong gói `database/sql` là gì?
- **Model Answer (EN)**: `QueryContext` is used for queries that return rows (like `SELECT`), returning a `*sql.Rows` cursor. `ExecContext` is used for statements that do not return result rows (like `INSERT`, `UPDATE`, `DELETE`), returning a `sql.Result` with `RowsAffected()` and `LastInsertId()`.
- **Model Answer (VI)**: `QueryContext` dùng cho các truy vấn trả về các dòng dữ liệu (như `SELECT`), trả về con trỏ `*sql.Rows`. `ExecContext` dùng cho các lệnh không trả về dòng dữ liệu (như `INSERT`, `UPDATE`, `DELETE`), trả về `sql.Result` chứa thông tin `RowsAffected()` và `LastInsertId()`.
- **Explanation**: Chọn đúng API tương ứng với mục đích SQL.
- **Follow-up**: When should you use `QueryRowContext`? (Answer: When you expect at most one single row, such as selecting by ID or using `RETURNING`).

### Question 3
- **English**: Why does `CategoryHandler` return HTTP 204 for DELETE instead of HTTP 200 with a message?
- **Vietnamese**: Tại sao `CategoryHandler` trả về HTTP 204 khi DELETE thay vì HTTP 200 kèm tin nhắn?
- **Model Answer (EN)**: HTTP 204 No Content is the standard RESTful status code for actions that succeed without needing to return a payload, saving network bandwidth and adhering strictly to HTTP specifications.
- **Model Answer (VI)**: HTTP 204 No Content là mã trạng thái chuẩn RESTful cho các thao tác thành công mà không cần trả về nội dung (payload), giúp tiết kiệm băng thông mạng và tuân thủ chặt chẽ đặc tả HTTP.
- **Explanation**: Thiết kế API chuẩn quốc tế.
- **Follow-up**: Can an HTTP 204 response contain a response body? (Answer: No, the HTTP specification forbids a body in 204 responses).

### Question 4
- **English**: What is the purpose of `res.RowsAffected()` after a DELETE statement?
- **Vietnamese**: Mục đích của `res.RowsAffected()` sau một câu lệnh DELETE là gì?
- **Model Answer (EN)**: In SQL, executing a `DELETE` on a non-existent row does not return an error (`err == nil`). Calling `res.RowsAffected()` allows Go to verify if a row was actually deleted; if 0, we can return `domain.ErrNotFound`.
- **Model Answer (VI)**: Trong SQL, thực thi lệnh `DELETE` trên một dòng không tồn tại không hề sinh lỗi (`err == nil`). Gọi `res.RowsAffected()` cho phép Go kiểm tra xem có dòng nào thực sự bị xóa hay không; nếu là 0, ta có thể báo lỗi `domain.ErrNotFound`.
- **Explanation**: Tránh hiện tượng false positive khi xóa dữ liệu.
- **Follow-up**: Do all database drivers support `LastInsertId()`? (Answer: PostgreSQL driver does not support `LastInsertId()`, you must use `RETURNING id` instead).

### Question 5
- **English**: How does FinTrack handle optional query parameters like `?type=INCOME`?
- **Vietnamese**: FinTrack xử lý tham số truy vấn tùy chọn như `?type=INCOME` như thế nào?
- **Model Answer (EN)**: The handler checks `c.Query("type")`. If present, it creates a pointer to a typed `domain.CategoryType` and passes it to the service. The repository dynamically switches between queries with and without the `AND type = $2` condition based on whether the pointer is nil.
- **Model Answer (VI)**: Handler kiểm tra `c.Query("type")`. Nếu có, nó tạo một con trỏ kiểu `domain.CategoryType` và truyền xuống service. Repository sẽ căn cứ vào con trỏ có `nil` hay không để linh hoạt chạy câu SQL có hoặc không có điều kiện `AND type = $2`.
- **Explanation**: Xử lý tham số tùy chọn bằng con trỏ trong Go.
- **Follow-up**: What if the client provides an invalid type string like `?type=FOO`? (Answer: The service validates against allowed enums and returns a `ValidationError`).

### Question 6
- **English**: Why does FinTrack use `RETURNING updated_at` in the Category UPDATE query?
- **Vietnamese**: Tại sao FinTrack lại dùng `RETURNING updated_at` trong câu query UPDATE Category?
- **Model Answer (EN)**: Using `RETURNING updated_at` directly in `QueryRowContext` allows updating and retrieving the timestamp in a single round-trip, while also confirming the row was found and matched the `user_id` ownership constraint.
- **Model Answer (VI)**: Dùng `RETURNING updated_at` trực tiếp trong `QueryRowContext` cho phép vừa cập nhật vừa lấy lại timestamp trong đúng 1 lượt mạng, đồng thời xác nhận rằng dòng đó có tồn tại và thuộc quyền sở hữu của user.
- **Explanation**: Tối ưu số lượng round-trip tới database.
- **Follow-up**: What error does `QueryRowContext` return if no row matched the WHERE clause? (Answer: `sql.ErrNoRows`).

### Question 7
- **English**: Why do we create an empty slice with `make([]domain.Category, 0)` instead of leaving it `nil`?
- **Vietnamese**: Tại sao ta lại tạo slice rỗng bằng `make([]domain.Category, 0)` thay vì để nó là `nil`?
- **Model Answer (EN)**: When serialized to JSON, a `nil` slice encodes as `null`, whereas an initialized empty slice `make([]T, 0)` encodes as an empty array `[]`. Frontends generally expect `[]` rather than `null` for list endpoints.
- **Model Answer (VI)**: Khi serialize sang JSON, một slice `nil` sẽ biến thành chữ `null`, trong khi một slice rỗng `make([]T, 0)` sẽ biến thành mảng rỗng `[]`. Phía Frontend hầu như luôn kỳ vọng nhận được `[]` cho các API danh sách để tránh lỗi `map is not a function`.
- **Explanation**: Tính thân thiện giữa Backend Go và Frontend React/Vue.
- **Follow-up**: Does `len(s)` work on a nil slice? (Answer: Yes, it safely returns 0).

### Question 8
- **English**: How do you prevent SQL Injection when querying with dynamic parameters?
- **Vietnamese**: Làm thế nào để chống tấn công SQL Injection khi truy vấn với các tham số động?
- **Model Answer (EN)**: Never concatenate raw strings into SQL queries. Always use parameterized placeholders (`$1`, `$2` in PostgreSQL) and pass the arguments into `QueryContext` or `ExecContext`.
- **Model Answer (VI)**: Tuyệt đối không cộng chuỗi thô vào câu lệnh SQL. Luôn sử dụng các ký tự giữ chỗ có tham số hóa (`$1`, `$2` trong PostgreSQL) và truyền các biến vào `QueryContext` hoặc `ExecContext`.
- **Explanation**: Nguyên tắc bảo mật cơ sở dữ liệu cốt lõi.
- **Follow-up**: Does the database driver automatically escape strings passed as query arguments? (Answer: Yes, the driver sends parameters out-of-band via the PostgreSQL binary protocol).

### Question 9
- **English**: What is the difference between string assignment and pointer assignment in Go?
- **Vietnamese**: Sự khác nhau giữa việc gán chuỗi và gán con trỏ trong Go là gì?
- **Model Answer (EN)**: Assigning a string copies the 16-byte string header (pointer to underlying array + length). Assigning a pointer copies only an 8-byte memory address pointing to the original value, allowing multiple variables to reference and mutate the same data.
- **Model Answer (VI)**: Gán một chuỗi sẽ copy 16-byte header của chuỗi (gồm con trỏ tới mảng byte ngầm bên dưới + độ dài). Gán một con trỏ chỉ copy 8-byte địa chỉ bộ nhớ trỏ tới giá trị gốc, cho phép nhiều biến cùng tham chiếu và sửa đổi cùng một dữ liệu.
- **Explanation**: Hiểu về memory layout trong Go.
- **Follow-up**: Are strings mutable in Go? (Answer: No, strings in Go are strictly immutable).

### Question 10
- **English**: Why does FinTrack validate category name length with `len([]rune(name)) <= 50`?
- **Vietnamese**: Tại sao FinTrack kiểm tra độ dài tên danh mục bằng `len([]rune(name)) <= 50`?
- **Model Answer (EN)**: Go strings are byte slices. Multi-byte characters (such as accented Vietnamese letters or emojis) consume 2 to 4 bytes each. Converting to `[]rune` counts actual Unicode characters rather than bytes, providing accurate length validation for the user.
- **Model Answer (VI)**: Chuỗi trong Go là các mảng byte. Các ký tự đa byte (như chữ tiếng Việt có dấu hoặc icon emoji) tốn từ 2 đến 4 bytes mỗi chữ. Chuyển sang `[]rune` giúp đếm chính xác số ký tự Unicode thay vì đếm số byte, giúp thông báo lỗi độ dài chính xác cho người dùng.
- **Explanation**: Hỗ trợ chuẩn quốc tế hóa (i18n / Unicode handling).
- **Follow-up**: What is the type of a `rune` in Go? (Answer: An alias for `int32`).

---

## 15. Knowledge Connection
- **Kế thừa từ Issue #5**: Sử dụng `AuthMiddleware` và helper `GetUserID(c)` để lấy định danh người dùng sở hữu danh mục.
- **Chuẩn bị cho Issue #7 (Transaction Management)**: Khi tạo một bản ghi thu/chi, trường `category_id` sẽ tham chiếu đến các danh mục được quản lý bởi Issue #6 này.
- **Chuẩn bị cho Issue #9 (Budget Management)**: Ngân sách chi tiêu hàng tháng sẽ được thiết lập theo từng danh mục cụ thể (`category_id`) thuộc về người dùng.

---

## 16. Learning Checkpoint

### I Must Understand
1. Bản chất của Duck Typing và tính đa hình thông qua Interface trong Go.
2. Tại sao mọi câu lệnh truy vấn dữ liệu cá nhân phải có `WHERE user_id = $userID`.
3. Cách duyệt `sql.Rows` an toàn với `rows.Next()`, `rows.Scan()`, `rows.Err()`, và `defer rows.Close()`.
4. Cách kiểm tra số dòng bị ảnh hưởng bằng `res.RowsAffected()`.
5. Sự khác biệt giữa `len(str)` và `len([]rune(str))` khi làm việc với chuỗi tiếng Việt.

### I Should Be Able To Code
1. Viết một hàm Repository thực hiện thao tác xóa có kiểm tra quyền sở hữu và `RowsAffected`.
2. Viết một hàm Service kiểm tra độ dài chuỗi Unicode bằng `[]rune`.
3. Viết một Handler ánh xạ HTTP Status 204 No Content cho API DELETE.

### I Should Be Able To Explain In An Interview
1. Tại sao cần gọi `rows.Err()` sau vòng lặp `rows.Next()`?
2. Tại sao nên trả về HTTP 404 thay vì 403 khi người dùng truy cập ID của người khác?
3. Tại sao không nên cho phép đổi `type` của danh mục sau khi đã tạo?

### Advanced Topics I Do Not Need Yet
- Row-Level Security (RLS) của PostgreSQL (chính sách phân quyền tự động ở tầng database engine).
- Soft Delete pattern (thêm cột `deleted_at` thay vì xóa vĩnh viễn dòng dữ liệu bằng SQL `DELETE`).
- Phân trang dạng Cursor-based cho danh sách Category nếu số lượng danh mục của một user lên tới hàng nghìn.
