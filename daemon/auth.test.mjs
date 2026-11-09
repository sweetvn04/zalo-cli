import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { loginWithSession } from "./auth.mjs";

function files(t) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "zalo-auth-"));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  return { sessionFile: path.join(directory, "session.json"), qrFile: path.join(directory, "qr.png") };
}

const api = { getContext() {
  return { imei: "test-imei", cookie: { serializeSync: () => ({ test: "cookie" }) }, userAgent: "test-agent" };
} };

test("phiên hợp lệ được dùng lại, không yêu cầu QR", async t => {
  const options = files(t);
  fs.writeFileSync(options.sessionFile, JSON.stringify({ imei: "saved" }));
  const result = await loginWithSession({
    async login(session) { assert.equal(session.imei, "saved"); return api; },
    async loginQR() { assert.fail("must not request QR for valid session"); },
  }, options);
  assert.equal(result, api);
});

test("phiên bị từ chối hoặc JSON hỏng tự chuyển QR và lưu 0600", async t => {
  for (const saved of ['{"imei":"expired"}', '{']) {
    const options = files(t);
    fs.writeFileSync(options.sessionFile, saved);
    fs.writeFileSync(options.qrFile, "test-QR");
    let callbacks = 0;
    const onQR = () => { callbacks++; };
    const result = await loginWithSession({
      async login() { throw new Error("Đăng nhập thất bại"); },
      async loginQR(settings, callback) {
        assert.equal(settings.qrPath, options.qrFile);
        callback();
        assert.equal(fs.readFileSync(options.sessionFile, "utf-8"), saved);
        return api;
      },
    }, { ...options, onQR });
    assert.equal(result, api);
    assert.equal(callbacks, 1);
    assert.equal(JSON.parse(fs.readFileSync(options.sessionFile, "utf-8")).imei, "test-imei");
    assert.equal(fs.statSync(options.sessionFile).mode & 0o777, 0o600);
    assert.equal(fs.existsSync(options.qrFile), false);
    assert.deepEqual(fs.readdirSync(path.dirname(options.sessionFile)), ["session.json"]);
  }
});

test("QR thất bại giữ nguyên phiên cũ", async t => {
  const options = files(t);
  const saved = '{"imei":"old"}';
  fs.writeFileSync(options.sessionFile, saved);
  await assert.rejects(loginWithSession({
    async login() { throw new Error("expired"); },
    async loginQR() { throw new Error("QR bị hủy"); },
  }, options), /QR bị hủy/);
  assert.equal(fs.readFileSync(options.sessionFile, "utf-8"), saved);
});

test("lần đầu đăng nhập trực tiếp bằng QR", async t => {
  const options = files(t);
  await loginWithSession({
    async login() { assert.fail("no session exists"); },
    async loginQR() { return api; },
  }, options);
  assert.equal(fs.existsSync(options.sessionFile), true);
});
