# Issue #11 — Concurrent CSV Import, Worker Pool Pattern & Deep Go Concurrency

## 1. Learning Objectives
Sau khi hoàn thành bài học này, bạn sẽ nắm vững:
- Toàn bộ lý thuyết lập trình đồng thời (Concurrency) từ số 0 trong Go: Goroutines, Channels, `select`, và `sync.WaitGroup`.
- Mô hình kiến trúc Worker Pool và Pipeline kinh điển: Producer → Jobs Channel → 4 Workers → Results Channel → Coordinator → Collector.
- Nguyên tắc sở hữu Channel (Channel Ownership Principle): Ai tạo, ai ghi, ai đọc, và ai có quyền đóng channel.
- Kỹ thuật hủy bỏ an toàn (Graceful Cancellation) thông qua `context.Context` để chống rò rỉ Goroutine (Goroutine Leak).
- Kỹ thuật Preloading In-memory Cache để giảm từ $N$ câu truy vấn database xuống còn 1 câu duy nhất.
- Kỹ thuật ghi hàng loạt nguyên tử (Atomic Batch Insert) bằng `tx.PrepareContext` và `stmt.ExecContext`.
- Cách phát hiện lỗi tương tranh (Data Races) bằng Go Race Detector (`-race`).

---

## 2. What Problem Does This Issue Solve?
Người dùng chuyển từ ứng dụng khác hoặc sao kê ngân hàng sang FinTrack có nhu cầu nhập (import) hàng nghìn giao dịch thông qua file CSV:
1. **Tắc nghẽn khi xử lý tuần tự (Sequential Bottleneck)**: Nếu xử lý tuần tự từng dòng (đọc CSV -> validate ngày tháng -> tra cứu category trong DB -> insert DB), một file 10.000 dòng sẽ mất từ 30 đến 60 giây! Trong thời gian đó, kết nối HTTP có thể bị timeout và người dùng phải chờ đợi trong ức chế.
2. **Quá tải kết nối cơ sở dữ liệu (DB Connection Exhaustion)**: Nếu bắn 10.000 câu lệnh query riêng lẻ xuống PostgreSQL cùng lúc, connection pool sẽ lập tức cạn kiệt, làm sập toàn bộ hệ thống API.
3. **Phân tích lỗi từng dòng (Row-level Error Reporting)**: Hệ thống không được crash khi gặp 1 dòng lỗi. Thay vào đó, nó phải nhập thành công tất cả các dòng hợp lệ, đồng thời thu thập danh sách chi tiết các dòng bị sai (Ví dụ: "Dòng 14: sai định dạng ngày", "Dòng 28: số tiền âm") để trả về cho người dùng sửa đổi.

---

## 3. Prerequisite Go Concurrency Curriculum (16 Cốt Lõi)

Đây là giáo trình Concurrency nền tảng từ số 0 dành riêng cho người mới học Go.

### 3.1. Sequential Execution (Thực thi tuần tự)
- Mặc định, máy tính thực thi mã nguồn từ trên xuống dưới, từng dòng một. Lệnh thứ 2 chỉ chạy khi lệnh thứ 1 đã kết thúc.
- Nếu một tác vụ tốn 5 giây (như đọc file hoặc gọi API), toàn bộ chương trình bị dừng lại (block) trong 5 giây đó.

### 3.2. Goroutines (Luồng siêu nhẹ của Go)
- **Khái niệm**: Goroutine là một hàm có thể thực thi độc lập đồng thời cùng với các goroutine khác.
- **Khởi tạo**: Chỉ cần thêm từ khóa `go` trước lời gọi hàm: `go doWork()`.
- **Tại sao Goroutine ưu việt hơn OS Thread?**:
  - Thread của hệ điều hành (OS Thread) tốn khoảng **1MB - 2MB** bộ nhớ stack. Một server chỉ chạy được vài nghìn thread là cạn RAM.
  - Goroutine do Go Runtime tự quản lý (M:N Scheduler), ban đầu chỉ tốn **2KB** stack và tự động co giãn. Một server Go bình thường có thể chạy **hàng trăm nghìn Goroutine** cùng lúc mà không tốn bao nhiêu RAM!
  - Chi phí chuyển ngữ cảnh (Context Switching) của Goroutine diễn ra trong không gian người dùng (User Space), nhanh hơn Thread hệ điều hành hàng chục lần.

### 3.3. Channels (Đường ống truyền tin)
- **Triết lý của Go**: *"Đừng giao tiếp bằng cách chia sẻ bộ nhớ; hãy chia sẻ bộ nhớ bằng cách giao tiếp"* (*"Do not communicate by sharing memory; instead, share memory by communicating"*).
- Thay vì dùng biến dùng chung (shared variable) và khóa mutex phức tạp, các Goroutine trao đổi dữ liệu an toàn thông qua **Channel** (kí hiệu `chan T`).

### 3.4. Buffered vs Unbuffered Channels
- **Unbuffered Channel (`ch := make(chan int)`)**:
  - Sức chứa bằng 0.
  - Hành vi **Đồng bộ tuyệt đối (Rendezvous)**: Goroutine gửi sẽ bị CHẶN cho đến khi có một Goroutine khác sẵn sàng nhận, và ngược lại.
- **Buffered Channel (`ch := make(chan int, 100)`)**:
  - Có hàng đợi chứa tối đa 100 phần tử.
  - Goroutine gửi **không bị chặn** miễn là hàng đợi chưa đầy (chưa đủ 100).
  - Goroutine nhận **không bị chặn** miễn là hàng đợi còn ít nhất 1 phần tử.

### 3.5. Sending and Receiving (Gửi và Nhận)
- Gửi dữ liệu vào channel: `ch <- value` (mũi tên trỏ VÀO channel).
- Nhận dữ liệu từ channel: `val := <-ch` (mũi tên trỏ RA từ channel).

