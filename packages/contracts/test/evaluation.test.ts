import { describe, it, expect } from "vitest";
import { DEMO_RUBRIC, SoloLevel, SOLO_LABELS } from "../src/rubric";
import { Evaluation, EvalLlmOutput, DimScore } from "../src/evaluation";

describe("rubric", () => {
  it("DEMO_RUBRIC has the 5 demo dims, each with L1–L4 anchors", () => {
    expect(DEMO_RUBRIC.map((d) => d.id)).toEqual(["D2", "D3", "D4", "D5", "D6"]);
    for (const d of DEMO_RUBRIC) {
      expect(d.name).toBeTruthy();
      expect(d.framework).toBeTruthy();
      for (const lvl of ["L1", "L2", "L3", "L4"] as const) expect(d.anchors[lvl]).toBeTruthy();
    }
  });
  it("SoloLevel + labels", () => {
    expect(SoloLevel.safeParse("L4").success).toBe(true);
    expect(SoloLevel.safeParse("L5").success).toBe(false);
    expect(SOLO_LABELS.L1).toBe("萌芽");
  });
});

describe("evaluation contracts", () => {
  const score = { dim_id: "D2", level: "L4", note: "主动溯到 NASA / Nature Sustainability" };
  it("EvalLlmOutput accepts scores + narrative", () => {
    expect(EvalLlmOutput.safeParse({ scores: [score], narrative: "..." }).success).toBe(true);
  });
  it("DimScore rejects a bad level", () => {
    expect(DimScore.safeParse({ ...score, level: "L9" }).success).toBe(false);
  });
  it("Evaluation requires task_id + created_at", () => {
    expect(Evaluation.safeParse({ task_id: "t_1", scores: [score], narrative: "x", created_at: "2026-06-21T10:00:00.000Z" }).success).toBe(true);
    expect(Evaluation.safeParse({ scores: [score], narrative: "x" }).success).toBe(false);
  });
});
