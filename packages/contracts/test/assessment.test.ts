import { describe, it, expect } from "vitest";
import { Assessment, DimensionScore } from "../src/assessment";

describe("DimensionScore", () => {
  it("parses a valid dimension row", () => {
    const parsed = DimensionScore.parse({
      code: "D2", name: "信源辨识", level: "L4", evidence: "学生溯源到 NASA 与 Nature Sustainability 原始数据。",
    });
    expect(parsed.level).toBe("L4");
  });

  it("rejects a non-SoloLevel level (a Chinese label, not the enum code)", () => {
    expect(() => DimensionScore.parse({
      code: "D2", name: "信源辨识", level: "卓越", evidence: "e",
    })).toThrow();
  });
});

describe("Assessment", () => {
  it("parses a valid payload with dimensions + narrative + generatedAt", () => {
    const parsed = Assessment.parse({
      dimensions: [
        { code: "D2", name: "信源辨识", level: "L4", evidence: "溯源到 NASA。" },
        { code: "D7", name: "让步与反驳", level: "NA", evidence: "" },
      ],
      narrative: "你在这次任务中主动溯源、并正面处理了反例。",
      generatedAt: "2026-07-13T12:34:56Z",
    });
    expect(parsed.dimensions).toHaveLength(2);
    expect(parsed.dimensions[1]!.level).toBe("NA");
  });

  it("rejects a dimension with a non-SoloLevel level nested in the array", () => {
    expect(() => Assessment.parse({
      dimensions: [{ code: "D2", name: "信源辨识", level: "卓越", evidence: "e" }],
      narrative: "n",
      generatedAt: "2026-07-13T12:34:56Z",
    })).toThrow();
  });

  it("parses the empty-state shape (no assessment generated yet)", () => {
    const parsed = Assessment.parse({ dimensions: [], narrative: "", generatedAt: "" });
    expect(parsed.dimensions).toEqual([]);
    expect(parsed.narrative).toBe("");
  });
});
