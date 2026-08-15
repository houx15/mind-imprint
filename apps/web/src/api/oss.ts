import { apiFetch, ApiError } from "./client";

// OSS storage client. The browser only ever does user-image uploads (the
// admin-key upload route is for backend scripts, not the app) and resolves
// object keys to short-lived signed GET URLs. The AccessKey stays server-side;
// the client only touches presigned URLs.

interface UploadURLResponse {
  putUrl: string;
  objectKey: string;
  requiredContentType: string;
  maxBytes: number;
  expiresAt: string;
}

// uploadUserImage uploads a File to OSS via a server-signed PUT URL and returns
// its object key (store it wherever the image is referenced). The caller passes
// the raw File; its MIME type must be in the user_image allowlist (png/jpeg/
// webp) or the server rejects it with 400 unsupported_type.
export async function uploadUserImage(file: File): Promise<string> {
  const signed = await apiFetch<UploadURLResponse>("/api/v1/oss/upload-url", {
    method: "POST",
    body: JSON.stringify({
      contentType: file.type,
      size: file.size,
      filename: file.name,
    }),
  });

  // Direct-to-OSS PUT. Content-Type must exactly match the signed value — it is
  // bound into the signature. Do NOT send credentials (this is OSS, not our API).
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

// uploadUserDoc uploads a reading document (PDF/DOCX) to OSS via a server-signed
// PUT URL (scope user_doc) and returns its object key. The server then downloads
// and extracts it (ingestReferenceFile). The File's MIME type must be in the
// user_doc allowlist (pdf / docx) or the server rejects it with 400.
export async function uploadUserDoc(file: File): Promise<string> {
  const signed = await apiFetch<UploadURLResponse>("/api/v1/oss/upload-url", {
    method: "POST",
    body: JSON.stringify({
      scope: "user_doc",
      contentType: file.type,
      size: file.size,
      filename: file.name,
    }),
  });

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

// resolveUrl turns an object key into a short-lived signed GET URL (served via
// CDN) usable as an <img src> or download link. It expires in ~5 minutes, so
// resolve on demand right before use rather than caching the URL.
export async function resolveUrl(objectKey: string): Promise<string> {
  const res = await apiFetch<{ url: string; expiresAt: string }>("/api/v1/oss/resolve-url", {
    method: "POST",
    body: JSON.stringify({ objectKey }),
  });
  return res.url;
}
