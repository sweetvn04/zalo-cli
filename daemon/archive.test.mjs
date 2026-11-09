import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createHash } from "node:crypto";
import { EventEmitter } from "node:events";
import { Archive } from "./archive.mjs";
import { handleCommand } from "./ipc.mjs";
import { syncArchive } from "./archive-sync.mjs";
import { validateImageURL, storeImage } from "./image-store.mjs";

function temporary(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(),"zalo-archive-"));
  t.after(() => fs.rmSync(root,{recursive:true,force:true}));
  return root;
}
function raw(id, content, extra = {}) {
  return {type:0,threadId:"friend",isSelf:false,data:{msgId:String(id),ts:String(id),uidFrom:"friend",dName:"An",content,...extra}};
}

test("SQLite giữ tin sau restart, raw đính kèm và tách tài khoản", async t => {
  const root = temporary(t);
  let archive = new Archive("1001",{root,worker:false});
  archive.capture(raw(1,"Tin còn mãi"));
  archive.capture(raw(1,"Tin còn mãi"));
  archive.capture(raw(2,{title:"Tài liệu",href:"https://example.org/file.pdf"},{msgType:"chat.file"}));
  archive.saveRecipients([{thread_type:"user",thread_id:"friend",name:"An"}]);
  await archive.close();
  archive = new Archive("1001",{root,worker:false});
  const result = await handleCommand(JSON.stringify({id:"offline",action:"get_history",payload:{thread_id:"friend",thread_type:"user"}}),undefined,archive);
  assert.equal(result.ok,true);
  assert.equal(result.messages.length,2);
  assert.equal(result.messages[0].content,"Tin còn mãi");
  assert.match(result.messages[1].content,/chat.file/);
  assert.match(archive.db.prepare("SELECT raw FROM messages WHERE msg_id='2'").get().raw,/file.pdf/);
  const contacts = await handleCommand(JSON.stringify({id:"list",action:"list_recipients"}),undefined,archive);
  assert.equal(contacts.recipients[0].name,"An");
  const other = new Archive("2002",{root,worker:false});
  assert.equal(other.history("friend","user").messages.length,0);
  assert.equal(fs.statSync(path.join(root,"1001","messages.sqlite3")).mode&0o777,0o600);
  await other.close();
  await archive.close();
});

test("cursor local không mất tin cùng timestamp hoặc ID không phải số", async t => {
  const archive = new Archive("1001",{root:temporary(t),worker:false});
  for (let i=0;i<250;i++) archive.save({msg_id:"local-"+i,thread_id:"friend",thread_type:"user",timestamp:1000,content:"Tin "+i,kind:"text"});
  const ids = [];
  let cursor = "";
  for (;;) {
    const page = archive.history("friend","user",cursor);
    ids.push(...page.messages.map(m=>m.msg_id));
    if (!page.has_more) break;
    cursor = page.archive_cursor;
  }
  assert.equal(ids.length,250);
  assert.equal(new Set(ids).size,250);
  await archive.close();
});

test("ảnh được tải nền một lần, giữ file sau restart và URL hết hạn", async t => {
  const root = temporary(t);
  const bytes = Buffer.from([137,80,78,71,13,10,26,10,1,2,3]);
  const name = createHash("sha256").update(bytes).digest("hex")+".png";
  let calls = 0;
  const download = async (url, directory) => {
    calls++;
    fs.writeFileSync(path.join(directory,name),bytes,{mode:0o600});
    return name;
  };
  let archive = new Archive("1001",{root,worker:false,download});
  const photo = raw(3,{href:"https://example.org/old.jpg"},{msgType:"chat.photo"});
  archive.capture(photo);
  await archive.pump();
  assert.equal(calls,1);
  const key = archive.history("friend","user").messages[0].image_local;
  assert.equal(key,`archive:1001/${name}`);
  await archive.close();
  archive = new Archive("1001",{root,worker:false,download:async()=>assert.fail("must use archived file")});
  archive.capture(photo);
  await archive.pump();
  assert.equal(archive.history("friend","user").messages[0].image_local,key);
  assert.deepEqual(fs.readFileSync(path.join(archive.mediaDirectory,name)),bytes);
  await archive.close();
});

