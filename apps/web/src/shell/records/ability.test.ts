import { describe, it, expect } from "vitest";
import { deriveAbility } from "./ability";
import { FULL_RUBRIC } from "@mind-imprint/contracts";
import type { Evaluation } from "@mind-imprint/contracts";

function ev(created_at: string, scores: { dim_id: string; level: "L1"|"L2"|"L3"|"L4" }[]): Evaluation {
  return { task_id: "t1", created_at, narrative: "",
    scores: scores.map((s) => ({ ...s, note: "" })) };
}

describe("deriveAbility", () => {
  it("returns one entry per rubric dim in rubric order", () => {
    const out = deriveAbility([], FULL_RUBRIC);
    expect(out.map((a) => a.dimId)).toEqual(FULL_RUBRIC.map((d) => d.id));
  });
  it("unscored dims default to L1", () => {
    const out = deriveAbility([], FULL_RUBRIC);
    expect(out.every((a) => a.level === "L1")).toBe(true);
  });
  it("uses the latest evaluation's score per dim", () => {
    const out = deriveAbility([
      ev("2026-05-01T00:00:00Z", [{ dim_id: "D2", level: "L2" }]),
      ev("2026-06-01T00:00:00Z", [{ dim_id: "D2", level: "L4" }]),
    ], FULL_RUBRIC);
    const d2 = out.find((a) => a.dimId === "D2")!;
    expect(d2.level).toBe("L4");
    expect(d2.levelLabel).toContain("卓越");
    expect(d2.segs.filter((s) => s.style.includes("#2A3B7A"))).toHaveLength(4);
  });
});
