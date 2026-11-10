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

### Nhận và mở tin nhắn ảnh
* **Thời gian:** 2026-11-09 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** feat(images).
* **Files Changed:** daemon/history.mjs, daemon/index.mjs, internal/ipc/protocol.go, internal/tui/model.go, internal/tui/view.go, internal/tui/history.go, internal/tui/images.go, internal/tui/images_test.go, internal/media/image.go, internal/media/image_test.go.
* **Chi tiết:** Chuẩn hóa chat.photo thành kind/image_url và chú thích cho cả realtime/lịch sử, ưu tiên ảnh chất lượng cao. TUI hiển thị [Ảnh], F8/Ctrl+O mở danh sách ảnh, mũi tên chọn và Enter mở, Esc quay về chat. Chọn bằng pointer để lịch sử mới chèn không đổi ảnh đang chọn; giữ nguyên draft. Tải theo yêu cầu vào cache riêng tư, file 0600, kiểm tra định dạng raster và giới hạn 25 MiB, chặn URL ngoài HTTP(S)/máy nội bộ kể cả DNS/redirect, timeout tải 30 giây, mở file cục bộ qua xdg-open không qua shell. F6 cập nhật URL ảnh cùng ID để xử lý URL cũ hết hạn.
* **Giới hạn:** Chưa hiển thị ảnh inline trong terminal hoặc gửi ảnh; phụ thuộc URL ảnh còn truy cập được và trình xem ảnh của Linux.
* **Tests:** Bổ sung daemon/history.test.mjs kiểm tra URL HD/params, ảnh trong lịch sử và URL không hợp lệ; media/image_test.go kiểm tra cache 0600/0700, giới hạn kích thước, MIME sniffing, URL hết hạn/nội bộ; tui/images_test.go kiểm tra chọn ảnh ổn định, giữ draft, menu resize và cập nhật URL ảnh hết hạn.
* **Files Changed bổ sung:** daemon/history.test.mjs, plan.md. Build, vet, race tests toàn bộ, 23 ca Node và thử terminal mẫu mở/đóng menu F8 đều qua. Kiểm thử tải ảnh dùng HTTP giả lập và mở ảnh dùng hàm giả lập; chưa mở trình xem thật hoặc kiểm chứng URL ảnh từ tài khoản thật.

### Tự chuyển đăng nhập QR khi phiên lưu không khôi phục được
* **Thời gian:** 2026-11-09 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** fix(auth).
* **Files Changed:** daemon/auth.mjs, daemon/auth.test.mjs, daemon/index.mjs.
* **Nguyên nhân:** login(session) ném lỗi làm daemon thoát, buộc người dùng tự đổi tên/xóa session để đăng nhập lại.
* **Chi tiết:** Tách khôi phục/QR thành loginWithSession. Phiên bị từ chối hoặc JSON hỏng chuyển sang QR, callback Kitty được giữ. Chỉ thay session sau QR thành công, ghi file tạm 0600 rồi rename nguyên tử, giữ phiên cũ nếu đăng nhập mới thất bại. File tạm đuôi .cookie đã được Git ignore. Dùng execFileSync thay shell để hiển thị QR.
* **Kiểm chứng:** Kiểm tra cú pháp và 27 ca Node đều qua, gồm phiên hợp lệ, phiên bị từ chối, JSON hỏng, QR thất bại giữ phiên cũ và quyền 0600 của phiên mới. Chưa đăng nhập lại tài khoản thật.

