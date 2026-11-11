import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import https from "node:https";
import { Readable } from "node:stream";
import path from "node:path";
import { createHash } from "node:crypto";
import { EventEmitter } from "node:events";
import { Archive } from "./archive.mjs";
import { handleCommand } from "./ipc.mjs";
import { syncArchive } from "./archive-sync.mjs";
import { validateImageURL, storeImage, videoExtension } from "./image-store.mjs";
import { normalizeMessage } from "./history.mjs";

test("sticker trong kho cũ được nâng cấp và các tin cùng sticker dùng chung ảnh", async t => {
  const root = temporary(t);
  const rawSticker = id => raw(id,JSON.stringify({sticker_id:123,cate_id:1,type:1}),{msgType:"chat.sticker"});
  let archive = new Archive("1001",{root,worker:false});
  archive.save({msg_id:"1",thread_id:"friend",thread_type:"user",kind:"unsupported",timestamp:1,content:"[chat.sticker]"},rawSticker(1));
  archive.db.prepare("DELETE FROM metadata WHERE key=?").run("sticker-schema-v1");
  await archive.close();
  let resolutions = 0;
  const key = "a".repeat(64)+".png";
  archive = new Archive("1001",{root,worker:false,resolveSticker:async id => {
    assert.equal(id,123);
    resolutions++;
    return "https://example.org/sticker.png";
  },download:async(address,directory,{kind}) => {
    assert.equal(address,"https://example.org/sticker.png");
    assert.equal(kind,"image");
    fs.writeFileSync(path.join(directory,key),Buffer.from("fixture"));
    return key;
  }});
  t.after(() => archive.close());
  archive.capture(rawSticker(2));
  await archive.pump();
  const messages = archive.history("friend","user").messages;
  assert.equal(resolutions,1);
  assert.equal(messages.length,2);
  for (const message of messages) {
    assert.equal(message.kind,"sticker");
    assert.equal(message.content,"");
    assert.equal(message.image_local,`archive:1001/${key}`);
  }
  const catalog = await handleCommand(JSON.stringify({id:"catalog",action:"list_stickers"}),null,archive);
  assert.equal(catalog.messages.length,1);
  assert.equal(catalog.messages[0].sticker_id,123);
  const sticker = {id:123,cateId:1,type:1};
  const result = await handleCommand(JSON.stringify({id:"send",action:"send_sticker",
    payload:{thread_id:"target",thread_type:"group",sticker_id:123}}),{
    getStickersDetail:async id => { assert.equal(id,123); return [sticker]; },
    sendSticker:async (payload,id,type) => {
      assert.deepEqual([payload,id,type],[sticker,"target",1]);
      return {msgId:"900"};
    },
  },archive);
  assert.equal(result.ok,true);
  assert.equal(result.msg_id,"900");
  assert.equal(archive.history("target","group").messages[0].sticker_id,123);
});

test("gửi lại ảnh local vào nhóm và giữ bản ảnh trong lịch sử đích", async t => {
  const archive = new Archive("1001", {root:temporary(t),worker:false});
  t.after(() => archive.close());
  const png = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/l9sAAAAASUVORK5CYII=","base64");
  const key = createHash("sha256").update(png).digest("hex")+".png";
  fs.writeFileSync(path.join(archive.mediaDirectory,key),png);
  let sends = 0;
  const api = {async sendMessage(payload,id,type) {
    sends++;
    assert.equal(id,"456");
    assert.equal(type,1);
    assert.deepEqual(payload.attachments,[{filename:key,data:png,metadata:{width:1,height:1,totalSize:png.length}}]);
    return {message:null,attachment:[{msgId:"789"}]};
  }};
  const send = image_local => handleCommand(JSON.stringify({id:"forward",action:"send_image",
    payload:{thread_id:"456",thread_type:"group",image_local}}),api,archive);
  for (const invalid of ["/etc/passwd",`archive:2002/${key}`,`archive:1001/${"a".repeat(64)}.png`]) {
    assert.equal((await send(invalid)).ok,false);
  }
  assert.equal(sends,0);
  const reference = `archive:1001/${key}`;
  const result = await send(reference);
  assert.equal(result.ok,true);
  assert.equal(result.msg_id,"789");
  assert.equal(sends,1);
  assert.equal(archive.history("456","group").messages[0].image_local,reference);
});

