import test from "node:test";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { ThreadType } from "zca-js";
import { loadHistory, normalizeMessage } from "./history.mjs";
import { handleCommand } from "./ipc.mjs";

function message(id, thread, ts, content, type = ThreadType.User, self = false) {
  return { type, threadId: thread, isSelf: self,
    data: { msgId: id, ts: String(ts), uidFrom: self ? "me" : "friend", dName: "Bảo", content } };
}

test("nhóm lọc đúng hội thoại, chỉ lấy văn bản và sắp thời gian", async () => {
  const api = { async getGroupChatHistory(id, count) {
    assert.equal(id, "group-1");
    assert.equal(count, 100);
    return { more: 1, groupMsgs: [
      message("3", id, 3000, "Ba", ThreadType.Group),
      message("1", id, 1000, "Một", ThreadType.Group, true),
      message("2", "other", 2000, "Sai nhóm", ThreadType.Group),
      message("4", id, 4000, { title: "Ảnh" }, ThreadType.Group),
    ] };
  } };
  const response = await handleCommand(JSON.stringify({ id: "req-h", action: "get_history",
    payload: { thread_id: "group-1", thread_type: "group" } }), api);
  assert.equal(response.event, "history_result");
  assert.equal(response.id, "req-h");
  assert.equal(response.partial, true);
  assert.deepEqual(response.messages.map(m => m.msg_id), ["1", "3"]);
  assert.equal(response.messages[0].timestamp, 1000);
  assert.equal(response.messages[0].sender_name, "Bạn");
});

test("cá nhân phân trang một luồng, chia sẻ snapshot và lọc từng người", async () => {
  const listener = new EventEmitter();
  const calls = [];
  listener.requestOldMessages = (type, cursor) => {
    calls.push([type, cursor]);
    const messages = cursor === null ? [message("200", "an", 2000, "Mới")]
      : cursor === "200" ? [message("100", "bao", 1000, "Cũ")] : [];
    queueMicrotask(() => listener.emit("old_messages", messages, type));
  };
  const api = { listener };
  const [an, bao] = await Promise.all([loadHistory(api, "an", "user"), loadHistory(api, "bao", "user")]);
  assert.deepEqual(an.messages.map(m => m.content), ["Mới"]);
  assert.deepEqual(bao.messages.map(m => m.content), ["Cũ"]);
  assert.equal(an.partial, false);
  assert.deepEqual(calls, [[ThreadType.User, null], [ThreadType.User, "200"], [ThreadType.User, "100"]]);
  await loadHistory(api, "an", "user");
  assert.equal(calls.length, 3);
  assert.equal(listener.listenerCount("old_messages"), 0);
});

test("lỗi lịch sử trả đúng ID và không giữ listener chờ", async () => {
  const listener = new EventEmitter();
  listener.requestOldMessages = () => { throw new Error("Socket đã đóng"); };
  const response = await handleCommand(JSON.stringify({ id: "failed-history", action: "get_history",
    payload: { thread_id: "an", thread_type: "user" } }), { listener });
  assert.equal(response.event, "history_result");
  assert.equal(response.id, "failed-history");
  assert.equal(response.ok, false);
  assert.match(response.error, /Socket đã đóng/);
  assert.equal(listener.listenerCount("old_messages"), 0);
});

test("trang lỗi sau trang hợp lệ vẫn trả lịch sử một phần", async () => {
  const listener = new EventEmitter();
  listener.requestOldMessages = (type, cursor) => {
    if (cursor !== null) throw new Error("Không tải được trang tiếp");
    queueMicrotask(() => listener.emit("old_messages", [message("100", "an", 1000, "Đã tải")], type));
  };
  const response = await loadHistory({ listener }, "an", "user");
  assert.equal(response.partial, true);
  assert.equal(response.messages[0].content, "Đã tải");
});

test("phản hồi gửi trả ID Zalo để ghép với lịch sử", async () => {
  const response = await handleCommand(JSON.stringify({ id: "send-1", action: "send_message",
    payload: { thread_id: "an", thread_type: "user", text: "Chào" } }), {
    async sendMessage() { return { message: { msgId: 12345 } }; },
  });
  assert.equal(response.msg_id, "12345");
  assert.equal(normalizeMessage(message("1", "an", "invalid", "Chào")).timestamp, 0);
});

test("HTTP nhóm 404 chuyển sang WebSocket và chỉ lọc đúng nhóm", async () => {
  const listener = new EventEmitter();
  const calls = [];
  let httpCalls = 0;
  listener.requestOldMessages = (type, cursor) => {
    calls.push([type, cursor]);
    const batch = cursor === null ? [
      message("300", "group-1", 3000, "Lịch sử nhóm", ThreadType.Group),
      message("200", "group-2", 2000, "Nhóm khác", ThreadType.Group),
    ] : [];
    queueMicrotask(() => listener.emit("old_messages", batch, type));
  };
  const api = { listener, async getGroupChatHistory() {
    httpCalls++;
    throw new Error("Request failed with status code 404");
  } };
  const first = await handleCommand(JSON.stringify({id: "fallback", action: "get_history",
    payload: {thread_id: "group-1", thread_type: "group"}}), api);
  assert.equal(first.ok, true);
  assert.equal(first.id, "fallback");
  assert.deepEqual(first.messages.map(m => m.content), ["Lịch sử nhóm"]);
  const second = await loadHistory(api, "group-2", "group");
  assert.deepEqual(second.messages.map(m => m.content), ["Nhóm khác"]);
  assert.equal(httpCalls, 1);
  assert.deepEqual(calls, [[ThreadType.Group, null], [ThreadType.Group, "200"]]);
  assert.equal(listener.listenerCount("old_messages"), 0);
});

