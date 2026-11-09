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
