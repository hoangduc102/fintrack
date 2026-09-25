# Issue #2 — Project Scaffold, Typed Configuration, HTTP Server, /health & Graceful Shutdown

---

## 1. Learning Objectives (Mục tiêu học tập)

Sau khi hoàn thành bài học này, bạn sẽ:
- Hiểu cấu trúc thư mục chuẩn của một dự án Go backend (`cmd/`, `internal/`) và cách quản lý module với `go.mod`.
- Làm chủ cơ chế đóng gói cấu hình với Struct và con trỏ (Pointers), fallback giá trị mặc định khi đọc biến môi trường.
- Hiểu kiến trúc HTTP Server của thư viện chuẩn Go (`net/http`) kết hợp cùng web framework Gin.
- Hiểu và tự tay triển khai được cơ chế **Graceful Shutdown** với `signal.NotifyContext` và `context.WithTimeout`.
- Thành thạo việc viết unit test kiểm tra route HTTP mà không cần mở cổng mạng thật bằng package `net/http/httptest`.

---

## 2. What Problem Does This Issue Solve? (Vấn đề thực tế)

Khi bắt đầu một dự án backend sản xuất (Production-ready):
1. **Thiếu cấu trúc chuẩn**: Nếu để tất cả code trong thư mục gốc hoặc phân chia lộn xộn, mã nguồn sẽ nhanh chóng trở thành mớ bòng bong, khó bảo trì và dễ bị import vòng (Cyclic Dependency).
2. **Cấu hình phân tán**: Nếu code ở chỗ này gọi `os.Getenv("PORT")`, chỗ kia gọi `os.Getenv("ENV")` mà không có kiểu dữ liệu (kiểu chuỗi lung tung), ứng dụng rất dễ sập khi deploy vì thiếu biến môi trường hoặc sai chính tả.
3. **Chết đột ngột (Ungraceful Shutdown)**: Khi deploy bản cập nhật mới trên Kubernetes hoặc khởi động lại container, hệ điều hành sẽ gửi tín hiệu yêu cầu dừng tiến trình. Nếu server bị dập tắt ngay lập tức, các khách hàng đang chuyển tiền hoặc gửi request dở dang sẽ nhận lỗi `502 Bad Gateway` hoặc dữ liệu bị hỏng.

Issue #2 giải quyết triệt để các vấn đề này bằng việc thiết lập bộ khung vững chắc cho FinTrack.

---

## 3. Prerequisite Go Knowledge (Kiến thức Go tiên quyết)

### A. Packages & Chữ hoa / Chữ thường (Export Rules)
Trong Go:
- Mọi file mã nguồn đều thuộc về một `package`.
- Không có từ khóa `public`, `private` hay `export`.
- **Quy tắc**:
  - Tên bắt đầu bằng chữ cái **IN HOA** (ví dụ: `Config`, `Load`, `Port`) là **Exported** (public — package khác có thể truy cập).
  - Tên bắt đầu bằng chữ cái **thường** (ví dụ: `getEnv`, `parseCommaSeparated`) là **Unexported** (private — chỉ dùng nội bộ trong package đó).

*Ví dụ nhỏ:*
```go
package counter

// Total có thể được gọi từ main: counter.Total
var Total = 0

// increment chỉ được dùng trong package counter
func increment() {
    Total++
}
```

### B. Struct & Con trỏ (Pointers)
- `struct` là kiểu dữ liệu tổng hợp (tương tự `interface` hay `object type` trong TypeScript).
- Trong Go, khi bạn truyền một biến vào hàm, Go mặc định **sao chép giá trị (Pass by Value)**.
- Dấu `&` dùng để lấy địa chỉ ô nhớ (tạo con trỏ). Dấu `*` dùng để định nghĩa kiểu con trỏ hoặc đọc/ghi giá trị tại ô nhớ đó.

*Ví dụ nhỏ:*
```go
type ServerConfig struct {
    Port string
}

func updatePort(cfg *ServerConfig) { // Nhận con trỏ *ServerConfig
    cfg.Port = "9000" // Sửa trực tiếp trên ô nhớ gốc
}
```

