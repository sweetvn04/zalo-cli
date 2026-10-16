import { Zalo } from "zca-js";
import fs from "node:fs";
import { execSync } from "node:child_process";

const SESSION_FILE = "session.json";
const QR_FILE = "qr.png";
const zalo = new Zalo();
let api;

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
  if (message.isSelf) return;

  const senderName = message.data.dName || "Người lạ";
  const content = message.data.content;

  console.log(`📩 [${senderName}]: ${content}`);
});

// Bắt đầu lắng nghe
api.listener.start();