### Chọn tin bằng mũi tên và xem ảnh ngay trong chat
* **Thời gian:** 2026-11-10 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** feat(tui).
* **Files Changed:** internal/tui/model.go, internal/tui/view.go, internal/tui/images.go, internal/tui/history.go, internal/tui/selection.go, internal/media/image.go, internal/media/preview.go, go.mod, go.sum.
* **Chi tiết:** ↑/↓ trong cột chat chọn từng tin, đánh dấu người gửi và giữ tin chọn trong viewport. Chọn ảnh tự tải/cache và giải mã PNG/JPEG/GIF/WebP/BMP, thu nhỏ theo kích thước chat và vẽ preview bằng ô màu truecolor ▀. Preview là nội dung viewport nên cuộn cùng chat; Ctrl+O mở ảnh gốc, Enter trống thử lại preview lỗi. Hủy preview cũ khi đổi tin/chat/resize, token chống kết quả đến muộn, giữ draft và cập nhật URL ảnh sau F6. Giới hạn giải mã 40 triệu pixel. F8 vẫn hoạt động tương thích.
* **Giới hạn:** Preview dùng ô màu terminal, độ nét giới hạn bởi số ô; GIF xem frame đầu. Chưa render bằng giao thức pixel Kitty.
* **Tests:** internal/media/preview_test.go kiểm tra màu pixel thực, tỷ lệ/kích thước và giới hạn decode; internal/tui/selection_test.go kiểm tra mũi tên chọn ảnh, giữ draft, cache preview và kết quả cũ sau đổi ảnh/resize. Cập nhật kiểm thử mũi tên theo hành vi chọn tin mới.
* **Files Changed bổ sung:** internal/tui/history_test.go, internal/tui/selection_test.go, internal/media/preview_test.go, plan.md. Build, vet, race tests toàn bộ và terminal mẫu chọn ảnh bằng Up, decode PNG thật từ cache rồi render inline đều qua. Chưa kiểm chứng với ảnh Zalo thật.

### Kho lưu bền vững tin nhắn và ảnh theo tài khoản
* **Thời gian:** 2026-11-10 (Asia/Ho_Chi_Minh).
* **Commit:** Chưa tạo.
* **Scope:** feat(archive).
* **Files Changed:** daemon/archive.mjs, daemon/archive-sync.mjs, daemon/image-store.mjs, daemon/index.mjs, daemon/ipc.mjs, daemon/history.mjs, daemon/package.json, daemon/package-lock.json, internal/ipc/protocol.go, internal/ipc/client.go, internal/media/archive.go, internal/media/image.go, internal/tui/model.go, internal/tui/history.go, internal/tui/view.go.
* **Chi tiết:** SQLite WAL/synchronous FULL với chỉ mục hội thoại và cursor local theo timestamp/msg_id. Lưu payload gốc, text/ảnh và placeholder cho kiểu khác, dedup theo type/thread/msg_id. Thu tin của chính mình (selfListen) và tin gửi từ CLI, lưu mọi batch old_messages, đồng bộ lịch sử khả dụng nền. Tải ảnh ngay khi nhận qua hàng đợi durable có retry, ghi file nguyên tử theo SHA-256 nội dung, tận dụng cache Go cũ, không xóa theo tuổi. Mỗi tài khoản có kho riêng 0700 và DB/ảnh 0600 dưới XDG_DATA_HOME hoặc ~/.local/share/zalo-cli/accounts. IPC ưu tiên lịch sử/danh sách local, phân trang local, thông báo media_updated, nhãn Đã lưu máy. Go chỉ đọc archive key hợp lệ trong kho và vẫn xem preview khi URL Zalo hết hạn. --offline [--account ID] đọc kho không đăng nhập Zalo. Bind IPC trên 127.0.0.1.
* **Giới hạn:** Chỉ giữ dữ liệu daemon đã nhận/lấy được; không khôi phục dữ liệu máy chủ đã mất. Ảnh giới hạn 100 MiB/file, Ảnh thất bại giữ job qua restart và thử lại với backoff tối đa 30 phút; định dạng đính kèm khác lưu payload gốc, chưa tải binary.
* **Tests:** daemon/archive.test.mjs kiểm tra restart/dedup/tách tài khoản, cursor cùng timestamp và ID local, job ảnh retry sau restart, ảnh local khi URL mất, đồng bộ nền user/group và tận dụng cache cũ. internal/media/archive_test.go kiểm tra preview từ kho offline và chặn key vượt thư mục.
* **Files Changed bổ sung:** cmd/zalo-cli/main.go, cmd/zalo-cli/recipients.go. Luồng gửi vẫn báo thành công nếu Zalo chấp nhận nhưng ghi kho lỗi, kèm cảnh báo lưu thất bại để tránh gửi trùng. Ảnh lỗi được giữ job và tiếp tục thử, ưu tiên ảnh mới; không bỏ job chỉ vì vượt số lượt.
* **Kiểm chứng bổ sung:** Ảnh tự tải khi không mở chat, job đã thử hơn 5 lượt vẫn được giữ, UI đổi sang nguồn archive và hiển thị trạng thái đã lưu. Cursor SQLite dùng so sánh tuple để tận dụng chỉ mục khi đọc sâu.
* **Files Changed bổ sung:** plan.md. IPC chỉ nhận native client không có browser Origin, giúp trang web không đọc được archive qua WebSocket. Kiểm thử restart/offline bằng daemon và TUI thật với dữ liệu mẫu đã qua, gồm ảnh local có URL gốc không còn tồn tại. Chưa chạy archive trên tài khoản thật.
* **Khả năng chạy lâu dài:** Đồng bộ bắt đầu sau cipher_key (socket đã có khóa giải mã), hủy trang đang chờ khi Zalo ngắt và lên lịch lại khi có khóa mới, xử lý event error để daemon vẫn cho đọc kho. Thêm hồi quy ngắt/kết nối lại trong daemon/history.test.mjs.
* **Kết quả cuối:** Build/vet và race tests Go đều qua, 37 ca Node qua. Thử daemon offline/TUI trên cổng ngẫu nhiên với kho mẫu sau restart hiển thị được tin và ảnh từ file local dù URL ảnh không tồn tại. Chưa kích hoạt trên tài khoản thật; cần khởi động lại daemon để bắt đầu ghi dữ liệu tài khoản.

