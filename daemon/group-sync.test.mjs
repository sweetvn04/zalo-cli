import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { Archive } from "./archive.mjs";
import { createCloudReader } from "./group-cloud.mjs";
import { createGroupSync } from "./group-sync.mjs";

function raw(id,group="123") {
  return {type:1,threadId:group,data:{msgId:String(id),idTo:group,ts:String(id),uidFrom:"42",content:"Tin "+id}};
}
function fixture(t) {
  const root=fs.mkdtempSync(path.join(os.tmpdir(),"zalo-group-cloud-"));
  const archive=new Archive("1001",{root,worker:false});
  t.after(async()=>{await archive.close();fs.rmSync(root,{recursive:true,force:true});});
  return archive;
}

test("cloud dùng recent/old, nretry và con trỏ chuỗi; không nhận sai nhóm",async()=>{
  const calls=[];
  let body={groupMsgs:[raw(200).data],hasMore:1,lastMsgId:"9007199254740993123",isFiltered:0};
  const reader=createCloudReader({zpwServiceMap:{group_cloud_message:["https://example.org"]}},{uid:"1001",imei:"device"},{
    encodeAES:value=>value,
    makeURL:(base,params)=>{const url=new URL(base);for(const [key,value] of Object.entries(params))url.searchParams.set(key,value);return url;},
    request:async(url,options)=>{calls.push({url,options});return body;},
    resolve:async(response,callback)=>callback({data:JSON.stringify(response)}),
  });
  const first=await reader("123");
  assert.equal(first.messages[0].threadId,"123");
  assert.equal(first.cursor,"9007199254740993123");
  assert.equal(first.more,true);
  await reader("123",first.cursor);
  assert.equal(calls[0].url.pathname,"/api/cm/getrecentv2");
  assert.equal(calls[1].url.pathname,"/api/cm/getoldv2");
  assert.equal(calls[0].url.searchParams.get("nretry"),"0");
  assert.deepEqual(JSON.parse(calls[1].url.searchParams.get("params")),{groupId:"123",globalMsgId:first.cursor,count:50,msgIds:[],imei:"device",src:-1});
  body={groupMsgs:[raw(200,"999").data],hasMore:0};
  await assert.rejects(reader("123"),/sai nhóm/);
  body={groupMsgs:[],lastMsgId:9007199254740993000};
  await assert.rejects(reader("123"),/độ chính xác/);
});

test("mã -69 là giới hạn server, không hủy các tin mới đã lưu",async t=>{
  const reader=createCloudReader({zpwServiceMap:{group_cloud_message:["https://example.org"]}},{uid:"1001"},{
    encodeAES:value=>value,makeURL:value=>value,request:async()=>({}),
    resolve:async()=>{throw Object.assign(new Error("Stop retry"),{code:-69});},
  });
  const page=await reader("123","100");
  assert.equal(page.limited,true);
  const archive=fixture(t);let count=0;
  await createGroupSync({},archive,{delay:0,fetchPage:async()=>++count===1 ? {messages:[raw(200)],cursor:"150",more:true} : page}).refresh("123");
  assert.equal(archive.history("123","group").messages.length,1);
  assert.equal(archive.history("123","group").has_more,false);
  assert.equal(archive.history("123","group").partial,true);
  assert.match(archive.history("123","group").notice,/chưa cung cấp thêm/);
});

test("cloud bổ sung tin mới hơn snapshot local, single-flight và dedup",async t=>{
  const archive=fixture(t);
  archive.capture(raw(100));
  const calls=[];let updates=0;
  const sync=createGroupSync({},archive,{delay:0,onUpdate:()=>updates++,fetchPage:async(_api,id,cursor)=>{
    calls.push(cursor);await new Promise(resolve=>setTimeout(resolve,5));
    return cursor ? {messages:[raw(100)],cursor:"100",more:false} : {messages:[raw(200)],cursor:"150",more:true};
  }});
  await Promise.all([sync.refresh("123"),sync.refresh("123")]);
  assert.deepEqual(calls,["","150"]);
  assert.deepEqual(archive.history("123","group").messages.map(msg=>msg.msg_id),["100","200"]);
  assert.equal(updates,2);
  await sync.refresh("123");
  assert.equal(calls.length,2);
});

test("F7 tiếp tục cloud cursor qua restart và giữ cursor sâu khi đọc lại head",async t=>{
  const archive=fixture(t);
  const calls=[];
  const options={delay:0,fetchPage:async(_api,id,cursor)=>{
    calls.push(cursor);
    const next=cursor ? Number(cursor)-10 : 1000;
    return {messages:[raw(next)],cursor:String(next),more:true};
  }};
  await createGroupSync({},archive,options).refresh("123");
  assert.equal(calls.length,5);
  const oldest=archive.meta("group-cloud-cursor:123");
  assert.equal(archive.history("123","group").has_more,true);
  await createGroupSync({},archive,options).refresh("123",true);
  assert.equal(calls[5],oldest);
  const deeper=archive.meta("group-cloud-cursor:123");
  await createGroupSync({},archive,options).refresh("123");
  assert.equal(archive.meta("group-cloud-cursor:123"),deeper);
});

test("cloud lỗi không xóa local và 429 chặn cả hàng đợi",async t=>{
  const archive=fixture(t);archive.capture(raw(100));
  let count=0;const statuses=[];
  const sync=createGroupSync({},archive,{onStatus:value=>statuses.push(value),fetchPage:async()=>{
    count++;throw new Error("Request failed with status code 429");
  }});
  await Promise.all([sync.refresh("123"),sync.refresh("456")]);
  assert.equal(count,1);
  assert.equal(archive.history("123","group").messages.length,1);
  assert.match(statuses[0],/429/);
});
