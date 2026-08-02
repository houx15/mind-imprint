import { describe, it, expect } from "vitest";
import { CardReflectReply, CardTurnRef } from "../src/cardReflect";

describe("CardReflectReply", () => {
  it("parses a filled-card reflect turn (id + reply)", () => {
    const r = CardReflectReply.parse({
      cardInstanceId: "11111111-1111-1111-1111-111111111111",
      reply: "你这个问题问得挺准——它可以再研究吗？",
    });
    expect(r.cardInstanceId).not.toBe("");
    expect(r.reply.length).toBeGreaterThan(0);
  });

  it("parses the empty-card no-op ({cardInstanceId:'', reply:''})", () => {
    const r = CardReflectReply.parse({ cardInstanceId: "", reply: "" });
    expect(r.cardInstanceId).toBe("");
    expect(r.reply).toBe("");
  });

  it("rejects a missing reply field", () => {
    expect(() => CardReflectReply.parse({ cardInstanceId: "x" })).toThrow();
  });

  it("parses a reflect turn that echoes the structured card reference", () => {
    const r = CardReflectReply.parse({
      cardInstanceId: "11111111-1111-1111-1111-111111111111",
      reply: "撞上反例了——让步段怎么接？",
      card: { cardId: "pee", fieldValues: { point: "中国碳排放全球第一", evidence: "IEA 2023" } },
    });
    expect(r.card?.cardId).toBe("pee");
    expect((r.card?.fieldValues as { point: string }).point).toContain("碳排放");
  });

  it("treats a card-less reply as valid (card optional/nullable)", () => {
    expect(CardReflectReply.parse({ cardInstanceId: "", reply: "" }).card ?? null).toBeNull();
    expect(CardReflectReply.parse({ cardInstanceId: "x", reply: "y", card: null }).card ?? null).toBeNull();
  });
});

describe("CardTurnRef", () => {
  it("parses a card reference with the student's raw answers", () => {
    const c = CardTurnRef.parse({ cardId: "question-card", fieldValues: { entry_direction: "中国是否让地球更可持续？" } });
    expect(c.cardId).toBe("question-card");
    expect(c.fieldValues).toHaveProperty("entry_direction");
  });

  it("rejects a card reference missing fieldValues", () => {
    expect(() => CardTurnRef.parse({ cardId: "pee" } as unknown)).toThrow();
  });
});
