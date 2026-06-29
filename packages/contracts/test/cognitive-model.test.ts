import { describe, it, expect } from "vitest";
import { COGNITIVE_MODEL, assembleImprint } from "../src/cognitive-model";
import { FULL_RUBRIC } from "../src/rubric";
import type { Evaluation } from "../src/evaluation";

describe("COGNITIVE_MODEL invariant", () => {
  it("maps every D1..D10 exactly once (no orphan, no dup, all present)", () => {
    const dimIds = COGNITIVE_MODEL.flatMap((f) => f.categories.flatMap((c) => c.dimIds));
    const expected = FULL_RUBRIC.map((d) => d.id).sort();
    expect([...dimIds].sort()).toEqual(expected);
    expect(new Set(dimIds).size).toBe(dimIds.length);
    expect(dimIds.length).toBe(10);
  });
});

describe("assembleImprint", () => {
  const evaluation: Evaluation = {
    id: "ev1", task_id: "t1", status: "done", completed_at: null,
    created_at: "2026-06-29T00:00:00.000Z", narrative: "n",
    scores: [
      { dim_id: "D1", level: "L3", note: "清晰" },
      { dim_id: "D10", level: "L2", note: "补充" },
      { dim_id: "D4", level: "NA", note: "" },
      // D5, D7 and the entire 批判式防护 face absent → treated as NA
    ],
  };

  it("joins scores onto the tree and resolves dim names from FULL_RUBRIC", () => {
    const out = assembleImprint(evaluation);
    const driving = out.faces.find((f) => f.id === "driving")!;
    const intent = driving.categories.find((c) => c.id === "intent")!;
    expect(intent.dims.map((d) => d.dimId)).toEqual(["D1", "D10"]);
    expect(intent.dims[0]!.name).toBe("提问清晰度");
    expect(intent.dims[0]!.level).toBe("L3");
  });

  it("treats an absent dim as NA with empty note, and counts scored vs na", () => {
    const out = assembleImprint(evaluation);
    const driving = out.faces.find((f) => f.id === "driving")!;
    const reasoning = driving.categories.find((c) => c.id === "reasoning")!;
    const d5 = reasoning.dims.find((d) => d.dimId === "D5")!;
    expect(d5.level).toBe("NA");
    expect(d5.note).toBe("");
    // intent: D1 L3 (scored), D10 L2 (scored) → scored 2, na 0
    const intent = driving.categories.find((c) => c.id === "intent")!;
    expect(intent.scored).toBe(2);
    expect(intent.na).toBe(0);
    // reasoning: D4 NA, D5 absent→NA, D7 absent→NA → scored 0, na 3
    expect(reasoning.scored).toBe(0);
    expect(reasoning.na).toBe(3);
    // face rollup sums its categories
    expect(driving.scored).toBe(2);
    expect(driving.na).toBe(3);
  });
});
