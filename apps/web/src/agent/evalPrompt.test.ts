import { describe, it, expect } from "vitest";
import { DEMO_RUBRIC, SOLO_LABELS } from "@mind-imprint/contracts";
import { buildEvalPrompt } from "./evalPrompt";

describe("buildEvalPrompt", () => {
  const prompt = buildEvalPrompt(DEMO_RUBRIC);

  it("contains each dimension name", () => {
    for (const dim of DEMO_RUBRIC) {
      expect(prompt).toContain(dim.name);
    }
  });

  it("contains each dimension's L1 anchor text", () => {
    for (const dim of DEMO_RUBRIC) {
      expect(prompt).toContain(dim.anchors.L1);
    }
  });

  it("contains each dimension's L4 anchor text", () => {
    for (const dim of DEMO_RUBRIC) {
      expect(prompt).toContain(dim.anchors.L4);
    }
  });

  it("contains SOLO label 萌芽 (L1)", () => {
    expect(prompt).toContain(SOLO_LABELS.L1); // 萌芽
  });

  it("contains SOLO label 卓越 (L4)", () => {
    expect(prompt).toContain(SOLO_LABELS.L4); // 卓越
  });

  it("contains a Phoebe few-shot example marker", () => {
    // Either the student name or the word 示例
    expect(prompt).toMatch(/Phoebe|示例/);
  });

  it("contains strict-JSON output instruction with required keys", () => {
    expect(prompt).toContain("scores");
    expect(prompt).toContain("narrative");
    expect(prompt).toContain("dim_id");
    expect(prompt).toContain("level");
    expect(prompt).toContain("note");
  });

  it("references SOLO level values L1 and L4 in output constraint", () => {
    expect(prompt).toContain("L1");
    expect(prompt).toContain("L4");
  });

  it("includes real Phoebe content: NASA and Nature Sustainability", () => {
    expect(prompt).toContain("NASA");
    expect(prompt).toContain("Nature Sustainability");
  });
});
