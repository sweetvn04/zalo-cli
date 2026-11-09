import { ThreadType } from "zca-js";

// Dùng cùng schema cho tin realtime và lịch sử.
export function normalizeMessage(message) {
  if (typeof message.data?.content !== "string") return null;
  const timestamp = Number(message.data.ts);
  return {
    msg_id: String(message.data.msgId ?? ""),
    thread_id: String(message.threadId),
    thread_type: message.type === ThreadType.Group ? "group" : "user",
    sender_id: String(message.data.uidFrom ?? ""),
    sender_name: message.isSelf ? "Bạn" : message.data.dName || "Người gửi",
    content: message.data.content,
    timestamp: Number.isSafeInteger(timestamp) ? timestamp : 0,
    is_self: !!message.isSelf,
  };
}

const snapshots = new WeakMap();

// old_messages không có request ID. Chỉ một chuỗi phân trang được chạy trên
// mỗi listener; các client cùng chờ một snapshot và tự lọc theo hội thoại.
async function userSnapshot(api) {
  let state = snapshots.get(api);
  if (!state) {
    state = { blocked: false, time: 0, result: null, pending: null };
    snapshots.set(api, state);
    api.listener.on("connected", () => {
      state.blocked = false;
      state.time = 0;
    });
  }
  if (state.pending) return state.pending;
  if (state.result && Date.now() - state.time < 30000) return state.result;
  if (state.blocked) throw new Error("Lịch sử cá nhân chưa phản hồi; cần kết nối lại daemon trước khi thử lại");
  state.pending = (async () => {
    const all = [];
    const seen = new Set();
    let cursor = null;
    let partial = true;
    const deadline = Date.now() + 15000;
    for (let page = 0; page < 10; page++) {
      const remaining = deadline - Date.now();
      if (remaining <= 0) break;
      const messages = await new Promise((resolve, reject) => {
        const cleanup = () => {
          clearTimeout(timer);
          api.listener.off("old_messages", onMessages);
        };
        const onMessages = (batch, type) => {
          if (type !== ThreadType.User) return;
          cleanup();
          resolve(batch);
        };
        const timer = setTimeout(() => {
          cleanup();
          // Không cho request mới nhầm phản hồi đến muộn của trang đã timeout.
          state.blocked = true;
          reject(new Error("Hết thời gian chờ lịch sử cá nhân từ Zalo"));
        }, Math.min(5000, remaining));
        api.listener.on("old_messages", onMessages);
        try {
          api.listener.requestOldMessages(ThreadType.User, cursor);
        } catch (error) {
          cleanup();
          reject(error);
        }
      }).catch(error => {
        if (all.length) return null;
        throw error;
      });
      if (messages === null) break;
      if (!messages.length) {
        partial = false;
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
      if (!added || oldest === null || oldest === cursor) break;
      cursor = oldest;
    }
    return { messages: all, partial };
  })();
  try {
    state.result = await state.pending;
    state.time = Date.now();
    return state.result;
  } finally {
    state.pending = null;
  }
}

export async function loadHistory(api, threadID, threadType) {
  let raw;
  let partial;
  if (threadType === "group") {
    const response = await api.getGroupChatHistory(threadID, 100);
    raw = response.groupMsgs;
    partial = Number(response.more) > 0;
  } else {
    const response = await userSnapshot(api);
    raw = response.messages;
    partial = response.partial;
  }
  const messages = raw.map(normalizeMessage).filter(message => message &&
    message.thread_id === threadID && message.thread_type === threadType);
  messages.sort((a, b) => a.timestamp - b.timestamp);
  return { messages: messages.slice(-100), partial: partial || messages.length > 100 };
}
