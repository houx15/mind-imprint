import { describe, it, expect } from "vitest";
import { EssayStage, SubQuestionVerdict, EvidenceMap, Reference } from "../src/index";

describe("essay track contracts", () => {
  it("EssayStage accepts the three stages", () => {
    expect(EssayStage.parse("research")).toBe("research");
    expect(() => EssayStage.parse("nope")).toThrow();
  });

  it("SubQuestionVerdict parses", () => {
    const v = SubQuestionVerdict.parse({ subQuestionId: "a", saturated: false, why: "只有支持材料", gaps: ["缺一个反例"] });
    expect(v.saturated).toBe(false);
    expect(v.gaps).toHaveLength(1);
  });

  it("EvidenceMap parses a seeded map", () => {
    const m = EvidenceMap.parse({
      mainQuestion: "中国是否让地球更可持续",
      subQuestions: [
        { id: "sq1", text: "新能源投资的净效应", papers: [{ id: "p1", title: "A", nature: "support", triage: "red", hasNote: true }] },
      ],
    });
    expect(m.subQuestions[0]!.papers[0]!.nature).toBe("support");
  });
});

describe("Reference evidence fields (slice 4a)", () => {
  const base = {
    id: "r1", title: "t", classification: "c", author: "", credentials: "", year: "", url: "",
    tags: [], collectionId: null, credibility: null, evaluation: "", decision: null, pending: false,
    searchHints: [], materialId: null, notes: [],
  };
  it("parses without the new fields (back-compat; optional → undefined)", () => {
    const r = Reference.parse(base);
    expect(r.triage).toBeUndefined();
    expect(r.evidenceNature).toBeUndefined();
    expect(r.archived).toBeUndefined();
  });
  it("parses with evidence fields set", () => {
    const r = Reference.parse({ ...base, triage: "red", evidenceNature: "challenge", evidenceArgument: "a", evidenceFinding: "f", evidencePlacement: "p", archived: true });
    expect(r.triage).toBe("red");
    expect(r.evidenceNature).toBe("challenge");
    expect(r.archived).toBe(true);
  });
});
