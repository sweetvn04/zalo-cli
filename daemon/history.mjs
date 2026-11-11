import { ThreadType } from "zca-js";

// Dùng cùng schema cho tin realtime và lịch sử.
export function normalizeMessage(message) {
  let content = message.data?.content;
  let kind = "text";
  let imageURL = "";
  let stickerID = 0;
  if (message.data?.msgType === "chat.sticker") {
    kind = "sticker";
    if (typeof content === "string") {
      try { content = JSON.parse(content); } catch { content = null; }
    }
    stickerID = Number(content?.sticker_id ?? content?.stickerId ?? content?.id);
    content = "";
  } else if (["chat.photo", "chat.video.msg"].includes(message.data?.msgType)) {
    kind = message.data.msgType === "chat.photo" ? "image" : "video";
    if (typeof content === "string") {
      try { content = JSON.parse(content); } catch { content = { href: content }; }
    }
    if (!content || typeof content !== "object") content = {};
    let params = content.params ?? {};
    if (typeof params === "string") {
      try { params = JSON.parse(params); } catch { params = {}; }
    }
    if (!params || typeof params !== "object") params = {};
    const candidates = kind === "video" ? [content.videoUrl, params.videoUrl, content.href] : [content.hdUrl, params.hdUrl, content.rawUrl, params.rawUrl,
      content.oriUrl, params.oriUrl, content.normalUrl, params.normalUrl,
      content.href, content.thumb, content.thumbUrl, params.thumbUrl];
    imageURL = candidates.find(value => {
      if (typeof value !== "string") return false;
      try {
        const url = new URL(value);
        return ["https:", "http:"].includes(url.protocol) && !url.username && !url.password;
      } catch { return false; }
    }) ?? "";
    content = String(content.title || content.description || "");
  } else if (typeof content !== "string") {
    return null;
  }
  const timestamp = Number(message.data.ts);
  return {
    msg_id: String(message.data.msgId ?? ""),
    thread_id: String(message.threadId),
    thread_type: message.type === ThreadType.Group ? "group" : "user",
    sender_id: String(message.data.uidFrom ?? ""),
    sender_name: message.isSelf ? "Bạn" : message.data.dName || "Người gửi",
    content,
    kind,
    image_url: kind === "image" ? imageURL : "",
    ...(kind === "sticker" && Number.isSafeInteger(stickerID) && stickerID > 0 ? {sticker_id:stickerID} : {}),
    ...(kind === "video" ? { video_url: imageURL } : {}),
    timestamp: Number.isSafeInteger(timestamp) ? timestamp : 0,
    is_self: !!message.isSelf,
  };
}

const snapshots = new WeakMap();
const unavailableGroupHTTP = new WeakSet();

// Đồng bộ nền dùng cùng hàng đợi WebSocket với các yêu cầu từ giao diện.
export function archiveHistoryPage(api, type, beforeID = "") {
  return oldMessageSnapshot(api, type === "group" ? ThreadType.Group : ThreadType.User, beforeID, 1);
}

