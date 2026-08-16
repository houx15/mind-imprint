import { apiFetch } from "./client";

// courseAssetUrls.ts — client for POST /api/v1/courses/{slug}/asset-urls. Sends
// the course's relative asset paths (collected client-side via
// collectAssetPaths) and gets back a { relativePath → signed CDN URL } map plus
// the window's expiry. Also the refresh endpoint: re-call with the same paths.

export interface CourseAssetUrls {
  assetUrls: Record<string, string>;
  expiresAt: string;
}

export async function fetchCourseAssetUrls(slug: string, paths: string[]): Promise<CourseAssetUrls> {
  return apiFetch<CourseAssetUrls>(`/api/v1/courses/${slug}/asset-urls`, {
    method: "POST",
    body: JSON.stringify({ paths }),
  });
}
