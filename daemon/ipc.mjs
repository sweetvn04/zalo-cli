import { ThreadType } from "zca-js";
import { loadHistory } from "./history.mjs";
import { loadRecipients } from "./recipients.mjs";

// Chuẩn hóa phản hồi IPC để client ghép đúng kết quả với lệnh đã gửi.
export async function handleCommand(raw, api, archive = null) {
  let id;
  let event = "send_result";
  try {
    const command = JSON.parse(raw.toString());
    id = command?.id;
    if (command?.action === "get_history") {
      event = "history_result";
      const { thread_id, thread_type, before_id = "", archive_before = "" } = command.payload ?? {};
      if (typeof thread_id !== "string" || !thread_id.trim() ||
          !["user", "group"].includes(thread_type)) {
        throw new Error("Hội thoại không hợp lệ");
      }
      if (archive) {
        if (typeof archive_before !== "string" || archive_before.length > 2048) throw new Error("Con trỏ kho local không hợp lệ");
        const local = archive.history(thread_id,thread_type,archive_before);
        return { event,id,ok:true,thread_id,thread_type,...local };
      }
      if (!api) throw new Error("Daemon chưa đăng nhập Zalo");
      if (typeof before_id !== "string" || (before_id && !/^\d+$/.test(before_id))) {
        throw new Error("Con trỏ lịch sử không hợp lệ");
      }
      const history = await loadHistory(api, thread_id, thread_type, before_id);
      return { event, id, ok: true, thread_id, thread_type, ...history };
    }
    if (command?.action === "list_recipients") {
      event = "recipients_result";
      if (archive) return {event,id,ok:true,recipients:archive.recipients(),notice:api ? "Đọc danh sách local; đang cập nhật từ Zalo ở nền." : "Đang đọc kho offline trên máy."};
      if (!api) throw new Error("Daemon chưa đăng nhập Zalo");
      const list = await loadRecipients(api);
      return { event, id, ok: true, ...list };
    }
    if (command?.action !== "send_message") {
      throw new Error("Lệnh IPC không được hỗ trợ");
    }
    const { thread_id, thread_type, text } = command.payload ?? {};
    if (typeof thread_id !== "string" || !thread_id.trim() ||
        typeof text !== "string" || !text.trim()) {
      throw new Error("Người nhận và nội dung không được để trống");
    }
    if (thread_type !== "user" && thread_type !== "group") {
      throw new Error("Loại hội thoại phải là user hoặc group");
    }
    if (!api) throw new Error("Daemon chưa đăng nhập Zalo");
    const sent = await api.sendMessage({ msg: text }, thread_id, thread_type === "group" ? ThreadType.Group : ThreadType.User);
    const result = { event: "send_result", id, ok: true };
    if (sent?.message?.msgId != null) result.msg_id = String(sent.message.msgId);
    if (archive) {
      try {
        archive.save({msg_id:result.msg_id || "",thread_id,thread_type,sender_id:archive.account,sender_name:"Bạn",
          content:text,kind:"text",timestamp:Date.now(),is_self:true});
      } catch (error) {
        result.notice = "Tin đã gửi nhưng chưa lưu được trên máy: "+error.message;
      }
    }
    return result;
  } catch (error) {
    const result = { event, id, ok: false, error: error.message };
    if (error.retryAfter) result.retry_after = error.retryAfter;
    return result;
  }
}
