import { describe, it, expect } from "vitest";
import { Mirror, MirrorSection } from "../src/mirror";

describe("Mirror", () => {
  it("parses sections + carry-forwards", () => {
    const ok = Mirror.parse({
      sections: [{ title: "你的溯源", body: "你把话题收窄到可持续。" }],
      carryForwards: ["先找反例", "标注一手来源"],
    });
    expect(ok.sections).toHaveLength(1);
    expect(ok.carryForwards).toHaveLength(2);
  });
  it("rejects a section missing its body", () => {
    expect(() => MirrorSection.parse({ title: "x" } as any)).toThrow();
  });
});