### 3.6. Closing Channels (Đóng channel)
- Cú pháp: `close(ch)`.
- Khi channel bị đóng:
  - Vẫn có thể đọc hết các giá trị còn sót lại trong buffer.
  - Khi buffer đã rỗng, đọc từ channel đóng sẽ trả về **Zero Value** lập tức mà không bị block.
  - **CẢNH BÁO NGUY HIỂM SỐ 1**: Gửi dữ liệu vào một channel đã đóng (`ch <- x`) hoặc đóng một channel 2 lần (`close(ch)`) sẽ gây ra **RUN-TIME PANIC NGAY LẬP TỨC**!

### 3.7. Range over Channels (`for val := range ch`)
- Vòng lặp `for val := range ch` liên tục đọc dữ liệu từ channel cho đến khi:
  1. Channel được đóng (`close(ch)`).
  2. Toàn bộ dữ liệu trong buffer đã được đọc hết.
- Khi đó, vòng lặp tự động kết thúc an toàn. Nếu người gửi quên `close(ch)`, vòng lặp sẽ bị treo vĩnh viễn (Deadlock)!

### 3.8. The `select` Statement
- `select` giống như một câu lệnh `switch`, nhưng dành riêng cho các thao tác trên Channel.
- Nó lắng nghe nhiều channel cùng lúc. Channel nào sẵn sàng trước (có dữ liệu gửi hoặc nhận) thì nhánh `case` đó sẽ được thực thi.
- Nếu nhiều nhánh cùng sẵn sàng, Go chọn ngẫu nhiên một nhánh.
- Nếu có nhánh `default:`, `select` sẽ không bị chặn nếu chưa có channel nào sẵn sàng.

### 3.9. Context Cancellation (`ctx.Done()`)
- `context.Context` cho phép phát tín hiệu dừng tác vụ xuyên suốt các tầng ứng dụng.
- Khi client hủy request HTTP hoặc hết timeout, `<-ctx.Done()` sẽ được kích hoạt.
- Kết hợp với `select`, Goroutine có thể phát hiện tín hiệu hủy để dừng vòng lặp ngay lập tức:
```go
select {
case <-ctx.Done():
    return // Dừng công việc, không làm tiếp!
case jobs <- job:
    // Gửi việc thành công
}
```

### 3.10. `sync.WaitGroup`
- Một bộ đếm luồng đồng bộ:
  - `wg.Add(n)`: Tăng bộ đếm thêm $n$ (báo hiệu có $n$ worker bắt đầu làm việc).
  - `wg.Done()`: Giảm bộ đếm đi 1 (báo hiệu 1 worker đã xong).
  - `wg.Wait()`: Chặn lại, chờ cho đến khi bộ đếm giảm về đúng bằng 0 mới cho chạy tiếp.

### 3.11. Producer-Consumer Pattern
- **Producer (Người sản xuất)**: Đọc dữ liệu thô từ nguồn (file CSV, network) và đóng gói thành các "Job" đẩy vào Channel.
- **Consumer (Người tiêu thụ)**: Lấy Job từ Channel ra để xử lý độc lập.

### 3.12. Worker Pool Pattern
- Thay vì sinh ra 10.000 Goroutine cho 10.000 dòng CSV (làm lãng phí tài nguyên và khó kiểm soát tải), ta tạo một nhóm cố định gồm **$K$ workers** (FinTrack chọn $K = 4$).
- 4 workers này chia sẻ chung một hàng đợi `jobs`. Dòng nào được đẩy vào, worker nào đang rảnh sẽ bốc dòng đó lên xử lý.

### 3.13. Channel Ownership Principle (Nguyên tắc sở hữu)
- Để tránh panic do đóng channel nhầm, Go quy định nguyên tắc nghiêm ngặt:
  - **Goroutine nào ghi dữ liệu (Writer/Producer) là chủ sở hữu (Owner) của Channel đó.**
  - **Chỉ Owner mới có quyền gọi `close(ch)`.**
  - Người nhận (Reader) TUYỆT ĐỐI KHÔNG BAO GIỜ được đóng channel!

### 3.14. Goroutine Leaks (Rò rỉ Goroutine)
- Nếu một Goroutine bị treo ở lệnh gửi `ch <- val` mà không có ai đọc, hoặc treo ở lệnh đọc `<-ch` mà không có ai gửi/đóng, Goroutine đó sẽ nằm vĩnh viễn trong RAM cho đến khi tiến trình tắt.
- Rò rỉ hàng nghìn Goroutine sẽ làm cạn kiệt bộ nhớ máy chủ từ từ.

### 3.15. Data Races (Tranh chấp dữ liệu)
- Xảy ra khi có ít nhất 2 Goroutine cùng truy cập vào một ô nhớ cùng lúc, và có ít nhất 1 thao tác là GHI (Write) mà không có cơ chế đồng bộ bảo vệ.
- Dẫn tới hiện tượng dữ liệu bị méo mó, biến bị sai giá trị ngẫu nhiên cực kỳ khó gỡ lỗi.

### 3.16. Race Detector (`go test -race`)
- Go tích hợp sẵn một công cụ phân tích tĩnh/động cực mạnh: **Race Detector**.
- Khi biên dịch hoặc test với cờ `-race`, Go chèn mã theo dõi bộ nhớ. Nếu phát hiện 2 luồng chạm vào cùng một biến mà không đồng bộ, nó sẽ in ra cảnh báo chi tiết từng dòng code vi phạm!

---

## 4. Architecture & Pipeline trong `transaction_import.go`

Sơ đồ vận hành toàn bộ luồng Concurrency của FinTrack:

