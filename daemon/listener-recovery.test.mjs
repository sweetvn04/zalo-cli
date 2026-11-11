import test from "node:test";
import assert from "node:assert/strict";
import { EventEmitter, once } from "node:events";
import { recoverListener } from "./listener-recovery.mjs";

test("sau sleep, bỏ socket cũ và bắt tay lại đúng một lần", async t => {
  const listener = new EventEmitter();
  const controller = new AbortController();
  t.after(()=>controller.abort());
  let time = 0;
  let reconnects = 0;
  let starts = 0;
  listener.start = () => {
    starts++;
    listener.ws = {terminate() {
      queueMicrotask(()=>{
        listener.ws = null;
        listener.emit("disconnected");
      });
    }};
    listener.emit("cipher_key");
  };
  const recovery = recoverListener(listener,{signal:controller.signal,now:()=>time,onReconnect:()=>reconnects++});
  const connected = once(listener,"cipher_key");
  const keepAlive = setTimeout(()=>controller.abort(),3000);
  t.after(()=>clearTimeout(keepAlive));
  time = 60000;
  recovery.tick();
  recovery.reconnect();
  assert.equal(reconnects,1);
  await connected;
  assert.equal(starts,2);
});

test("socket im lặng và không trả pong được nối lại", t => {
  const listener = new EventEmitter();
  const controller = new AbortController();
  t.after(()=>controller.abort());
  let time = 0;
  let probes = 0;
  let reconnects = 0;
  listener.start = () => {
    listener.ws = Object.assign(new EventEmitter(),{readyState:1,ping:()=>probes++,terminate:()=>{}});
    listener.emit("cipher_key");
  };
  const recovery = recoverListener(listener,{signal:controller.signal,now:()=>time,onReconnect:()=>reconnects++});
  for (time=5000;time<=20000;time+=5000) recovery.tick();
  assert.equal(probes,1);
  assert.equal(reconnects,1);
});