// old_messages không có request ID. Chỉ một chuỗi phân trang được chạy trên
// mỗi loại hội thoại; các client cùng chờ snapshot rồi lọc đúng hội thoại.
async function oldMessageSnapshot(api, threadType, beforeID = "", maxPages = 10) {
  let states = snapshots.get(api);
  if (!states) {
    states = new Map();
    snapshots.set(api, states);
  }
  let state = states.get(threadType);
  if (!state) {
    state = { blocked: false, time: 0, result: null, pending: null };
    states.set(threadType, state);
    api.listener.on("connected", () => {
      state.blocked = false;
      state.time = 0;
    });
  }
  if (state.pending) {
    const requestedCursor = state.requestedCursor;
    const result = await state.pending;
    if (requestedCursor === beforeID) return result;
    return oldMessageSnapshot(api, threadType, beforeID, maxPages);
  }
  if (state.result && state.requestedCursor === beforeID && Date.now() - state.time < 30000) return state.result;
  const label = threadType === ThreadType.Group ? "nhóm" : "cá nhân";
  if (state.blocked) throw new Error(`Lịch sử ${label} chưa phản hồi; cần kết nối lại daemon trước khi thử lại`);
  state.requestedCursor = beforeID;
  state.pending = (async () => {
    const all = [];
    const seen = new Set();
    let cursor = beforeID || null;
    let partial = true;
    let more = true;
    const deadline = Date.now() + 15000;
    for (let page = 0; page < maxPages; page++) {
      const remaining = deadline - Date.now();
      if (remaining <= 0) break;
      const messages = await new Promise((resolve, reject) => {
        const cleanup = () => {
          clearTimeout(timer);
          api.listener.off("old_messages", onMessages);
          api.listener.off("disconnected", onDisconnected);
          api.listener.off("closed", onDisconnected);
        };
        const onDisconnected = () => {
          cleanup();
          state.blocked = true;
          reject(new Error("Kết nối Zalo đã ngắt khi tải lịch sử"));
        };
        const onMessages = (batch, type) => {
          if (type !== threadType) return;
          cleanup();
          resolve(batch);
        };
        const timer = setTimeout(() => {
          cleanup();
          // Không cho request mới nhầm phản hồi đến muộn của trang đã timeout.
          state.blocked = true;
          reject(new Error(`Hết thời gian chờ lịch sử ${label} từ Zalo`));
        }, Math.min(5000, remaining));
        api.listener.on("old_messages", onMessages);
        api.listener.once("disconnected", onDisconnected);
        api.listener.once("closed", onDisconnected);
        try {
          api.listener.requestOldMessages(threadType, cursor);
        } catch (error) {
          cleanup();
          reject(error);
        }
      }).catch(error => {
        if (all.length) return null;
        throw error;
      });
      if (messages === null) { more = false; break; }
      if (!messages.length) {
        partial = false;
        more = false;
        break;
      }
      let oldest = null;
      let added = 0;
      for (const message of messages) {
        const id = String(message.data.msgId ?? "");
        const identity = message.threadId + ":" + id;
        if (!seen.has(identity)) {
          seen.add(identity);
          all.push(message);
          added++;
        }
        if (/^\d+$/.test(id) && (oldest === null || BigInt(id) < BigInt(oldest))) oldest = id;
      }
      if (!added || oldest === null || (cursor !== null && BigInt(oldest) >= BigInt(cursor))) {
        more = false;
        break;
      }
      cursor = oldest;
    }
    return { messages: all, partial, cursor: cursor ?? "", more };
  })();
  try {
    state.result = await state.pending;
    state.time = Date.now();
    return state.result;
  } finally {
    state.pending = null;
  }
}

export async function loadHistory(api, threadID, threadType, beforeID = "") {
  let raw;
  let partial;
  let cursor = "";
  let more = false;
  if (threadType === "group") {
    if (!beforeID && !unavailableGroupHTTP.has(api)) {
      try {
        const response = await api.getGroupChatHistory(threadID, 100);
        raw = response.groupMsgs;
        partial = Number(response.more) > 0;
        more = partial;
        for (const message of raw) {
          const id = String(message.data?.msgId ?? "");
          if (/^\d+$/.test(id) && (!cursor || BigInt(id) < BigInt(cursor))) cursor = id;
        }
      } catch (error) {
        // zca-js chỉ giữ message HTTP, không đưa status vào ZaloApiError.
        if (error.message !== "Request failed with status code 404") throw error;
        unavailableGroupHTTP.add(api);
      }
    }
    if (!raw) {
      const response = await oldMessageSnapshot(api, ThreadType.Group, beforeID);
      raw = response.messages;
      partial = response.partial;
      cursor = response.cursor;
      more = response.more;
    }
  } else {
    const response = await oldMessageSnapshot(api, ThreadType.User, beforeID);
    raw = response.messages;
    partial = response.partial;
    cursor = response.cursor;
    more = response.more;
  }
  const messages = raw.map(normalizeMessage).filter(message => message &&
    message.thread_id === threadID && message.thread_type === threadType &&
    (!beforeID || (/^\d+$/.test(message.msg_id) && BigInt(message.msg_id) < BigInt(beforeID))));
  messages.sort((a, b) => a.timestamp - b.timestamp);
  const selected = messages.slice(-100);
  // Nếu đã lấy nhiều hơn 100 tin của chat này, trang kế tiếp phải bắt đầu
  // từ tin cũ nhất vừa hiển thị, tránh bỏ qua phần bị cắt trong snapshot.
  if (messages.length > 100 || (!cursor && selected.length)) {
    cursor = selected.reduce((oldest, message) => {
      if (!/^\d+$/.test(message.msg_id)) return oldest;
      return !oldest || BigInt(message.msg_id) < BigInt(oldest) ? message.msg_id : oldest;
    }, "");
  }
  more = more || messages.length > 100;
  if (!cursor || (beforeID && BigInt(cursor) >= BigInt(beforeID))) more = false;
  return { messages: selected, partial: partial || messages.length > 100, cursor, has_more: more };
}
