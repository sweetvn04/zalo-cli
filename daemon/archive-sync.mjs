import { setTimeout as pause } from "node:timers/promises";
import { archiveHistoryPage } from "./history.mjs";

// Luân phiên từng trang: lấy tin gần nhất của cả cá nhân và nhóm trước
// khi đào sâu lịch sử, để một loại hội thoại không chặn loại còn lại.
export async function syncArchive(api, archive, signal, onStatus = () => {}) {
  const states = ["user","group"].map(type => ({type,cursor:"",done:false,error:false}));
  let count = 0;
  for (const {type} of states) {
    archive.meta("sync-done:"+type,false);
    archive.meta("sync-running:"+type,true);
    archive.meta("sync-error:"+type,null);
    archive.meta("sync-started:"+type,Date.now());
  }
  onStatus("Đang lấy tin gần đây từ Zalo; tin local vẫn đọc được.");
  try {
    while (states.some(state => !state.done)) {
      for (const state of states) {
        if (signal.aborted) return;
        if (state.done) continue;
        const {type} = state;
        try {
          const page = await archiveHistoryPage(api,type,state.cursor);
          if (signal.aborted) return;
          archive.captureBatch(page.messages);
          count += page.messages.length;
          if (!page.more || !page.cursor || page.cursor === state.cursor) {
            state.done = true;
            archive.meta("sync-done:"+type,!page.partial);
            archive.meta("sync-finished:"+type,Date.now());
            archive.meta("sync-running:"+type,false);
          }
          state.cursor = page.cursor;
          onStatus(`Đã đọc ${count} tin từ Zalo trong lượt đồng bộ; đang cập nhật kho local.`);
        } catch (error) {
          if (signal.aborted) return;
          state.done = true;
          state.error = true;
          archive.meta("sync-error:"+type,error.message);
          archive.meta("sync-running:"+type,false);
          console.warn(`⚠️ Đồng bộ lịch sử ${type} tạm dừng:`,error.message);
        }
        if (states.some(state => !state.done)) await pause(2000,undefined,{signal});
      }
    }
    const incomplete = states.some(state => state.error || !archive.meta("sync-done:"+state.type));
    onStatus(incomplete ? "Đã lưu phần lịch sử nhận được; Zalo chưa cung cấp đủ tin. Kho local vẫn đọc được." :
      "Đã cập nhật phần lịch sử Zalo hiện cung cấp. Tin mới sẽ tiếp tục được lưu trên máy.");
  } finally {
    for (const {type} of states) archive.meta("sync-running:"+type,false);
  }
}
