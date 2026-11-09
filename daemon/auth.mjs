import fs from "node:fs";
import { randomUUID } from "node:crypto";

export async function loginWithSession(zalo, { sessionFile, qrFile, onQR }) {
  if (fs.existsSync(sessionFile)) {
    console.log("🔄 Đang khôi phục phiên đăng nhập cũ...");
    try {
      const session = JSON.parse(fs.readFileSync(sessionFile, "utf-8"));
      const api = await zalo.login(session);
      return api;
    } catch {
      console.warn("⚠️ Không khôi phục được phiên đã lưu. Đang chuyển sang đăng nhập QR...");
    }
  }

  console.log("📱 Đang yêu cầu mã QR chính chủ từ server Zalo...");
  const api = await zalo.loginQR({ qrPath: qrFile }, onQR);
  const ctx = api.getContext();
  const data = {
    imei: ctx.imei,
    cookie: ctx.cookie.serializeSync(),
    userAgent: ctx.userAgent,
  };
  // Ghi hoàn chỉnh file riêng trước khi thay phiên cũ. Nếu QR hoặc ghi file
  // thất bại, session cũ vẫn còn. Đuôi .cookie nằm trong .gitignore.
  const temporary = `${sessionFile}.${randomUUID()}.cookie`;
  try {
    fs.writeFileSync(temporary, JSON.stringify(data, null, 2), { mode: 0o600, flag: "wx" });
    fs.renameSync(temporary, sessionFile);
  } finally {
    fs.rmSync(temporary, { force: true });
  }
  console.log("💾 Đã lưu phiên đăng nhập mới.");
  if (fs.existsSync(qrFile)) fs.unlinkSync(qrFile);
  return api;
}
