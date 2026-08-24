import { describe, it, expect } from "vitest";
import { parsePath, routePath, type AppRoute } from "@/shell/routing";

describe("routing.parsePath", () => {
  it("maps the root and /home to the home tab", () => {
    expect(parsePath("/")).toEqual({ tab: "home" });
    expect(parsePath("/home")).toEqual({ tab: "home" });
    expect(parsePath("")).toEqual({ tab: "home" });
  });

  it("maps the four top tabs", () => {
    expect(parsePath("/projects")).toEqual({ tab: "projects" });
    expect(parsePath("/courses")).toEqual({ tab: "courses" });
    expect(parsePath("/me")).toEqual({ tab: "me" });
  });

  it("captures a project id and a course slug", () => {
    expect(parsePath("/projects/abc-123")).toEqual({ tab: "projects", projectId: "abc-123" });
    expect(parsePath("/courses/craap-basics")).toEqual({ tab: "courses", slug: "craap-basics" });
  });

  it("tolerates trailing slashes and /index.html", () => {
    expect(parsePath("/projects/")).toEqual({ tab: "projects" });
    expect(parsePath("/projects/abc-123/")).toEqual({ tab: "projects", projectId: "abc-123" });
    expect(parsePath("/index.html")).toEqual({ tab: "home" });
    expect(parsePath("/")).toEqual({ tab: "home" });
  });

  it("url-decodes segments", () => {
    expect(parsePath("/courses/craap%20basics")).toEqual({ tab: "courses", slug: "craap basics" });
  });

  it("keeps a malformed escape rather than throwing", () => {
    expect(() => parsePath("/courses/%")).not.toThrow();
    expect(parsePath("/courses/%")).toEqual({ tab: "courses", slug: "%" });
  });

  it("falls back to home for unknown paths", () => {
    expect(parsePath("/nope")).toEqual({ tab: "home" });
    expect(parsePath("/settings/deep/link")).toEqual({ tab: "home" });
  });

  it("ignores extra segments past the id", () => {
    expect(parsePath("/projects/abc-123/room/reading")).toEqual({ tab: "projects", projectId: "abc-123" });
  });
});

describe("routing.routePath", () => {
  it("formats each route to its canonical path", () => {
    expect(routePath({ tab: "home" })).toBe("/");
    expect(routePath({ tab: "projects" })).toBe("/projects");
    expect(routePath({ tab: "projects", projectId: "abc-123" })).toBe("/projects/abc-123");
    expect(routePath({ tab: "courses" })).toBe("/courses");
    expect(routePath({ tab: "courses", slug: "craap-basics" })).toBe("/courses/craap-basics");
    expect(routePath({ tab: "me" })).toBe("/me");
  });

  it("encodes ids with special characters", () => {
    expect(routePath({ tab: "courses", slug: "a b" })).toBe("/courses/a%20b");
  });

  it("never emits an absolute URL (domain-agnostic)", () => {
    const routes: AppRoute[] = [
      { tab: "home" },
      { tab: "projects", projectId: "x" },
      { tab: "courses", slug: "y" },
      { tab: "me" },
    ];
    for (const r of routes) {
      const p = routePath(r);
      expect(p.startsWith("/")).toBe(true);
      expect(p).not.toMatch(/^https?:/);
      expect(p).not.toContain("//");
    }
  });
});

describe("routing round-trip", () => {
  it("parse ∘ routePath is identity on canonical routes", () => {
    const routes: AppRoute[] = [
      { tab: "home" },
      { tab: "projects" },
      { tab: "projects", projectId: "abc-123" },
      { tab: "courses" },
      { tab: "courses", slug: "craap-basics" },
      { tab: "me" },
    ];
    for (const r of routes) {
      expect(parsePath(routePath(r))).toEqual(r);
    }
  });
});
