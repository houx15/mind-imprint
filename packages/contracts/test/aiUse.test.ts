import { describe, it, expect } from "vitest";
import { AIUseStatement, AIUseDraft } from "../src/aiUse";

describe("aiUse", () => {
  it("parses a statement", () => {
    expect(AIUseStatement.parse({ usedFor: "溯源提问", notUsedFor: "代写正文" }).usedFor).toBe("溯源提问");
  });

  it("parses a draft with an objective record (incl. the absences)", () => {
    const d = AIUseDraft.parse({
      record: {
        coachTurns: 12,
        cardsProposed: 2,
        cardsAccepted: 1,
        cardsDismissed: 1,
        sourcesOpened: 5,
        llmCallsByPurpose: { coach: 12, classify: 2 },
        ghostwroteEssay: false,
        predictedScore: false,
      },
      draft: { usedFor: "x", notUsedFor: "y" },
    });
    expect(d.record.ghostwroteEssay).toBe(false);
    expect(d.record.llmCallsByPurpose.coach).toBe(12);
  });

  it("rejects a record missing a required count", () => {
    expect(() =>
      AIUseDraft.parse({ record: { coachTurns: 1 }, draft: { usedFor: "", notUsedFor: "" } }),
    ).toThrow();
  });
});
