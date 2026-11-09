import { Zalo } from "zca-js";
import { handleCommand } from "./ipc.mjs";
import { normalizeMessage } from "./history.mjs";
import fs from "node:fs";
import { execSync } from "node:child_process";
import WebSocket, { WebSocketServer } from "ws";

const PORT = 8989;
const SESSION_FILE = "session.json";
const QR_FILE = "qr.png";
const zalo = new Zalo();
let api;

// khoi tao WebSocket
const wss = new WebSocketServer({port: PORT})
console.log(`🚀 Daemon WebSocket Server đang mở tại ws://127.0.0.1:${PORT}`);

// ham phat du lieu cho go cli
function broadcast(event, data) {
  const payload = JSON.stringify({ event, data });
  for (const client of wss.clients) {
    if (client.readyState === 1) { // 1 nghia la client san sang
      client.send(payload);
    }
  }
}

// khi co go ket noi vao
wss.on("connection", (socket) => {
  console.log("[IPC] Đã có client kết nối vào Web socket server!")
  //lang nghe lenh tu go, vi du gui tin nhan.
  socket.on("message", async (raw) => {
    const result = await handleCommand(raw, api);
    if (socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(result));
    }
  });

  socket.on("close", () => {
    console.log("🔌 [IPC] Client đã ngắt kết nối.");
  });
});

// 1. Kiểm tra xem đã từng đăng nhập chưa
if (fs.existsSync(SESSION_FILE)) {
  console.log("🔄 Đang khôi phục phiên đăng nhập cũ...");
  const session = JSON.parse(fs.readFileSync(SESSION_FILE, "utf-8"));
  api = await zalo.login(session);
} else {
  console.log("📱 Đang yêu cầu mã QR chính chủ từ server Zalo...");

  api = await zalo.loginQR({ qrPath: QR_FILE }, async (event) => {
    // Khi server Zalo gửi ảnh QR về:
    if (event.type === 0 && event.data?.image) {
      // Lưu ảnh gốc dạng binary PNG xuống đĩa
      fs.writeFileSync(QR_FILE, Buffer.from(event.data.image, "base64"));
      console.log(`\n📷 Đã lưu ảnh QR chuẩn tại: ${QR_FILE}`);
      console.log("👉 Đang phóng to ảnh QR trên Kitty Terminal:\n");

      // Tự động gọi kitten icat để vẽ ảnh trực tiếp lên Kitty
      try {
        execSync(`kitten icat --align left ${QR_FILE}`, { stdio: "inherit" });
      } catch {
        console.log(`(Nếu Kitty chưa hiện ảnh, gõ: xdg-open ${QR_FILE})`);
      }
    }
  });

  // Đăng nhập xong thì lưu lại session
  const ctx = api.getContext();
  const sessionData = {
    imei: ctx.imei,
    cookie: ctx.cookie.serializeSync(),
    userAgent: ctx.userAgent,
  };
  fs.writeFileSync(SESSION_FILE, JSON.stringify(sessionData, null, 2));
  console.log("💾 Đã lưu session vào session.json!");

  // Xóa file QR tạm sau khi đăng nhập thành công
  if (fs.existsSync(QR_FILE)) fs.unlinkSync(QR_FILE);
}

console.log("✅ ĐÃ ĐĂNG NHẬP THÀNH CÔNG! Đang lắng nghe tin nhắn...\n");

// 2. Lắng nghe tin nhắn đến
api.listener.on("message", (message) => {
  const eventData = normalizeMessage(message);
  if (!eventData) return;

  console.log(`📩 [${eventData.sender_name}]: ${eventData.content}`);

  // 👈 MỚI: Bắn tin nhắn sang Go CLI qua WebSocket!
  broadcast("new_message", eventData);
});

// Bắt đầu lắng nghe
api.listener.start();
