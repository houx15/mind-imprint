import { apiFetch, ApiError } from "./client";

/**
 * OSS —— 她带回来的照片存在哪儿。
 *
 * 🚨 基础设施 2026-07-27 就上线了（presigned PUT/GET + CDN，见
 * [[oss-storage-infra]]），端点是共用的、按会话鉴权的，lite 学生本来就能调。
 * 缺的一直只是这个薄客户端——它当时只写进了 apps/web。所以「观察日记还不能拍
 * 照」不是因为 OSS 没准备好，是因为这一层没接上。
 *
 * 浏览器只做两件事：拿一个签好的 PUT 把文件直传上去，以及把 key 换成一个签好
 * 的 GET。AccessKey 永远在服务端。
 */

interface UploadURLResponse {
  putUrl: string;
  objectKey: string;
  requiredContentType: string;
  maxBytes: number;
  expiresAt: string;
}

/**
 * 上传一张照片，返回它的 object key。
 *
 * 存 key，不存 URL：签出来的 URL 几分钟就过期，存进库里第二天就是一条死链。
 */
export async function uploadUserImage(file: File): Promise<string> {
  const signed = await apiFetch<UploadURLResponse>("/api/v1/oss/upload-url", {
    method: "POST",
    body: JSON.stringify({ contentType: file.type, size: file.size, filename: file.name }),
  });

  // 直传 OSS。Content-Type 必须和签名里的一模一样——它被绑进了签名。
  // 这一枪打的是 OSS，不是我们的 API，所以绝不带 credentials。
  const put = await fetch(signed.putUrl, {
    method: "PUT",
    headers: { "Content-Type": signed.requiredContentType },
    body: file,
  });
  if (!put.ok) {
    throw new ApiError("oss_put_failed", `上传失败（HTTP ${put.status}）`, put.status);
  }
  return signed.objectKey;
}

/**
 * 把 key 换成一个能直接放进 <img src> 的签名 URL。
 *
 * 五分钟左右就过期，所以用之前现换，不要缓存下来。
 */
export async function resolveUrl(objectKey: string): Promise<string> {
  const res = await apiFetch<{ url: string; expiresAt: string }>("/api/v1/oss/resolve-url", {
    method: "POST",
    body: JSON.stringify({ objectKey }),
  });
  return res.url;
}