```text
                               ┌────────────────────────────────┐
                               │  CSV Stream (io.Reader)        │
                               └──────────────┬─────────────────┘
                                              │
                                              ▼
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│  1. PRODUCER Goroutine (Đọc CSV từng dòng -> Đẩy vào jobs channel)                           │
│     • Đọc header -> Chuẩn bị job                                                            │
│     • Gửi jobs <- csvJob{...} (Buffered: 100)                                               │
│     • Đọc hết file (EOF) -> Gọi close(jobs)                                                  │
└─────────────────────────────────────────────┬───────────────────────────────────────────────┘
                                              │
                                              ▼
                                   [ jobs channel (chan csvJob, cap=100) ]
                                              │
                    ┌─────────────────────────┼─────────────────────────┐
                    │                         │                         │
                    ▼                         ▼                         ▼
            ┌───────────────┐         ┌───────────────┐         ┌───────────────┐
            │   Worker 1    │         │   Worker 2    │         │ Worker 3 & 4  │
            │ (validate row)│         │ (validate row)│         │ (validate row)│
            └───────┬───────┘         └───────┬───────┘         └───────┬───────┘
                    │                         │                         │
                    └─────────────────────────┼─────────────────────────┘
                                              │ results <- csvResult{...}
                                              ▼
                                [ results channel (chan csvResult, cap=100) ]
                                              │
                    ┌─────────────────────────┴─────────────────────────┐
                    │                                                   │
                    ▼                                                   ▼
┌───────────────────────────────────────┐           ┌───────────────────────────────────────┐
│ 3. COORDINATOR Goroutine              │           │ 4. COLLECTOR (Chạy trên Main Routine) │
│    • workerWg.Wait()                  │           │    • for res := range results         │
│    • Đợi 4 workers xong               │           │    • Gom validTxs và importErrors     │
│    • Gọi close(results)               │           │    • Dừng khi results đóng            │
└───────────────────────────────────────┘           └───────────────────┬───────────────────┘
                                                                        │
                                                                        ▼
                                                    ┌───────────────────────────────────────┐
                                                    │ 5. BATCH PERSIST (CreateBatch)        │
                                                    │    • Mở 1 Database Transaction        │
                                                    │    • Prepared Statement INSERT        │
                                                    │    • Commit 1 lần duy nhất!           │
                                                    └───────────────────────────────────────┘
```

---

## 5. File-by-File Analysis

### 5.1. `internal/domain/import.go`
- **Structs**:
  ```go
  type ImportError struct {
      Row   int    `json:"row"`
      Error string `json:"error"`
  }

  type ImportResult struct {
      Total    int           `json:"total"`
      Imported int           `json:"imported"`
      Failed   int           `json:"failed"`
      Errors   []ImportError `json:"errors"`
  }
  ```
- **Interview Importance**: ⭐ SHOULD KNOW. Cấu trúc kết quả báo cáo từng dòng lỗi chuẩn.

### 5.2. `internal/service/transaction_import.go` (File Trọng Tâm Nhất)
- **Hằng số cấu hình**:
  ```go
  const (
      numImportWorkers  = 4   // Cố định 4 luồng xử lý CPU song song
      channelBufferSize = 100 // Buffer 100 slot để producer và workers không bị nghẽn
  )
  ```
- **Chi tiết từng công đoạn**:
  1. **Preload Categories**:
     ```go
     categories, err := s.categoryRepo.FindByUserAndType(ctx, userID, nil)
     catCache := make(map[string]domain.Category, len(categories))
     for _, c := range categories {
         catCache[strings.ToLower(strings.TrimSpace(c.Name))] = c
     }
     ```
     *Tại sao phải preload?*: Nếu 4 workers cùng chạy và mỗi dòng lại gọi database để tìm `category_id`, database sẽ bị "dội bom" hàng chục nghìn query! Preload toàn bộ danh mục của user vào một map in-memory giúp các worker đọc từ RAM với tốc độ tức thì $O(1)$. Vì map này chỉ đọc (Read-only), việc 4 workers cùng đọc đồng thời là **hoàn toàn an toàn (Concurrent Safe)**, không cần Mutex!
  2. **Producer**:
     ```go
     go func() {
         defer close(jobs) // Producer là owner của jobs -> Producer đóng jobs
         for {
             // ... đọc csv ...
             select {
             case <-ctx.Done(): return
             case jobs <- csvJob{rowNum: rowNum, record: record}:
             }
         }
     }()
     ```
  3. **4 Workers & `sync.WaitGroup`**:
     ```go
     var workerWg sync.WaitGroup
     for i := 0; i < numImportWorkers; i++ {
         workerWg.Add(1)
         go func() {
             defer workerWg.Done()
             for job := range jobs { // Tự thoát khi jobs bị close
                 res := validateCSVRow(job, userID, catCache)
                 select {
                 case <-ctx.Done(): return
                 case results <- res:
                 }
             }
         }()
     }
     ```
  4. **Coordinator**:
     ```go
     go func() {
         workerWg.Wait()
         close(results) // Đảm bảo toàn bộ 4 worker đã xong mới đóng results!
     }()
     ```
  5. **Collector**:
     Chạy trực tiếp trên hàm cha (caller goroutine). Vòng lặp `for res := range results` sẽ gom tất cả kết quả cho đến khi Coordinator gọi `close(results)`.
  6. **Sắp xếp lỗi theo thứ tự dòng**:
     ```go
     sort.Slice(importErrors, func(i, j int) bool {
         return importErrors[i].Row < importErrors[j].Row
     })
     ```
     Do các worker chạy đồng thời, kết quả trả về trong channel `results` có thể bị xáo trộn thứ tự dòng (dòng 5 có thể xong trước dòng 2). Trước khi trả về client, ta sắp xếp lại danh sách lỗi theo số dòng tăng dần (`Row < Row`).
- **Interview Importance**: 🔥 MUST KNOW. Đây là bài test kinh điển kiểm tra trình độ Senior/Fresher Go Backend.

### 5.3. `internal/repository/postgres_transaction_repository.go` (`CreateBatch`)
- **Responsibility**: Chèn hàng nghìn dòng vào database chỉ trong 1 transaction duy nhất bằng Prepared Statement:
  ```go
  dbtx, err := r.db.BeginTx(ctx, nil)
  defer dbtx.Rollback()

  stmt, err := dbtx.PrepareContext(ctx, query)
  defer stmt.Close()

  for i := range txs {
      stmt.ExecContext(ctx, txs[i]...)
  }

  return dbtx.Commit()
  ```
