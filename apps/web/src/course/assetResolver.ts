import type { AssetResolver } from "@mind-imprint/course-runtime";

// assetResolver.ts — the production AssetResolver (Course Runtime §4). Course
// assets are served as CDN URL鉴权 links minted server-side (POST
// /courses/{slug}/asset-urls) and handed to the client as a { relativePath → url }
// map. The runtime resolves paths SYNCHRONOUSLY, so resolution is a pure map
// lookup: no awaiting, no per-object signing here. Absolute (http(s)://) and
// inline (data:) references pass through untouched.

/** Pure lookup: mapped URL on a hit, the path unchanged on a miss/pass-through. */
export function resolveAssetPath(assetUrls: Record<string, string>, relativePath: string): string {
  if (/^https?:\/\//i.test(relativePath) || relativePath.startsWith("data:")) {
    return relativePath;
  }
  return assetUrls[relativePath] ?? relativePath;
}

/**
 * makeCdnAssetResolver returns a synchronous AssetResolver backed by a live map.
 * The getter is read on every resolve() so the host can refresh the signed URLs
 * (before the URL鉴权 window lapses) without rebuilding the resolver or the
 * adapters that own the session.
 */
export function makeCdnAssetResolver(getAssetUrls: () => Record<string, string>): AssetResolver {
  return {
    resolve(relativePath: string): string {
      return resolveAssetPath(getAssetUrls(), relativePath);
    },
  };
}
