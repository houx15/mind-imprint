import { describe, it, expect } from "vitest";
import { CardProposal, CoachReply } from "../src/cardProposal";

describe("CardProposal / CoachReply", () => {
  it("parses a summon proposal", () => {
    const p = CardProposal.parse({
      cardId: "sift",
      reason: "横向核查这条来源",
      nudgeText: "要不要打开 SIFT？",
    });
    expect(p.cardId).toBe("sift");
  });

  it("coach reply allows absent or null proposal (respond/hint rungs)", () => {
    expect(CoachReply.parse({ reply: "你怎么看？" }).proposal ?? null).toBeNull();
    expect(CoachReply.parse({ reply: "x", proposal: null }).proposal).toBeNull();
  });

  it("coach reply carries a proposal on the summon rung", () => {
    const r = CoachReply.parse({
      reply: "你想更严格地核这条来源吗？",
      proposal: { cardId: "sift", reason: "横向核查", nudgeText: "打开 SIFT？" },
    });
    expect(r.proposal?.cardId).toBe("sift");
  });

  it("rejects a proposal missing cardId", () => {
    expect(() => CardProposal.parse({ reason: "x", nudgeText: "y" })).toThrow();
  });
});