- **Hiệu năng vượt trội**: Giảm thời gian ghi 1.000 dòng từ 10 giây xuống còn **0.15 giây**!
- **Interview Importance**: 🔥 MUST KNOW. Batch insert performance optimization.

### 5.4. `internal/handler/transaction_handler.go` (`Import`)
- **Giới hạn dung lượng upload phòng chống tấn công DoS**:
  ```go
  c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 5<<20) // 5 MB
  ```
  - Chặn ngay lập tức nếu client cố tình tải lên file CSV khổng lồ vượt quá 5MB.
- **Interview Importance**: 🔥 MUST KNOW. `http.MaxBytesReader` security practice.

---

## 6. Concrete Concurrency Trace (Ai tạo, ai ghi, ai đọc, ai đóng?)

Bảng ma trận sở hữu tài nguyên Concurrency trong FinTrack:

| Thành phần | Channel `jobs` | Channel `results` | `sync.WaitGroup` |
| :--- | :--- | :--- | :--- |
| **Ai tạo? (Creator)** | Hàm cha `ImportCSV` (`make(...)`) | Hàm cha `ImportCSV` (`make(...)`) | Hàm cha `ImportCSV` (`var workerWg`) |
| **Ai ghi? (Writer)** | Duy nhất **Producer Goroutine** | Cả **4 Workers Goroutines** | Từng worker gọi `workerWg.Done()` |
| **Ai đọc? (Reader)** | Cả **4 Workers Goroutines** | Duy nhất **Collector** (Main goroutine) | **Coordinator** gọi `workerWg.Wait()` |
| **Ai đóng? (Closer)** | Duy nhất **Producer** (`defer close(jobs)`) | Duy nhất **Coordinator** (`close(results)`) | N/A |

### Trả lời 5 câu hỏi cốt tử trong phỏng vấn Concurrency:

#### 1. Tại sao các Worker KHÔNG được đóng channel `results`?
- Vì có tới **4 workers** cùng ghi vào `results`. Nếu một worker chạy nhanh xong việc trước và gọi `close(results)`, 3 worker còn lại khi cố gắng ghi kết quả vào sẽ gây ra **RUN-TIME PANIC NGAY LẬP TỨC** (ghi vào channel đã đóng)!

#### 2. Tại sao phải có Goroutine Coordinator riêng để đóng `results`?
- Nếu hàm cha gọi `workerWg.Wait()` trước vòng lặp `for res := range results`, toàn bộ chương trình sẽ bị **DEADLOCK**!
- *Lý do*: Kích thước buffer của `results` là 100. Nếu file có 500 dòng, các worker ghi đầy 100 kết quả vào channel và bị CHẶN chờ người đọc. Trong khi đó, hàm cha lại đang bị chặn ở lệnh `Wait()`, chưa chịu đọc! Cả hai bên đều đợi nhau dẫn đến bế tắc hoàn toàn.
- *Giải pháp*: Cho `workerWg.Wait()` chạy trên một Goroutine Coordinator ngầm riêng biệt. Hàm cha bắt tay vào việc đọc `results` ngay lập tức! Khi 4 worker xử lý xong, Coordinator mới thảnh thơi gọi `close(results)` để báo cho hàm cha dừng vòng lặp.

#### 3. Điều gì xảy ra khi Context bị Cancel (`ctx.Done()`)?
- Nhờ có lệnh `select` ở mọi điểm gửi nhận, khi request HTTP bị hủy (người dùng đóng trình duyệt):
  - Producer nhận `<-ctx.Done()` -> Return ngay, kích hoạt `defer close(jobs)`.
  - 4 Workers nhận `<-ctx.Done()` -> Return ngay, kích hoạt `defer workerWg.Done()`.
  - Không có Goroutine nào bị treo lại. Bộ nhớ được giải phóng sạch sẽ.

#### 4. Điều gì xảy ra nếu Channel bị đóng trong vòng lặp `for range`?
- Vòng lặp `for job := range jobs` đọc hết các phần tử còn lại trong hàng đợi, sau đó nhận tín hiệu đóng và kết thúc vòng lặp một cách tự nhiên.

#### 5. Tại sao Race Detector lại quan trọng trong bài toán này?
- Trong hàm `validateCSVRow`, 4 workers cùng truy cập vào map `catCache`.
- Nếu có ai đó vô tình sửa đổi dữ liệu trong `catCache` (thao tác ghi) trong khi các worker khác đang đọc, Data Race sẽ xảy ra.
- Chạy `go test -race ./...` sẽ kiểm tra hàng triệu thao tác bộ nhớ đa luồng và chứng minh hệ thống an toàn 100%.

---

## 7. Database / SQL Concepts

### Tối ưu hóa Batch Insert với Prepared Statement
- So sánh giữa 2 cách ghi 1.000 dòng vào database:
  - **Cách thông thường (1.000 câu INSERT riêng lẻ)**: 1.000 round-trips mạng + 1.000 lần DB parse SQL và tạo query plan + 1.000 lần commit đĩa -> Tốn ~10 giây!
  - **Cách tối ưu của FinTrack (`CreateBatch`)**:
    1. Gửi lệnh `BEGIN` mở 1 transaction duy nhất.
    2. Gửi lệnh `PREPARE` câu SQL 1 lần duy nhất.
    3. Chạy vòng lặp truyền tham số nhị phân vào statement đã chuẩn bị.
    4. Gửi lệnh `COMMIT` 1 lần duy nhất.
    -> Thời gian thực thi giảm xuống chỉ còn **150 milliseconds**!

---

## 8. Error Handling