## 2026-11-10T09:45:09.543915 — feat: lưu video local (commit hiện tại)
- Scope: nhận diện chat.video.msg trong realtime/lịch sử; lưu binary MP4/WebM vào kho media riêng theo tài khoản.
- Files Changed: daemon/history.mjs, daemon/image-store.mjs, daemon/archive.mjs, daemon/archive.test.mjs; internal/ipc/protocol.go; internal/media/archive.go, archive_test.go; internal/tui/images.go, model.go, model_test.go, view.go; plan.md; COMMAND_LOG.md.
- Kỹ thuật: chọn videoUrl/href thay vì thumbnail; dùng hàng đợi media bền vững và retry sẵn có, stream file tạm 0600 + SHA256 + rename atomic, kiểm tra chữ ký container, tối đa video 1 GiB/10 phút. Ảnh giữ giới hạn 100 MiB. Các field image_local/image_state được giữ để tương thích IPC, dùng cho cả ảnh/video; video_url riêng cho nguồn video.
- Nâng cấp: một lần đọc lại raw chat.video.msg đã lưu từ bản cũ để đưa video vào hàng đợi mà không cần nhận lại từ server.
- Giao diện: ↑↓ chọn tin video; Enter (ô soạn trống) hoặc Ctrl+O mở file local qua xdg-open; chờ tải xong, không đưa video qua bộ giải mã ảnh. Không phát chuyển động trong terminal.
- Kiểm chứng: kiểm thử nhận diện URL, stream file/loại trang lỗi và dọn file tạm, lưu/restart/migration; Go kiểm tra đường dẫn video local và thao tác Enter. Kết quả chi tiết ghi COMMAND_LOG.md. Chưa thử với video thật của tài khoản.

- Kết quả cuối: 40 kiểm thử Node qua (archive 12), Go test -race ./..., go build ./..., go vet ./... qua; sau điều chỉnh giữ giới hạn ảnh chạy lại media test.