### C. Defer
`defer` lên lịch cho một hàm chạy ngay trước khi hàm bao ngoài kết thúc (return), bất kể hàm kết thúc bình thường hay có lỗi. Thứ tự chạy nhiều `defer` là LIFO (vào sau ra trước).

*Ví dụ nhỏ:*
```go
func demo() {
    defer fmt.Println("1: Chạy cuối cùng")
    defer fmt.Println("2: Chạy kế cuối")
    fmt.Println("3: Chạy đầu tiên")
}
// Output: 3 -> 2 -> 1
```

---

## 4. Architecture (Kiến trúc phân tầng của Issue #2)

```text
Hệ điều hành (OS Signals: SIGINT, SIGTERM)
       │
       ▼
cmd/server/main.go (Composition Root)
  ├── 1. internal/config (Nạp biến môi trường)
  ├── 2. signal.NotifyContext (Đón bắt tín hiệu tắt)
  ├── 3. internal/server/router.go (Tạo Gin Engine & gắn route /health)
  └── 4. internal/server/server.go (Chạy ListenAndServe & kích hoạt Graceful Shutdown)
```

- **`cmd/server/main.go`**: Chỉ làm nhiệm vụ điều phối và kết nối các thành phần lúc khởi động.
- **`internal/config`**: Độc lập hoàn toàn, cung cấp struct dữ liệu cấu hình.
- **`internal/server/router.go`**: Định tuyến HTTP, không quan tâm server lắng nghe trên cổng nào hay tắt bằng cách nào.
- **`internal/server/server.go`**: Quản lý vòng đời chạy của cổng mạng và xử lý tắt an toàn.

---

## 5. File-by-File Analysis (Phân tích chi tiết từng file)

### File 1: `internal/config/config.go`

#### Trách nhiệm của file
- Đọc biến môi trường từ hệ điều hành (`PORT`, `APP_ENV`, `CORS_ALLOWED_ORIGINS`, `DATABASE_URL`, `JWT_SECRET`).
- Cung cấp giá trị fallback an toàn (mặc định port `8080`, env `development`, cors `http://localhost:5173`).
- **Không được làm**: Không chứa logic khởi động server hay kết nối cơ sở dữ liệu.

#### Cú pháp Go & Thiết kế
```go
type Config struct {
	Port               string
	Environment        string
	CORSAllowedOrigins []string
	DatabaseURL        string
	JWTSecret          string
}

func Load() *Config { ... }
```
- `Load()` trả về `*Config` (con trỏ) giúp chia sẻ một cấu hình duy nhất xuyên suốt ứng dụng, tránh sao chép lãng phí tài nguyên bộ nhớ.

#### Idiomatic Go
- **Hàm helper `getEnv(key, fallback)`**: Tách nhỏ logic kiểm tra chuỗi rỗng để tái sử dụng sạch sẽ.
- **Cắt tỉa chuỗi với `strings.TrimSpace`**: Ngăn chặn lỗi khoảng trắng vô tình khi cấu hình file `.env`.

#### Đánh giá phỏng vấn
- 🔥 **MUST KNOW**: Tại sao struct `Config` có các trường viết hoa chữ cái đầu?
- ⭐ **SHOULD KNOW**: Tại sao hàm `Load()` trả về con trỏ `*Config` thay vì struct thường?

---

### File 2: `internal/server/router.go`

#### Trách nhiệm của file
- Khởi tạo Gin Engine (`*gin.Engine`).
- Cài đặt chế độ chạy (Release Mode khi lên Production, Test Mode khi chạy test, Debug Mode khi dev).
- Gắn middleware phòng thủ: `Recovery()` bắt lỗi panic, `Logger()` ghi log truy cập.
- Đăng ký endpoint `GET /health`.

#### Cú pháp Go & Thiết kế
```go
router := gin.New()
router.Use(gin.Recovery())
if cfg.Environment != "test" {
    router.Use(gin.Logger())
}
```
- Dùng `gin.New()` thay vì `gin.Default()`: Giúp kiểm soát chính xác middleware nào được gắn vào. Tắt `Logger()` khi chạy kiểm thử để terminal luôn sạch đẹp.

