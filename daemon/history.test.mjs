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
