import { describe, expect, it } from "vitest";
import { flowCanCarryMethod, flowLooksDone } from "./flowStructures";

describe("flowStructures", () => {
  it("只有正文那一层能标论证方法", () => {
    expect(flowCanCarryMethod("point")).toBe(true);
    expect(flowCanCarryMethod("counter")).toBe(true);
  });

  it("🚨 开篇 / 结尾 / 论据都不能标 —— 开法收法由 kind 定，论据不是一段", () => {
    for (const k of ["opening", "closing", "thesis", "evidence", "reference", "reasoning", "gap", "rebuttal"]) {
      expect(flowCanCarryMethod(k), `${k} 不该能标方法`).toBe(false);
    }
  });

  it("选了整篇的结构就算想清楚了，方法不强求", () => {
    // 她可以只想清楚「这几条是并列的」就去写，方法在写的时候再定也来得及。
    expect(flowLooksDone("struct_parallel")).toBe(true);
    expect(flowLooksDone("")).toBe(false);
    expect(flowLooksDone("   ")).toBe(false);
  });
});