#### Đánh giá phỏng vấn
- 🔥 **MUST KNOW**: Middleware `gin.Recovery()` làm gì? (Bắt các panic chưa xử lý, tránh sập tiến trình Go và trả về 500 cho client).

---

### File 3: `internal/server/server.go`

#### Trách nhiệm của file
- Đóng gói struct `http.Server` chuẩn của Go.
- Cung cấp method `Run(ctx context.Context)` thực hiện chạy bất đồng bộ và đón bắt tín hiệu hủy để Graceful Shutdown.

#### Cú pháp Go & Thiết kế chuyên sâu
```go
func (s *Server) Run(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var shutdownErr error
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			shutdownErr = err
		}

		if s.db != nil {
			if err := s.db.Close(); err != nil && shutdownErr == nil {
				shutdownErr = err
			}
		}

		if shutdownErr != nil {
			return shutdownErr
		}

		return <-errChan
	}
}
```
1. `go func() { s.httpServer.ListenAndServe() }()`: `ListenAndServe` là hàm chặn luồng. Bắt buộc phải đẩy nó vào Goroutine để luồng chính tiếp tục chạy xuống khối `select`.
2. `errChan := make(chan error, 1)`: Buffered channel nhận lỗi khởi động server.
3. `select`: Lắng nghe đa kênh. Nếu server lỗi cổng, `errChan` kích hoạt. Nếu nhận tín hiệu tắt máy, `<-ctx.Done()` kích hoạt.
4. `context.WithTimeout(context.Background(), 10*time.Second)`: Đặt giới hạn tối đa 10 giây cho việc hoàn tất các request dở dang. Nếu quá 10 giây, ép đóng kết nối để bảo vệ hệ thống.

#### Đánh giá phỏng vấn
- 🔥 **MUST KNOW**: Tại sao bỏ qua lỗi `http.ErrServerClosed`? (Khi `Shutdown()` được gọi, `ListenAndServe()` sẽ lập tức trả về lỗi này để báo rằng server đang chủ động đóng).

---

### File 4: `cmd/server/main.go`

#### Trách nhiệm của file
- Entry point của toàn bộ ứng dụng.
- Đón bắt tín hiệu từ Kernel hệ điều hành:
  ```go
  ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
  defer stop()
  ```
- Gọi `srv.Run(ctx)` và ghi log kết thúc cleanly.

---

## 6. Concrete Runtime Flow (Luồng thực thi hoàn chỉnh)

### Kịch bản 1: Request `GET /health`
1. Hệ thống giám sát (Monitoring) gửi HTTP request: `GET http://localhost:8080/health`.
2. Socket TCP nhận gói tin, Gin Engine xử lý.
3. Đi qua middleware `Recovery()` (được bảo vệ).
4. Khớp route `/health`.
5. Handler trả về mã `200 OK` kèm JSON: `{"status": "ok"}`.

### Kịch bản 2: Người dùng bấm `Ctrl + C` trên Terminal (Graceful Shutdown)
1. Hệ điều hành gửi tín hiệu `SIGINT` (Signal Interrupt) tới tiến trình.
2. Hàm `signal.NotifyContext` nhận tín hiệu và kích hoạt hủy bỏ `ctx` (`ctx.Done()`).
3. Khối `select` trong `server.Run(ctx)` bắt được sự kiện `case <-ctx.Done()`.
4. Tạo `shutdownCtx` có hạn chót 10 giây (`WithTimeout`).
5. Gọi `s.httpServer.Shutdown(shutdownCtx)`:
   - Ngừng nhận các kết nối TCP mới.
   - Chờ các request đang xử lý hoàn thành.
6. Nếu có database, gọi `s.db.Close()` đóng an toàn pool kết nối.
7. Thoát khỏi `Run`, ghi log: `"server exited cleanly"`. Tiến trình kết thúc an toàn.

---

## 7. Important Go Concepts (Khái niệm Go cốt lõi)

