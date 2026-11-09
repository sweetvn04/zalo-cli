import test from "node:test";
import assert from "node:assert/strict";
import { ThreadType } from "zca-js";
import { handleCommand } from "./ipc.mjs";

const command = {
  id: "req-7", action: "send_message",
  payload: { thread_id: "test-user", thread_type: "user", text: "Chào Bảo" },
};

test("chờ Zalo hoàn thành rồi trả đúng mã yêu cầu", async () => {
  let finish;
  const pending = new Promise(resolve => { finish = resolve; });
  let settled = false;
  const response = handleCommand(JSON.stringify(command), {
    async sendMessage(message, thread, kind) {
      assert.deepEqual(message, { msg: "Chào Bảo" });
      assert.equal(thread, "test-user");
      assert.equal(kind, ThreadType.User);
      await pending;
    },
  }).then(value => { settled = true; return value; });
  await Promise.resolve();
  assert.equal(settled, false);
  finish();
  assert.deepEqual(await response, { event: "send_result", id: "req-7", ok: true });
});

test("trả lỗi Zalo và mã yêu cầu cho client", async () => {
  const result = await handleCommand(JSON.stringify(command), {
    async sendMessage() { throw new Error("Zalo từ chối"); },
  });
  assert.equal(result.id, "req-7");
  assert.equal(result.ok, false);
  assert.equal(result.error, "Zalo từ chối");
});

test("từ chối khi chưa đăng nhập hoặc payload không hợp lệ", async () => {
  const notReady = await handleCommand(JSON.stringify(command), undefined);
  assert.equal(notReady.ok, false);
  assert.match(notReady.error, /chưa đăng nhập/);
  let calls = 0;
  const api = { async sendMessage() { calls++; } };
  for (const raw of ["{", "null", JSON.stringify({ ...command, payload: {} })]) {
    assert.equal((await handleCommand(raw, api)).ok, false);
  }
  assert.equal(calls, 0);
});

test("gửi nhóm dùng đúng ThreadType.Group", async () => {
  const group = { ...command, payload: { ...command.payload, thread_type: "group" } };
  const response = await handleCommand(JSON.stringify(group), {
    async sendMessage(message, thread, kind) { assert.equal(kind, ThreadType.Group); },
  });
  assert.equal(response.ok, true);
});


test("danh sách gồm bạn bè và nhóm được chuẩn hóa, phân trang và bỏ nhóm đã rời", async () => {
  const pages = [];
  const result = await handleCommand(JSON.stringify({ id: "list-1", action: "list_recipients" }), {
    async getAllFriends(count, page) {
      pages.push([count, page]);
      if (page === 1) return Array.from({ length: 200 }, (_, i) => ({
        userId: String(i), displayName: "Bạn " + i,
      }));
      return [{ userId: "last", displayName: "An" }];
    },
    async getAllGroups() { return { gridVerMap: { "group-1": "v1", "removed": "v1" } }; },
    async getGroupInfo(ids) {
      assert.deepEqual(ids, ["group-1", "removed"]);
      return { gridInfoMap: { "group-1": { name: "Nhóm Go" } } };
    },
  });
  assert.equal(result.id, "list-1");
  assert.equal(result.event, "recipients_result");
  assert.equal(result.ok, true);
  assert.equal(result.recipients.length, 202);
  assert.deepEqual(pages, [[200, 1], [200, 2]]);
  assert.equal(result.recipients[0].name, "An");
  assert.ok(result.recipients.some(r => r.thread_id === "group-1" && r.thread_type === "group"));
});

test("lỗi lấy danh sách trả đúng loại sự kiện và mã yêu cầu", async () => {
  const result = await handleCommand(JSON.stringify({ id: "list-2", action: "list_recipients" }), {
    async getAllFriends() { throw new Error("Mất mạng"); },
  });
  assert.deepEqual(result, { event: "recipients_result", id: "list-2", ok: false, error: "Mất mạng" });
});
