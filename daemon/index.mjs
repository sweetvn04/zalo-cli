import { Zalo, ThreadType } from "zca-js";
import { handleCommand } from "./ipc.mjs";
import { normalizeMessage } from "./history.mjs";
import { loginWithSession } from "./auth.mjs";
import { Archive, archiveRoot } from "./archive.mjs";
import { syncArchive } from "./archive-sync.mjs";
import { createGroupSync } from "./group-sync.mjs";
import { recoverListener } from "./listener-recovery.mjs";
import { loadRecipients } from "./recipients.mjs";
import { parseArgs } from "node:util";
import path from "node:path";
import fs from "node:fs";
import { execFileSync, spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import WebSocket, { WebSocketServer } from "ws";

const PORT = 8989;
const directory = path.dirname(fileURLToPath(import.meta.url));
const SESSION_FILE = path.join(directory,"session.json");
const QR_FILE = path.join(directory,"qr.png");
const { values: options } = parseArgs({ options: { managed: {type:"boolean",default:false}, offline: { type: "boolean", default: false }, account: { type: "string" } } });
const zalo = new Zalo({ selfListen: true });
let api;
let archive;
let syncRun;
let groupSync;
let groupScan;
let listenerRecovery;
let requestedGroup;
const syncAbort = new AbortController();
let online = false;
let loginNotice = options.offline ? "Đang đọc kho offline." : "Đang đọc kho local; kết nối Zalo và đồng bộ nền...";
const root = archiveRoot();
const accounts = fs.existsSync(root) ? fs.readdirSync(root).filter(id => /^\d{1,32}$/.test(id) && fs.existsSync(path.join(root,id,"messages.sqlite3"))) : [];
const localAccount = options.account || (accounts.length === 1 ? accounts[0] : "");
if (options.offline && (!localAccount || !accounts.includes(localAccount))) {
  throw new Error("Chọn kho đã lưu bằng --offline --account ID. Các tài khoản: "+accounts.join(", "));
}
// Mở kho trước khi đợi mạng/QR. Worker chỉ tải khi tài khoản đã xác minh.
if (localAccount && accounts.includes(localAccount)) archive = new Archive(localAccount,{worker:false});

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
    let action;
    try { action = JSON.parse(raw.toString()); } catch { /* handleCommand trả lỗi JSON. */ }
    if (action?.action === "daemon_status") {
      socket.send(JSON.stringify({event:"daemon_status",id:action.id,ready:!!archive,online,notice:loginNotice}));
      return;
    }
    if (action?.action === "sync_now") {
      const {thread_id,thread_type} = action.payload ?? {};
      if (!listenerRecovery) {
        socket.send(JSON.stringify({event:"archive_status",data:{notice:"Daemon chưa kết nối Zalo; đang đọc kho local."}}));
      } else if (online && thread_type === "group" && typeof thread_id === "string" && /^\d+$/.test(thread_id)) {
        groupSync.refresh(thread_id,false,true).catch(error=>broadcast("archive_status",{notice:error.message}));
      } else {
        if (thread_type === "group" && typeof thread_id === "string" && /^\d+$/.test(thread_id)) requestedGroup = thread_id;
        listenerRecovery.reconnect();
      }
      return;
    }
    if (action?.action === "list_recipients" && online) refreshRecipients();
    if (action?.action === "get_history" && action.payload?.thread_type === "group" &&
        typeof action.payload.thread_id === "string" && /^\d+$/.test(action.payload.thread_id) && online) {
      groupSync?.refresh(action.payload.thread_id,!!action.payload.archive_before).catch(error=>{
        broadcast("archive_status",{notice:"Chưa cập nhật được nhóm: "+error.message});
      });
    }
    const result = await handleCommand(raw, online ? api : undefined, archive);
    if (socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(result));
    }
  });

  socket.on("close", () => {
    console.log("🔌 [IPC] Client đã ngắt kết nối.");
  });
  socket.on("error", error => console.warn("⚠️ Kết nối CLI gặp lỗi:",error.message));
  socket.send(JSON.stringify({event:"archive_status",data:{notice:loginNotice}}));
  socket.send(JSON.stringify({event:"zalo_connection",data:{online}}));
});

let recipientRefresh;
function refreshRecipients(forceGroups = false) {
  if (!online) return;
  if (recipientRefresh) {
    if (forceGroups) recipientRefresh.then(()=>scanGroupHeads(true));
    return;
  }
  recipientRefresh = loadRecipients(api).then(list => {
    const previous = JSON.stringify(archive.recipients());
    archive.saveRecipients(list.recipients);
    if (JSON.stringify(archive.recipients()) !== previous) broadcast("recipients_updated",{});
  }).catch(error => {
    broadcast("archive_status",{notice:"Đang dùng danh sách local; chưa cập nhật được từ Zalo: "+error.message});
  }).finally(() => {
    recipientRefresh = null;
    scanGroupHeads(forceGroups);
  });
}

function scanGroupHeads(force = false) {
  if (!groupSync || !online) return;
  if (groupScan) {
    if (force) groupScan.then(()=>scanGroupHeads(true));
    return;
  }
  groupScan = (async()=>{
    for (const recipient of archive.recipients()) {
      if (syncAbort.signal.aborted || !online) return;
      if (recipient.thread_type === "group") await groupSync.refresh(recipient.thread_id,false,force);
    }
  })().catch(error=>console.warn("Chưa cập nhật được lịch sử cloud:",error.message)).finally(()=>{groupScan=null;});
}