### Functional Options Pattern
Tại sao `server.go` có:
```go
type Option func(*Server)

func WithDB(db *sql.DB) Option {
	return func(s *Server) {
		s.db = db
	}
}
```
- Go không hỗ trợ giá trị mặc định cho tham số hàm, cũng không có nạp chồng hàm (Function Overloading).
- Mẫu thiết kế này cho phép tạo đối tượng với các tùy chọn mở rộng linh hoạt: `server.New(cfg)` hoặc `server.New(cfg, server.WithDB(db))`.

---

## 8. Database / SQL Concepts
Trong Issue #2, tầng cơ sở dữ liệu chưa được kích hoạt trực tiếp, nhưng kiến trúc đã thiết kế sẵn hook `server.WithDB(db)` để khi khởi động hoặc tắt máy, server sẽ tự động đóng kết nối cơ sở dữ liệu nếu có. Kết nối PostgreSQL chi tiết sẽ học ở **Issue #3**.

---

## 9. Error Handling (Xử lý lỗi)

- **Không có `try/catch`**: Go trả về lỗi tường minh qua kiểu `error`.
- **Bỏ qua lỗi dự kiến**: Dùng `errors.Is(err, http.ErrServerClosed)` để phân biệt giữa lỗi sự cố thực sự (như trùng cổng mạng) và hành vi tắt server chủ động.

---

## 10. Testing (Kiểm thử)

