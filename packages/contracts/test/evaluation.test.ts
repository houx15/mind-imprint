import { describe, it, expect } from "vitest";
import { FULL_RUBRIC, SoloLevel, SOLO_LABELS } from "../src/rubric";
import { Evaluation, EvalLlmOutput, DimScore } from "../src/evaluation";

describe("rubric", () => {
  it("FULL_RUBRIC has 10 dims D1..D10, each with L1–L4 anchors", () => {
    expect(FULL_RUBRIC.map((d) => d.id)).toEqual(["D1", "D2", "D3", "D4", "D5", "D6", "D7", "D8", "D9", "D10"]);
    for (const d of FULL_RUBRIC) {
      expect(d.name).toBeTruthy();
      expect(d.framework).toBeTruthy();
      for (const lvl of ["L1", "L2", "L3", "L4"] as const) expect(d.anchors[lvl]).toBeTruthy();
    }
  });
  it("SoloLevel + labels", () => {
    expect(SoloLevel.safeParse("L4").success).toBe(true);
    expect(SoloLevel.safeParse("NA").success).toBe(true);
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

describe("Evaluation contract (v2)", () => {
  it("accepts an NA level and a status field", () => {
    const parsed = Evaluation.parse({
      id: "ev1", task_id: "t1", status: "done",
      scores: [{ dim_id: "D2", level: "NA", note: "" }, { dim_id: "D7", level: "L3", note: "好" }],
      narrative: "n", created_at: "2026-06-29T00:00:00.000Z", completed_at: "2026-06-29T00:00:10.000Z",
    });
    expect(parsed.status).toBe("done");
    expect(parsed.scores[0]!.level).toBe("NA");
  });

  it("defaults status to 'done' and the new fields when absent (legacy/fixture data)", () => {
    const parsed = Evaluation.parse({
      task_id: "t1", scores: [], narrative: "n", created_at: "2026-06-29T00:00:00.000Z",
    });
    expect(parsed.status).toBe("done");
    expect(parsed.id).toBe("");
    expect(parsed.completed_at).toBeNull();
  });
});
