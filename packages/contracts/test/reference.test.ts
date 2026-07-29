import { describe, it, expect } from "vitest";
import { Reference, UseDecision, Credibility, ReadingNote, PhaseTag, ReadingTakeaway } from "../src/reference";

const ref = {
  id: "r1",
  title: "China's carbon trajectory",
  classification: "期刊论文",
  author: "Nature Sustainability",
  credentials: "同行评议",
  year: "2024",
  url: "https://www.nature.com/",
  tags: ["气候", "一手"],
  collectionId: null,
  credibility: "strong" as const,
  evaluation: "权威且新近",
  decision: "use" as const,
  pending: false,
  searchHints: ["carbon emissions china"],
  materialId: null,
  notes: [{ quote: "全球第一排放国", finding: "需要让步段" }],
};

describe("Reference", () => {
  it("parses a full row", () => {
    expect(Reference.parse(ref).decision).toBe("use");
  });
  it("accepts nullable credibility / decision", () => {
    const parsed = Reference.parse({ ...ref, credibility: null, decision: null });
    expect(parsed.credibility).toBeNull();
    expect(parsed.decision).toBeNull();
  });
  it("rejects an unknown decision", () => {
    expect(() => Reference.parse({ ...ref, decision: "keep" })).toThrow();
  });
  it("UseDecision / Credibility / ReadingNote validate directly", () => {
    expect(UseDecision.parse(null)).toBeNull();
    expect(Credibility.parse("mixed")).toBe("mixed");
    expect(ReadingNote.parse({ quote: "q", finding: "f" }).finding).toBe("f");
  });
  it("accepts the reference WITHOUT the optional phaseTag/takeaway", () => {
    const parsed = Reference.parse(ref);
    expect(parsed.phaseTag).toBeUndefined();
    expect(parsed.takeaway).toBeUndefined();
  });
  it("accepts the reference WITHOUT the optional readingReason/readingFocus", () => {
    const parsed = Reference.parse(ref);
    expect(parsed.readingReason).toBeUndefined();
    expect(parsed.readingFocus).toBeUndefined();
  });
  it("carries the persisted brief's readingReason/readingFocus (Task 9 fix)", () => {
    const withBrief = Reference.parse({
      ...ref,
      phaseTag: "反例检验",
      readingReason: "验证碳排放反例",
      readingFocus: "看引用来源是否可信",
    });
    expect(withBrief.readingReason).toBe("验证碳排放反例");
    expect(withBrief.readingFocus).toBe("看引用来源是否可信");
  });
  it("accepts a null readingReason/readingFocus explicitly", () => {
    const parsed = Reference.parse({ ...ref, readingReason: null, readingFocus: null });
    expect(parsed.readingReason).toBeNull();
    expect(parsed.readingFocus).toBeNull();
  });
  it("carries an optional structured takeaway + phaseTag", () => {
    const withTakeaway = Reference.parse({
      ...ref,
      phaseTag: "反例检验",
      takeaway: {
        findings: ["中国碳排放总量全球第一"],
        credibility: { verdict: "strong", why: "NASA 一手数据" },
        keyQuotes: [{ quote: "China emits the most", why: "直接反例" }],
        newLeads: ["核实人均口径"],
        proposalImpact: "作为让步段的反例证据",
      },
    });
    expect(withTakeaway.phaseTag).toBe("反例检验");
    expect(withTakeaway.takeaway?.proposalImpact).toBe("作为让步段的反例证据");
  });
  it("accepts a null phaseTag/takeaway explicitly", () => {
    const parsed = Reference.parse({ ...ref, phaseTag: null, takeaway: null });
    expect(parsed.phaseTag).toBeNull();
    expect(parsed.takeaway).toBeNull();
  });
});

describe("PhaseTag", () => {
  it("accepts the enum", () => {
    expect(PhaseTag.parse("反例检验")).toBe("反例检验");
    expect(PhaseTag.parse("立题探索")).toBe("立题探索");
  });
  it("rejects an unknown phase", () => {
    expect(() => PhaseTag.parse("random")).toThrow();
  });
});

describe("ReadingTakeaway", () => {
  it("round-trips the full 5-field object", () => {
    const raw = {
      findings: ["f1", "f2"],
      credibility: { verdict: "strong", why: "w" },
      keyQuotes: [{ quote: "q", why: "w" }],
      newLeads: ["l1"],
      proposalImpact: "impact",
    };
    expect(ReadingTakeaway.parse(raw)).toEqual(raw);
  });
});
