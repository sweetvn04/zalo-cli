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
