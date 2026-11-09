import { Zalo, ThreadType } from "zca-js";
import { handleCommand } from "./ipc.mjs";
import { normalizeMessage } from "./history.mjs";
import { loginWithSession } from "./auth.mjs";
import { Archive, archiveRoot } from "./archive.mjs";
import { syncArchive } from "./archive-sync.mjs";
import { loadRecipients } from "./recipients.mjs";
import { parseArgs } from "node:util";
import path from "node:path";
import fs from "node:fs";
import { execFileSync } from "node:child_process";
import WebSocket, { WebSocketServer } from "ws";

const PORT = 8989;
const SESSION_FILE = "session.json";
const QR_FILE = "qr.png";
const { values: options } = parseArgs({ options: { offline: { type: "boolean", default: false }, account: { type: "string" } } });
const zalo = new Zalo({ selfListen: true });
let api;
let archive;
let syncRun;
const syncAbort = new AbortController();

// khoi tao WebSocket
const wss = new WebSocketServer({
  port: PORT,
  host: "127.0.0.1",
  // CLI native không gửi Origin; không cho trang web đọc kho local qua WebSocket.
  verifyClient: ({ origin }) => !origin,
});
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
    const result = await handleCommand(raw, api, archive);
    if (socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(result));
    }
  });

  socket.on("close", () => {
    console.log("🔌 [IPC] Client đã ngắt kết nối.");
  });
  socket.on("error", error => console.warn("⚠️ Kết nối CLI gặp lỗi:",error.message));
});

// Khôi phục phiên; nếu thất bại tự yêu cầu QR và chỉ thay session sau khi thành công.
if (!options.offline) {
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
  archive = new Archive(String(api.getOwnId()), { onMedia: event => broadcast(event.event, event.data) });
} else {
  const root = archiveRoot();
  const accounts = fs.existsSync(root) ? fs.readdirSync(root).filter(id => /^\d{1,32}$/.test(id) && fs.existsSync(path.join(root,id,"messages.sqlite3"))) : [];
  const account = options.account || (accounts.length === 1 ? accounts[0] : "");
  if (!account || !accounts.includes(account)) throw new Error("Chọn kho đã lưu bằng --offline --account ID. Các tài khoản: "+accounts.join(", "));
  archive = new Archive(account, { worker: false });
}

console.log("💾 Kho tin nhắn trên máy:", archive.directory);
if (options.offline) console.log("📂 Đang đọc kho offline; gửi tin cần đăng nhập Zalo.");

if (api) {
  console.log("✅ ĐÃ ĐĂNG NHẬP THÀNH CÔNG! Đang lắng nghe tin nhắn...\n");

  // 2. Lắng nghe tin nhắn đến
  api.listener.on("message", (message) => {
    let eventData;
    try {
      eventData = archive.capture(message);
    } catch (error) {
      console.error("❌ Không lưu được tin trên máy:",error.message);
      broadcast("archive_status", { notice: "Không lưu được tin trên máy: "+error.message });
      eventData = normalizeMessage(message);
    }
    if (!eventData) return;

    const content = eventData.kind === "image" ? "[Ảnh] " + eventData.content : eventData.content;
    console.log(`📩 [${eventData.sender_name}]: ${content}`);

    // 👈 MỚI: Bắn tin nhắn sang Go CLI qua WebSocket!
    broadcast("new_message", eventData);
  });
  api.listener.on("old_messages", (messages, type) => {
    try {
      archive.captureBatch(messages);
      broadcast("archive_updated", {thread_type:type === ThreadType.Group ? "group" : "user"});
    } catch (error) {
      console.error("❌ Không ghi được lịch sử vào kho:",error.message);
      broadcast("archive_status", {notice:"Không ghi được lịch sử trên máy: "+error.message});
    }
  });
  let connectionAbort = new AbortController();
  let syncQueued = false;
  function beginSync() {
    if (syncAbort.signal.aborted || connectionAbort.signal.aborted) return;
    if (syncRun) { syncQueued = true; return; }
    syncQueued = false;
    const signal = AbortSignal.any([syncAbort.signal,connectionAbort.signal]);
    syncRun = syncArchive(api,archive,signal).catch(error => {
      console.error("❌ Đồng bộ kho local tạm dừng:",error.message);
      broadcast("archive_status",{notice:"Đồng bộ local tạm dừng: "+error.message});
    }).finally(() => {
      syncRun = null;
      if (syncQueued) beginSync();
    });
  }
  // connected chỉ báo socket mở; cipher_key mới báo sẵn sàng giải mã lịch sử.
  api.listener.on("cipher_key", () => {
    connectionAbort = new AbortController();
    beginSync();
  });
  api.listener.on("disconnected", () => connectionAbort.abort());
  api.listener.on("error", error => {
    console.warn("⚠️ Kết nối Zalo gặp lỗi:",error.message || String(error));
    broadcast("archive_status",{notice:"Kết nối Zalo gặp lỗi; kho local vẫn đọc được."});
  });
  api.listener.on("connected", () => {
    loadRecipients(api).then(list => archive.saveRecipients(list.recipients)).catch(error => console.warn("⚠️ Danh sách người nhận chưa cập nhật:",error.message));
  });

  // Bắt đầu lắng nghe
  api.listener.start();
}

let closing = false;
async function shutdown() {
  if (closing) return;
  closing = true;
  syncAbort.abort();
  api?.listener.stop();
  for (const client of wss.clients) client.terminate();
  wss.close();
  await syncRun;
  await archive.close();
  console.log("💾 Đã đóng kho local an toàn.");
}
process.on("SIGINT", () => shutdown().catch(error => { console.error(error.message); process.exitCode=1; }));
process.on("SIGTERM", () => shutdown().catch(error => { console.error(error.message); process.exitCode=1; }));
