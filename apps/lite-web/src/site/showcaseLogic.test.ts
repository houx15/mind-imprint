import { describe, expect, it } from "vitest";
import { safeShowcaseDate, safeShowcaseWorkPath, selectedShowcaseWorks, timelineShowcaseWorks } from "./Showcase";
import { SHOWCASE_ILLUSTRATIONS, SHOWCASE_PRESETS } from "./showcasePresets";
import type { ShowcaseWork } from "./showcaseTypes";

describe("showcase publication boundaries", () => {
  it("keeps only selected works in the supplied order", () => {
    const works: ShowcaseWork[] = [
      { id: "a", kind: "writing", title: "A", summary: "" },
      { id: "b", kind: "reading", title: "B", summary: "" },
      { id: "c", kind: "project", title: "C", summary: "" },
    ];
    expect(selectedShowcaseWorks(works, ["c", "a", "missing", "c"]).map((work) => work.id)).toEqual(["c", "a"]);
    expect(selectedShowcaseWorks(works, [])).toEqual([]);
  });

  it("accepts only a local single-token shared-work route", () => {
    expect(safeShowcaseWorkPath("/s/Abc_123-xy")).toBe("/s/Abc_123-xy");
    expect(safeShowcaseWorkPath("https://example.com/s/token")).toBeUndefined();
    expect(safeShowcaseWorkPath("/s/token/record")).toBeUndefined();
    expect(safeShowcaseWorkPath("/s/token?next=https://example.com")).toBeUndefined();
    expect(safeShowcaseWorkPath("/p/token")).toBeUndefined();
  });

  it("uses only real ISO calendar dates", () => {
    expect(safeShowcaseDate("2026-09-22")).toBe("2026-09-22");
    expect(safeShowcaseDate("2026-02-30")).toBeUndefined();
    expect(safeShowcaseDate("22/09/2026")).toBeUndefined();
    expect(safeShowcaseDate()).toBeUndefined();
  });

  it("orders a timeline by real date and leaves undated work last", () => {
    const works: ShowcaseWork[] = [
      { id: "none", kind: "project", title: "No date", summary: "" },
      { id: "old", kind: "writing", title: "Old", summary: "", date: "2026-01-10" },
      { id: "new", kind: "reading", title: "New", summary: "", date: "2026-09-22" },
    ];
    expect(timelineShowcaseWorks(works).map((work) => work.id)).toEqual(["new", "old", "none"]);
  });
});

describe("showcase visual presets", () => {
  it("change only visual fields and reference known local illustrations", () => {
    const protectedFields = ["name", "bio", "tagline", "heroTitle", "interests", "selectedWorkIds", "sectionOrder", "avatarKey", "heroImageKey"];
    const illustrationIds = new Set(SHOWCASE_ILLUSTRATIONS.map((item) => item.id));
    expect(SHOWCASE_PRESETS).toHaveLength(5);
    for (const preset of SHOWCASE_PRESETS) {
      expect(Object.keys(preset.config).some((key) => protectedFields.includes(key))).toBe(false);
      expect(illustrationIds.has(preset.config.illustration ?? "none")).toBe(true);
    }
    for (const illustration of SHOWCASE_ILLUSTRATIONS) {
      if (illustration.src) expect(illustration.src).toMatch(/^\/images\/showcase\/[a-z]+\.webp$/);
    }
  });
});
