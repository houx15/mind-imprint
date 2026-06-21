import { describe, it, expect } from "vitest";
import { SummonCardCall } from "../src/summonCard";

const valid = {
  id: "call_1", name: "summon_card",
  args: { card_id: "sift_craap", reason: "学生要直接采信一个来源", nudge_text: "先核一下这个来源？" },
  card_instance_id: "ci_1",
};

describe("SummonCardCall", () => {
  it("accepts a valid summon_card call", () => {
    expect(SummonCardCall.safeParse(valid).success).toBe(true);
  });
  it("rejects a wrong tool name", () => {
    expect(SummonCardCall.safeParse({ ...valid, name: "other" }).success).toBe(false);
  });
  it("rejects when args are incomplete", () => {
    expect(SummonCardCall.safeParse({ ...valid, args: { card_id: "x" } }).success).toBe(false);
  });
  it("rejects a missing card_instance_id", () => {
    const { card_instance_id, ...rest } = valid;
    expect(SummonCardCall.safeParse(rest).success).toBe(false);
  });
});
