import { setTimeout as pause } from "node:timers/promises";
import { groupCloudPage } from "./group-cloud.mjs";

// Independent of the old_messages replay, which can stop at a stale snapshot.
export function createGroupSync(api,archive,{signal,onUpdate = () => {},onStatus = () => {},fetchPage = groupCloudPage,delay = 1000} = {}) {
  const states = new Map();
  let queue = Promise.resolve();
  let blockedUntil = 0;
  function refresh(groupId,older = false) {
    const state = states.get(groupId) || {next:0,pending:null};
    states.set(groupId,state);
    if (state.pending) return state.pending;
    if (signal?.aborted || Date.now()<blockedUntil || (!older && Date.now()<state.next)) return Promise.resolve();
    state.pending = queue.then(async () => {
      if (signal?.aborted || Date.now()<blockedUntil) return;
      let cursor = older ? archive.meta("group-cloud-cursor:"+groupId) || "" : "";
      if (older && !cursor) return;
      state.next = Date.now()+60000;
      let count = 0;
      let more = false;
      let filtered = false;
      let limited = false;
      try {
        for (let page=0;page<5;page++) {
          if (signal?.aborted) return;
          const response = await fetchPage(api,groupId,cursor,signal);
          if (signal?.aborted) return;
          archive.captureBatch(response.messages);
          count += response.messages.length;
          filtered ||= response.filtered;
          limited ||= response.limited;
          more = !!response.more && !!response.cursor && response.cursor !== cursor;
          if (response.more && !more) filtered = true;
          cursor = response.cursor || "";
          const previous = archive.meta("group-cloud-cursor:"+groupId);
          const nextCursor = more && previous && /^\d+$/.test(previous) && BigInt(previous)<BigInt(cursor) ? previous : cursor;
          archive.meta("group-cloud-cursor:"+groupId,more ? nextCursor : "");
          archive.meta("group-cloud-more:"+groupId,more);
          archive.meta("group-cloud-partial:"+groupId,more || filtered || limited);
          archive.meta("group-cloud-notice:"+groupId,limited ? "Đã cập nhật tin nhóm mới; Zalo chưa cung cấp thêm các tin cũ hơn." : filtered ? "Zalo đang lọc một phần lịch sử nhóm; đã lưu các tin nhận được." : "Đã cập nhật lịch sử nhóm từ cloud.");
          onUpdate(groupId);
          if (!more) break;
          await pause(delay,undefined,{signal});
        }
        onStatus(`Đã lấy ${count} tin của nhóm từ cloud.`+(more ? " F7 để lấy thêm tin cũ." : "")+(filtered ? " Zalo có lọc một phần lịch sử." : "")+(limited ? " Zalo chưa cung cấp thêm tin cũ hơn." : ""));
      } catch (error) {
        if (signal?.aborted) return;
        if (error.message.includes("429")) blockedUntil = Date.now()+60000;
        archive.meta("group-cloud-notice:"+groupId,"Chưa cập nhật được tin mới của nhóm: "+error.message);
        onStatus("Chưa cập nhật được tin mới của nhóm: "+error.message);
      }
    }).finally(()=>{state.pending=null;});
    queue = state.pending.catch(()=>{});
    return state.pending;
  }
  return {refresh,done:()=>queue};
}
