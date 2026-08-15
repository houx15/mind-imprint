import type { AssetResolver } from "@mind-imprint/course-runtime";
import { API_BASE } from "@/api/client";

// assetResolver.ts — the production AssetResolver (Course Runtime §4, Slice 8).
// The runtime resolves authored relative asset paths (`assets/videos/case.mp4`,
// audio/captions/posters, …) into loadable URLs SYNCHRONOUSLY — the interface is
// `resolve(relativePath) => string`, so it cannot await a per-object signed OSS
// URL (that seam, api/oss.ts resolveUrl, is async). Instead a course's assets
// live under one slug-scoped OSS/CDN prefix and we join the relative path onto
// it. Already-absolute (`http(s):`) and inline (`data:`) references pass through
// untouched so authored fixtures that hard-code a CDN URL still work.

const DEFAULT_PREFIX = "/oss/course-assets";

export interface CdnAssetResolverConfig {
  /** The course this resolver serves; scopes the asset base by slug. */
  slug: string;
  /**
   * Base URL/prefix the course's assets live under. Defaults to a slug-scoped
   * path under the API origin. Pass an explicit CDN base to point elsewhere.
   */
  baseUrl?: string;
}

/** Joins `base` and `rel` with exactly one slash between them. */
function joinUrl(base: string, rel: string): string {
  return `${base.replace(/\/+$/, "")}/${rel.replace(/^\/+/, "")}`;
}

/**
 * makeCdnAssetResolver returns a synchronous AssetResolver that resolves a
 * course's relative asset paths against a slug-scoped base URL.
 */
export function makeCdnAssetResolver(config: CdnAssetResolverConfig): AssetResolver {
  const base = config.baseUrl ?? `${API_BASE}${DEFAULT_PREFIX}/${config.slug}`;
  return {
    resolve(relativePath: string): string {
      if (/^https?:\/\//i.test(relativePath) || relativePath.startsWith("data:")) {
        return relativePath;
      }
      return joinUrl(base, relativePath);
    },
  };
}
