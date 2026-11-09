import { ThreadType } from "zca-js";

// Chuẩn hóa phản hồi IPC để client ghép đúng kết quả với lệnh đã gửi.
export async function handleCommand(raw, api) {
  let id;
  let event = "send_result";
  try {
    const command = JSON.parse(raw.toString());
    id = command?.id;
    if (command?.action === "list_recipients") {
      event = "recipients_result";
      if (!api) throw new Error("Daemon chưa đăng nhập Zalo");
      const friends = [];
      const pageSize = 200;
      for (let page = 1; ; page++) {
        const batch = await api.getAllFriends(pageSize, page);
        friends.push(...batch);
        if (batch.length < pageSize) break;
      }
      const groups = await api.getAllGroups();
      const recipients = friends.map(friend => ({
        thread_id: String(friend.userId),
        thread_type: "user",
        name: friend.displayName || friend.zaloName || String(friend.userId),
      }));
      // Lấy tên từng nhóm theo batch nhỏ, tránh một request quá lớn.
      const groupIDs = Object.keys(groups.gridVerMap);
      for (let start = 0; start < groupIDs.length; start += 50) {
        const ids = groupIDs.slice(start, start + 50);
        const info = await api.getGroupInfo(ids);
        for (const id of ids) {
          const group = info.gridInfoMap[id];
          if (group) recipients.push({ thread_id: id, thread_type: "group", name: group.name || id });
        }
      }
      recipients.sort((a, b) => a.name.localeCompare(b.name, "vi") ||
        a.thread_type.localeCompare(b.thread_type) || a.thread_id.localeCompare(b.thread_id));
      return { event, id, ok: true, recipients };
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
    await api.sendMessage({ msg: text }, thread_id, thread_type === "group" ? ThreadType.Group : ThreadType.User);
    return { event: "send_result", id, ok: true };
  } catch (error) {
    return { event, id, ok: false, error: error.message };
  }
}
