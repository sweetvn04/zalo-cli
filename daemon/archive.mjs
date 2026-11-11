import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createHash } from "node:crypto";
import Database from "better-sqlite3";
import { imageSize } from "image-size";
import { ThreadType } from "zca-js";
import { normalizeMessage } from "./history.mjs";
import { storeImage } from "./image-store.mjs";

export function archiveRoot() {
  const configured = process.env.XDG_DATA_HOME;
  return path.join(configured && path.isAbsolute(configured) ? configured : path.join(os.homedir(), ".local", "share"), "zalo-cli", "accounts");
}

export class Archive {
  constructor(account, { root = archiveRoot(), download = storeImage, resolveSticker, onMedia = () => {}, worker = true } = {}) {
    if (!/^\d{1,32}$/.test(account)) throw new Error("ID tài khoản archive không hợp lệ");
    this.account = account;
    this.directory = path.join(root, account);
    this.mediaDirectory = path.join(this.directory, "media");
    fs.mkdirSync(this.mediaDirectory, { recursive: true, mode: 0o700 });
    fs.chmodSync(this.directory, 0o700);
    fs.chmodSync(this.mediaDirectory, 0o700);
    const databaseFile = path.join(this.directory, "messages.sqlite3");
    fs.closeSync(fs.openSync(databaseFile, "a", 0o600));
    fs.chmodSync(databaseFile, 0o600);
    this.db = new Database(databaseFile);
    this.db.pragma("journal_mode = WAL");
    this.db.pragma("synchronous = FULL");
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS messages (
        thread_type TEXT NOT NULL, thread_id TEXT NOT NULL, msg_id TEXT NOT NULL,
        timestamp INTEGER NOT NULL, data TEXT NOT NULL, raw TEXT,
        media_url TEXT NOT NULL DEFAULT '', media_key TEXT NOT NULL DEFAULT '',
        media_state TEXT NOT NULL DEFAULT '', attempts INTEGER NOT NULL DEFAULT 0,
        retry_at INTEGER NOT NULL DEFAULT 0, media_error TEXT NOT NULL DEFAULT '',
        PRIMARY KEY(thread_type, thread_id, msg_id));
      CREATE INDEX IF NOT EXISTS message_history ON messages(thread_type, thread_id, timestamp DESC, msg_id DESC);
      CREATE TABLE IF NOT EXISTS recipients (thread_type TEXT NOT NULL, thread_id TEXT NOT NULL, name TEXT NOT NULL,
        PRIMARY KEY(thread_type, thread_id));
      CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
    `);
    this.put = this.db.prepare(`INSERT INTO messages(thread_type,thread_id,msg_id,timestamp,data,raw,media_url,media_state)
      VALUES(@type,@thread,@id,@time,@data,@raw,@url,@state)
      ON CONFLICT(thread_type,thread_id,msg_id) DO UPDATE SET
        timestamp=excluded.timestamp, data=excluded.data, raw=COALESCE(excluded.raw,messages.raw),
        attempts=CASE WHEN messages.media_url<>excluded.media_url AND messages.media_key='' THEN 0 ELSE messages.attempts END,
        retry_at=CASE WHEN messages.media_url<>excluded.media_url AND messages.media_key='' THEN 0 ELSE messages.retry_at END,
        media_state=CASE WHEN messages.media_key<>'' THEN 'ready' WHEN messages.media_url<>excluded.media_url THEN excluded.media_state ELSE messages.media_state END,
        media_url=excluded.media_url`);
    this.contact = this.db.prepare("INSERT INTO recipients VALUES(?,?,?) ON CONFLICT(thread_type,thread_id) DO NOTHING");
    this.writeMessage = this.db.transaction((parameters, name) => {
      this.put.run(parameters);
      this.contact.run(parameters.type,parameters.thread,name);
    });
    this.download = download;
    this.resolveSticker = resolveSticker;
    this.onMedia = onMedia;
    this.abort = new AbortController();
    this.closed = false;
    this.enabled = worker;
    // Re-index videos already retained as raw attachments before video support.
    if (!this.meta("video-schema-v1")) {
      this.db.transaction(() => {
        const rows = this.db.prepare("SELECT raw FROM messages WHERE raw IS NOT NULL AND json_extract(raw,'$.data.msgType')='chat.video.msg' AND json_extract(data,'$.kind')<>'video'").all();
        for (const row of rows) this.capture(JSON.parse(row.raw), false);
        this.meta("video-schema-v1", true);
      })();
    }
    if (!this.meta("sticker-schema-v1")) {
      this.db.transaction(() => {
        const rows = this.db.prepare("SELECT raw FROM messages WHERE raw IS NOT NULL AND json_extract(raw,'$.data.msgType')='chat.sticker'").all();
        for (const row of rows) this.capture(JSON.parse(row.raw), false);
        this.meta("sticker-schema-v1",true);
      })();
    }
    this.db.prepare("UPDATE messages SET media_state='pending', retry_at=0 WHERE media_state='downloading'").run();
    if (worker) {
      this.timer = setInterval(() => this.wake(), 1000);
      this.timer.unref();
      this.wake();
    }
  }

  capture(raw, wake = true) {
    let message = normalizeMessage(raw);
    if (!message) {
      message = {
        msg_id: String(raw.data?.msgId ?? ""), thread_id: String(raw.threadId),
        thread_type: raw.type === ThreadType.Group ? "group" : "user", sender_id: String(raw.data?.uidFrom ?? ""),
        sender_name: raw.isSelf ? "Bạn" : raw.data?.dName || "Người gửi",
        content: `[${raw.data?.msgType || "Tin đính kèm"}]`, kind: "unsupported",
        timestamp: Number(raw.data?.ts) || 0, is_self: !!raw.isSelf,
      };
    }
    message = this.save(message, raw, wake);
    return this.withMedia(message);
  }

  captureBatch(messages) {
    this.db.transaction(() => { for (const message of messages) this.capture(message, false); })();
    this.wake();
  }

  save(message, raw = null, wake = true) {
    if (!message.thread_id || !["user","group"].includes(message.thread_type)) throw new Error("Tin nhắn thiếu hội thoại");
    const data = { ...message };
    data.timestamp = Number.isSafeInteger(data.timestamp) ? data.timestamp : 0;
    delete data.image_local;
    if (!data.msg_id) data.msg_id = "local-"+createHash("sha256").update(JSON.stringify(raw ?? data)).digest("hex");
    const url = data.kind === "video" ? data.video_url || "" : data.kind === "image" ? data.image_url || "" :
      data.kind === "sticker" && data.sticker_id ? `sticker:${data.sticker_id}` : "";
    const parameters = {type:data.thread_type,thread:data.thread_id,id:data.msg_id,time:data.timestamp || 0,
      data:JSON.stringify(data),raw:raw ? JSON.stringify(raw) : null,url,state:url ? "pending" : ""};
    const name = data.thread_type === "group" ? "Nhóm "+data.thread_id : data.is_self ? data.thread_id : data.sender_name || data.thread_id;
    if (this.db.inTransaction) {
      this.put.run(parameters);
      this.contact.run(data.thread_type,data.thread_id,name);
    } else this.writeMessage(parameters,name);
    if (wake) this.wake();
    return data;
  }

  withMedia(message) {
    const row = this.db.prepare("SELECT media_key,media_state FROM messages WHERE thread_type=? AND thread_id=? AND msg_id=?")
      .get(message.thread_type, message.thread_id, message.msg_id);
    if (!["image", "video", "sticker"].includes(message.kind)) return message;
    const wire = { ...message, image_state:row?.media_state || "" };
    if (row?.media_key) {
      const file = path.join(this.mediaDirectory,row.media_key);
      let exists = false;
      try { exists = fs.lstatSync(file).isFile(); } catch { /* File đã được di chuyển/xóa. */ }
      if (exists) wire.image_local = `archive:${this.account}/${row.media_key}`;
      else {
        this.db.prepare(`UPDATE messages SET media_key='',media_state='pending',attempts=0,retry_at=0
          WHERE thread_type=? AND thread_id=? AND msg_id=?`).run(message.thread_type,message.thread_id,message.msg_id);
        wire.image_state = "pending";
        this.wake();
      }
    }
    return wire;
  }

  async imageForSend(reference) {
    const match = typeof reference === "string" && /^archive:(\d{1,32})\/([a-f0-9]{64}\.(?:jpg|png|gif|webp|bmp))$/.exec(reference);
    if (!match || match[1] !== this.account) throw new Error("Ảnh không thuộc kho của tài khoản này");
    const key = match[2];
    if (key.endsWith(".bmp")) throw new Error("Gửi lại ảnh hỗ trợ JPG, PNG, GIF và WebP");
    const file = path.join(this.mediaDirectory,key);
    const info = fs.lstatSync(file,{throwIfNoEntry:false});
    if (!info?.isFile() || !info.size || info.size > 100*1024*1024 ||
        fs.realpathSync(file) !== path.join(fs.realpathSync(this.directory),"media",key)) {
      throw new Error("Không tìm thấy tệp ảnh hợp lệ trong kho local");
    }
    const handle = await fs.promises.open(file,fs.constants.O_RDONLY | fs.constants.O_NOFOLLOW);
    try {
      const data = await handle.readFile();
      const {width,height} = imageSize(data);
      if (!width || !height) throw new Error("Không đọc được kích thước ảnh");
      return {key,attachment:{data,filename:key,metadata:{width,height,totalSize:data.length}}};
    } finally {
      await handle.close();
    }
  }

  saveImageCopy(message, key) {
    this.db.transaction(() => {
      const saved = this.save(message,null,false);
      this.db.prepare(`UPDATE messages SET media_key=?,media_state='ready'
        WHERE thread_type=? AND thread_id=? AND msg_id=?`).run(key,saved.thread_type,saved.thread_id,saved.msg_id);
    })();
  }

  recipients() {
    return this.db.prepare("SELECT thread_type,thread_id,name FROM recipients ORDER BY name,thread_id").all();
  }

  saveRecipients(recipients) {
    const put = this.db.prepare("INSERT INTO recipients VALUES(?,?,?) ON CONFLICT(thread_type,thread_id) DO UPDATE SET name=excluded.name");
    this.db.transaction(() => { for (const r of recipients) put.run(r.thread_type,r.thread_id,r.name); })();
  }

  history(thread, type, cursor = "") {
    let before = null;
    if (cursor) {
      try { before = JSON.parse(Buffer.from(cursor, "base64url").toString()); } catch { throw new Error("Con trỏ kho local không hợp lệ"); }
      if (!Array.isArray(before) || before.length !== 2 || !Number.isSafeInteger(before[0]) || typeof before[1] !== "string") throw new Error("Con trỏ kho local không hợp lệ");
    }
    const rows = before ? this.db.prepare(`SELECT * FROM messages WHERE thread_type=? AND thread_id=?
      AND (timestamp,msg_id)<(?,?) ORDER BY timestamp DESC,msg_id DESC LIMIT 101`)
      .all(type,thread,before[0],before[1]) : this.db.prepare(
        "SELECT * FROM messages WHERE thread_type=? AND thread_id=? ORDER BY timestamp DESC,msg_id DESC LIMIT 101").all(type,thread);
    const selected = rows.slice(0,100);
    const oldest = selected.at(-1);
    const messages = selected.reverse().map(row => {
      const message = JSON.parse(row.data);
      return this.withMedia(message);
    });
    const cloudMore = type === "group" && !!this.meta("group-cloud-more:"+thread);
    const cloudPartial = type === "group" ? this.meta("group-cloud-partial:"+thread) : null;
    return {messages,has_more:rows.length>100 || cloudMore,archive_cursor:oldest ? Buffer.from(JSON.stringify([oldest.timestamp,oldest.msg_id])).toString("base64url") : cloudMore ? cursor || Buffer.from(JSON.stringify([Number.MAX_SAFE_INTEGER,"\uffff"])).toString("base64url") : "",
      cursor:oldest && /^\d+$/.test(oldest.msg_id) ? oldest.msg_id : "", partial:cloudPartial ?? !this.meta("sync-done:"+type),
      notice:(type === "group" && this.meta("group-cloud-notice:"+thread)) || (this.meta("sync-running:"+type) ? "Đọc kho local; đang đồng bộ lịch sử nền, tin sẽ cập nhật khi có dữ liệu." : "Đang đọc tin đã lưu trên máy.")};
  }

  meta(key, value) {
    if (value !== undefined) {
      this.db.prepare("INSERT INTO metadata VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value").run(key,JSON.stringify(value));
      return value;
    }
    const row = this.db.prepare("SELECT value FROM metadata WHERE key=?").get(key);
    return row ? JSON.parse(row.value) : null;
  }

  wake() {
    if (!this.enabled || this.closed || this.running) return this.running;
    this.running = this.pump().catch(error => console.error("❌ Không lưu được ảnh vào kho:",error.message))
      .finally(() => { this.running = null; });
    return this.running;
  }

  async pump() {
    while (!this.closed) {
      const job = this.db.prepare(`SELECT * FROM messages WHERE media_state IN ('pending','error') AND retry_at<=?
        ORDER BY CASE media_state WHEN 'pending' THEN 0 ELSE 1 END,retry_at,timestamp DESC LIMIT 1`).get(Date.now());
      if (!job) return;
      this.db.prepare("UPDATE messages SET media_state='downloading',attempts=attempts+1 WHERE thread_type=? AND thread_id=? AND msg_id=?")
        .run(job.thread_type,job.thread_id,job.msg_id);
      try {
        const message = JSON.parse(job.data);
        let name;
        if (message.kind === "sticker") {
          const saved = this.db.prepare("SELECT media_key FROM messages WHERE media_url=? AND media_state='ready' LIMIT 1").get(job.media_url);
          if (saved?.media_key && fs.lstatSync(path.join(this.mediaDirectory,saved.media_key),{throwIfNoEntry:false})?.isFile()) {
            name = saved.media_key;
          } else {
            const address = await this.resolveSticker?.(message.sticker_id);
            if (!address) throw new Error("Zalo chưa trả ảnh sticker");
            name = await this.download(address,this.mediaDirectory,{signal:this.abort.signal,kind:"image"});
          }
        } else {
          name = await this.download(job.media_url,this.mediaDirectory,{signal:this.abort.signal,kind:message.kind});
        }
        if (!/^[a-f0-9]{64}\.(jpg|png|gif|webp|bmp|mp4|webm)$/.test(name)) throw new Error("Tên file media archive không hợp lệ");
        this.db.prepare(`UPDATE messages SET media_key=?,media_state='ready',media_error='' WHERE thread_type=? AND thread_id=? AND msg_id=? AND media_url=?`)
          .run(name,job.thread_type,job.thread_id,job.msg_id,job.media_url);
        this.onMedia({event:"media_updated",data:this.withMedia(JSON.parse(job.data))});
      } catch (error) {
        const cancelled = this.abort.signal.aborted;
        this.db.prepare(`UPDATE messages SET media_state=?,attempts=CASE WHEN ? THEN MAX(0,attempts-1) ELSE attempts END,retry_at=?,media_error=?
          WHERE thread_type=? AND thread_id=? AND msg_id=? AND media_url=?`)
          .run(cancelled ? "pending" : "error",cancelled ? 1 : 0, cancelled ? 0 : Date.now()+Math.min(1800000,30000*2**Math.min(job.attempts,10)),
            error.message,job.thread_type,job.thread_id,job.msg_id,job.media_url);
        if (!cancelled) {
          if (job.attempts < 3 || job.attempts % 6 === 0) console.warn("⚠️ Media chưa lưu được, sẽ thử lại:",error.message);
          this.onMedia({event:"media_updated",data:this.withMedia(JSON.parse(job.data))});
        }
      }
    }
  }

  async close() {
    this.closed = true;
    clearInterval(this.timer);
    this.abort.abort();
    await this.running;
    this.db.close();
  }
}
