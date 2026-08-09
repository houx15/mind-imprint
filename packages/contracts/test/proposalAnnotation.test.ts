import { describe, it, expect } from "vitest";
import { DraftAnnotation } from "../src/index";

describe("DraftAnnotation", () => {
  it("parses a paper-level good annotation", () => {
    const a = DraftAnnotation.parse({ id: "1", level: "paper", nature: "good", quote: "", locator: "", note: "整体结构清晰" });
    expect(a.level).toBe("paper");
    expect(a.nature).toBe("good");
  });

  it("parses a sentence-level problem with a quote + locator", () => {
    const a = DraftAnnotation.parse({
      id: "2", level: "sentence", nature: "problem", quote: "中国一定会成功", locator: "第2段", note: "这是断言，缺证据",
    });
    expect(a.quote).toContain("中国");
    expect(a.locator).toBe("第2段");
  });

  it("rejects an invalid nature", () => {
    expect(() => DraftAnnotation.parse({ id: "3", level: "sentence", nature: "purple", quote: "x", locator: "", note: "n" })).toThrow();
  });
});