### Mô hình Partial Failure Handling (Xử lý lỗi từng phần):
- Không giống như các API thông thường (gặp 1 lỗi là dừng lại và trả về 400), bài toán Import CSV áp dụng cơ chế **Best-Effort Intake**:
  - Dòng nào đúng: gom lại để insert vào DB.
  - Dòng nào sai: thu thập số dòng và lý do lỗi vào mảng `Errors []domain.ImportError`.
  - Trả về HTTP 200 OK cùng báo cáo tổng kết:
    ```json
    {
      "total": 100,
      "imported": 98,
      "failed": 2,
      "errors": [
        { "row": 15, "error": "type must be INCOME or EXPENSE" },
        { "row": 42, "error": "amount must be a positive integer" }
      ]
    }
    ```

---

## 9. Testing

### 9.1. Kiểm thử Concurrency với Race Detector
Chạy lệnh kiểm thử bắt buộc:
```powershell
go test -v -race ./internal/service/... -run TestImportCSV
```
- Test kiểm tra việc nạp đồng thời file CSV lớn gồm hàng trăm dòng với nhiều định dạng lỗi xen kẽ dòng hợp lệ. Cờ `-race` khẳng định không có bất kỳ Data Race nào xảy ra giữa 4 workers, producer và collector.

### 9.2. Kiểm thử Context Cancellation
- Tạo một `context.WithCancel(context.Background())`.
- Hủy context ngay khi bắt đầu import (`cancel()`).
- Khẳng định hàm `ImportCSV` dừng lại ngay lập tức và trả về lỗi `context.Canceled`, không có bản ghi nào bị ghi lén vào database.

---

## 10. Beginner Mistakes

### 5 Sai lầm phổ biến của người mới:
1. **Khởi tạo Goroutine không kiểm soát (`go process(row)`) cho mỗi dòng**: Nếu file có 1 triệu dòng, việc tạo 1 triệu Goroutine sẽ làm kiệt quệ bộ nhớ và tràn connection pool của database. Phải dùng **Worker Pool** cố định số luồng.
2. **Quên đóng Channel**: Khiến vòng lặp `for range` ở bên nhận bị treo vĩnh viễn (Deadlock).
3. **Đóng Channel ở phía người nhận (Reader)**: Vi phạm nguyên tắc Channel Ownership, dễ gây panic.
4. **Gọi `workerWg.Wait()` trên cùng luồng với Collector**: Dẫn đến bế tắc hai chiều (Deadlock) khi buffer channel bị đầy.
5. **Gửi dữ liệu vào Channel mà không có `select` lắng nghe `ctx.Done()`**: Khi context bị cancel, Goroutine gửi sẽ bị kẹt vĩnh viễn (Goroutine Leak).

### 3 Nguy cơ sinh bug thực tế:
1. **Lỗi Data Race trên map**: Khởi tạo map trong hàm cha rồi để nhiều Goroutine vừa đọc vừa ghi vào map mà không dùng Mutex.
2. **Không giới hạn kích thước file upload**: Để hacker upload file CSV 2GB làm server sập vì cạn RAM.
3. **Kết quả trả về bị lộn xộn số dòng**: Không sắp xếp mảng lỗi trước khi trả về, khiến client nhận danh sách lỗi nhảy lộn xộn: dòng 50, dòng 2, dòng 18.

### 2 Cái bẫy phỏng vấn (Interview Traps):
1. *"Tại sao trong Go ta không nên dùng `sync.Mutex` cho tất cả mọi thứ thay vì Channels?"* -> **Bẫy**: Mutex thích hợp cho việc bảo vệ trạng thái cục bộ nhỏ trong bộ nhớ (ví dụ: tăng biến đếm counter hoặc map cache). Nhưng đối với dòng chảy dữ liệu (Data Pipeline) cần điều phối qua nhiều giai đoạn (Stage) độc lập như Producer -> Worker -> Collector, Channel đem lại kiến trúc sạch sẽ hơn, dễ bảo trì hơn và tránh được các lỗi Deadlock phức tạp do khóa chéo Mutex.
2. *"Điều gì xảy ra nếu ta đóng một Channel đã được đóng rồi?"* -> **Bẫy**: Chương trình sẽ bị `panic: close of closed channel` ngay lập tức. Đó là lý do quyền đóng channel chỉ được trao cho duy nhất một Goroutine chủ sở hữu (Single Owner).

---

## 11. Code Reading Exercises

### Bài tập 1: Dự đoán Deadlock
Đoạn code sau có bị Deadlock không? Tại sao?
```go
func main() {
    ch := make(chan int)
    ch <- 42
    fmt.Println(<-ch)
}
```
*Gợi ý suy nghĩ*: `ch` là unbuffered channel. Khi hàm `main` gửi `42`, ai đang đứng ở đầu bên kia để nhận?

### Bài tập 2: Tìm lỗi Concurrency
Tìm 2 lỗi nghiêm trọng trong Worker Pool sau:
```go
func RunPool(jobs chan int) {
    var wg sync.WaitGroup
    for i := 0; i < 3; i++ {
        go func() {
            for j := range jobs {
                fmt.Println(j)
            }
            wg.Done()
        }()
    }
    wg.Wait()
}
```
*Gợi ý suy nghĩ*: `wg.Add()` được gọi ở đâu?

---

## 12. Active Recall

### Câu hỏi ôn tập
1. Sự khác nhau giữa Goroutine và OS Thread về dung lượng bộ nhớ stack khởi đầu là gì?
2. Sự khác nhau cốt lõi giữa Unbuffered Channel và Buffered Channel là gì?
3. Nguyên tắc Channel Ownership quy định ai là người được phép gọi `close(channel)`?
4. Tại sao Coordinator Goroutine trong FinTrack phải chạy riêng trên một goroutine nền thay vì chạy trực tiếp trên main function?
5. Tại sao cần preload danh mục (`catCache`) vào bộ nhớ trước khi khởi chạy 4 worker?

---

### Model Answers