test("job ảnh lỗi được giữ trên đĩa và tiếp tục sau restart", async t => {
  const root = temporary(t);
  let archive = new Archive("1001",{root,worker:false,download:async()=>{throw new Error("Mất mạng");}});
  archive.capture(raw(4,{href:"https://example.org/a.jpg"},{msgType:"chat.photo"}));
  await archive.pump();
  assert.equal(archive.db.prepare("SELECT media_state FROM messages").get().media_state,"error");
  await archive.close();
  const name = "a".repeat(64)+".jpg";
  archive = new Archive("1001",{root,worker:false,download:async(_,directory)=>{
    fs.writeFileSync(path.join(directory,name),Buffer.from([255,216,255])); return name;
  }});
  archive.db.prepare("UPDATE messages SET retry_at=0,attempts=6").run();
  await archive.pump();
  assert.equal(archive.history("friend","user").messages[0].image_state,"ready");
  await archive.close();
});

test("đồng bộ nền lưu lịch sử cả user/group khi không có giao diện", async t => {
  const archive = new Archive("1001",{root:temporary(t),worker:false});
  const listener = new EventEmitter();
  listener.requestOldMessages = (type,cursor) => {
    const message = raw(type===0?100:200,"Tin cũ "+type);
    message.type = type;
    queueMicrotask(()=>listener.emit("old_messages",cursor===null?[message]:[],type));
  };
  await syncArchive({listener},archive,new AbortController().signal);
  assert.equal(archive.history("friend","user").messages.length,1);
  assert.equal(archive.history("friend","group").messages.length,1);
  assert.equal(archive.meta("sync-done:user"),true);
  assert.equal(archive.meta("sync-done:group"),true);
  await archive.close();
});

test("tận dụng cache ảnh cũ mà không truy cập URL đã hết hạn", async t => {
  const root = temporary(t);
  const oldCache = process.env.XDG_CACHE_HOME;
  process.env.XDG_CACHE_HOME=root;
  t.after(()=>{ if(oldCache===undefined) delete process.env.XDG_CACHE_HOME; else process.env.XDG_CACHE_HOME=oldCache; });
  const url="https://no-longer-exists.invalid/a.jpg";
  const dir=path.join(root,"zalo-cli","images");
  fs.mkdirSync(dir,{recursive:true});
  const bytes=Buffer.from([255,216,255,1,2,3]);
  fs.writeFileSync(path.join(dir,createHash("sha256").update(url).digest("hex")+".jpg"),bytes);
  const target=path.join(root,"archive-images");
  fs.mkdirSync(target);
  const name=await storeImage(url,target);
  assert.deepEqual(fs.readFileSync(path.join(target,name)),bytes);
  assert.equal(fs.statSync(path.join(target,name)).mode&0o777,0o600);
});

test("tải ảnh chặn mạng nội bộ và URL không phải HTTP", () => {
  for (const url of ["file:///etc/passwd","http://127.0.0.1/a","http://192.168.1.2/a","http://[::1]/a","http://[::ffff:127.0.0.1]/a"]) {
    assert.throws(()=>validateImageURL(url));
  }
  assert.equal(validateImageURL("https://93.184.216.34/a").hostname,"93.184.216.34");
});

test("gửi tin được lưu local và lỗi lưu không báo nhầm là gửi thất bại", async t => {
  const archive = new Archive("1001",{root:temporary(t),worker:false});
  const api = {async sendMessage() { return {message:{msgId:500}}; }};
  const command=JSON.stringify({id:"send",action:"send_message",payload:{thread_id:"friend",thread_type:"user",text:"Tin gửi lưu local"}});
  const response=await handleCommand(command,api,archive);
  assert.equal(response.ok,true);
  assert.equal(archive.history("friend","user").messages[0].content,"Tin gửi lưu local");
  const diskError=await handleCommand(command,api,{save(){throw new Error("Ổ đĩa đầy");}});
  assert.equal(diskError.ok,true);
  assert.match(diskError.notice,/Ổ đĩa đầy/);
  await archive.close();
});

test("ảnh được tự tải khi capture, không cần mở chat hoặc gọi pump", async t => {
  const name="b".repeat(64)+".jpg";
  let notify;
  const completed = new Promise(resolve=>{notify=resolve;});
  const archive = new Archive("1001",{root:temporary(t),download:async(_,directory)=>{
    fs.writeFileSync(path.join(directory,name),Buffer.from([255,216,255])); return name;
  },onMedia:notify});
  archive.capture(raw(6,{href:"https://example.org/live.jpg"},{msgType:"chat.photo"}));
  let timer;
  try {
    const result=await Promise.race([completed,new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error("Ảnh không tự tải")),3000);})]);
    assert.equal(result.event,"media_updated");
    assert.equal(result.data.image_state,"ready");
  } finally {
    clearTimeout(timer);
    await archive.close();
  }
});
