import { describe, expect, it } from "vitest";
import { flowLooksDone } from "./flowStructures";

describe("flowStructures", () => {
  it("选了整篇的结构就算想清楚了", () => {
    expect(flowLooksDone("struct_parallel")).toBe(true);
    expect(flowLooksDone("")).toBe(false);
    expect(flowLooksDone("   ")).toBe(false);
  });
});
