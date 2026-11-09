# CHANGELOG (NHẬT KÝ THAY ĐỔI MÃ NGUỒN)

> File này ghi lại chi tiết các thay đổi mã nguồn, tính năng, refactor và fix bug trong suốt vòng đời dự án `zalo-cli`, theo đúng quy định tại [`AGENTS.md`](./AGENTS.md).

---

## 📜 Phase 0: Setup Workspace & Project Architecture

### [Phase 0 - Init] Thiết lập quy tắc Agent và Kế hoạch Tổng thể
* **Thời gian:** 2026-10-16 02:00
* **Scope:** `docs(workspace)`
* **Files Created/Modified:**
  - `AGENTS.md`: Định nghĩa quy tắc bắt buộc của Agent, tiêu chuẩn Git và định hướng Tech Stack.
  - `COMMAND_LOG.md`: Khởi tạo bảng theo dõi nhật ký lệnh thực thi.
  - `CHANGELOG.md`: Khởi tạo bảng theo dõi nhật ký thay đổi mã nguồn.
  - `plan.md`: Tài liệu kiến trúc và lộ trình thực thi theo micro-steps 15–30 phút.
* **Nguyên nhân kỹ thuật & Rationale:**
  - Thiết lập nền móng vững chắc, kỷ luật và có thể theo dõi tiến độ rõ ràng trước khi viết dòng code đầu tiên. Đảm bảo Bảo vừa thực hành vừa nắm trọn vẹn bản chất kiến trúc hệ thống Client – Daemon.

---

## 📜 Phase 1: Bridge Daemon (`daemon/`)

### [Phase 1 - Step 1.1] Hoàn thiện luồng tạo và hiển thị ảnh QR Zalo
* **Thời gian:** 2026-10-16 17:23
* **Commit:** `e5f20e3`
* **Scope:** `fix(daemon)`
* **Files Created/Modified:**
  - `daemon/index.mjs`: Lưu ảnh PNG gốc từ server Zalo xuống đĩa `qr.png` và gọi lệnh `kitten icat` để render trực tiếp lên Kitty Terminal.
* **Nguyên nhân kỹ thuật & Rationale:**
  - Zalo sử dụng mã QR ảnh đa tầng (Multi-layer watermark) để xác thực app di động. Thư viện vẽ mã ASCII thuần (`qrcode-terminal`) chỉ chứa chuỗi token thô khiến app Zalo trên điện thoại không nhận diện được phiên đăng nhập. Việc lưu ảnh binary PNG gốc và hiển thị bằng giao thức đồ họa của Kitty giải quyết triệt để lỗi không quét được.

### [Phase 1 - Step 1.3] Khởi tạo Local WebSocket Server phục vụ IPC
* **Thời gian:** 2026-10-16 20:04
* **Commit:** `ab70117`
* **Scope:** `feat(daemon)`
* **Files Created/Modified:**
  - `daemon/index.mjs`: Mở WebSocketServer tại cổng 8989, xây dựng hàm `broadcast` đẩy sự kiện `new_message` sang client, và lắng nghe lệnh `send_message` từ client để gọi `api.sendMessage` của `zca-js`.
* **Nguyên nhân kỹ thuật & Rationale:**
  - Hiện thực hóa kiến trúc Client - Daemon. Tách biệt hoàn toàn việc kết nối Zalo mạng ngoài của Daemon và tầng UI terminal của Go CLI thông qua giao thức IPC JSON trên Local WebSocket (`ws://127.0.0.1:8989`).

### [Docs] Khởi tạo Sổ tay Kiến trúc & Khái niệm Cốt lõi (CONCEPTS.md)
* **Thời gian:** 2026-10-18 02:33
* **Commit:** `c656687`
* **Scope:** `docs(concepts)`
* **Files Created/Modified:**
  - `CONCEPTS.md`: Giải thích chi tiết và toàn diện 7 trụ cột lý thuyết của dự án: Client-Daemon, IPC Local WebSocket, Standard Go Layout, Go Concurrency (Goroutine/Channel), The Elm Architecture (Bubble Tea/Lip Gloss), và cơ chế bảo mật Zalo.
