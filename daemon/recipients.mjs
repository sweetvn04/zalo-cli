const caches = new WeakMap();
const ttl = 60000;

function rateLimitedResult(state, now) {
  const retryAfter = Math.max(1, Math.ceil((state.blockedUntil - now) / 1000));
  const notice = `Zalo đang giới hạn yêu cầu. Ứng dụng tạm chờ ${retryAfter} giây trước khi tải lại.`;
  if (state.value !== null) {
    return { recipients: state.value, notice: notice + " Đang giữ danh sách đã tải.", retry_after: retryAfter };
  }
  const error = new Error(notice);
  error.retryAfter = retryAfter;
  throw error;
}

// Cache dùng chung cho mọi CLI/TUI của cùng phiên daemon. now có thể truyền
// từ kiểm thử để kiểm tra hết hạn mà không phải chờ thời gian thực.
export async function loadRecipients(api, now = Date.now()) {
  let state = caches.get(api);
  if (!state) {
    state = { value: null, expires: 0, blockedUntil: 0, failures: 0, pending: null };
    caches.set(api, state);
  }
  if (state.pending) return state.pending;
  if (now < state.blockedUntil) return rateLimitedResult(state, now);
  if (state.value !== null && now < state.expires) {
    const retryAfter = Math.max(1, Math.ceil((state.expires - now) / 1000));
    return { recipients: state.value, notice: `Đang dùng danh sách đã tải. Có thể tải mới sau ${retryAfter} giây.`, retry_after: retryAfter };
  }
  state.pending = (async () => {
    try {
      const recipients = await fetchRecipients(api);
      state.value = recipients;
      state.expires = now + ttl;
      state.failures = 0;
      state.blockedUntil = 0;
      return { recipients, retry_after: ttl / 1000 };
    } catch (error) {
      if (error.message !== "Request failed with status code 429") throw error;
      const delay = Math.min(300000, ttl * 2 ** state.failures);
      state.failures++;
      state.blockedUntil = now + delay;
      return rateLimitedResult(state, now);
    }
  })();
  try {
    return await state.pending;
  } finally {
    state.pending = null;
  }
}

async function fetchRecipients(api) {
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
  const ids = Object.keys(groups.gridVerMap);
  for (let start = 0; start < ids.length; start += 50) {
    const batch = ids.slice(start, start + 50);
    const info = await api.getGroupInfo(batch);
    for (const id of batch) {
      const group = info.gridInfoMap[id];
      if (group) recipients.push({ thread_id: id, thread_type: "group", name: group.name || id });
    }
  }
  recipients.sort((a, b) => a.name.localeCompare(b.name, "vi") ||
    a.thread_type.localeCompare(b.thread_type) || a.thread_id.localeCompare(b.thread_id));
  return recipients;
}
