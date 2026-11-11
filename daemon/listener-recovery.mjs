// Own reconnects here because zca-js only retries selected close codes.
export function recoverListener(listener, {signal, onReconnect, now = Date.now, interval = 5000} = {}) {
  let lastTick = now();
  let started = lastTick;
  let online = false;
  let recycling = false;
  let duplicate = false;
  let retry;
  let failures = 0;
  let ping;

  function start() {
    retry = undefined;
    recycling = false;
    if (signal.aborted) return;
    started = now();
    try { listener.start(); }
    catch { schedule(); }
  }

  function schedule() {
    if (signal.aborted || retry) return;
    retry = setTimeout(start, Math.min(30000, 1000 * 2 ** Math.min(failures++, 5)));
    retry.unref();
  }

  function reconnect() {
    if (signal.aborted || retry || recycling) return;
    recycling = true;
    duplicate = false;
    online = false;
    ping = undefined;
    onReconnect();
    // Terminate a half-open Node ws socket. Its close handler resets the SDK
    // listener before emitting disconnected, so the next start cannot race it.
    if (listener.ws?.terminate) listener.ws.terminate();
    else if (listener.ws) listener.stop();
    else schedule();
  }

  function tick() {
    const time = now();
    const gap = time - lastTick;
    lastTick = time;
    if (duplicate) return;
    if (gap > interval * 3 || (!online && time - started >= 20000) || (ping && time - ping.time >= 15000)) {
      reconnect();
      return;
    }
    const socket = listener.ws;
    if (online && !ping && socket?.readyState === 1 && socket.ping) {
      const probe = {time};
      ping = probe;
      socket.once("pong", () => { if (ping === probe) ping = undefined; });
      socket.ping();
    }
  }

  listener.on("cipher_key", () => { online = true; failures = 0; ping = undefined; });
  listener.on("disconnected", code => {
    online = false;
    recycling = false;
    ping = undefined;
    duplicate = code === 3000;
    if (!duplicate) schedule();
  });
  const timer = setInterval(tick, interval);
  timer.unref();
  signal.addEventListener("abort", () => { clearInterval(timer); clearTimeout(retry); }, {once:true});
  start();
  return {reconnect,tick};
}
