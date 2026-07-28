import { describe, it, expect } from "vitest";
import { Proposal } from "../src/proposal";

describe("Proposal", () => {
  it("parses the four kick-off dimensions", () => {
    const ok = Proposal.parse({
      objective: "探究中国是否让地球更可持续",
      reason: "与全球气候议题相关",
      activities: "溯源到 NASA / Nature Sustainability",
      resources: "两篇一手研究",
    });
    expect(ok.objective).toContain("可持续");
  });
  it("rejects a missing dimension", () => {
    expect(() => Proposal.parse({ objective: "x", reason: "y", activities: "z" } as any)).toThrow();
  });
});
