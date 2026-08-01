import { describe, it, expect } from "vitest";
import { parseSections, serializeSections, sectionsFromOutline, newSection } from "@/workspace/blocks/draftSections";

describe("draftSections", () => {
  it("parses headings into sections, with pre-heading text as an intro", () => {
    const md = "开场白。\n\n# 背景\n\n第一段。\n\n## 反例\n\n第二段。";
    const secs = parseSections(md);
    expect(secs.map((s) => [s.level, s.heading, s.body])).toEqual([
      [0, "", "开场白。"],
      [1, "背景", "第一段。"],
      [2, "反例", "第二段。"],
    ]);
  });

  it("empty draft → no sections", () => {
    expect(parseSections("")).toEqual([]);
    expect(parseSections("   \n  ")).toEqual([]);
  });

  it("round-trips: parse(serialize(x)) preserves level/heading/body", () => {
    const md = "# 论点\n\n中国的贡献是实质性的。\n\n## 反例\n\n碳排放全球第一。";
    const once = parseSections(md);
    const back = serializeSections(once);
    const twice = parseSections(back);
    expect(twice.map((s) => [s.level, s.heading, s.body])).toEqual(
      once.map((s) => [s.level, s.heading, s.body]),
    );
    expect(back).toContain("# 论点");
    expect(back).toContain("## 反例");
  });

  it("serialize drops wholly-empty sections and emits heading-only for empty bodies", () => {
    const out = serializeSections([
      { id: "a", level: 1, heading: "有标题无正文", body: "" },
      { id: "b", level: 1, heading: "", body: "" }, // wholly empty → dropped
      { id: "c", level: 2, heading: "小节", body: "内容" },
    ]);
    expect(out).toBe("# 有标题无正文\n\n## 小节\n\n内容");
  });

  it("sectionsFromOutline is additive and skips headings already present", () => {
    const existing = parseSections("# 背景\n\n已经写了。");
    const gen = sectionsFromOutline(["背景", "反例", "结论"], existing);
    expect(gen.map((s) => s.heading)).toEqual(["反例", "结论"]); // 背景 skipped
    expect(gen.every((s) => s.level === 1)).toBe(true);
  });

  it("newSection yields a blank level-1 section by default", () => {
    const s = newSection();
    expect(s.level).toBe(1);
    expect(s.heading).toBe("");
    expect(s.body).toBe("");
  });
});
