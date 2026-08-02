import { describe, it, expect } from "vitest";
import { CardReflectReply } from "../src/cardReflect";

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
});
