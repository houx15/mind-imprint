import { useEffect, useState } from "react";
import type { CourseAsset } from "@mind-imprint/contracts";
import { api } from "../../api";

// objectKeyFrom resolves the OSS object key a `CourseAsset` points at. The
// authoring convention embeds it as a `objectKey=` query param on `src` (URL
// -encoded, since `src` doubles as a human-followable link in authoring
// tools); assets authored directly against OSS instead carry it on `ossKey`.
export function objectKeyFrom(asset: CourseAsset): string {
  const src = asset.src || "";
  const match = /[?&]objectKey=([^&]+)/.exec(src);
  if (match) {
    const raw = match[1]!;
    try {
      return decodeURIComponent(raw);
    } catch {
      return raw;
    }
  }
  return asset.ossKey || "";
}

// AssetView renders one course_asset per its `type` — image (resolved via the
// OSS gateway, never a raw object key), link (opens in a new tab — we never
// embed a third party page), or text (an inline titled block). This is the
// leaf the render cache ultimately points students at.
export function AssetView({ asset }: { asset: CourseAsset }) {
  const [resolvedUrl, setResolvedUrl] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  const title = asset.title || "素材";

  useEffect(() => {
    if (asset.type !== "image") return;
    setResolvedUrl(null);
    setFailed(false);
    const key = objectKeyFrom(asset);
    if (!key) {
      setFailed(true);
      return;
    }
    let cancelled = false;
    void api
      .resolveUrl(key)
      .then((url) => {
        if (!cancelled) setResolvedUrl(url);
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [asset.type, asset.src, asset.ossKey]);

  if (asset.type === "image") {
    return (
      <figure style={{ margin: 0, borderRadius: 14, overflow: "hidden", border: "1px solid #EAECF2", background: "#F6F7FA" }}>
        {resolvedUrl && !failed ? (
          <img
            src={resolvedUrl}
            alt={title}
            onError={() => setFailed(true)}
            style={{ display: "block", width: "100%", maxHeight: 340, objectFit: "cover" }}
          />
        ) : (
          <div style={{ height: 160, display: "flex", alignItems: "center", justifyContent: "center", color: "#9AA1B0", fontSize: 13, fontWeight: 600 }}>
            {failed ? "图片加载失败" : "加载中…"}
          </div>
        )}
        <figcaption style={{ padding: "8px 12px", fontSize: 12.5, color: "#6B7384", fontWeight: 600 }}>{title}</figcaption>
      </figure>
    );
  }

  if (asset.type === "link") {
    return (
      <a
        href={asset.src}
        target="_blank"
        rel="noreferrer noopener"
        title={title}
        style={{ display: "block", border: "1px solid #EAECF2", borderRadius: 12, padding: "12px 14px", background: "#fff", textDecoration: "none" }}
      >
        <span style={{ display: "inline-flex", fontSize: 11, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 9px", borderRadius: 999 }}>
          链接
        </span>
        <div style={{ marginTop: 6, fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{title}</div>
        {asset.note && <div style={{ marginTop: 4, fontSize: 12.5, color: "#6B7384" }}>{asset.note}</div>}
      </a>
    );
  }

  // type === "text"
  return (
    <div title={title} style={{ border: "1px solid #EAECF2", borderRadius: 12, padding: "12px 14px", background: "#fff" }}>
      <div style={{ fontSize: 13, fontWeight: 700, color: "#1C2333" }}>{title}</div>
      {asset.src && <div style={{ marginTop: 6, fontSize: 13.5, lineHeight: 1.7, color: "#2B3346" }}>{asset.src}</div>}
      {asset.note && <div style={{ marginTop: 4, fontSize: 12.5, color: "#6B7384" }}>{asset.note}</div>}
    </div>
  );
}
