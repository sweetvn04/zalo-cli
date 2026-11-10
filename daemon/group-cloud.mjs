import { GroupMessage } from "zca-js";

// Compatibility adapter for zca-js 2.2.0; upstream fix:
// https://github.com/RFS-ADRENO/zca-js/pull/397
// Keep signing, cookies and AES in the library instead of reimplementing them.
const { apiFactory } = await import(new URL("./utils.js", import.meta.resolve("zca-js")));
const readers = new WeakMap();

export function createCloudReader(api, ctx, utils) {
  const service = api.zpwServiceMap?.group_cloud_message?.[0];
  if (!service) throw new Error("Phiên Zalo chưa cung cấp dịch vụ lịch sử cloud của nhóm");
  return async (groupId, before = "", signal) => {
    if (!/^\d+$/.test(groupId) || (before && !/^\d+$/.test(before))) throw new Error("ID nhóm hoặc con trỏ cloud không hợp lệ");
    const params = utils.encodeAES(JSON.stringify({groupId,globalMsgId:before || "0",count:50,msgIds:[],imei:ctx.imei,src:-1}));
    if (!params) throw new Error("Không mã hóa được yêu cầu lịch sử nhóm");
    const url = utils.makeURL(service+"/api/cm/"+(before ? "getoldv2" : "getrecentv2"),{params,nretry:0});
    const timeout = AbortSignal.timeout(20000);
    const response = await utils.request(url,{method:"GET",signal:signal ? AbortSignal.any([signal,timeout]) : timeout});
    try { return await utils.resolve(response,result => {
      const data = typeof result.data === "string" ? JSON.parse(result.data) : result.data;
      if (!data || !Array.isArray(data.groupMsgs)) throw new Error("Phản hồi lịch sử cloud không đúng định dạng");
      const messages = data.groupMsgs.map(raw => new GroupMessage(ctx.uid,{...raw}));
      if (messages.some(message => String(message.threadId) !== groupId)) throw new Error("Lịch sử cloud trả sai nhóm");
      const cursor = data.lastMsgId == null ? "" : String(data.lastMsgId);
      if (cursor && !/^\d+$/.test(cursor)) throw new Error("Con trỏ cloud không hợp lệ");
      // Do not round a cursor silently if the upstream decoder returned a number.
      if (typeof data.lastMsgId === "number" && !Number.isSafeInteger(data.lastMsgId)) throw new Error("Con trỏ cloud vượt độ chính xác số nguyên");
      return {messages,cursor,more:data.hasMore === true || data.hasMore === 1,
        filtered:data.isFiltered === true || data.isFiltered === 1 || data.isFilteredByTimeJoin === true || data.isFilteredByPhase === true};
    }); } catch (error) {
      // Server's explicit stop marker; recent messages already fetched stay valid.
      if (error.code === -69) return {messages:[],cursor:"",more:false,filtered:false,limited:true};
      throw error;
    }
  };
}

export function groupCloudPage(api,groupId,before = "",signal) {
  if (!readers.has(api)) readers.set(api,apiFactory()(createCloudReader)(api.getContext(),api));
  return readers.get(api)(groupId,before,signal);
}
