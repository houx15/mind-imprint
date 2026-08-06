import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
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
  const [zoomed, setZoomed] = useState(false);
  const title = asset.title || "素材";

  // Close the lightbox on Escape while it's open.
  useEffect(() => {
    if (!zoomed) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setZoomed(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [zoomed]);

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
    const canZoom = Boolean(resolvedUrl && !failed);
    return (
      <figure style={{ margin: 0, borderRadius: 14, overflow: "hidden", border: "1px solid var(--mk-border)", background: "var(--mk-paper)" }}>
        {resolvedUrl && !failed ? (
          <div
            role="button"
            aria-label="放大图片"
            title="点击放大"
            // stopPropagation so opening the lightbox doesn't also fire the
            // reading pane's click-to-continue (this wrapper is a div, which the
            // reveal handler's button/a/input exclusion wouldn't otherwise catch).
            onClick={(e) => { e.stopPropagation(); setZoomed(true); }}
            style={{ position: "relative", cursor: "zoom-in", display: "block" }}
          >
            <img
              src={resolvedUrl}
              alt={title}
              onError={() => setFailed(true)}
              // `contain` (not `cover`) shows the whole image uncropped so nothing
              // important is hidden; the light figure background letterboxes it.
              style={{ display: "block", width: "100%", maxHeight: 420, objectFit: "contain" }}
            />
            {/* zoom affordance — SVG magnifier badge, not an emoji */}
            <span
              aria-hidden="true"
              style={{ position: "absolute", right: 8, bottom: 8, width: 26, height: 26, borderRadius: 8, background: "color-mix(in srgb, var(--mk-ink) 62%, transparent)", display: "flex", alignItems: "center", justifyContent: "center" }}
            >
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="var(--mk-surface)" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><circle cx="11" cy="11" r="7" /><path d="M21 21l-4.3-4.3M11 8v6M8 11h6" /></svg>
            </span>
          </div>
        ) : (
          <div style={{ height: 160, display: "flex", alignItems: "center", justifyContent: "center", color: "var(--mk-muted)", fontSize: 13, fontWeight: 600 }}>
            {failed ? "图片加载失败" : "加载中…"}
          </div>
        )}
        <figcaption style={{ padding: "8px 12px", fontSize: 12.5, color: "var(--mk-secondary)", fontWeight: 600 }}>{title}</figcaption>

        {/* Lightbox: full-size image over a dimmed backdrop, portaled to <body>
            so no scroll-pane overflow can clip it. Click anywhere or press Esc
            to close. */}
        {zoomed && canZoom && createPortal(
          <div
            role="dialog"
            aria-label={title}
            // stopPropagation: React re-dispatches portal events through the
            // component tree, so a backdrop click would otherwise bubble to the
            // reading pane's click-to-continue and advance the lesson on close.
            onClick={(e) => { e.stopPropagation(); setZoomed(false); }}
            style={{ position: "fixed", inset: 0, zIndex: 2000, background: "color-mix(in srgb, var(--mk-ink) 86%, transparent)", display: "flex", alignItems: "center", justifyContent: "center", padding: 24, cursor: "zoom-out" }}
          >
            <img src={resolvedUrl!} alt={title} style={{ maxWidth: "94vw", maxHeight: "90vh", objectFit: "contain", borderRadius: 8, boxShadow: "var(--mk-shadow-lg)" }} />
          </div>,
          document.body,
        )}
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
        style={{ display: "block", border: "1px solid var(--mk-border)", borderRadius: 12, padding: "12px 14px", background: "var(--mk-surface)", textDecoration: "none" }}
      >
        <span style={{ display: "inline-flex", fontSize: 11, fontWeight: 700, color: "var(--mk-accent-500)", background: "var(--mk-accent-50)", padding: "2px 9px", borderRadius: 999 }}>
          链接
        </span>
        <div style={{ marginTop: 6, fontSize: 14, fontWeight: 700, color: "var(--mk-ink)" }}>{title}</div>
        {asset.note && <div style={{ marginTop: 4, fontSize: 12.5, color: "var(--mk-secondary)" }}>{asset.note}</div>}
      </a>
    );
  }

  // type === "text"
  return (
    <div title={title} style={{ border: "1px solid var(--mk-border)", borderRadius: 12, padding: "12px 14px", background: "var(--mk-surface)" }}>
      <div style={{ fontSize: 13, fontWeight: 700, color: "var(--mk-ink)" }}>{title}</div>
      {asset.src && <div style={{ marginTop: 6, fontSize: 13.5, lineHeight: 1.7, color: "var(--mk-ink)" }}>{asset.src}</div>}
      {asset.note && <div style={{ marginTop: 4, fontSize: 12.5, color: "var(--mk-secondary)" }}>{asset.note}</div>}
    </div>
  );
}