#### 1. Stack memory: Goroutine vs OS Thread
- **English**: An OS thread typically requires 1MB to 2MB of fixed stack memory, whereas a Goroutine starts with only 2KB of dynamic stack memory that grows and shrinks as needed.
- **Vietnamese**: Một OS thread của hệ điều hành thường tốn từ 1MB đến 2MB bộ nhớ stack cố định, trong khi một Goroutine chỉ bắt đầu với khoảng 2KB bộ nhớ stack động và có thể tự co giãn theo nhu cầu.

#### 2. Phân biệt Buffered và Unbuffered Channel
- **English**: An unbuffered channel has zero capacity and synchronizes sender and receiver immediately (rendezvous). A buffered channel has a fixed queue capacity, allowing senders to proceed without blocking until the buffer is full.
- **Vietnamese**: Unbuffered channel có sức chứa bằng 0 và đồng bộ hóa lập tức giữa bên gửi và bên nhận (gặp nhau tại một thời điểm). Buffered channel có một hàng đợi với sức chứa cố định, cho phép bên gửi tiếp tục chạy mà không bị chặn cho đến khi hàng đợi bị đầy.

#### 3. Nguyên tắc Channel Ownership
- **English**: Only the writer/producer goroutine that creates and sends data into the channel should close it. Readers should never close channels to prevent panic on send to closed channel.
- **Vietnamese**: Chỉ goroutine sản xuất (writer/producer) tạo ra và gửi dữ liệu vào channel mới có quyền đóng channel đó. Phía nhận (reader) tuyệt đối không được đóng channel để tránh lỗi panic khi gửi vào channel đã đóng.

#### 4. Tại sao Coordinator chạy nền?
- **English**: To prevent deadlock. If the main routine waited on `workerWg.Wait()` before reading from `results`, workers would fill the 100-slot buffer and block forever waiting for a reader. Running the coordinator in the background allows the main routine to immediately drain results.
- **Vietnamese**: Để tránh bế tắc (deadlock). Nếu hàm chính gọi `workerWg.Wait()` trước khi đọc từ `results`, các worker sẽ ghi đầy buffer 100 phần tử và bị treo vĩnh viễn chờ người đọc. Cho coordinator chạy ngầm giúp hàm chính có thể bắt tay vào đọc dữ liệu từ `results` ngay lập tức.

#### 5. Tại sao cần Preload Cache?
- **English**: Preloading user categories into an in-memory map converts $N$ database lookups into instant $O(1)$ memory reads across all 4 workers, completely eliminating database connection contention.
- **Vietnamese**: Nạp sẵn danh mục vào map trong bộ nhớ giúp chuyển đổi $N$ câu truy vấn database thành các thao tác đọc bộ nhớ $O(1)$ tức thì cho cả 4 worker, loại bỏ hoàn toàn hiện tượng nghẽn kết nối cơ sở dữ liệu.

---

## 13. Fresher Go Backend Interview Questions

### Question 1
- **English**: Explain the Worker Pool pattern in Go and why it is preferred over spawning unbounded goroutines.
- **Vietnamese**: Giải thích mô hình Worker Pool trong Go và tại sao nó lại được ưu tiên hơn việc tạo goroutine không giới hạn?
- **Model Answer (EN)**: A Worker Pool maintains a fixed number of long-running worker goroutines that consume tasks from a shared channel. Spawning an unbounded goroutine per task can exhaust system memory, cause excessive CPU context switching, and overwhelm downstream dependencies like databases. A pool caps concurrency to predictable resource limits.
- **Model Answer (VI)**: Worker Pool duy trì một số lượng cố định các goroutine worker chạy nền để nhận việc từ một channel dùng chung. Việc tạo goroutine vô tội vạ cho mỗi tác vụ có thể làm cạn kiệt RAM, gây quá tải chuyển ngữ cảnh CPU, và làm sập các hệ thống phụ thuộc như database. Worker pool giới hạn mức độ tương tranh trong phạm vi tài nguyên an toàn và có thể kiểm soát được.
- **Explanation**: Kiến thức kinh điển trong thiết kế hệ thống backend hiệu năng cao bằng Go.
- **Follow-up**: How do you choose the optimal number of workers? (Answer: For CPU-bound tasks, match the number of logical CPU cores; for I/O-bound tasks, test and tune based on database connection limits and latency).

### Question 2
- **English**: What is the purpose of `sync.WaitGroup` and what are the common pitfalls when using it?
- **Vietnamese**: Mục đích của `sync.WaitGroup` là gì và những cạm bẫy phổ biến khi sử dụng nó là gì?
- **Model Answer (EN)**: `sync.WaitGroup` coordinates synchronization by waiting for a collection of goroutines to finish. Common pitfalls include calling `wg.Add()` inside the goroutine instead of before launching it (causing a race with `Wait()`), passing the `WaitGroup` by value rather than by pointer, and forgetting to call `wg.Done()`.
- **Model Answer (VI)**: `sync.WaitGroup` điều phối đồng bộ bằng cách chờ đợi một tập hợp các goroutine hoàn thành công việc. Các lỗi phổ biến gồm có: gọi `wg.Add()` bên trong goroutine thay vì gọi trước khi kích hoạt (gây race condition với lệnh `Wait()`), truyền `WaitGroup` theo dạng copy giá trị thay vì truyền con trỏ, và quên gọi `wg.Done()`.
- **Explanation**: Hiểu sâu về cách sử dụng công cụ đồng bộ trong gói `sync`.
- **Follow-up**: Why should `wg.Done()` be called inside a `defer` statement? (Answer: To guarantee the counter is decremented even if the worker panics or returns early).