* **Nguyên nhân kỹ thuật & Rationale:**
  - Cung cấp nền tảng lý thuyết trực quan, dễ hiểu giúp Bảo nắm trọn bản chất kiến trúc hệ thống trước khi triển khai sâu mã nguồn Golang.


## Phase 2: Go IPC Client

### Sửa Git ignore và chuẩn bị lưu nguyên mẫu Go IPC client
* **Thời gian cập nhật nhật ký:** 2026-10-29 23:34:18 (Asia/Ho_Chi_Minh).
* **Commit:** `1d56e44` — `feat(cli): add initial Go client and fix binary ignore rule`.
* **Scope:** `fix(gitignore)`, nguyên mẫu `feat(cli)` hiện có.
* **Files Changed / Đã lưu trong commit:**
  - `.gitignore`: Bảo đổi `zalo-cli` thành `/zalo-cli` để chỉ bỏ qua binary ở gốc repository, không bỏ qua thư mục `cmd/zalo-cli/`.
  - `cmd/zalo-cli/main.go`: Mã nguồn hiện có, đã được Git theo dõi; kết nối WebSocket daemon, giải mã sự kiện JSON, in tin nhắn qua goroutine, chờ SIGINT/SIGTERM và đóng kết nối khi thoát.
  - `go.mod`, `go.sum`: File hiện có, đã được Git theo dõi; module `zalo-cli`, dependency `github.com/gorilla/websocket v1.5.3`.
  - `COMMAND_LOG.md`, `CHANGELOG.md`: Agent bổ sung nhật ký theo thao tác và kết quả Bảo cung cấp.
* **Nguyên nhân kỹ thuật:**
  - Quy tắc `zalo-cli` trước đây khớp cả thư mục mã nguồn cùng tên, khiến `main.go` bị bỏ qua khi đưa file vào Git.
  - Ghi nhận nguyên mẫu Go đã có trước lượt cập nhật này; agent không viết mới hay chỉnh sửa logic Go.
* **Kiểm chứng:**
  - Output Bảo gửi xác nhận Git đã nhìn thấy `cmd/zalo-cli/main.go`, `go.mod`, `go.sum`.
  - Bảo báo `go build ./...` và `git diff --check` không in output hoặc báo lỗi; agent chưa chạy lại các lệnh này.
  - Chưa kiểm chứng kết nối Zalo thực tế trong lượt này.

### Sửa kiểu timestamp trong sự kiện IPC
* **Thời gian:** 2026-11-06 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** `fix(daemon)`.
* **Files Changed:** `daemon/index.mjs`.
* **Chi tiết:** Chuẩn hóa `message.data.ts` thành số trước khi phát `new_message`. Zalo có thể trả timestamp dưới dạng chuỗi, khiến Go không giải mã được trường `int64` và bỏ qua cả tin nhắn.

### Hoàn thiện luồng nhận IPC qua channel
* **Thời gian:** 2026-11-09 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** refactor(ipc).
* **Files Changed:** internal/ipc/protocol.go, internal/ipc/client.go, cmd/zalo-cli/main.go.
* **Chi tiết:** Tách struct JSON và WebSocket client khỏi entrypoint. Goroutine chuyển sự kiện/lỗi qua channel có buffer. Hủy context đóng socket để giải phóng ReadMessage. Close chờ goroutine dừng. JSON lỗi được báo rồi tiếp tục đọc, mất kết nối kết thúc stream và CLI.
* **Nguyên nhân:** Chuẩn bị đưa dữ liệu vào TUI và khắc phục CLI chờ Ctrl+C vô hạn sau khi daemon ngắt.
* **Kiểm thử bổ sung:** internal/ipc/client_test.go dùng WebSocket cục bộ kiểm tra JSON lỗi rồi tin hợp lệ, ngắt kết nối và hủy goroutine đang chờ đọc.
* **Kết quả:** go build ./..., go vet ./..., go test -race ./... đều qua khi dùng cache tạm và cho phép cổng localhost. Chưa kiểm chứng lại với phiên Zalo thật.

