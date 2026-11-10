import fs from "node:fs";
import path from "node:path";
import os from "node:os";
import { createHash, randomUUID } from "node:crypto";
import dns from "node:dns";
import net from "node:net";
import http from "node:http";
import https from "node:https";

const blocked = new net.BlockList();
for (const [address, prefix] of [["0.0.0.0",8],["10.0.0.0",8],["127.0.0.0",8],["169.254.0.0",16],
  ["172.16.0.0",12],["192.168.0.0",16],["100.64.0.0",10],["224.0.0.0",4]]) blocked.addSubnet(address, prefix, "ipv4");
for (const [address, prefix] of [["::",128],["::1",128],["fc00::",7],["fe80::",10],["ff00::",8]]) blocked.addSubnet(address, prefix, "ipv6");

export function validateImageURL(value) {
  const url = new URL(value);
  if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) throw new Error("URL ảnh không hợp lệ");
  const host = url.hostname.replace(/^\[|\]$/g, "");
  if (host === "localhost" || host.endsWith(".localhost")) throw new Error("URL ảnh trỏ tới máy cục bộ");
  const family = net.isIP(host);
  if (family && blocked.check(host, family === 4 ? "ipv4" : "ipv6")) throw new Error("URL ảnh trỏ tới mạng nội bộ");
  return url;
}

function publicLookup(host, options, callback) {
  dns.lookup(host, { all: true }, (error, addresses) => {
    if (error) return callback(error);
    if (!addresses.length || addresses.some(item => blocked.check(item.address, item.family === 4 ? "ipv4" : "ipv6"))) {
      return callback(new Error("Máy chủ ảnh trỏ tới mạng nội bộ"));
    }
    if (options?.all) return callback(null, addresses);
    callback(null, addresses[0].address, addresses[0].family);
  });
}

async function imageResponse(address, signal, redirects = 0) {
  const url = validateImageURL(address);
  const response = await new Promise((resolve, reject) => {
    const request = (url.protocol === "https:" ? https : http).get(url, { lookup: publicLookup, signal }, resolve);
    request.on("error", reject);
  });
  if ([301,302,303,307,308].includes(response.statusCode)) {
    response.resume();
    if (redirects >= 5 || !response.headers.location) throw new Error("Ảnh chuyển hướng không hợp lệ");
    return imageResponse(new URL(response.headers.location, url).href, signal, redirects+1);
  }
  if (response.statusCode !== 200) {
    response.resume();
    throw new Error(`Máy chủ ảnh trả HTTP ${response.statusCode}`);
  }
  return response;
}

export function imageExtension(header) {
  if (header.subarray(0,8).equals(Buffer.from([137,80,78,71,13,10,26,10]))) return ".png";
  if (header[0] === 255 && header[1] === 216 && header[2] === 255) return ".jpg";
  if (["GIF87a","GIF89a"].includes(header.subarray(0,6).toString())) return ".gif";
  if (header.subarray(0,4).toString() === "RIFF" && header.subarray(8,12).toString() === "WEBP") return ".webp";
  if (header.subarray(0,2).toString() === "BM") return ".bmp";
  throw new Error("Dữ liệu không phải ảnh được hỗ trợ");
}

// Tận dụng ảnh đã được Go tải trước đó, kể cả khi URL Zalo vừa hết hạn.
function previousCache(address) {
  const root = process.env.XDG_CACHE_HOME || path.join(os.homedir(), ".cache");
  const hash = createHash("sha256").update(address).digest("hex");
  for (const extension of [".jpg",".png",".gif",".webp",".bmp"]) {
    const file = path.join(root, "zalo-cli", "images", hash+extension);
    try { if (fs.lstatSync(file).isFile()) return fs.createReadStream(file); } catch { /* Chưa có cache. */ }
  }
  return null;
}

export function videoExtension(header) {
  const brand = header.subarray(8,12).toString();
  if (header.subarray(4,8).toString() === "ftyp" &&
      /^(isom|iso[2-9]|mp4[12]|avc1|M4V |MSNV|qt  |3gp[4-9])$/.test(brand)) return ".mp4";
  if (header.subarray(0,4).equals(Buffer.from([0x1a,0x45,0xdf,0xa3])) && header.includes(Buffer.from("webm"))) return ".webm";
  throw new Error("Dữ liệu không phải video MP4/WebM được hỗ trợ");
}

export async function storeImage(address, directory, { signal, kind = "image" } = {}) {
  validateImageURL(address);
  const timeout = AbortSignal.timeout(kind === "video" ? 600000 : 90000);
  const combined = signal ? AbortSignal.any([signal, timeout]) : timeout;
  const source = (kind === "image" ? previousCache(address) : null) || await imageResponse(address, combined);
  const temporary = path.join(directory, ".download-"+randomUUID());
  let file;
  try { file = await fs.promises.open(temporary, "wx", 0o600); }
  catch (error) { source.destroy(); throw error; }
  const hash = createHash("sha256");
  let size = 0;
  let header = Buffer.alloc(0);
  try {
    for await (const chunk of source) {
      if (combined.aborted) throw combined.reason;
      size += chunk.length;
      if (size > (kind === "video" ? 1024 : 100)*1024*1024) throw new Error("Media vượt giới hạn (ảnh 100 MiB, video 1 GiB)");
      if (header.length < 512) header = Buffer.concat([header, chunk.subarray(0, 512-header.length)]);
      hash.update(chunk);
      await file.writeFile(chunk);
    }
    const extension = kind === "video" ? videoExtension(header) : imageExtension(header);
    await file.sync();
    await file.close();
    const name = hash.digest("hex")+extension;
    await fs.promises.rename(temporary, path.join(directory, name));
    return name;
  } finally {
    await file.close().catch(() => {});
    await fs.promises.rm(temporary, { force: true });
    source.destroy();
  }
}
