// Entrances into the actual product. The marketing site never holds secrets;
// it only links out to the app. Override with PUBLIC_APP_URL at build time.
const APP_URL = import.meta.env.PUBLIC_APP_URL ?? "http://localhost:5173";

export const appUrl = APP_URL;
export const loginUrl = APP_URL;
// Demo deep-links into the app with ?trial=1, which auto-signs-in as the seeded
// sample student (handled in apps/web AppShell). Note: ?demo is a different,
// pre-existing surface (the dev card gallery), so the trial param is distinct.
export const demoUrl = `${APP_URL}/?trial=1`;

// --- media -----------------------------------------------------------------
//
// Site media (screenshots, portraits, diagrams) lives in its own Aliyun bucket,
// served through its own CDN domain. That bucket is SEPARATE from the student
// platform's — different bucket, different CDN host, different keys — so
// nothing here can reach course material, and vice versa.
//
// No credential is involved on this side. The bucket is private but the CDN is
// authorised to read it, so a public visitor just fetches a plain URL. Signing
// keys belong to the upload script (deploy/upload-site-assets.sh), which runs
// from a developer machine — a static site could not keep a secret anyway.
//
// Unset (local dev): falls back to apps/site/public/media/, so you can drop a
// file in and see it without touching object storage.
const ASSET_BASE = (import.meta.env.PUBLIC_ASSET_BASE_URL ?? "").replace(/\/+$/, "");

/**
 * Resolve a media key to a URL.
 *   assetUrl("home/course-player.png")
 *     → https://mind-assets.uni-robot.cn/home/course-player.png   (prod)
 *     → /media/home/course-player.png                             (local)
 *
 * Keep keys stable: the CDN caches by URL, so the clean way to replace an
 * image is a new key (hero-v2.png), not a new file behind the same one.
 */
export const assetUrl = (key: string): string => {
  const clean = key.replace(/^\/+/, "");
  return ASSET_BASE ? `${ASSET_BASE}/${clean}` : `/media/${clean}`;
};

export const assetBase = ASSET_BASE;