### Gửi tin từ Go và chờ kết quả Zalo
* **Thời gian:** 2026-11-09 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** feat(cli).
* **Files Changed:** cmd/zalo-cli/main.go, internal/ipc/client.go, internal/ipc/protocol.go, daemon/index.mjs, daemon/ipc.mjs.
* **Chi tiết:** Thêm -to, -type, -text; mã yêu cầu IPC; serialize writer với mutex và deadline. Daemon kiểm tra payload, chờ api.sendMessage và trả send_result cùng ID. CLI ghép phản hồi theo ID và chỉ báo thành công sau phản hồi OK, chờ tối đa 30 giây, không tự gửi lại khi kết quả chưa rõ. Không có tham số tiếp tục chế độ nhận tin cũ.
* **Nguyên nhân:** Ghi lệnh vào socket thành công chưa xác nhận Zalo đã chấp nhận gửi tin.
* **Tests:** daemon/ipc.test.mjs và internal/ipc/client_test.go kiểm tra lệnh JSON, phản hồi đúng ID, Zalo trả lỗi, payload lỗi, gửi nhóm và chờ API hoàn thành.
* **Tests CLI:** cmd/zalo-cli/main_test.go chạy entrypoint logic với WebSocket giả lập để xác nhận bỏ qua ID khác và xử lý đúng thành công/thất bại.
* **Trải nghiệm:** Log nhận tin hiển thị loại và ID hội thoại để người dùng lấy đúng -to và -type. Build, vet, race tests và kiểm thử daemon giả lập đều qua. Chưa gửi tin qua tài khoản Zalo thật.

### Chọn người nhận từ danh sách bạn bè và nhóm
* **Thời gian:** 2026-11-09 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** feat(cli).
* **Files Changed:** daemon/ipc.mjs, internal/ipc/protocol.go, internal/ipc/client.go, cmd/zalo-cli/main.go, cmd/zalo-cli/recipients.go.
* **Chi tiết:** Thêm list_recipients/recipients_result, phân trang bạn bè và lấy tên nhóm theo batch. -list in danh sách người nhận sắp xếp theo tên. -list -text cho phép chọn số từ chính snapshot đang hiển thị, Enter để hủy, Ctrl+C hủy chờ nhập. Tự chọn user/group khi gửi và chờ phản hồi theo request ID.
* **Giới hạn:** Đây là danh sách bạn bè/nhóm, chưa gồm lịch sử hoặc thứ tự hội thoại gần đây. Thao tác lấy danh sách lỗi sẽ báo thất bại toàn bộ, không hiển thị danh sách thiếu mà không báo.
* **Tests:** daemon/ipc.test.mjs kiểm tra phân trang, nhóm đã rời, danh sách lỗi; cmd/zalo-cli/recipients_test.go kiểm tra chọn đúng nhóm, hủy và lựa chọn ngoài phạm vi không gửi tin.
* **Kiểm chứng:** Build và vet Go, go test -race ./..., sáu ca Node chạy trực tiếp bằng node daemon/ipc.test.mjs đều qua. Chưa gọi API danh sách trên tài khoản thật.

