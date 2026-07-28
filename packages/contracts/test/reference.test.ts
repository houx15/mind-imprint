import { describe, it, expect } from "vitest";
import { Reference, UseDecision, Credibility, ReadingNote } from "../src/reference";

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
});
