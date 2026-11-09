import { setTimeout as pause } from "node:timers/promises";
import { archiveHistoryPage } from "./history.mjs";

// Lấy phần lịch sử máy chủ còn cung cấp khi daemon chạy, kể cả không mở TUI.
// Giữa các lượt có khoảng nghỉ; lỗi không làm mất dữ liệu đã ghi.
export async function syncArchive(api, archive, signal) {
  for (const type of ["user", "group"]) {
    if (signal.aborted) return;
    archive.meta("sync-done:"+type, false);
    archive.meta("sync-running:"+type, true);
    archive.meta("sync-error:"+type, null);
    try {
      let cursor = "";
      for (;;) {
        if (signal.aborted) return;
        const page = await archiveHistoryPage(api, type, cursor);
        if (signal.aborted) return;
        // Listener old_messages đã lưu toàn bộ payload; gọi lại idempotent
        // bảo đảm vẫn đúng khi dùng module này với một API/fixture khác.
        archive.captureBatch(page.messages);
        if (!page.more || !page.cursor || page.cursor === cursor) {
          archive.meta("sync-done:"+type, !page.partial);
          break;
        }
        cursor = page.cursor;
        await pause(2000, undefined, { signal });
      }
    } catch (error) {
      if (signal.aborted) return;
      archive.meta("sync-error:"+type, error.message);
      console.warn(`⚠️ Đồng bộ lịch sử ${type} tạm dừng:`, error.message);
    } finally {
      archive.meta("sync-running:"+type, false);
    }
  }
}