### Giao diện chat Bubble Tea
* **Thời gian:** 2026-11-09 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** feat(tui).
* **Files Changed:** internal/tui/model.go, internal/tui/view.go, cmd/zalo-cli/main.go, go.mod, go.sum.
* **Chi tiết:** Thêm -tui với màn hình hai cột, tìm người nhận, chọn chat, lưu draft theo hội thoại, thông báo chưa đọc, cuộn chat và nhập tiếng Việt. UI đọc IPC bất đồng bộ qua tea.Cmd, gửi tin có trạng thái chờ/thành công/lỗi và timeout. Ghép phản hồi theo request ID kể cả phản hồi đến trước kết quả command, bỏ qua command/timeout cũ. Xử lý ngắt kết nối và hủy khi thoát. Giới hạn 500 tin mỗi hội thoại trong phiên, lọc ANSI/control từ dữ liệu mạng.
* **Giới hạn:** Hiển thị tin từ lúc mở UI, chưa tải lịch sử cũ hoặc tự reconnect.
* **Tests:** internal/tui/model_test.go kiểm tra phản hồi đến sớm, đổi chat khi gửi, draft riêng, routing/dedup/unread, disconnect, timeout cũ, Unicode/resize và event loop Bubble Tea thật với transport giả lập.
* **Files Changed bổ sung:** internal/tui/model_test.go, plan.md. Cập nhật các mốc Phase 2/3 theo thực tế. Khi cửa sổ quá nhỏ chỉ nhận phím thoát để tránh gửi tin ngoài ý muốn.
* **Kiểm chứng:** Build và vet qua; race tests toàn bộ qua; entrypoint -tui đã thử trong PTY với daemon mẫu, chọn người nhận, nhập tiếng Việt, gửi/nhận xác nhận và Esc. Chưa thử UI bằng tài khoản Zalo thật.

### Tải tin nhắn cũ khi mở cuộc chat
* **Thời gian:** 2026-11-09 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** feat(history).
* **Files Changed:** daemon/history.mjs, daemon/ipc.mjs, daemon/index.mjs, internal/ipc/client.go, internal/ipc/protocol.go, internal/tui/model.go, internal/tui/history.go, internal/tui/view.go, internal/tui/model_test.go.
* **Chi tiết:** Thêm get_history/history_result. Nhóm lấy tối đa 100 tin qua getGroupChatHistory; cá nhân lấy snapshot toàn cục qua old_messages, tối đa 10 trang/15 giây rồi lọc đúng hội thoại, cache 30 giây và single-flight để không ghép nhầm phản hồi không có request ID. Ngừng request cá nhân sau timeout cho tới kết nối lại nhằm tránh phản hồi đến muộn bị nhầm. UI tải lịch sử khi mở chat, F6 tải lại, ghép với realtime theo ID và timestamp, xử lý phản hồi sớm/đổi chat. Phản hồi gửi bổ sung ID tin để loại trùng tin của chính mình trong lịch sử.
* **Giới hạn:** Tin văn bản gần đây do Zalo cung cấp, có cờ lịch sử một phần; chưa lưu tin xuống đĩa hay tải toàn bộ lịch sử.
* **Tests:** daemon/history.test.mjs và internal/tui/history_test.go kiểm tra phân trang/single-flight, lọc đúng hội thoại, loại trùng realtime và tin của chính mình, lỗi giữ tin đã có, timeout cũ và phản hồi đến sớm.
* **Files Changed bổ sung:** plan.md. Build, vet, race tests, 11 ca Node và thử terminal mẫu tải tin cũ rồi gửi tiếng Việt đều qua. Chưa kiểm chứng lịch sử trên tài khoản Zalo thật.

### Sửa tải lịch sử nhóm khi API HTTP trả 404
* **Thời gian:** 2026-11-09 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** fix(history).
* **Files Changed:** daemon/history.mjs, daemon/history.test.mjs.
* **Nguyên nhân:** API getGroupChatHistory có trong zca-js nhưng endpoint Zalo trả HTTP 404 trên tài khoản thực tế; kiểm thử trước chỉ mô phỏng API thành công.
* **Chi tiết:** Chuyển sang requestOldMessages(ThreadType.Group) khi HTTP 404 và bỏ qua HTTP này trong các lần gọi sau của phiên. Tách snapshot, cursor, cache và single-flight theo user/group để tránh trộn lịch sử. Giữ lỗi HTTP khác để không che lỗi quyền truy cập.
* **Tests:** Tái hiện đúng chuỗi lỗi 404, lọc từng nhóm, cache sau fallback, user/group đồng thời và bảo toàn lỗi 403.
* **Kết quả:** Kiểm tra cú pháp và 14 ca Node đều qua. Chưa xác nhận fallback với tài khoản Zalo thật.