Xem file [internal/server/router_test.go](file:///d:/Go/pet-projects/fin-track/internal/server/router_test.go):
- Sử dụng `httptest.NewRecorder()`: Tạo một đối tượng giả lập `http.ResponseWriter` trong bộ nhớ RAM.
- Gọi `router.ServeHTTP(recorder, req)`: Chạy trực tiếp qua router của Gin mà không cần mở socket TCP thật của máy tính, giúp test chạy cực nhanh (vài mili-giây) và không sợ xung đột cổng mạng khi chạy test song song.

Xem file [internal/server/server_test.go](file:///d:/Go/pet-projects/fin-track/internal/server/server_test.go):
- `net.Listen("tcp", "127.0.0.1:0")`: Lắng nghe trên cổng số `0` (Ephemeral Port). Hệ điều hành sẽ tự cấp phát một cổng ngẫu nhiên đang rảnh rỗi. Đây là kỹ thuật đỉnh cao để test server chạy thật mà không bao giờ bị đụng cổng.

---

## 11. Beginner Mistakes (Sai lầm phổ biến)

1. **Quên gọi `defer stop()` sau `signal.NotifyContext`**: Khiến trình lắng nghe tín hiệu OS bị rò rỉ bộ nhớ.
2. **Gọi `os.Exit(1)` trong hàm con**: Bỏ qua toàn bộ câu lệnh `defer`, khiến tài nguyên không được dọn dẹp.
3. **Dùng `gin.Default().Run(":8080")`**: Hàm này chặn luồng và không hỗ trợ graceful shutdown chuẩn mực của Go.
4. **Không đặt timeout cho Shutdown context**: Nếu dùng `context.Background()` mà có request bị deadlock, server sẽ bị treo vĩnh viễn không tắt được.
5. **Dùng nhầm `make(chan error)` không có buffer**: Khiến goroutine con bị block vĩnh viễn (Goroutine Leak) nếu không có ai đọc channel.

---

## 12. Code Reading Exercises (Bài tập đọc code)

### Bài tập 1: Dự đoán kết quả
```go
package main

import "fmt"

func updateEnv(env string) {
    env = "production"
}

func main() {
    current := "development"
    updateEnv(current)
    fmt.Println(current)
}
```
*Câu hỏi*: Màn hình sẽ in ra chữ gì? Tại sao?

### Bài tập 2: Tìm bug
```go
func startServer() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    // quên cancel()
    server.Shutdown(ctx)
}
```
*Câu hỏi*: Đoạn code trên tiềm ẩn vấn đề gì về mặt tài nguyên?

---

## 13. Active Recall (Câu hỏi tự kiểm tra)

1. Điểm khác biệt giữa `package main` và các package thông thường trong Go là gì?
2. Trong Go, làm thế nào để xuất khẩu (export) một hàm hoặc một struct để package khác sử dụng được?
3. Tại sao trong `server.Run`, hàm `ListenAndServe` phải được chạy trong một Goroutine riêng?
4. Tín hiệu `SIGTERM` và `SIGKILL` khác nhau như thế nào về khả năng xử lý Graceful Shutdown?
5. `httptest.NewRecorder()` mang lại lợi ích gì khi viết test cho HTTP handler?

---

### Model Answers (Đáp án mẫu)

1. **English**: `package main` defines a standalone executable program containing the `main()` entrypoint function. Other packages are libraries designed to be imported by other code.
   **Tiếng Việt**: `package main` định nghĩa một chương trình thực thi độc lập chứa hàm điểm vào `main()`. Các package khác là thư viện được viết để cho mã nguồn khác import vào.
2. **English**: Go uses capitalization: any identifier starting with an uppercase letter is exported (public), while lowercase identifiers are unexported (private).
   **Tiếng Việt**: Go dùng quy tắc viết hoa: định danh bắt đầu bằng chữ hoa là public (exported), bắt đầu bằng chữ thường là private (unexported).
3. **English**: Because `ListenAndServe()` is a blocking call. Running it in a separate goroutine unblocks the main flow so it can listen for OS termination signals in the `select` block.
   **Tiếng Việt**: Vì `ListenAndServe()` là hàm chặn luồng. Chạy nó trong goroutine riêng giúp giải phóng luồng chính để lắng nghe tín hiệu tắt từ hệ điều hành trong khối `select`.
4. **English**: `SIGTERM` can be caught by the application to execute cleanup logic before exiting. `SIGKILL` immediately terminates the process at the OS kernel level without giving the process any chance to clean up.
   **Tiếng Việt**: `SIGTERM` có thể được ứng dụng đón bắt để chạy dọn dẹp trước khi thoát. `SIGKILL` xóa sổ tiến trình ngay lập tức ở tầng nhân hệ điều hành, tiến trình không có cơ hội chạy code dọn dẹp.
5. **English**: It mocks `http.ResponseWriter` in memory, allowing fast and deterministic handler testing without binding to real TCP network ports.
   **Tiếng Việt**: Nó giả lập `http.ResponseWriter` trong bộ nhớ RAM, giúp kiểm thử handler nhanh chóng và tất định mà không cần chiếm cổng mạng TCP thật.

---

## 14. Fresher Go Backend Interview Questions (Bộ câu hỏi phỏng vấn)

### Câu 1
- **English**: What is Graceful Shutdown and how does it work in Go?
- **Vietnamese**: Graceful Shutdown là gì và cơ chế hoạt động của nó trong Go như thế nào?
- **Model Answer (EN)**: Graceful shutdown allows a running server to stop accepting new requests while giving in-flight requests a grace period to complete and closing resources safely before exiting. In Go, we use `signal.NotifyContext` to intercept `SIGINT`/`SIGTERM` and call `http.Server.Shutdown(ctx)` with a timeout context.
- **Model Answer (VI)**: Graceful shutdown cho phép server ngừng nhận request mới, đồng thời dành một khoảng thời gian chờ để hoàn tất các request đang xử lý dở và đóng an toàn các tài nguyên trước khi tắt hẳn. Trong Go, ta dùng `signal.NotifyContext` để bắt `SIGINT`/`SIGTERM` và gọi `http.Server.Shutdown(ctx)` với context có timeout.
- **Explanation**: Giúp dịch vụ hoạt động với độ sẵn sàng cao (High Availability), không làm đứt kết nối người dùng khi triển khai phiên bản mới (Zero-Downtime Deployment).
- **Follow-up**: What happens if an active request takes longer than the shutdown timeout? (Nếu request chạy quá thời gian timeout thì sao? -> Hàm Shutdown trả về lỗi timeout và ép đóng kết nối).

### Câu 2
- **English**: Why is `ctx context.Context` conventionally the first parameter of Go functions?
- **Vietnamese**: Tại sao theo quy chuẩn của Go, `ctx context.Context` luôn là tham số đầu tiên của hàm?
- **Model Answer (EN)**: Conventionally passing context as the first parameter ensures standard propagation of cancellation signals, deadlines, and request-scoped values across API boundaries and concurrency trees.
- **Model Answer (VI)**: Đặt context làm tham số đầu tiên theo quy chuẩn giúp lan truyền nhất quán tín hiệu hủy bỏ, thời hạn hết giờ (deadline) và các dữ liệu gắn liền với request xuyên suốt các tầng kiến trúc.

### Câu 3
- **English**: What is the difference between passing a struct by value and by pointer?
- **Vietnamese**: Sự khác nhau giữa việc truyền struct theo giá trị và theo con trỏ là gì?
- **Model Answer (EN)**: Passing by value creates a full memory copy of the struct, so modifications inside the function do not affect the caller. Passing by pointer passes the memory address, allowing in-place mutation and avoiding copying overhead.
- **Model Answer (VI)**: Truyền theo giá trị sẽ tạo ra một bản sao toàn bộ struct trong bộ nhớ, mọi thay đổi trong hàm không ảnh hưởng đến biến gốc. Truyền theo con trỏ chỉ truyền địa chỉ ô nhớ, cho phép chỉnh sửa trực tiếp và không tốn chi phí sao chép dữ liệu.

### Câu 4
- **English**: What is the Functional Options Pattern in Go?
- **Vietnamese**: Functional Options Pattern trong Go là gì?
- **Model Answer (EN)**: It is a design pattern using closures to provide flexible, extensible, and self-documenting configuration for constructors without needing function overloading or bulky config structs.
- **Model Answer (VI)**: Là một mẫu thiết kế sử dụng closure để cung cấp cấu hình linh hoạt, dễ mở rộng và tường minh cho hàm khởi tạo mà không cần nạp chồng hàm (overloading) hay truyền struct cấu hình cồng kềnh.

### Câu 5
- **English**: How does `defer` evaluate its arguments?
- **Vietnamese**: Lệnh `defer` đánh giá (evaluate) các tham số của nó tại thời điểm nào?
- **Model Answer (EN)**: Arguments to a deferred function are evaluated immediately when the `defer` statement is encountered, but the actual execution of the function is deferred until the surrounding function returns.
- **Model Answer (VI)**: Các đối số truyền vào hàm `defer` được tính toán ngay tại thời điểm dòng lệnh `defer` được duyệt qua, nhưng bản thân hàm đó chỉ được thực thi khi hàm bao ngoài kết thúc.

---

## 15. Knowledge Connection (Kết nối kiến thức)

- **Issue #2** tạo ra khung xương HTTP Server và nạp cấu hình (`Config`).
- Trong **Issue #3**, chúng ta sẽ lấy trường `cfg.DatabaseURL` từ `Config` của Issue #2 để khởi tạo kết nối cơ sở dữ liệu PostgreSQL thực tế bằng `database/sql` và gắn vào Server thông qua `server.WithDB(db)`.

---

## 16. Learning Checkpoint (Trạm kiểm soát)

### I Must Understand (Bắt buộc phải hiểu)
- [x] Quy tắc đóng gói Package và quy ước chữ hoa (Exported) / chữ thường (Unexported).
- [x] Con trỏ `*` và toán tử lấy địa chỉ `&`.
- [x] Bản chất của Graceful Shutdown và các tín hiệu hệ điều hành (`SIGINT`, `SIGTERM`).
- [x] Cách hoạt động của khối `select` đa kênh và `chan error`.
- [x] Cách kiểm thử route bằng `httptest.ResponseRecorder`.

### I Should Be Able To Code (Có thể tự code)
- Tự viết hàm `getEnv(key, default)` đọc biến môi trường.
- Tự cấu hình một Gin router với endpoint `/health`.
- Tự viết struct đóng gói Server có hàm `Run(ctx)`.

### Advanced Topics I Do Not Need Yet (Chưa cần đào sâu ngay)
- Chi tiết cơ chế lập lịch của Go Scheduler (M:N scheduler) cho Goroutines.
- Cơ chế gom rác (Garbage Collector) chi tiết của Go.
