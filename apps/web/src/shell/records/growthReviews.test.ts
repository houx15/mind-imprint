import { describe, it, expect } from "vitest";
import { deriveGrowthReviews } from "./growthReviews";
import type { Evaluation } from "@mind-imprint/contracts";

function ev(created_at: string, narrative: string): Evaluation {
  return { task_id: "t1", scores: [], narrative, created_at };
}

describe("deriveGrowthReviews", () => {
  it("empty → []", () => {
    expect(deriveGrowthReviews([])).toEqual([]);
  });
  it("orders newest first and formats the period", () => {
    const out = deriveGrowthReviews([
      ev("2026-05-01T00:00:00.000Z", "早"),
      ev("2026-06-10T00:00:00.000Z", "晚"),
    ]);
    expect(out.map((r) => r.text)).toEqual(["晚", "早"]);
    expect(out[0]!.period).toBe("2026 年 6 月 10 日");
  });
});
