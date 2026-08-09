import { describe, it, expect } from "vitest";
import { assembleGuidedDoc, partSectionKey } from "@/workspace/blocks/docSections";
import type { StepRef } from "@mind-imprint/contracts";

const steps: StepRef[] = [
  { key: "understanding", title: "对题目的理解", kind: "fixed" },
  { key: "question-scope", title: "研究问题与范围", kind: "fixed" },
  { key: "thesis", title: "暂定论点", kind: "fixed" },
];

describe("docSections", () => {
  it("partSectionKey prefixes the step key", () => {
    expect(partSectionKey("thesis")).toBe("prop:thesis");
  });

  it("assembles written parts in step order, skipping empties, as ## sections", () => {
    const doc = assembleGuidedDoc(steps, { understanding: "我的理解……", thesis: "我的论点……", "question-scope": "  " });
    expect(doc).toBe("## 对题目的理解\n我的理解……\n\n## 暂定论点\n我的论点……");
    // the empty question-scope is skipped; order follows steps not the object.
    expect(doc).not.toContain("研究问题与范围");
  });

  it("empty when nothing written", () => {
    expect(assembleGuidedDoc(steps, {})).toBe("");
  });
});