// Khôi phục phiên; nếu thất bại tự yêu cầu QR và chỉ thay session sau khi thành công.
async function connectZalo() {
if (!options.offline) {
  api = await loginWithSession(zalo, {
    sessionFile: SESSION_FILE,
    qrFile: QR_FILE,
    onQR: async (event) => {
      if (event.type === 0 && event.data?.image) {
        fs.writeFileSync(QR_FILE, Buffer.from(event.data.image, "base64"), { mode: 0o600 });
        console.log("📷 Quét mã QR bên dưới bằng app Zalo trên điện thoại.");
        if (options.managed) {
          const viewer = spawn("xdg-open",[QR_FILE],{stdio:"ignore"});
          viewer.on("error",error => console.warn("Mở QR bằng: xdg-open "+QR_FILE,error.message));
          broadcast("archive_status",{notice:"Cần quét QR để kết nối Zalo. Ảnh QR: "+QR_FILE});
        } else try {
          execFileSync("kitten", ["icat", "--align", "left", QR_FILE], { stdio: "inherit" });
        } catch {
          console.log("Mở ảnh QR bằng: xdg-open " + QR_FILE);
        }
      }
    },
  });
  if (syncAbort.signal.aborted) { api.listener.stop(); return; }
  const account = String(api.getOwnId());
  if (archive && archive.account !== account) {
    throw new Error("Phiên Zalo khác tài khoản kho local. Dừng daemon rồi chọn đúng --account "+account);
  }
  if (archive) await archive.close();
  archive = new Archive(account, {
    onMedia: event => broadcast(event.event, event.data),
    resolveSticker: async id => {
      const details = await api.getStickersDetail(id);
      const sticker = details.find(item => Number(item.id) === id);
      return sticker?.stickerUrl || sticker?.stickerWebpUrl;
    },
  });
  groupSync = createGroupSync(api,archive,{
    signal:syncAbort.signal,
    onUpdate:()=>{broadcast("archive_updated",{thread_type:"group"});broadcast("recipients_updated",{});},
    onStatus:notice=>{console.log("☁️ "+notice);broadcast("archive_status",{notice});},
  });
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
      broadcast("recipients_updated",{});
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
    syncRun = syncArchive(api,archive,signal,notice => {
      loginNotice = notice;
      broadcast("archive_status",{notice});
    }).catch(error => {
      console.error("❌ Đồng bộ kho local tạm dừng:",error.message);
      broadcast("archive_status",{notice:"Đồng bộ local tạm dừng: "+error.message});
    }).finally(() => {
      syncRun = null;
      if (syncQueued) beginSync();
    });
  }
  // connected chỉ báo socket mở; cipher_key mới báo sẵn sàng giải mã lịch sử.
  api.listener.on("cipher_key", () => {
    online = true;
    loginNotice = "Đã kết nối Zalo; đang đồng bộ tin trong thời gian vắng mặt.";
    broadcast("archive_status",{notice:loginNotice});
    broadcast("zalo_connection",{online:true});
    if (requestedGroup) {
      groupSync.refresh(requestedGroup,false,true).catch(error=>broadcast("archive_status",{notice:error.message}));
      requestedGroup = undefined;
    }
    refreshRecipients(true);
    connectionAbort = new AbortController();
    beginSync();
  });
  api.listener.on("disconnected", code => {
    online = false;
    connectionAbort.abort();
    loginNotice = code === 3000 ? "Zalo báo có kết nối khác. F6 để kết nối lại." :
      "Mất kết nối Zalo; vẫn đọc được tin local, đang chờ kết nối lại.";
    broadcast("archive_status",{notice:loginNotice});
    broadcast("zalo_connection",{online:false});
  });
  api.listener.on("error", error => {
    console.warn("⚠️ Kết nối Zalo gặp lỗi:",error.message || String(error));
    broadcast("archive_status",{notice:"Kết nối Zalo gặp lỗi; kho local vẫn đọc được."});
  });

  // Bắt đầu lắng nghe
  listenerRecovery = recoverListener(api.listener,{signal:syncAbort.signal,onReconnect:()=>{
    online = false;
    connectionAbort.abort();
    broadcast("zalo_connection",{online:false});
    broadcast("archive_status",{notice:"Đang nối lại Zalo và đồng bộ tin mới..."});
  }});
}
}
// Không để lỗi mạng/đăng nhập lấy mất quyền đọc kho local.
let loginRetry;
function connectInBackground() { connectZalo().catch(error => {
  if (syncAbort.signal.aborted) return;
  online = false;
  loginNotice = "Chưa kết nối được Zalo; kho local vẫn đọc được: "+error.message;
  console.error(loginNotice);
  broadcast("archive_status",{notice:loginNotice});
  loginRetry = setTimeout(connectInBackground,30000);
  loginRetry.unref();
}); }
connectInBackground();

let closing = false;
async function shutdown() {
  if (closing) return;
  closing = true;
  syncAbort.abort();
  clearTimeout(loginRetry);
  api?.listener.stop();
  for (const client of wss.clients) client.terminate();
  wss.close();
  await syncRun;
  await recipientRefresh;
  await groupScan;
  await groupSync?.done();
  await archive?.close();
  console.log("💾 Đã đóng kho local an toàn.");
}
process.on("SIGINT", () => shutdown().catch(error => { console.error(error.message); process.exitCode=1; }));
process.on("SIGTERM", () => shutdown().catch(error => { console.error(error.message); process.exitCode=1; }));
