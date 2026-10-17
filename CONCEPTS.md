# SỔ TAY KIẾN TRÚC & KHÁI NIỆM CỐT LÕI (CONCEPTS.md)

> **Dành riêng cho Bảo:** Tài liệu giải thích bản chất kiến trúc, hệ điều hành và lập trình mạng trong dự án **Zalo Terminal Client**.  
> *Đọc tài liệu này xong, Bảo sẽ hiểu rõ tại sao từng dòng code lại xuất hiện và tự tin trả lời phỏng vấn Backend/DevOps.*

---

## 📑 MỤC LỤC
1. [Bức tranh toàn cảnh & Tư duy Thiết kế Hệ thống](#1-bức-tranh-toàn-cảnh--tư-duy-thiết-kế-hệ-thống)
2. [Kiến trúc Client – Daemon: Tại sao phải tách 2 tiến trình?](#2-kiến-trúc-client--daemon-tại-sao-phải-tách-2-tiến-trình)
3. [Giao tiếp Liên tiến trình - IPC (Inter-Process Communication)](#3-giao-tiếp-liên-tiến-trình---ipc-inter-process-communication)
4. [Tổ chức Mã nguồn chuẩn Golang (Standard Go Project Layout)](#4-tổ-chức-mã-nguồn-chuẩn-golang-standard-go-project-layout)
5. [Lập trình Đồng thời trong Go (Concurrency: Goroutine & Channel)](#5-lập-trình-đồng-thời-trong-go-concurrency-goroutine--channel)
6. [Kiến trúc Giao diện Terminal (TUI & Elm Architecture với Bubble Tea)](#6-kiến-trúc-giao-diện-terminal-tui--elm-architecture-với-bubble-tea)
7. [Bản chất Giao thức Mạng & Cơ chế Bảo mật của Zalo](#7-bản-chất-giao-thức-mạng--cơ-chế-bảo-mật-của-zalo)

---

## 1. Bức tranh toàn cảnh & Tư duy Thiết kế Hệ thống

Mọi ứng dụng nhắn tin (Chat Application) thực tế đều gồm 3 thành phần cốt lõi:
1. **Lớp Giao diện (Presentation Layer):** Nơi người dùng nhìn thấy danh sách bạn bè, đọc tin nhắn, gõ chữ (UI/TUI).
2. **Lớp Kết nối Mạng & Phiên (Network & Session Layer):** Nơi giữ kết nối liên tục với máy chủ (WebSocket), giải mã các gói tin bảo mật, duy trì trạng thái đăng nhập.
3. **Lớp Máy chủ Đám mây (Cloud Server):** Hệ thống máy chủ Zalo lưu trữ dữ liệu và định tuyến tin nhắn.

```
┌────────────────────────────────────────────────────────┐
│                   MÁY TÍNH CỦA BẢO                     │
│                                                        │
│  [ Go CLI (Giao diện TUI) ]                            │
│           ▲                                            │
│           │  IPC: Local WebSocket (ws://127.0.0.1:8989)│
│           ▼                                            │
│  [ Node.js Daemon (Dịch vụ nền) ]                      │
└───────────┬────────────────────────────────────────────┘
            │
            │  Mạng Internet công cộng (WSS / HTTPS Zalo mã hóa AES)
            ▼
┌────────────────────────────────────────────────────────┐
│                   SERVER CLOUD CỦA ZALO                │
└────────────────────────────────────────────────────────┘
```

---

## 2. Kiến trúc Client – Daemon: Tại sao phải tách 2 tiến trình?

### ❓ Câu hỏi đặt ra:
*Tại sao không viết tất cả trong 1 file Go duy nhất, hoặc 1 file Node.js duy nhất, mà lại phải tách thành 2 chương trình chạy độc lập?*

### 💡 Câu trả lời từ thực tế kỹ thuật:

#### 1. Daemon là gì trong Hệ điều hành Linux?
- **Daemon** (bắt nguồn từ thần thoại Hy Lạp: linh hồn hộ mệnh vô hình) là một tiến trình chạy ngầm trong hệ điều hành (Background Process), không có giao diện đồ họa hay màn hình tương tác, sống liên tục để phục vụ các yêu cầu.
- *Ví dụ kinh điển trong thế giới DevOps:*
  - Bạn gõ lệnh `docker ps` trên terminal $\rightarrow$ lệnh này kết thúc trong nửa giây. Nhưng con `dockerd` (Docker Daemon) vẫn đang âm thầm chạy ngầm 24/7 để quản lý container.
  - Bạn dùng `tmux` $\rightarrow$ lệnh `tmux attach` là Client, còn `tmux server` là Daemon giữ các phiên terminal không bị mất khi bạn tắt máy hay mất mạng.

#### 2. Lợi ích vượt trội của mô hình Client - Daemon trong Zalo CLI:
- **UI không bao giờ bị đơ (Decoupling):** Kết nối mạng Internet có thể bị chập chờn, reconnect, mạng chậm. Nếu gộp chung, giao diện gõ chữ sẽ bị giật lag. Tách riêng giúp giao diện Go CLI luôn mượt mà 60fps.
- **Độc lập vòng đời (Lifecycle):** Bảo có thể tắt mở cửa sổ terminal Go CLI thoải mái. Daemon Node.js vẫn chạy ngầm giữ kết nối Zalo, không bị văng phiên (logout) hay mất tin nhắn.
- **Tận dụng thế mạnh đa ngôn ngữ (Polyglot Engineering):**
  - **Node.js:** Có sẵn thư viện `zca-js` giải quyết trọn vẹn thuật toán giải mã AES/MD5 phức tạp của Zalo Web.
  - **Golang:** Cực mạnh về hiệu năng, bộ nhớ siêu nhẹ (~15MB RAM), và có hệ sinh thái Charm (`bubbletea`, `lipgloss`) làm Terminal UI đẹp nhất hiện nay.

---

## 3. Giao tiếp Liên tiến trình - IPC (Inter-Process Communication)

Khi 2 chương trình chạy độc lập trong hệ điều hành Linux (1 bằng Go, 1 bằng JavaScript), làm sao chúng nói chuyện được với nhau? Đó chính là bài toán **IPC**.

```
Node.js Daemon (Cổng 8989)  <==== (Dây cáp mạng ảo) ====>  Go CLI
```

### 1. Tại sao chọn Local WebSocket (`ws://127.0.0.1:8989`)?
- **Song công toàn phần (Full-Duplex):** Cả 2 bên đều có thể chủ động bắn tin nhắn cho nhau bất kỳ lúc nào mà không cần chờ bên kia hỏi (khác với HTTP thông thường: client phải gửi request thì server mới được trả lời).
- **Độ trễ bằng 0 (Zero Latency):** Địa chỉ `127.0.0.1` (Loopback Interface) là card mạng ảo trong RAM máy tính. Gói tin không hề đi ra ngoài modem hay internet, tốc độ truyền đạt hàng gigabyte mỗi giây.

### 2. Định dạng trao đổi: Gói tin JSON phẳng (JSON RPC / Event Stream)
Cả Go và JavaScript đều đọc/ghi JSON cực kỳ xuất sắc. Ta quy định 2 dạng gói tin:

#### A. Dạng Sự kiện (Event): Từ Daemon báo cho Go CLI
*Khi có tin nhắn mới từ Zalo đến:*
```json
{
  "event": "new_message",
  "data": {
    "sender_name": "Quốc Bảo",
    "content": "Alo test CLI!",
    "thread_id": "123456789"
  }
}
```

#### B. Dạng Hành động (Action): Từ Go CLI ra lệnh cho Daemon
*Khi Bảo bấm Enter muốn gửi tin:*
```json
{
  "action": "send_message",
  "payload": {
    "thread_id": "123456789",
    "text": "Chào bạn, mình gửi từ Terminal!"
  }
}
```

---

## 4. Tổ chức Mã nguồn chuẩn Golang (Standard Go Project Layout)

Trong dự án Golang chuyên nghiệp, không bao giờ gom tất cả code vào thư mục gốc. Ta chia thư mục theo chuẩn công nghiệp:

```
zalo-cli/
├── cmd/
│   └── zalo-cli/
│       └── main.go       <-- Cửa vào (Entrypoint): Chỉ khởi động, không chứa logic nặng
├── internal/             <-- Logic cốt lõi bí mật (Chỉ code trong repo này được dùng)
│   ├── ipc/              <-- Kết nối WebSocket tới Daemon
│   └── tui/              <-- Vẽ giao diện Bubble Tea
├── go.mod                <-- Khai sinh dự án & lưu thư viện (giống package.json)
└── go.sum                <-- Khóa mã băm bảo mật (giống package-lock.json)
```

### Điểm đặc biệt của thư mục `internal/` trong Go:
Đây là **tính năng bảo mật cấp ngôn ngữ** của Golang. Bất kỳ package nào nằm trong thư mục `internal/` sẽ **không cho phép** các dự án bên ngoài import vào. Điều này giúp kiến trúc code của Bảo không bị phụ thuộc lộn xộn.

---

## 5. Lập trình Đồng thời trong Go (Concurrency: Goroutine & Channel)

Đây là khối kiến thức **quan trọng nhất** để đi phỏng vấn vị trí Backend Golang!

### 1. Luồng chính (Main Thread) bị nghẽn (Blocking) là gì?
Nếu Bảo viết code theo kiểu tuần tự:
```go
// Code chạy tuần tự (BLOCKING - NGUY HIỂM):
conn.ReadMessage()  // <-- DÒNG NÀY SẼ ĐỨNG ĐỢI MÃI MÃI CHO TỚI KHI CÓ TIN NHẮN!
renderUI()          // Dòng này KHÔNG BAO GIỜ được chạy -> App bị đơ cứng!
```

### 2. Giải pháp: Goroutine (`go func()`)
Goroutine là một luồng thực thi siêu nhẹ (Lightweight Thread) do Go Runtime quản lý (chỉ tốn ~2KB RAM, trong khi 1 Thread của Hệ điều hành tốn ~1MB - 2MB).

```go
// Tách việc lắng nghe mạng sang một nhánh chạy ngầm:
go func() {
    for {
        _, msg, _ := conn.ReadMessage() // Nhánh ngầm này cứ việc đứng đợi
        log.Println("Có tin nhắn:", msg)
    }
}()

// Luồng chính (Main) vẫn thảnh thơi vẽ giao diện và nhận phím bấm của Bảo!
runUserInterface()
```

### 3. Graceful Shutdown (Tắt ứng dụng êm ái):
Thay vì bị tắt đột ngột (gây đứt kết nối mạng bậy bạ), ta dùng Channel lắng nghe tín hiệu ngắt từ hệ điều hành (`Ctrl+C` / `SIGINT`):
```go
interrupt := make(chan os.Signal, 1)
signal.Notify(interrupt, os.Interrupt)
<-interrupt // Đứng đợi ở đây, khi nào Bảo bấm Ctrl+C thì mới chạy tiếp để đóng socket sạch sẽ!
```

---

## 6. Kiến trúc Giao diện Terminal (TUI & Elm Architecture với Bubble Tea)

Giao diện Terminal không có DOM, không có CSS, không có thẻ `<div>`. Mọi thứ trên terminal thực chất chỉ là **các ký tự văn bản và mã màu ANSI** được in ra với tốc độ cao.

Hệ sinh thái Charm (`bubbletea`) áp dụng kiến trúc **The Elm Architecture** cực kỳ chặt chẽ qua mô hình 3 bước lặp vô tận:

```
                  ┌──────────────────────┐
                  │    Model (State)     │
                  │ (Lưu tin nhắn, text) │
                  └──────────┬───────────┘
                             │
            Update(msg)      │       View()
       (Nhận phím / socket)  │   (Vẽ chuỗi ký tự ra terminal)
                             ▼
                  ┌──────────────────────┐
                  │    Màn hình Kitty    │
                  └──────────────────────┘
```

1. **Model (Trạng thái - State):** Lưu trữ toàn bộ dữ liệu (Danh sách các tin nhắn đã nhận, chuỗi text Bảo đang gõ dở trong ô input).
2. **Update (Bộ não xử lý sự kiện):** 
   - Khi Bảo bấm 1 phím $\rightarrow$ `Update` nhận event phím, thêm ký tự vào ô input.
   - Khi WebSocket có tin nhắn Zalo tới $\rightarrow$ `Update` nhận event, nhét tin nhắn mới vào danh sách.
3. **View (Họa sĩ trình bày):** Cứ sau mỗi lần `Update` thay đổi dữ liệu, hàm `View()` sẽ dùng thư viện `lipgloss` để tô màu viền, chia cột và in toàn bộ giao diện mới ra màn hình.

---

## 7. Bản chất Giao thức Mạng & Cơ chế Bảo mật của Zalo

### 1. Tại sao Zalo không có API mở cho tài khoản cá nhân?
Zalo bảo vệ hệ thống rất nghiêm ngặt để chống spam và tài khoản ảo. Khi trình duyệt web chat (`chat.zalo.me`) nói chuyện với máy chủ Zalo:
- Mọi gói tin gửi đi đều bị băm một chuỗi kiểm tra gọi là **SignKey (MD5)** với tiền tố bí mật `zsecure...`. Nếu không có chữ ký này, server Zalo lập tức từ chối request.
- Toàn bộ nội dung tin nhắn và danh bạ đều được mã hóa đối xứng **AES-CBC** và **AES-GCM**.

### 2. Quản lý Phiên (Session Lifecycle):
- Khi Bảo quét mã QR, server Zalo sinh ra một cặp vé thông hành:
  - `cookie`: Chuỗi định danh phiên làm việc trên web.
  - `imei`: Mã định danh thiết bị giả lập.
  - `userAgent`: Nhận diện trình duyệt (ví dụ: Google Chrome trên Windows/Linux).
- File `daemon/session.json` lưu lại 3 thông tin này. Mỗi lần khởi động lại app, `zca-js` chỉ cần nộp 3 thông tin này lên server là Zalo tự động nhận diện lại Bảo mà không cần quét lại QR!

### 3. Tại sao mã QR Zalo phải dùng ảnh PNG gốc (`kitten icat`)?
Mã QR của Zalo chứa một lớp thủy vân bảo mật đồ họa đa tầng (Multi-layer watermark). Nếu dùng thư viện vẽ chữ ASCII trắng đen thông thường, camera app Zalo sẽ không đọc được lớp bảo mật này. Bằng cách lấy file ảnh PNG gốc từ server Zalo và dùng lệnh `kitten icat` của Kitty Terminal để hiển thị ảnh nguyên bản, camera điện thoại sẽ nhận diện và xác thực tức thì 100%.

---

## 🎓 TỔNG KẾT: TỪ ĐIỂN THUẬT NGỮ NHANH

| Thuật ngữ | Ý nghĩa thực tế |
| :--- | :--- |
| **Daemon** | Tiến trình chạy ngầm liên tục ở background (ở đây là `daemon/index.mjs`). |
| **IPC** | Giao tiếp giữa 2 tiến trình trên cùng 1 máy tính (ở đây qua Local WebSocket `8989`). |
| **Goroutine** | Luồng thực thi siêu nhẹ trong Go giúp chạy nhiều việc cùng lúc mà không tốn tài nguyên. |
| **Channel** | Đường ống dẫn dữ liệu an toàn giữa các Goroutine trong Go. |
| **Bubble Tea** | Thư viện làm TUI theo chuẩn Elm Architecture trong Go (`Init`, `Update`, `View`). |
| **Lip Gloss** | Thư viện tạo kiểu dáng, màu sắc, viền khung (như CSS) cho Terminal. |
| **Session** | Vé thông hành lưu đăng nhập của Zalo (`cookie`, `imei`, `userAgent`). |
