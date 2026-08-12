import { describe, it, expect } from "vitest";
import { guidedSectionLabel, isGuidedSection } from "@/workspace/blocks/sectionLabels";

const SUBQS = [
  { id: "076139ab", text: "What is the physical mechanism by which canopy lowers heat exposure?" },
  { id: "3aa928b0", text: "How strong is the raw association between canopy and mortality?" },
];

describe("guidedSectionLabel", () => {
  it("names fixed proposal parts", () => {
    expect(guidedSectionLabel("prop:thesis")).toBe("暂定论点");
    expect(guidedSectionLabel("prop:understanding")).toBe("对题目的理解");
  });

  it("names fixed essay statement + submission steps", () => {
    expect(guidedSectionLabel("synthesis")).toBe("比较 / 综合");
    expect(guidedSectionLabel("sub:intro")).toBe("引言");
    expect(guidedSectionLabel("challenges")).toBe("面对反方观点");
  });

  it("resolves a claim to 论点 N + the sub-question text (never a raw uuid)", () => {
    const label = guidedSectionLabel("claim:3aa928b0", SUBQS);
    expect(label).toContain("论点 2");
    expect(label).toContain("How strong is the raw association");
    expect(label).not.toContain("3aa928b0");
  });

  it("resolves a proposal sub-question card the same way (shared id)", () => {
    const label = guidedSectionLabel("prop:subq:076139ab", SUBQS);
    expect(label).toContain("子问题 1");
    expect(label).toContain("What is the physical mechanism");
  });

  it("clips an over-long sub-question so the header stays short", () => {
    const long = [{ id: "x", text: "a".repeat(200) }];
    const label = guidedSectionLabel("claim:x", long)!;
    expect(label.endsWith("…")).toBe(true);
    expect(label.length).toBeLessThan(60);
  });

  it("degrades to the bare ordinal-less label when the sub-question is unknown", () => {
    expect(guidedSectionLabel("claim:missing", SUBQS)).toBe("论点");
    expect(guidedSectionLabel("prop:subq:missing", [])).toBe("子问题");
  });

  it("returns null for a genuine student snippet label", () => {
    expect(guidedSectionLabel("阅读笔记", SUBQS)).toBeNull();
    expect(guidedSectionLabel("body1-greening")).toBeNull();
    expect(guidedSectionLabel(null)).toBeNull();
  });

  it("isGuidedSection flags guided keys and clears free labels", () => {
    expect(isGuidedSection("claim:076139ab")).toBe(true);
    expect(isGuidedSection("prop:subq:abc")).toBe(true);
    expect(isGuidedSection("sub:conclusion")).toBe(true);
    expect(isGuidedSection("阅读笔记")).toBe(false);
    expect(isGuidedSection(null)).toBe(false);
  });
});
