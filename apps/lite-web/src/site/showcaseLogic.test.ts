import { describe, expect, it } from "vitest";
import { safeShowcaseWorkPath, selectedShowcaseWorks } from "./Showcase";
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
});