test("video được stream ra file theo nội dung, không lưu trang lỗi HTTP thành video", async t => {
  const root = temporary(t);
  let body = Buffer.concat([Buffer.from([0,0,0,24]),Buffer.from("ftypisom"),Buffer.alloc(20)]);
  t.mock.method(https,"get",(_url,_options,callback) => {
    const request = new EventEmitter();
    const response = Readable.from([body.subarray(0,5),body.subarray(5)]);
    response.statusCode = 200;
    response.headers = {};
    process.nextTick(() => callback(response));
    return request;
  });
  const name = await storeImage("https://example.org/movie",root,{kind:"video"});
  assert.equal(name,createHash("sha256").update(body).digest("hex")+".mp4");
  assert.deepEqual(fs.readFileSync(path.join(root,name)),body);
  assert.equal(fs.statSync(path.join(root,name)).mode&0o777,0o600);
  body = Buffer.from("<html>Expired video</html>");
  await assert.rejects(storeImage("https://example.org/expired",root,{kind:"video"}),/không phải video/);
  assert.deepEqual(fs.readdirSync(root),[name]);
});

test("video nhận đúng URL, không lấy thumbnail và nhận diện container", () => {
  for (const content of [
    {href:"https://example.org/a.mp4",thumb:"https://example.org/thumb.jpg"},
    JSON.stringify({params:JSON.stringify({videoUrl:"https://example.org/a.mp4"})}),
  ]) {
    const video = normalizeMessage(raw(10,content,{msgType:"chat.video.msg"}));
    assert.equal(video.kind,"video");
    assert.equal(video.video_url,"https://example.org/a.mp4");
    assert.equal(video.image_url,"");
  }
  assert.equal(normalizeMessage(raw(10,{thumb:"https://example.org/thumb.jpg"},{msgType:"chat.video.msg"})).video_url,"");
  assert.equal(videoExtension(Buffer.from("0000ftypisom")),".mp4");
  assert.throws(() => videoExtension(Buffer.from("<html>error</html>")));
  assert.throws(() => videoExtension(Buffer.from("0000ftypavif")));
});

test("video tải nền, giữ local sau restart và nâng cấp payload cũ", async t => {
  const root = temporary(t);
  let calls = 0;
  const name = "a".repeat(64)+".mp4";
  let archive = new Archive("1001",{root,worker:false,download:async (url,dir,options) => {
    calls++;
    assert.equal(options.kind,"video");
    fs.writeFileSync(path.join(dir,name),Buffer.from("0000ftypisom"));
    return name;
  }});
  const message = raw(80,{href:"https://example.org/movie.mp4"},{msgType:"chat.video.msg"});
  archive.capture(message);
  await archive.pump();
  assert.equal(calls,1);
  await archive.close();
  archive = new Archive("1001",{root,worker:false});
  let video = archive.history("friend","user").messages[0];
  assert.equal(video.image_local,"archive:1001/"+name);
  assert.equal(video.image_state,"ready");
  archive.db.prepare("DELETE FROM metadata WHERE key='video-schema-v1'").run();
  archive.db.prepare("UPDATE messages SET data=?").run(JSON.stringify({...video,kind:"unsupported"}));
  await archive.close();
  archive = new Archive("1001",{root,worker:false});
  video = archive.history("friend","user").messages[0];
  assert.equal(video.kind,"video");
  assert.equal(video.image_local,"archive:1001/"+name);
  await archive.close();
});

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

test("lỗi lịch sử cá nhân không chặn nhóm và không báo đồng bộ đầy đủ", async t => {
  const archive = new Archive("1001",{root:temporary(t),worker:false});
  t.after(()=>archive.close());
  const listener = new EventEmitter();
  listener.requestOldMessages = (type,cursor) => {
    if(type===0) throw new Error("fixture: lịch sử cá nhân chưa có");
    const message=raw(300,"Tin nhóm lúc vắng mặt");message.type=type;
    queueMicrotask(()=>listener.emit("old_messages",cursor===null?[message]:[],type));
  };
  const statuses=[];
  await syncArchive({listener},archive,new AbortController().signal,notice=>statuses.push(notice));
  assert.equal(archive.history("friend","group").messages[0].content,"Tin nhóm lúc vắng mặt");
  assert.equal(archive.meta("sync-done:user"),false);
  assert.equal(archive.meta("sync-running:user"),false);
  assert.match(archive.meta("sync-error:user"),/fixture/);
  assert.match(statuses.at(-1),/chưa cung cấp đủ/);
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
