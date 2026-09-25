# Issue #5 — JWT Authentication, Auth Middleware & Protected Profile Route

## 1. Learning Objectives
Sau khi hoàn thành bài học này, bạn sẽ nắm vững:
- Cơ chế xác thực phi trạng thái (Stateless Authentication) bằng JSON Web Token (JWT) thuật toán HMAC-SHA256 (`HS256`).
- Cách xây dựng và quản lý JWT Token bằng thư viện chuẩn `github.com/golang-jwt/jwt/v5`.
- Cách viết Gin HTTP Middleware (`gin.HandlerFunc`) để bảo vệ các private routes.
- Kỹ thuật Parse Header `Authorization: Bearer <token>` chuẩn RFC 6750 bằng `strings.Fields` và `strings.EqualFold`.
- Cơ chế lưu trữ và trích xuất dữ liệu trong ngữ cảnh request (`c.Set` / `c.Get` và Type Assertion trong Go).
- Luồng ngắt request chuẩn trong Gin bằng `c.AbortWithStatusJSON` so với `return`.
- Viết Unit Test cho Middleware và Endpoint được bảo vệ bằng `net/http/httptest`.

---

## 2. What Problem Does This Issue Solve?
Sau khi đã có tài khoản (Issue #4), ứng dụng cần giải quyết 3 bài toán lớn:
1. **Xác thực danh tính khi đăng nhập (Login)**: So sánh mật khẩu người dùng gửi lên với chuỗi hash bcrypt trong cơ sở dữ liệu.
2. **Chứng chỉ phiên không trạng thái (Stateless Session)**: Trong kiến trúc microservices hoặc REST API hiện đại, lưu session trong RAM server hoặc Redis tạo ra chi phí đồng bộ hóa. FinTrack sử dụng JWT token chứa mã định danh người dùng (`sub`) có hạn 24 giờ. Server chỉ cần giải mã và kiểm tra chữ ký số mà không cần query database mỗi khi kiểm tra token.
3. **Bảo vệ tài nguyên cá nhân (Multi-tenant Isolation)**: Tất cả các API xem thông tin cá nhân (`/api/v1/auth/me`), xem danh mục (`/categories`), và xem giao dịch (`/transactions`) bắt buộc phải chặn các request không có token hoặc token giả mạo, đồng thời tự động truyền `user_id` đã được xác thực vào Handler.

---

## 3. Prerequisite Go Knowledge

### 3.1. Go `time.Duration`
- **Khái niệm**: Trong Go, thời gian chênh lệch (duration) là một kiểu dữ liệu có định kiểu mạnh (`time.Duration`), bản chất là số nanoseconds (`int64`). Bạn biểu diễn thời gian qua các hằng số như `time.Second`, `time.Minute`, `time.Hour`.
- **Ví dụ cơ bản**:
```go
ttl := 24 * time.Hour
expireAt := time.Now().Add(ttl)
```
- **Kết nối FinTrack**: Trong `JWTManager`, `duration time.Duration` lưu thời hạn sống của token (ví dụ: `24 * time.Hour`).

### 3.2. First-class Functions và Higher-Order Functions
- **Khái niệm**: Trong Go, hàm là công dân hạng nhất (first-class citizen). Hàm có thể được gán cho biến, truyền vào như tham số, hoặc trả về từ một hàm khác. Hàm trả về một hàm thường được dùng để cấu hình Middleware (Closure).
- **Ví dụ cơ bản**:
```go
func Greeter(prefix string) func(string) string {
    return func(name string) string {
        return prefix + " " + name
    }
}
```
- **Kết nối FinTrack**: `AuthMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc` nhận dependency `jwtManager` và trả về một hàm middleware ẩn danh `func(c *gin.Context)` ghi nhớ dependency đó.

### 3.3. Type Assertion (`val.(Type)`)
- **Khái niệm**: Interface rỗng `interface{}` (hoặc `any` từ Go 1.18) có thể chứa bất kỳ kiểu dữ liệu nào. Để lấy lại giá trị gốc có kiểu cụ thể, Go sử dụng cú pháp Type Assertion: `v, ok := x.(T)`.
- **Ví dụ cơ bản**:
```go
var data any = "hello"
str, ok := data.(string)
if !ok {
    // không phải string!
}
```
- **Kết nối FinTrack**: Trong `GetUserID(c *gin.Context)`, `c.Get("user_id")` trả về `any`. Ta dùng `userID, ok := val.(uuid.UUID)` để ép kiểu an toàn về `uuid.UUID`.

---

## 4. Architecture

Kiến trúc xác thực và luồng bảo vệ route trong FinTrack:

```text
Public Route: POST /api/v1/auth/login
Client ────► Router ────► AuthHandler.Login ────► AuthService.Login ────► PostgresUserRepository
                                                         │
                                                  (Bcrypt Compare)
                                                         │
                                                  (JWTManager.Generate)
                                                         │
Client ◄──── Trả về { "token": "ey..." } ◄───────────────┘

─────────────────────────────────────────────────────────────────────────────

Protected Route: GET /api/v1/auth/me
Client (Header: "Authorization: Bearer <token>")
  │
  ▼
[Router Engine]
  │
  ▼
[AuthMiddleware]
  • Đọc Header "Authorization"
  • Tách chuỗi bằng strings.Fields
  • Kiểm tra prefix "Bearer" (case-insensitive)
  • Gọi JWTManager.Validate(token)
  • NẾU LỖI: c.AbortWithStatusJSON(401) ──► DỪNG NGAY
  • NẾU HỢP LỆ: c.Set("user_id", userID)
  • c.Next()
  │
  ▼
[AuthHandler.Me]
  • userID, err := GetUserID(c)
  • Gọi AuthService.GetProfile(ctx, userID)
  • Trả về HTTP 200 OK cùng UserResponse
```

---

## 5. File-by-File Analysis

### 5.1. `internal/auth/jwt.go`
- **Responsibility**: Khởi tạo, ký (sign), giải mã (parse) và xác thực (validate) JWT token.
- **Important Syntax & Logic**:
  - `JWTManager struct`: Đóng gói `secretKey []byte` và `duration time.Duration`.
  - `jwt.RegisteredClaims`: Sử dụng các claim chuẩn theo RFC 7519: `Subject` (lưu User UUID), `IssuedAt`, `ExpiresAt`.
  - `jwt.NewWithClaims(jwt.SigningMethodHS256, claims)`: Khởi tạo token với thuật toán HMAC-SHA256.
  - `Keyfunc Callback`:
    ```go
    token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return m.secretKey, nil
    })
    ```
    Đây là kiểm tra an ninh sống còn: Ngăn chặn cuộc tấn công "Algorithm Confusion" (khi hacker cố tình đổi thuật toán sang `none` hoặc RSA Public Key).
- **Interview Importance**: 🔥 MUST KNOW. Kỹ thuật chống "alg: none" và thuật toán ký JWT.

### 5.2. `internal/handler/middleware.go`
- **Responsibility**: Chứa Gin middleware kiểm tra quyền truy cập và helper function `GetUserID`.
- **Important Syntax & Logic**:
  - `const AuthPayloadKey = "user_id"`: Tránh "magic string" khi truy xuất context.
  - `strings.Fields(authHeader)`: Tự động tách chuỗi theo khoảng trắng mà không sợ người dùng gõ nhiều dấu cách (`"Bearer    ey..."`).
  - `strings.EqualFold(fields[0], "Bearer")`: So sánh không phân biệt hoa thường (`bearer`, `Bearer`, `BEARER`).
  - `c.AbortWithStatusJSON(...)`: Báo cho Gin ngắt chuỗi middleware ngay lập tức, không thực thi các handler phía sau.
  - `GetUserID(c *gin.Context)`: Hàm dùng chung cho tất cả các handler cần định danh người dùng.
- **Interview Importance**: 🔥 MUST KNOW. Phân biệt `c.Abort()` vs `return` và cơ chế Type Assertion.

### 5.3. `internal/service/auth_service.go` (`Login` & `GetProfile`)
- **Responsibility**:
  - `Login`: Tìm kiếm user bằng email, so khớp mật khẩu bằng `bcrypt.CompareHashAndPassword`, sinh JWT token thông qua `jwtManager.Generate`.
  - `GetProfile`: Lấy thông tin user từ `UserRepository.FindByID`.
- **Security Logic**:
  - Khi email không tồn tại hoặc mật khẩu sai, **luôn trả về cùng một lỗi**: `domain.ErrInvalidCredentials`. Không bao giờ thông báo "Email không tồn tại" ở màn hình đăng nhập để chống tấn công dò quét tài khoản (User Enumeration Attack).
- **Interview Importance**: 🔥 MUST KNOW. Bảo mật đăng nhập và nguyên tắc chống User Enumeration.

### 5.4. `internal/handler/auth_handler.go` (`Login` & `Me`)
- **Responsibility**:
  - `Login`: Tiếp nhận JSON `{ email, password }`, gọi `AuthService.Login`, trả về `{ token: "..." }`.
  - `Me`: Gọi `GetUserID(c)`, gọi `AuthService.GetProfile`, trả về JSON thông tin người dùng.
- **Interview Importance**: ⭐ SHOULD KNOW. Mapping lỗi 401 Unauthorized và 404 Not Found.

---

## 6. Concrete Runtime Flow

### Kịch bản A: Đăng nhập thành công và lấy Token
1. Client gửi `POST /api/v1/auth/login` với body: `{"email":"user@fintrack.io","password":"securePassword123"}`.
2. `AuthHandler.Login` bind dữ liệu vào struct `LoginRequest`.
3. Gọi `AuthService.Login`:
   - Chuẩn hóa email về `user@fintrack.io`.
   - Gọi `userRepo.FindByEmail(ctx, email)`. Tìm thấy bản ghi với `PasswordHash`.
   - Gọi `bcrypt.CompareHashAndPassword(hash, plainPassword)`. Hash trùng khớp (err == nil).
   - Gọi `jwtManager.Generate(user.ID)`.
   - Tạo `jwt.RegisteredClaims` với `Subject = user.ID.String()`, thời hạn 24 giờ.
   - Ký bằng `m.secretKey` -> sinh chuỗi `ey...`.
4. `AuthHandler` trả về HTTP 200: `{"token": "ey..."}`.

### Kịch bản B: Truy cập API cần bảo vệ (`GET /api/v1/auth/me`)
1. Client gửi `GET /api/v1/auth/me` với header `Authorization: Bearer ey...`.
2. Request đi vào `AuthMiddleware`:
   - Đọc header: `"Authorization: Bearer ey..."`.
   - `strings.Fields` tách thành `["Bearer", "ey..."]` (đúng 2 phần tử).
   - `strings.EqualFold` xác nhận là `"Bearer"`.
   - Gọi `jwtManager.Validate("ey...")`.
   - JWT library xác minh chữ ký HMAC-SHA256 với secret key và kiểm tra hạn sử dụng `exp > now`.
   - Đọc claim `Subject`, parse chuỗi thành `uuid.UUID`.
   - Lưu vào context của Gin: `c.Set("user_id", parsedUUID)`.
   - Gọi `c.Next()` để chuyển sang handler tiếp theo.
3. `AuthHandler.Me` được thực thi:
   - Gọi `GetUserID(c)`: Trích xuất `c.Get("user_id")`, ép kiểu sang `uuid.UUID` thành công.
   - Gọi `authService.GetProfile(ctx, userID)`.
   - Trả về HTTP 200 OK với profile người dùng.

---

## 7. Important Go Concepts

### 7.1. Gin Context Values (`c.Set` / `c.Get`)
- `gin.Context` duy trì một danh sách key-value nội bộ (`Keys map[string]any`).
- Dữ liệu lưu trong `c.Set` chỉ tồn tại trong vòng đời của duy nhất HTTP request đó.
- Khi lấy ra bằng `c.Get`, giá trị trả về là `any`. Lập trình viên bắt buộc phải kiểm tra tồn tại và ép kiểu (Type Assertion):
```go
val, exists := c.Get("user_id")
if !exists { /* lỗi */ }
userID, ok := val.(uuid.UUID)
if !ok { /* sai kiểu */ }
```

### 7.2. Tại sao phải có cả `c.AbortWithStatusJSON()` VÀ `return`?
- **Nguyên lý của Gin**: `c.Abort()` chỉ đánh dấu một cờ nội bộ `index = abortIndex` để Gin không gọi các Middleware hoặc Handler *kế tiếp*.
- **Tuy nhiên**: Bản thân hàm Go hiện tại vẫn sẽ tiếp tục chạy hết các dòng code phía dưới nếu bạn không viết lệnh `return`!
- **Hậu quả nếu quên `return`**: Code bên dưới vẫn chạy tiếp, có thể gây panic hoặc cố gắng ghi response lần thứ 2 (dẫn tới lỗi "http: superfluous response.WriteHeader call").

### 7.3. Type Alias và Custom Types
- Go cho phép định nghĩa kiểu dữ liệu mới từ kiểu cơ bản (ví dụ: `type CategoryType string`). Điều này ngăn ngừa việc vô tình truyền nhầm một string bất kỳ vào hàm yêu cầu Enum hợp lệ.

---

## 8. Database / SQL Concepts

### 8.1. Tìm kiếm không phân biệt hoa thường trong PostgreSQL
Trong `FindByEmail`:
```sql
SELECT id, email, password_hash, display_name, created_at, updated_at
FROM users
WHERE LOWER(email) = LOWER($1)
```
- Dùng hàm `LOWER()` ở cả hai vế đảm bảo truy vấn tìm thấy user bất kể người dùng nhập `User@domain.com` hay `user@domain.com`.
- Trong Issue #3, chúng ta đã tạo index `CREATE INDEX idx_users_email_lower ON users (LOWER(email));` để biến câu query này thành một thao tác Index Scan cực nhanh $O(\log N)$.

### 8.2. Xử lý `sql.ErrNoRows`
- Khi hàm `QueryRowContext` không tìm thấy bản ghi nào khớp điều kiện, nó trả về lỗi đặc biệt `sql.ErrNoRows`.
- Repository FinTrack bắt lỗi này và chuẩn hóa thành Domain Sentinel Error:
```go
if errors.Is(err, sql.ErrNoRows) {
    return nil, fmt.Errorf("%w: user not found", domain.ErrNotFound)
}
```

---

## 9. Error Handling

### Ma trận xử lý lỗi xác thực:
| Tình huống | Lỗi tầng Service | HTTP Status | Response Message | Lý do bảo mật |
| :--- | :--- | :--- | :--- | :--- |
| Email không tồn tại | `domain.ErrInvalidCredentials` | 401 | `"invalid email or password"` | Chống User Enumeration |
| Sai mật khẩu | `domain.ErrInvalidCredentials` | 401 | `"invalid email or password"` | Không cho biết sai email hay mật khẩu |
| Thiếu header Auth | N/A (bị chặn tại Middleware) | 401 | `"missing authorization header"` | Chặn ngay tại cổng vào |
| Token hết hạn | `domain.ErrUnauthorized` | 401 | `"invalid or expired token"` | Không chấp nhận token quá hạn |
| Chữ ký token sai | `domain.ErrUnauthorized` | 401 | `"invalid or expired token"` | Chống giả mạo payload |

---

## 10. Testing

### 10.1. Kiểm thử JWT Manager độc lập
Trong `internal/auth/jwt_test.go`:
- Test Generate và Validate thành công với UUID hợp lệ.
- Test Token hết hạn (truyền duration âm `-1 * time.Hour`).
- Test Token bị ký bằng secret key khác (giả lập hacker).
- Test Token bị chỉnh sửa payload (tampered string).

### 10.2. Kiểm thử Auth Middleware với `httptest.ResponseRecorder`
```go
w := httptest.NewRecorder()
c, r := gin.CreateTestContext(w)

r.Use(AuthMiddleware(jwtManager))
r.GET("/protected", func(c *gin.Context) {
    userID, _ := GetUserID(c)
    c.String(http.StatusOK, userID.String())
})

req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
req.Header.Set("Authorization", "Bearer "+validToken)
r.ServeHTTP(w, req)

assert.Equal(t, http.StatusOK, w.Code)
```

---

## 11. Beginner Mistakes

### 5 Sai lầm phổ biến của người mới:
1. **Quên gọi `return` sau `c.AbortWithStatusJSON`**: Nghĩ rằng `c.Abort()` sẽ dừng hàm Go lập tức giống như lệnh `return`.
2. **Lưu dữ liệu nhạy cảm trong JWT Claims**: Quên mất rằng JWT chỉ được *ký* (signed) chứ không hề được *mã hóa* (encrypted). Bất kỳ ai có token đều có thể base64-decode xem được payload! Tuyệt đối không lưu mật khẩu hoặc thông tin thẻ tín dụng trong JWT.
3. **Hardcode JWT Secret Key trong mã nguồn**: Đưa secret key vào code rồi commit lên GitHub. Phải nạp từ biến môi trường `JWT_SECRET`.
4. **Không kiểm tra thuật toán ký (`token.Method`)**: Chấp nhận token mà không kiểm tra xem nó có đúng là HMAC hay không, dẫn tới lỗ hổng nguy hiểm "alg: none".
5. **So sánh token header bằng `strings.HasPrefix(h, "Bearer ")` đơn giản**: Dễ bị lỗi nếu client gửi nhiều dấu space hoặc tab giữa chữ "Bearer" và token.

### 3 Nguy cơ sinh bug thực tế:
1. **Panic do ép kiểu không an toàn**: Viết `c.MustGet("user_id").(uuid.UUID)` mà không dùng cú pháp 2 biến `val, ok := ...`. Nếu middleware bị lỗi hoặc route chưa được gắn middleware, app sẽ bị panic sập server.
2. **Token không có hạn sử dụng (`ExpiresAt`)**: Tạo token vĩnh viễn khiến hacker nếu đánh cắp được token sẽ có quyền truy cập tài khoản mãi mãi.
3. **Lệch múi giờ khi so khớp hạn dùng**: Không quy đổi thời gian về UTC hoặc so sánh chênh lệch giữa server tạo token và server kiểm tra token.

### 2 Cái bẫy phỏng vấn (Interview Traps):
1. *"JWT có an toàn hơn Session lưu trong Database/Redis không?"* -> **Bẫy**: Không có cái nào "an toàn hơn", chỉ là đánh đổi (trade-off). JWT stateless nhẹ server nhưng nhược điểm chí mạng là **không thể thu hồi tức thì (Instant Revocation)** trước khi nó hết hạn (trừ khi duy trì blacklist trong Redis, khi đó lại mất tính chất stateless).
2. *"Tại sao trong hàm `Login` khi sai email hoặc sai mật khẩu lại trả về cùng một thông báo?"* -> **Bẫy**: Nếu trả về "Email not found", kẻ tấn công sẽ dùng từ điển email để dò xem công ty/hệ thống có những user nào đang sử dụng dịch vụ (User Enumeration Attack).

---

## 12. Code Reading Exercises

### Bài tập 1: Dự đoán kết quả
Điều gì sẽ xảy ra khi chạy đoạn code middleware sau?
```go
func DummyMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        if c.GetHeader("X-Api-Key") == "" {
            c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
        }
        println("Processed successfully!")
        c.Next()
    }
}
```
*Gợi ý suy nghĩ*: Dòng chữ `"Processed successfully!"` có được in ra màn hình khi header bị thiếu không?

### Bài tập 2: Tìm lỗ hổng bảo mật
Đoạn code parse JWT sau có lỗ hổng gì?
```go
func ParseToken(tokenStr string, secret []byte) (*jwt.Token, error) {
    return jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
        return secret, nil
    })
}
```
*Gợi ý suy nghĩ*: Nếu một kẻ tấn công gửi một token có Header `{"alg": "none"}` thì hàm này có xác thực thành công không?

---

## 13. Active Recall

### Câu hỏi ôn tập
1. Cấu trúc của một chuỗi JWT gồm mấy phần và được ngăn cách bởi ký tự gì?
2. Tại sao trong hàm `Validate` của `JWTManager`, chúng ta phải kiểm tra `token.Method.(*jwt.SigningMethodHMAC)`?
3. Sự khác nhau giữa `c.Abort()` và `c.Next()` trong Gin Middleware là gì?
4. Tại sao hàm `GetUserID` lại trả về `(uuid.UUID, error)` thay vì chỉ trả về `uuid.UUID`?
5. Ưu điểm và nhược điểm lớn nhất của việc dùng Stateless JWT thay vì Database Session là gì?

---

### Model Answers

#### 1. Cấu trúc JWT
- **English**: A JWT consists of 3 parts separated by dots (`.`): Header (algorithm & token type), Payload (claims/data), and Signature.
- **Vietnamese**: JWT gồm 3 phần ngăn cách bởi dấu chấm (`.`): Header (thuật toán và kiểu token), Payload (dữ liệu/claims), và Signature (chữ ký số).

#### 2. Tại sao phải kiểm tra `SigningMethodHMAC`?
- **English**: To prevent algorithm confusion attacks where an attacker changes the algorithm to `none` or an asymmetric public key to forge arbitrary valid tokens.
- **Vietnamese**: Ngăn chặn cuộc tấn công tráo đổi thuật toán (Algorithm Confusion), khi hacker đổi thuật toán sang `none` hoặc dùng Public Key để làm giả chữ ký hợp lệ.

#### 3. Phân biệt `c.Abort()` và `c.Next()`
- **English**: `c.Next()` invokes the remaining handlers in the chain. `c.Abort()` stops the execution of pending handlers in the chain for the current request.
- **Vietnamese**: `c.Next()` tiếp tục gọi các handler/middleware kế tiếp trong chuỗi. `c.Abort()` ngăn không cho các handler phía sau được chạy.

#### 4. Tại sao `GetUserID` trả về error?
- **English**: Because the context might not contain the key (if the handler was inadvertently routed without AuthMiddleware) or the stored value might have an unexpected type. Returning an error avoids unexpected panics.
- **Vietnamese**: Vì context có thể không chứa key (nếu route vô tình không được gắn AuthMiddleware) hoặc giá trị lưu trữ bị sai kiểu. Trả về error giúp xử lý an toàn và tránh gây panic sập server.

#### 5. Đánh đổi giữa JWT và Database Session
- **English**: Benefit: Scalable and stateless, eliminating database lookups on every request. Drawback: Cannot easily be revoked or invalidated immediately before expiration without maintaining a token blacklist.
- **Vietnamese**: Ưu điểm: Khả năng mở rộng cao, không tốn tài nguyên tra cứu database ở mỗi request. Nhược điểm: Rất khó thu hồi hoặc hủy token ngay lập tức trước khi nó hết hạn nếu không dùng thêm blacklist.

---

## 14. Fresher Go Backend Interview Questions

### Question 1
- **English**: What is a Gin Middleware and how does it work internally?
- **Vietnamese**: Gin Middleware là gì và cơ chế hoạt động bên trong của nó như thế nào?
- **Model Answer (EN)**: A Gin middleware is a function with signature `gin.HandlerFunc` (`func(*gin.Context)`). Gin chains middlewares and route handlers into a slice of handlers. When a request arrives, Gin calls them sequentially. Middleware can intercept the request, execute code before and after `c.Next()`, or stop execution using `c.Abort()`.
- **Model Answer (VI)**: Gin middleware là một hàm có kiểu `gin.HandlerFunc` (`func(*gin.Context)`). Gin liên kết các middleware và handler thành một slice danh sách hàm. Khi request đến, Gin gọi lần lượt từng hàm. Middleware có thể can thiệp trước và sau khi gọi `c.Next()`, hoặc dừng chuỗi bằng `c.Abort()`.
- **Explanation**: Hiểu về Onion Architecture / Pipeline trong HTTP router.
- **Follow-up**: What is the order of execution when multiple middlewares use `c.Next()`? (Answer: Middlewares execute code before `c.Next()` in registration order, and code after `c.Next()` in reverse order).

### Question 2
- **English**: Why does FinTrack use `Subject` in `jwt.RegisteredClaims` to store the user ID?
- **Vietnamese**: Tại sao FinTrack lại sử dụng trường `Subject` trong `jwt.RegisteredClaims` để lưu ID người dùng?
- **Model Answer (EN)**: According to RFC 7519, the `sub` (Subject) claim identifies the principal that is the subject of the JWT. Storing the user UUID in `sub` follows industry standards and allows using standard JWT libraries without custom claim structs.
- **Model Answer (VI)**: Theo chuẩn RFC 7519, claim `sub` (Subject) dùng để định danh chủ thể của token. Lưu UUID người dùng vào `sub` tuân thủ tiêu chuẩn ngành và cho phép sử dụng struct chuẩn `jwt.RegisteredClaims` mà không cần tạo custom claim struct.
- **Explanation**: Tuân thủ tiêu chuẩn IETF RFC.
- **Follow-up**: Can we store the user's role or permissions in custom claims? (Answer: Yes, by embedding `jwt.RegisteredClaims` into a custom struct).

### Question 3
- **English**: What is the difference between `c.Set` and `c.Request.Context()`?
- **Vietnamese**: Sự khác nhau giữa `c.Set` của Gin và `c.Request.Context()` chuẩn của Go là gì?
- **Model Answer (EN)**: `c.Set` stores key-value pairs in Gin's internal map, optimized for fast lookups within Gin handlers. `c.Request.Context()` is standard Go `context.Context`, used to pass deadlines, cancellation signals, and trace metadata across standard library boundaries and database drivers.
- **Model Answer (VI)**: `c.Set` lưu key-value vào map nội bộ của Gin, được tối ưu cho việc truy xuất nhanh giữa các handler của Gin. `c.Request.Context()` là context chuẩn của Go, dùng để truyền deadline, tín hiệu hủy request, và metadata qua các tầng nghiệp vụ và driver database.
- **Explanation**: Hiểu ranh giới giữa framework Gin và thư viện chuẩn Go.
- **Follow-up**: Can you inject values from `gin.Context` into `c.Request.Context()`? (Answer: Yes, using `context.WithValue`).

### Question 4
- **English**: How do you prevent Timing Attacks during password verification?
- **Vietnamese**: Làm thế nào để chống tấn công phân tích thời gian (Timing Attack) khi xác thực mật khẩu?
- **Model Answer (EN)**: Bcrypt's comparison functions (`bcrypt.CompareHashAndPassword`) use constant-time byte comparisons internally (`subtle.ConstantTimeCompare`) so that the comparison takes the same amount of time regardless of how many characters match.
- **Model Answer (VI)**: Hàm so khớp của Bcrypt (`bcrypt.CompareHashAndPassword`) bên dưới sử dụng cơ chế so sánh thời gian bất biến (constant-time comparison) để thời gian thực thi không bị thay đổi dựa trên số lượng ký tự trùng khớp, ngăn hacker suy đoán mật khẩu.
- **Explanation**: Nguyên lý bảo mật mật mã học (Cryptographic Security).
- **Follow-up**: Does standard string comparison `a == b` use constant time? (Answer: No, it returns `false` on the first mismatched byte).

### Question 5
- **English**: Why does `AuthMiddleware` check `strings.Fields(authHeader)` instead of splitting by a single space?
- **Vietnamese**: Tại sao `AuthMiddleware` lại dùng `strings.Fields(authHeader)` thay vì tách bằng dấu cách đơn?
- **Model Answer (EN)**: `strings.Fields` splits around each instance of one or more consecutive white space characters. If a client accidentally sends multiple spaces or tabs (e.g. `Bearer   <token>`), `strings.Fields` handles it robustly without producing empty string tokens.
- **Model Answer (VI)**: `strings.Fields` tách chuỗi dựa trên một hoặc nhiều ký tự khoảng trắng liên tiếp. Nếu client vô tình gửi nhiều dấu cách hoặc phím tab, `strings.Fields` vẫn xử lý chính xác mà không sinh ra các phần tử chuỗi rỗng.
- **Explanation**: Viết code phòng thủ (Defensive Programming).
- **Follow-up**: What should the length of `fields` be for a valid Bearer header? (Answer: Exactly 2).

### Question 6
- **English**: What is the danger of returning a 404 vs 401 when an unauthenticated user accesses a non-existent route?
- **Vietnamese**: Mối nguy hại của việc trả về 404 thay vì 401 khi một user chưa đăng nhập truy cập route không tồn tại là gì?
- **Model Answer (EN)**: If authentication middleware runs before routing or on route groups, unauthenticated requests are rejected with 401 immediately, concealing whether private internal endpoints exist or not.
- **Model Answer (VI)**: Nếu middleware xác thực được gắn bảo vệ toàn bộ nhóm route, request chưa đăng nhập sẽ bị từ chối 401 ngay lập tức, ngăn kẻ tấn công thăm dò xem hệ thống có tồn tại các endpoint nội bộ nhạy cảm nào hay không.
- **Explanation**: Bảo vệ cấu trúc hệ thống (Endpoint Enumeration Protection).
- **Follow-up**: How do route groups work in Gin? (Answer: `v1 := r.Group("/api/v1"); v1.Use(AuthMiddleware)`).

### Question 7
- **English**: What happens when a JWT token expires while a user is actively using the app?
- **Vietnamese**: Điều gì xảy ra khi JWT token hết hạn trong lúc người dùng đang sử dụng ứng dụng?
- **Model Answer (EN)**: `jwtManager.Validate` will fail with an expiration error, and `AuthMiddleware` returns `401 Unauthorized`. In production systems, a Refresh Token mechanism or silent token renewal is typically implemented to obtain a new access token without forcing the user to log in again.
- **Model Answer (VI)**: `jwtManager.Validate` sẽ trả về lỗi hết hạn và `AuthMiddleware` phản hồi HTTP 401 Unauthorized. Trong hệ thống production, cơ chế Refresh Token thường được áp dụng để cấp access token mới tự động mà không bắt người dùng đăng nhập lại từ đầu.
- **Explanation**: Vòng đời của token xác thực.
- **Follow-up**: Should Refresh Tokens be stored in LocalStorage? (Answer: Preferably in `HttpOnly`, `Secure`, `SameSite` cookies to mitigate XSS attacks).

### Question 8
- **English**: Why is Type Assertion with `ok` check (`val, ok := x.(T)`) mandatory in idiomatic Go?
- **Vietnamese**: Tại sao Type Assertion có kiểm tra `ok` (`val, ok := x.(T)`) là bắt buộc trong Go chuẩn mực?
- **Model Answer (EN)**: Single-value type assertion `x.(T)` causes a runtime `panic` if the type does not match. The two-value form `val, ok := x.(T)` allows the program to inspect `ok` and gracefully handle the mismatch without crashing the process.
- **Model Answer (VI)**: Dạng ép kiểu một biến `x.(T)` sẽ gây ra `panic` làm sập chương trình nếu kiểu dữ liệu không khớp. Dạng 2 biến `val, ok := x.(T)` cho phép kiểm tra cờ `ok` để xử lý lỗi một cách an toàn mà không làm sập server.
- **Explanation**: Khả năng phục hồi và độ tin cậy của phần mềm Go.
- **Follow-up**: What is the zero value of `val` if `ok` is `false`? (Answer: The zero value of type `T`).

### Question 9
- **English**: How does `AuthService` handle a `nil` JWTManager?
- **Vietnamese**: `AuthService` xử lý trường hợp `jwtManager` bị `nil` như thế nào?
- **Model Answer (EN)**: If `jwtManager` is nil, `AuthService.Login` returns an explicit error (`errors.New("jwt manager is not configured")`) instead of dereferencing a nil pointer, preventing a server panic.
- **Model Answer (VI)**: Nếu `jwtManager` bị nil, `AuthService.Login` trả về một lỗi tường minh thay vì truy cập con trỏ nil, giúp bảo vệ server không bị panic đột ngột.
- **Explanation**: Thiết kế chống crash do nil pointer dereference.
- **Follow-up**: Why would `jwtManager` be nil in some test cases? (Answer: When testing registration logic where token generation is not required).

### Question 10
- **English**: How does Gin handle CORS preflight `OPTIONS` requests when `AuthMiddleware` is used?
- **Vietnamese**: Gin xử lý request CORS preflight `OPTIONS` như thế nào khi sử dụng `AuthMiddleware`?
- **Model Answer (EN)**: Browsers do not attach Authorization headers to preflight `OPTIONS` requests. If `AuthMiddleware` runs before CORS middleware, preflights will fail with 401. Therefore, CORS middleware must always execute before `AuthMiddleware` and immediately respond 200/204 to `OPTIONS`.
- **Model Answer (VI)**: Trình duyệt không đính kèm header Authorization trong các request preflight `OPTIONS`. Nếu `AuthMiddleware` chạy trước middleware CORS, request `OPTIONS` sẽ bị chặn 401. Do đó, middleware CORS luôn phải được đặt trước `AuthMiddleware` và phản hồi ngay 200/204 cho method `OPTIONS`.
- **Explanation**: Bẫy thực tế phổ biến nhất khi tích hợp Frontend React/Next.js với Go backend.
- **Follow-up**: What HTTP status code should CORS preflight return? (Answer: 204 No Content or 200 OK).

---

## 15. Knowledge Connection
- **Kế thừa từ Issue #4**: Sử dụng tài khoản người dùng đã được tạo và lưu hash mật khẩu trong bảng `users`.
- **Chuẩn bị cho Issue #6 (Category CRUD)**: Toàn bộ các API quản lý danh mục bắt buộc phải đi qua `AuthMiddleware` để lấy `user_id`, đảm bảo người dùng chỉ xem và sửa danh mục của chính họ (Tenant Isolation).
- **Chuẩn bị cho Issue #7 (Transaction Management)**: Giao dịch tài chính luôn gắn liền với `user_id` được trích xuất từ JWT token qua `GetUserID(c)`.

---

## 16. Learning Checkpoint

### I Must Understand
1. Cấu tạo 3 phần của JSON Web Token (Header, Claims, Signature).
2. Cách thức hoạt động của Gin Middleware và cơ chế ngắt bằng `c.AbortWithStatusJSON()`.
3. Tại sao phải có lệnh `return` ngay sau khi gọi `c.Abort()`.
4. Cách parse chuỗi Authorization header an toàn với `strings.Fields`.
5. Kỹ thuật ép kiểu an toàn (Safe Type Assertion) với `val, ok := x.(T)`.

### I Should Be Able To Code
1. Viết một struct `JWTManager` với hai hàm `Generate` và `Validate`.
2. Viết một Gin Middleware chặn các request không có header hợp lệ.
3. Viết hàm helper trích xuất giá trị từ `gin.Context` an toàn.

### I Should Be Able To Explain In An Interview
1. Tại sao phải kiểm tra thuật toán ký trong hàm callback của `jwt.ParseWithClaims`?
2. Ưu và nhược điểm của Stateless JWT so với Stateful Session.
3. Tại sao cần phòng chống lỗi User Enumeration ở màn hình đăng nhập?

### Advanced Topics I Do Not Need Yet
- Token Refresh Flow với Rotation và Reuse Detection.
- Thuật toán ký bất đối xứng Asymmetric Keys (RS256 / EdDSA) dùng cho OAuth2 / OpenID Connect.
- Phân tán quyền kiểm soát truy cập dựa trên vai trò nâng cao (RBAC / ABAC).