test("snapshot WebSocket cá nhân và nhóm đồng thời không lẫn dữ liệu", async () => {
  const listener = new EventEmitter();
  listener.requestOldMessages = (type, cursor) => {
    const batch = cursor === null ? [message("100", type === ThreadType.User ? "an" : "group-1",
      1000, type === ThreadType.User ? "Cá nhân" : "Nhóm", type)] : [];
    queueMicrotask(() => listener.emit("old_messages", batch, type));
  };
  const api = { listener, async getGroupChatHistory() {
    throw new Error("Request failed with status code 404");
  } };
  const [user, group] = await Promise.all([
    loadHistory(api, "an", "user"), loadHistory(api, "group-1", "group"),
  ]);
  assert.deepEqual(user.messages.map(m => m.content), ["Cá nhân"]);
  assert.deepEqual(group.messages.map(m => m.content), ["Nhóm"]);
});

test("lỗi quyền truy cập nhóm không bị coi là endpoint 404", async () => {
  const api = { async getGroupChatHistory() { throw new Error("Request failed with status code 403"); } };
  await assert.rejects(loadHistory(api, "group-1", "group"), /403/);
});

test("phân trang giữ đủ hơn 100 tin và dùng con trỏ mới", async () => {
  const listener = new EventEmitter();
  const cursors = [];
  listener.requestOldMessages = (type, cursor) => {
    cursors.push(cursor);
    const ceiling = cursor === null ? 251 : Number(cursor);
    const batch = Array.from({ length: Math.min(250, ceiling-1) }, (_, i) => {
      const id = ceiling-i-1;
      return message(String(id), "an", id, "Tin " + id);
    });
    queueMicrotask(() => listener.emit("old_messages", batch, type));
  };
  const api = { listener };
  let before = "";
  const loaded = [];
  for (let page = 0; page < 5; page++) {
    const response = await loadHistory(api, "an", "user", before);
    loaded.push(...response.messages.map(m => m.msg_id));
    if (!response.has_more) break;
    assert.ok(BigInt(response.cursor) < BigInt(before || "9999"));
    before = response.cursor;
  }
  assert.equal(new Set(loaded).size, 250);
  assert.equal(loaded.length, 250);
  assert.ok(cursors.includes("151"));
  assert.ok(cursors.includes("51"));
});

test("hai yêu cầu con trỏ khác nhau cùng loại được tuần tự hóa", async () => {
  const listener = new EventEmitter();
  let requestsInFlight = 0;
  listener.requestOldMessages = (type, cursor) => {
    assert.equal(requestsInFlight++, 0);
    const messages = cursor === null ? [message("200", "an", 200, "Mới")]
      : cursor === "150" ? [message("100", "an", 100, "Cũ")] : [];
    setImmediate(() => { requestsInFlight--; listener.emit("old_messages", messages, type); });
  };
  const api = { listener };
  const [first, older] = await Promise.all([
    loadHistory(api, "an", "user"), loadHistory(api, "an", "user", "150"),
  ]);
  assert.deepEqual(first.messages.map(m => m.msg_id), ["200"]);
  assert.deepEqual(older.messages.map(m => m.msg_id), ["100"]);
});

test("tin ảnh ưu tiên URL HD trong params và giữ chú thích", () => {
  const photo = message("photo-1", "an", 1000, {
    title: "Ảnh đi chơi", href: "https://photo.talk.zdn.vn/normal.jpg",
    params: JSON.stringify({hdUrl: "https://photo.talk.zdn.vn/hd.jpg"}),
  });
  photo.data.msgType = "chat.photo";
  const normalized = normalizeMessage(photo);
  assert.equal(normalized.kind, "image");
  assert.equal(normalized.image_url, "https://photo.talk.zdn.vn/hd.jpg");
  assert.equal(normalized.content, "Ảnh đi chơi");
});

test("ảnh trong lịch sử dùng cùng schema với ảnh realtime", async () => {
  const photo = message("123", "group-1", 1000, JSON.stringify({href: "https://photo.talk.zdn.vn/old.jpg"}), ThreadType.Group);
  photo.data.msgType = "chat.photo";
  const response = await loadHistory({ async getGroupChatHistory() { return {more: 0, groupMsgs: [photo]}; } }, "group-1", "group");
  assert.equal(response.messages.length, 1);
  assert.equal(response.messages[0].kind, "image");
  assert.equal(response.messages[0].image_url, "https://photo.talk.zdn.vn/old.jpg");
});

test("ảnh thiếu URL vẫn có placeholder và không nhận URL file hoặc script", () => {
  const photo = message("photo-2", "an", 1000, {href: "file:///tmp/a.png", thumb: "javascript:alert(1)"});
  photo.data.msgType = "chat.photo";
  const normalized = normalizeMessage(photo);
  assert.equal(normalized.kind, "image");
  assert.equal(normalized.image_url, "");
});

test("mất kết nối dừng trang chờ ngay và kết nối lại có thể lấy lịch sử", async () => {
  const listener = new EventEmitter();
  listener.requestOldMessages = () => queueMicrotask(()=>listener.emit("disconnected",1006,"network"));
  const api = {listener};
  await assert.rejects(loadHistory(api,"an","user"),/đã ngắt/);
  assert.equal(listener.listenerCount("old_messages"),0);
  assert.equal(listener.listenerCount("disconnected"),0);
  listener.emit("connected");
  listener.requestOldMessages = type => queueMicrotask(()=>listener.emit("old_messages",[],type));
  const result = await loadHistory(api,"an","user");
  assert.equal(result.messages.length,0);
});
