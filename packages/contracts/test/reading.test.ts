import { describe, it, expect } from "vitest";
import { SelectionEval, ReadingBrief, TakeawayDraft } from "../src/reading";

describe("SelectionEval", () => {
  it("parses a program-verdict payload", () => {
    const ok = SelectionEval.parse({
      verdict: "rethink",
      verdictLabel: "暂不匹配",
      verdictReason: "对象没找对",
      checks: [
        {
          key: "target",
          label: "找对对象",
          status: "miss",
          evidence: "",
          explanation: "x",
        },
      ],
      finding: "f",
      judgment: "j",
      support: "s",
      caveat: "",
      nextStep: "n",
      spanIds: ["s0"],
    });
    expect(ok.verdict).toBe("rethink");
  });
  it("rejects an unknown verdict", () => {
    expect(() => SelectionEval.parse({ verdict: "amazing" } as any)).toThrow();
  });
});

describe("ReadingBrief", () => {
  it("accepts a set phase tag", () => {
    const b = ReadingBrief.parse({
      readingReason: "验证碳排放是否构成反例",
      readingFocus: "看它引用了谁",
      phaseTag: "反例检验",
    });
    expect(b.phaseTag).toBe("反例检验");
  });
  it("accepts the empty-string phase (not yet set)", () => {
    const b = ReadingBrief.parse({ readingReason: "", readingFocus: "", phaseTag: "" });
    expect(b.phaseTag).toBe("");
  });
  it("rejects an unknown phase tag", () => {
    expect(() =>
      ReadingBrief.parse({ readingReason: "", readingFocus: "", phaseTag: "random" }),
    ).toThrow();
  });
});

describe("TakeawayDraft", () => {
  it("matches getTakeawayDraft's response shape", () => {
    const draft = TakeawayDraft.parse({
      record: {
        findings: ["中国碳排放总量常年全球第一"],
        credibility: { verdict: "三手 · 需溯源", why: "引用链未溯源到一手数据" },
        keyQuotes: [{ quote: "中国碳排放全球第一", why: "支撑论点" }],
      },
      suggestedNewLeads: ["核实中国近五年的人均碳排放变化趋势"],
      suggestedProposalImpact: "作为让步段里承认反例的关键证据",
    });
    expect(draft.record.findings).toHaveLength(1);
    expect(draft.suggestedProposalImpact).toContain("让步段");
  });
});
