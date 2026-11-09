import { Zalo } from "zca-js";
import { handleCommand } from "./ipc.mjs";
import { normalizeMessage } from "./history.mjs";
import { loginWithSession } from "./auth.mjs";
import fs from "node:fs";
import { execFileSync } from "node:child_process";
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

// Khôi phục phiên; nếu thất bại tự yêu cầu QR và chỉ thay session sau khi thành công.
api = await loginWithSession(zalo, {
  sessionFile: SESSION_FILE,
  qrFile: QR_FILE,
  onQR: async (event) => {
    if (event.type === 0 && event.data?.image) {
      fs.writeFileSync(QR_FILE, Buffer.from(event.data.image, "base64"), { mode: 0o600 });
      console.log("📷 Quét mã QR bên dưới bằng app Zalo trên điện thoại.");
      try {
        execFileSync("kitten", ["icat", "--align", "left", QR_FILE], { stdio: "inherit" });
      } catch {
        console.log("Mở ảnh QR bằng: xdg-open " + QR_FILE);
      }
    }
  },
});

console.log("✅ ĐÃ ĐĂNG NHẬP THÀNH CÔNG! Đang lắng nghe tin nhắn...\n");

// 2. Lắng nghe tin nhắn đến
api.listener.on("message", (message) => {
  const eventData = normalizeMessage(message);
  if (!eventData) return;

  const content = eventData.kind === "image" ? "[Ảnh] " + eventData.content : eventData.content;
  console.log(`📩 [${eventData.sender_name}]: ${content}`);

  // 👈 MỚI: Bắn tin nhắn sang Go CLI qua WebSocket!
  broadcast("new_message", eventData);
});

// Bắt đầu lắng nghe
api.listener.start();
