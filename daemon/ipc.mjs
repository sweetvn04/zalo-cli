import { ThreadType } from "zca-js";

// Chuẩn hóa phản hồi IPC để client ghép đúng kết quả với lệnh đã gửi.
export async function handleCommand(raw, api) {
  let id;
  try {
    const command = JSON.parse(raw.toString());
    id = command?.id;
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
    await api.sendMessage({ msg: text }, thread_id, thread_type === "group" ? ThreadType.Group : ThreadType.User);
    return { event: "send_result", id, ok: true };
  } catch (error) {
    return { event: "send_result", id, ok: false, error: error.message };
  }
}
