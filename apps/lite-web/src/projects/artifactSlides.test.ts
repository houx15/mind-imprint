import { describe, expect, it } from "vitest";
import { artifactSlides } from "./artifactSlides";

describe("artifactSlides", () => {
  it("separates comparison from explanation without losing introduction or limits", () => {
    const slides = artifactSlides("# 展示\n说明\n\n## 比较\n旧版、新版、差不多、不确定\n\n## 理由\n设计判断\n\n## 未验证\n尚未现场试用");
    expect(slides).toHaveLength(4);
    expect(slides[1]!.markdown).toContain("不确定");
    expect(slides[1]!.markdown).not.toContain("设计判断");
    expect(slides[3]!.markdown).toContain("尚未现场试用");
  });
  it("keeps fenced headings inside their section and resolves reference links on every slide", () => {
    const slides = artifactSlides("## 比较\n```md\n## 不是一页\n```\n[来源][ref]\n\n## 理由\n正文\n\n[ref]: https://example.com/source");
    expect(slides).toHaveLength(2);
    expect(slides[0]!.markdown).toContain("## 不是一页");
    expect(slides[0]!.markdown).toContain("https://example.com/source");
  });
  it("does not split headings nested in a quote or single-section material", () => {
    expect(artifactSlides("> ## 引文\n\n## 正文\n一段话")).toEqual([]);
  });
});
