import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { pathToFileURL, fileURLToPath } from "node:url";
import { spawn } from "node:child_process";
import { setTimeout as pause } from "node:timers/promises";
import WebSocket from "ws";
import { Archive } from "./archive.mjs";

test("đọc local trước login, tự cập nhật tin vắng mặt và đồng bộ cả user/group", async t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(),"zalo-startup-"));
  const dataHome = path.join(root,"data");
  const archive = new Archive("1001",{root:path.join(dataHome,"zalo-cli","accounts"),worker:false});
  archive.save({msg_id:"1",thread_id:"friend",thread_type:"user",sender_name:"An",timestamp:1,kind:"text",content:"Tin đã lưu"});
  await archive.close();
  const directory = path.dirname(fileURLToPath(import.meta.url));
  const fixture = path.join(root,"fake.mjs");
  const gate = path.join(root,"login-ready");
  const calls = path.join(root,"calls");
  fs.writeFileSync(fixture,`
    import fs from 'node:fs';
    import {EventEmitter} from 'node:events';
    import {setTimeout as pause} from 'node:timers/promises';
    const listener = new EventEmitter();
    listener.start = () => queueMicrotask(() => {listener.emit('connected');listener.emit('cipher_key','fixture');});
    listener.stop = () => listener.emit('disconnected');
    listener.requestOldMessages = (type,cursor) => {
      fs.appendFileSync(${JSON.stringify(calls)},type+':'+cursor+'\\n');
      const message={type,threadId:'friend',data:{msgId:type===0?'500':'600',ts:'500',uidFrom:'friend',dName:'An',content:'Tin gửi khi tắt máy '+type}};
      queueMicrotask(() => listener.emit('old_messages',cursor===null?[message]:[],type));
    };
    export async function loginWithSession(){
      while(!fs.existsSync(${JSON.stringify(gate)})) await pause(20);
      return {getOwnId:()=> '1001',listener};
    }
    export async function loadRecipients(){ return {recipients:[{thread_id:'friend',thread_type:'user',name:'An mới'}]}; }
  `);
  let source = fs.readFileSync(path.join(directory,"index.mjs"),"utf8");
  source = source.replace(/from "\.\/([^"\n]+)"/g,(_,name) => `from ${JSON.stringify(pathToFileURL(path.join(directory,name)).href)}`);
  for (const name of ["auth.mjs","recipients.mjs"]) source = source.replace(pathToFileURL(path.join(directory,name)).href,pathToFileURL(fixture).href);
  source = source.replace('from "zca-js"',`from ${JSON.stringify(pathToFileURL(path.join(directory,"node_modules/zca-js/dist/index.js")).href)}`)
    .replace('from "ws"',`from ${JSON.stringify(pathToFileURL(path.join(directory,"node_modules/ws/wrapper.mjs")).href)}`)
    .replace("const PORT = 8989;","const PORT = 0;")
    .replace('console.log(`🚀', 'wss.on("listening",()=>console.log("TEST_PORT="+wss.address().port));\nconsole.log(`🚀');
  const entry = path.join(root,"index.mjs");
  fs.writeFileSync(entry,source);
  const child = spawn(process.execPath,[entry,"--managed"],{env:{...process.env,XDG_DATA_HOME:dataHome},stdio:["ignore","pipe","pipe"]});
  let output = "";
  child.stdout.on("data",chunk=>{output+=chunk;});
  child.stderr.on("data",chunk=>{output+=chunk;});
  t.after(async()=>{
    child.kill("SIGTERM");
    await Promise.race([new Promise(resolve=>child.once("exit",resolve)),pause(1500)]);
    if (child.exitCode === null) child.kill("SIGKILL");
    fs.rmSync(root,{recursive:true,force:true});
  });
  async function until(predicate){
    const end=Date.now()+12000;
    while(Date.now()<end){if(predicate())return;await pause(20);}
    assert.fail("fixture timeout: "+output);
  }
  await until(()=>/TEST_PORT=(\d+)/.test(output));
  const socket = new WebSocket("ws://127.0.0.1:"+output.match(/TEST_PORT=(\d+)/)[1]);
  await new Promise((resolve,reject)=>{socket.once("open",resolve);socket.once("error",reject);});
  t.after(()=>socket.terminate());
  const events=[];
  socket.on("message",raw=>events.push(JSON.parse(raw)));
  let sequence=0;
  async function request(action,payload){
    const id=String(++sequence);
    socket.send(JSON.stringify({id,action,payload}));
    await until(()=>events.some(event=>event.id===id));
    return events.find(event=>event.id===id);
  }
  const status=await request("daemon_status");
  assert.equal(status.ready,true);
  assert.equal(status.online,false);
  assert.equal((await request("list_recipients")).recipients.length,1);
  assert.equal((await request("get_history",{thread_id:"friend",thread_type:"user"})).messages[0].content,"Tin đã lưu");
  assert.equal((await request("send_message",{thread_id:"friend",thread_type:"user",text:"Không gửi"})).ok,false);
  fs.writeFileSync(gate,"");
  await until(()=>events.some(event=>event.event==="archive_status" && event.data.notice.startsWith("Đã cập nhật phần lịch sử")));
  const personal=await request("get_history",{thread_id:"friend",thread_type:"user"});
  assert.equal(personal.messages.length,2);
  assert.equal(personal.messages[1].content,"Tin gửi khi tắt máy 0");
  assert.equal((await request("get_history",{thread_id:"friend",thread_type:"group"})).messages.length,1);
  assert.equal((await request("daemon_status")).online,true);
  assert.ok(events.some(event=>event.event==="recipients_updated"));
  assert.deepEqual(fs.readFileSync(calls,"utf8").trim().split("\n").slice(0,2),["0:null","1:null"]);
});
