import test from "node:test";
import assert from "node:assert/strict";
import { loadRecipients } from "./recipients.mjs";
import { handleCommand } from "./ipc.mjs";

test("cache F5 60 giây, hết hạn mới gọi lại Zalo", async () => {
  let calls = 0;
  const api = {
    async getAllFriends() { calls++; return [{ userId: "1", displayName: "An" }]; },
    async getAllGroups() { return { gridVerMap: {} }; },
  };
  const first = await loadRecipients(api, 0);
  const cached = await loadRecipients(api, 1000);
  assert.deepEqual(cached.recipients, first.recipients);
  assert.equal(calls, 1);
  assert.equal(cached.retry_after, 59);
  await loadRecipients(api, 60001);
  assert.equal(calls, 2);
});

test("429 giữ cache cũ và không gọi mạng trong thời gian chờ", async () => {
  let calls = 0;
  const api = {
    async getAllFriends() {
      if (++calls > 1) throw new Error("Request failed with status code 429");
      return [{ userId: "1", displayName: "An" }];
    },
    async getAllGroups() { return { gridVerMap: {} }; },
  };
  await loadRecipients(api, 0);
  const limited = await loadRecipients(api, 60001);
  assert.equal(limited.recipients[0].name, "An");
  assert.match(limited.notice, /giới hạn/);
  assert.equal(limited.retry_after, 60);
  await loadRecipients(api, 61001);
  assert.equal(calls, 2);
  const repeated = await loadRecipients(api, 120002);
  assert.equal(calls, 3);
  assert.equal(repeated.retry_after, 120);
});

test("nhiều client dùng chung một lượt tải danh sách", async () => {
  let calls = 0;
  let finish;
  const waiting = new Promise(resolve => { finish = resolve; });
  const api = {
    async getAllFriends() { calls++; await waiting; return []; },
    async getAllGroups() { return { gridVerMap: {} }; },
  };
  const first = loadRecipients(api, 0);
  const second = loadRecipients(api, 0);
  finish();
  assert.deepEqual(await first, await second);
  assert.equal(calls, 1);
});

test("429 khi chưa có cache trả lỗi tiếng Việt cùng request ID và retry_after", async () => {
  let calls = 0;
  const api = { async getAllFriends() {
    calls++;
    throw new Error("Request failed with status code 429");
  } };
  const request = JSON.stringify({ id: "list-429", action: "list_recipients" });
  const result = await handleCommand(request, api);
  assert.equal(result.id, "list-429");
  assert.equal(result.ok, false);
  assert.equal(result.retry_after, 60);
  assert.match(result.error, /giới hạn yêu cầu/);
  await handleCommand(request, api);
  assert.equal(calls, 1);
});
