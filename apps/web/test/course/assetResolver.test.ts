import { describe, it, expect } from "vitest";
import { resolveAssetPath, makeCdnAssetResolver } from "@/course/assetResolver";

describe("resolveAssetPath", () => {
  const map = { "assets/a.png": "https://cdn/courses/x/assets/a.png?auth_key=1-0-0-ab" };

  it("returns the mapped URL on a hit", () => {
    expect(resolveAssetPath(map, "assets/a.png")).toBe(map["assets/a.png"]);
  });

  it("returns the path unchanged on a miss", () => {
    expect(resolveAssetPath(map, "assets/missing.png")).toBe("assets/missing.png");
  });

  it("passes absolute and data URIs through untouched", () => {
    expect(resolveAssetPath(map, "https://x/y.png")).toBe("https://x/y.png");
    expect(resolveAssetPath(map, "data:image/png;base64,AAAA")).toBe("data:image/png;base64,AAAA");
  });

  it("passes absolute URIs through case-insensitively", () => {
    expect(resolveAssetPath(map, "HTTP://x/y.png")).toBe("HTTP://x/y.png");
    expect(resolveAssetPath(map, "HTTPS://x/y.png")).toBe("HTTPS://x/y.png");
  });
});

describe("makeCdnAssetResolver", () => {
  it("reads the map live via the getter", () => {
    let m: Record<string, string> = {};
    const r = makeCdnAssetResolver(() => m);
    expect(r.resolve("assets/a.png")).toBe("assets/a.png");
    m = { "assets/a.png": "https://cdn/a?auth_key=x" };
    expect(r.resolve("assets/a.png")).toBe("https://cdn/a?auth_key=x");
  });
});