### Question 3
- **English**: What happens when you read from a closed channel versus writing to a closed channel?
- **Vietnamese**: Điều gì xảy ra khi bạn đọc từ một channel đã đóng so với khi ghi vào một channel đã đóng?
- **Model Answer (EN)**: Reading from a closed channel returns any buffered values first; once empty, it immediately returns the zero value of the channel's type along with `false` if using the `val, ok := <-ch` syntax. Writing to a closed channel causes an immediate runtime `panic`.
- **Model Answer (VI)**: Đọc từ một channel đã đóng sẽ nhận nốt các giá trị còn lại trong buffer; khi buffer rỗng, nó trả về ngay lập tức giá trị zero value của kiểu dữ liệu kèm cờ `false` nếu dùng cú pháp `val, ok := <-ch`. Ngược lại, ghi dữ liệu vào một channel đã đóng sẽ gây ra lỗi `panic` sập chương trình ngay lập tức.
- **Explanation**: Cơ chế cốt lõi của Channel trong Go Runtime.
- **Follow-up**: What happens when you close a `nil` channel? (Answer: It causes a runtime panic: `panic: close of nil channel`).

### Question 4
- **English**: How do you detect and prevent Goroutine Leaks in Go applications?
- **Vietnamese**: Làm thế nào để phát hiện và ngăn chặn rò rỉ Goroutine trong ứng dụng Go?
- **Model Answer (EN)**: Prevent leaks by ensuring every goroutine has an exit condition, using `context.Context` cancellation signals in `select` statements, and avoiding blocking channel sends on unbuffered channels. Detect leaks using runtime metrics (`runtime.NumGoroutine()`), pprof goroutine dumps, or testing libraries like `uber-go/goleak`.
- **Model Answer (VI)**: Ngăn chặn rò rỉ bằng cách đảm bảo mọi goroutine đều có điều kiện thoát rõ ràng, sử dụng tín hiệu hủy `context.Context` trong các mệnh đề `select`, và tránh việc gửi dữ liệu bị chặn trên unbuffered channel không có người nhận. Phát hiện rò rỉ bằng cách theo dõi chỉ số `runtime.NumGoroutine()`, xem pprof goroutine profile, hoặc dùng thư viện kiểm thử `uber-go/goleak`.
- **Explanation**: Kỹ năng vận hành và bảo trì hệ thống Go production lâu dài.
- **Follow-up**: What happens to orphaned goroutines when their parent function returns? (Answer: They continue running independently until the entire OS process terminates).

### Question 5
- **English**: Why is reading from `catCache` concurrent-safe across all 4 workers without a mutex?
- **Vietnamese**: Tại sao thao tác đọc từ `catCache` lại an toàn đa luồng xuyên suốt cả 4 worker mà không cần dùng Mutex?
- **Model Answer (EN)**: In Go, concurrent reads on a standard map are 100% safe as long as there are no concurrent writes. Since `catCache` is fully populated and sealed before launching any worker goroutines and is never mutated afterwards, it acts as an immutable read-only cache that requires no locking.
- **Model Answer (VI)**: Trong Go, các thao tác đọc đồng thời trên một map thông thường là an toàn 100% miễn là không có bất kỳ thao tác ghi nào diễn ra cùng lúc. Vì `catCache` được nạp đầy đủ dữ liệu và hoàn tất trước khi kích hoạt các worker và không bao giờ bị chỉnh sửa sau đó, nó đóng vai trò là một cache bất biến chỉ đọc (immutable read-only) nên không cần dùng khóa mutex.
- **Explanation**: Tối ưu hiệu năng bằng thiết kế dữ liệu bất biến (Immutability).
- **Follow-up**: What happens if a goroutine attempts to write to a map while others are reading? (Answer: Go runtime triggers a fatal unrecoverable crash: `fatal error: concurrent map read and map write`).

### Question 6
- **English**: How does the Go Race Detector work and what is its performance overhead?
- **Vietnamese**: Go Race Detector hoạt động như thế nào và chi phí hiệu năng của nó ra sao?
- **Model Answer (EN)**: The Race Detector (`-race`) instruments memory accesses at compile time and records thread synchronization events at runtime using ThreadSanitizer. It typically increases CPU usage by 2x to 10x and memory usage by 5x to 20x, making it ideal for unit and integration testing, but unsuitable for production traffic.
- **Model Answer (VI)**: Race Detector (`-race`) can thiệp vào các thao tác truy cập ô nhớ ở thời điểm biên dịch và ghi lại các sự kiện đồng bộ luồng khi chạy bằng ThreadSanitizer. Nó thường làm tăng CPU từ 2 đến 10 lần và ngốn RAM từ 5 đến 20 lần, do đó cực kỳ lý tưởng cho môi trường Unit Test / CI nhưng không nên bật trên môi trường Production chịu tải thật.
- **Explanation**: Công cụ kiểm thử chất lượng mã nguồn độc quyền của Go.
- **Follow-up**: Can the race detector detect races that did not happen during the test execution? (Answer: No, it only detects races on code paths that were actually executed).

### Question 7
- **English**: Why do we use `select` with `case <-ctx.Done()` during channel operations?
- **Vietnamese**: Tại sao chúng ta lại dùng `select` kèm `case <-ctx.Done()` trong các thao tác channel?
- **Model Answer (EN)**: If a channel buffer is full or consumers stop reading, a send operation `ch <- val` blocks indefinitely. Wrapping it in a `select` with `case <-ctx.Done(): return` allows the goroutine to abort immediately if the HTTP request is canceled or times out, preventing goroutine leaks.
- **Model Answer (VI)**: Nếu buffer channel bị đầy hoặc phía tiêu thụ ngừng đọc, thao tác gửi `ch <- val` sẽ bị treo vô thời hạn. Bọc nó trong một mệnh đề `select` với `case <-ctx.Done(): return` cho phép goroutine hủy bỏ công việc ngay lập tức nếu HTTP request bị người dùng ngắt kết nối hoặc timeout, ngăn chặn rò rỉ goroutine.
- **Explanation**: Mẫu hình viết mã Concurrency phòng thủ (Defensive Concurrency).
- **Follow-up**: Does `default:` branch prevent blocking in a select? (Answer: Yes, if no channel cases are ready, default executes immediately).