### Cuộn chat và tải tiếp lịch sử theo con trỏ
* **Thời gian:** 2026-11-09 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** fix(history), feat(tui).
* **Files Changed:** daemon/history.mjs, daemon/history.test.mjs, daemon/ipc.mjs, internal/ipc/client.go, internal/ipc/protocol.go, internal/tui/model.go, internal/tui/history.go, internal/tui/history_test.go, internal/tui/model_test.go, internal/tui/view.go.
* **Nguyên nhân:** Chưa bật hoặc xử lý mouse wheel, mũi tên khi nhập không cuộn chat; mỗi phản hồi chỉ trả 100 tin, UI cắt còn 500 tin, F6 lặp lại trang gần nhất.
* **Chi tiết:** Bật mouse cell motion, xử lý wheel theo cột, mũi tên khi soạn và PgUp/PgDn cuộn chat; Ctrl+Home/End đầu/cuối. before_id/cursor/has_more cho phép tải tiếp với F7 hoặc khi cuộn tới đầu. Cache riêng từng con trỏ, tuần tự hóa các request cùng loại để không nhầm phản hồi WebSocket. Không cắt 500 tin đã tải; giữ vị trí đọc khi thêm tin cũ phía trên. Hiển thị số tin và phần trăm cuộn.
* **Giới hạn:** Chỉ tải tiếp khi Zalo trả con trỏ tiến được; không bảo đảm toàn bộ lịch sử máy chủ còn lưu hoặc cho phép truy cập.
* **Files Changed bổ sung:** plan.md. Build, vet, race tests toàn bộ, 16 ca Node và terminal mẫu với F7/mouse wheel đã qua. Kiểm thử xác nhận đủ 250 tin qua nhiều trang, giữ trên 500 tin và giữ nguyên viewport khi thêm tin phía trước. Chưa xác nhận phân trang mới trên tài khoản thật.

### Chống HTTP 429 khi nhấn F5 tải danh sách
* **Thời gian:** 2026-11-09 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** fix(recipients).
* **Files Changed:** daemon/recipients.mjs, daemon/ipc.mjs, internal/ipc/protocol.go, internal/tui/model.go, cmd/zalo-cli/recipients.go.
* **Nguyên nhân:** Mỗi lần F5 gọi lại chuỗi API bạn bè, danh sách nhóm và thông tin nhóm, dễ nhận HTTP 429 khi thao tác lặp lại hoặc mở nhiều client.
* **Chi tiết:** Cache 60 giây và single-flight theo phiên api. Khi HTTP 429, giữ cache cũ, thông báo tiếng Việt và tạm dừng gọi API 60 giây, tăng dần tối đa 5 phút nếu tiếp tục bị 429. Không tự retry. IPC trả notice/retry_after, UI chặn F5 trong thời gian chờ, CLI hiển thị thông báo cache. Nếu chưa có cache, trả lỗi có thời gian chờ và UI giữ danh sách hiện tại.
* **Tests:** daemon/recipients.test.mjs và internal/tui/model_test.go kiểm tra TTL, single-flight, 429 có/không có cache, backoff tăng dần, giữ chat và chặn/mở lại F5 theo cooldown.
* **Kết quả:** Build, vet, kiểm tra race Go và 20 ca Node đều qua, gồm tái hiện HTTP 429 với cache cũ/không cache. Chưa xác nhận thời gian Zalo thực tế gỡ giới hạn; cooldown là chính sách cục bộ của ứng dụng.
