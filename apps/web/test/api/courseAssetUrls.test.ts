import { describe, it, expect, vi, afterEach } from "vitest";
import { fetchCourseAssetUrls } from "@/api/courseAssetUrls";

afterEach(() => { vi.restoreAllMocks(); });

describe("fetchCourseAssetUrls", () => {
  it("POSTs the paths and returns the assetUrls/expiresAt envelope", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({
        assetUrls: { "assets/a.png": "https://cdn/courses/demo/assets/a.png?auth_key=1-0-0-ab" },
        expiresAt: "2026-08-16T12:00:00Z",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const out = await fetchCourseAssetUrls("demo", ["assets/a.png"]);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/demo/asset-urls");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ paths: ["assets/a.png"] });

    expect(out.assetUrls["assets/a.png"]).toContain("auth_key=");
    expect(out.expiresAt).toBe("2026-08-16T12:00:00Z");
  });
});
