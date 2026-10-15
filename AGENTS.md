# AGENTS.md — WORKSPACE INSTRUCTIONS & RULES

> **Context:** Dự án **Zalo Terminal Client (CLI / TUI)** — Nhắn tin Zalo trực tiếp từ Terminal Linux (`Konoha`) theo mô hình **Client – Daemon**.  
> **Tech Stack:** Frontend Golang kết hợp hệ sinh thái Charm (`bubbletea`, `lipgloss`, `bubbles`) + Bridge Daemon Node.js/Bun sử dụng `zca-js` + Giao tiếp IPC (Local WebSocket/Unix Domain Socket).

---

## 🤖 MANDATORY AGENT BEHAVIOR RULES

Mọi AI Agent khi làm việc trong repository này **BẮT BUỘC** tuân thủ các quy tắc sau:

### 1. Nhật ký lệnh bắt buộc (`COMMAND_LOG.md`)
* Mọi lệnh terminal được thực thi thông qua công cụ (`run_command`) đều **PHẢI ĐƯỢC GHI LẠI NGAY LẬP TỨC** vào file [`COMMAND_LOG.md`](./COMMAND_LOG.md).
* Định dạng ghi nhận gồm:
  - STT & Thời gian (Timestamp).
  - Lệnh chính xác đã chạy.
  - Mục đích / Chức năng kỹ thuật của lệnh.
  - Kết quả thực thi (Exit code / commit hash / output tóm tắt).

### 2. Nhật ký thay đổi code bắt buộc (`CHANGELOG.md`)
* Mọi thay đổi mã nguồn (tạo file mới, sửa đổi module/logic, refactor, tối ưu hóa hiệu năng, sửa lỗi bug) đều **PHẢI ĐƯỢC GHI NHẬN CHI TIẾT** vào file [`CHANGELOG.md`](./CHANGELOG.md).
* Định dạng ghi nhận gồm:
  - Thời gian (Timestamp) & Mã commit liên quan.
  - Phạm vi thay đổi (Scope / Feature / Fix).
  - Danh sách các file bị tác động (`Files Changed`).
  - Giải thích nguyên nhân kỹ thuật và chi tiết những gì đã thay đổi.

### 3. Tư duy & Phương pháp làm việc (Pair-programming cùng Bảo)
* **Bite-sized steps:** Chia nhỏ công việc thành các bước micro-step từ 15–30 phút. Mỗi bước đều có mục tiêu rõ ràng và kiểm chứng được ngay.
* **Vừa làm vừa hiểu:** Mỗi bước kỹ thuật phải kèm giải thích ngắn gọn *"Tại sao lại làm như vậy"* để Bảo nắm chắc kiến thức Backend, Concurrency, Hệ điều hành và Thiết kế hệ thống.
* **Pragmatic execution:** *"Done is better than perfect"* — ưu tiên nguyên mẫu chạy được trước (Mô hình Client-Daemon), sau đó mới tối ưu và port dần sang thuần Go.
* **Giải thích trước khi sửa:** Luôn giải thích nguyên nhân và hướng giải quyết trước khi thực hiện các thay đổi mã nguồn quan trọng.

### 4. Tiêu chuẩn Git & Code Quality
* Giữ git working tree sạch sẽ, commit thường xuyên theo chuẩn **Conventional Commits** (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `perf:`, `style:`).
* Sau mỗi thay đổi Go, chạy kiểm tra biên dịch (`go build ./...` hoặc `go vet`) để đảm bảo không có lỗi cú pháp hoặc race condition.
* Đối với Daemon (TypeScript/JavaScript), đảm bảo chạy mượt mà với Bun / Node.

### 5. Định hướng Tech Stack & Kiến trúc
* **CLI / TUI Frontend (Go):** Golang kết hợp hệ sinh thái Charm (`bubbletea`, `lipgloss`, `bubbles`) để dựng giao diện dòng lệnh tương tác trực quan, thanh lịch.
* **Bridge Daemon (Node.js / Bun):** Service chạy ngầm chịu trách nhiệm kết nối Zalo Web qua `zca-js` (xử lý mã hóa AES, MD5 signing và WebSocket Zalo).
* **IPC (Inter-Process Communication):** Giao tiếp nội bộ giữa Go CLI và Node Daemon qua Local WebSocket (`ws://127.0.0.1:port`) hoặc Unix Domain Socket (`/tmp/zalo-cli.sock`) với payload JSON phẳng, đơn giản.
* **Lộ trình dài hạn:** Từng bước bóc tách và port các module mã hóa (`src/utils.ts`) và WebSocket của `zca-js` sang thuần Golang để tiến tới Single Static Binary.