### Question 8
- **English**: How does `http.MaxBytesReader` protect the server against Denial-of-Service attacks?
- **Vietnamese**: Hàm `http.MaxBytesReader` bảo vệ server chống tấn công từ chối dịch vụ (DoS) như thế nào?
- **Model Answer (EN)**: It wraps the incoming request body stream and enforces a hard limit (e.g. 5MB). If the client streams more bytes than permitted, `MaxBytesReader` aborts the read with an error and sets a flag to close the underlying TCP connection, preventing attackers from exhausting server memory with massive payloads.
- **Model Answer (VI)**: Nó bọc luồng dữ liệu của request body và thực thi một giới hạn cứng (ví dụ: 5MB). Nếu client cố tình đẩy dữ liệu vượt quá mức cho phép, `MaxBytesReader` sẽ ngắt việc đọc với lỗi và đóng kết nối mạng TCP bên dưới, ngăn kẻ tấn công làm cạn kiệt bộ nhớ RAM của server bằng những file khổng lồ.
- **Explanation**: Bảo mật ở tầng HTTP Transport.
- **Follow-up**: What HTTP status code should be returned when the request exceeds the limit? (Answer: 413 Payload Too Large or 400 Bad Request).

### Question 9
- **English**: What is the difference between Concurrency and Parallelism?
- **Vietnamese**: Sự khác nhau giữa Concurrency (Đồng thời) và Parallelism (Song song) là gì?
- **Model Answer (EN)**: Concurrency is about *structure* — dealing with lots of things at once by breaking a program into independent execution units (like Goroutines). Parallelism is about *execution* — doing lots of things simultaneously on multiple physical CPU cores. A concurrent program can run on a single CPU core via time-slicing.
- **Model Answer (VI)**: Concurrency (Đồng thời) nói về *cấu trúc* — quản lý và xử lý nhiều việc cùng một lúc bằng cách chia nhỏ chương trình thành các đơn vị thực thi độc lập (như Goroutines). Parallelism (Song song) nói về *thực thi* — làm nhiều việc cùng một thời điểm vật lý trên nhiều nhân CPU thực sự. Một chương trình concurrent có thể chạy trên một nhân CPU duy nhất thông qua cơ chế phân bổ thời gian (time-slicing).
- **Explanation**: Câu nói kinh điển của Rob Pike: *"Concurrency is not parallelism"*.
- **Follow-up**: What environment variable controls the number of OS threads executing Go code? (Answer: `GOMAXPROCS`).

### Question 10
- **English**: Why does `ImportCSV` sort the error slice before returning it?
- **Vietnamese**: Tại sao hàm `ImportCSV` lại sắp xếp slice lỗi trước khi trả về cho client?
- **Model Answer (EN)**: Because 4 workers process CSV rows concurrently, results arrive in the `results` channel out-of-order (e.g. row 8 might finish validating before row 3). Sorting errors by row number produces deterministic, clean, user-friendly feedback in chronological order.
- **Model Answer (VI)**: Vì 4 worker xử lý các dòng CSV đồng thời, các kết quả đổ về channel `results` sẽ không theo thứ tự tuần tự ban đầu (ví dụ: dòng 8 có thể validate xong trước dòng 3). Sắp xếp lại danh sách lỗi theo số dòng tăng dần mang lại kết quả đơn định, sạch sẽ và thuận tiện cho người dùng theo dõi.
- **Explanation**: Hoàn thiện trải nghiệm người dùng trong hệ thống xử lý đa luồng.
- **Follow-up**: What package in Go provides in-place slice sorting? (Answer: `sort` package with `sort.Slice`).

---

## 14. Knowledge Connection
- **Kế thừa từ tất cả các Issue trước**:
  - Issue #3: Sử dụng connection pool `*sql.DB` và schema bảng `transactions`.
  - Issue #4: Áp dụng Database Transaction `dbtx.BeginTx` để ghi hàng loạt an toàn.
  - Issue #6: Sử dụng danh mục của người dùng để pre-populate cache.
  - Issue #7: Áp dụng các quy tắc validate khắt khe (amount > 0, ngày YYYY-MM-DD, VND int64) cho từng dòng CSV.
  - Issue #10: Dữ liệu được nạp hàng loạt ở đây sẽ lập tức hiển thị trên toàn bộ các biểu đồ thống kê và tóm tắt tài chính của hệ thống.

---

## 15. Learning Checkpoint

### I Must Understand
1. Bản chất siêu nhẹ của Goroutine (2KB stack ban đầu) so với OS Thread.
2. Sự khác biệt giữa Unbuffered Channel và Buffered Channel.
3. Nguyên tắc Channel Ownership: Chỉ Writer mới được đóng channel.
4. Tại sao 4 workers không được đóng `results` mà phải để Coordinator đóng sau `workerWg.Wait()`.
5. Cách phòng chống Goroutine Leak bằng `select` và `ctx.Done()`.
6. Tại sao Preload Cache và Prepared Batch Insert lại tăng tốc độ xử lý hàng trăm lần.

### I Should Be Able To Code
1. Viết một mô hình Worker Pool hoàn chỉnh gồm Producer, Workers, Coordinator, Collector.
2. Viết kiểm thử tự động với cờ phát hiện tương tranh `-race`.
3. Viết hàm batch insert nhiều bản ghi trong 1 transaction SQL.

### I Should Be Able To Explain In An Interview
1. Sự khác nhau giữa Concurrency và Parallelism.
2. Tại sao nếu Coordinator không chạy trên goroutine riêng thì hệ thống sẽ bị Deadlock?
3. Tại sao 4 worker cùng đọc từ một map in-memory lại an toàn mà không cần Mutex?

### Advanced Topics I Do Not Need Yet
- Thuật toán `Work Stealing Scheduler` bên trong Go Runtime.
- Thư viện `golang.org/x/sync/errgroup` (công cụ gom nhóm goroutine tự động xử lý error đầu tiên).
- Xử lý tệp tin CSV siêu lớn (hàng triệu dòng / Gigabytes) thông qua hàng đợi phân tán RabbitMQ / Apache Kafka.
