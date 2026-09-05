import { describe, it, expect } from "vitest";
import { selectHomeCourses } from "./selectHomeCourses";
import type { CourseSummary } from "@mind-imprint/contracts";

function c(slug: string, learnedAt: string | null = null): CourseSummary {
  return {
    slug, branch: "A", title: slug, blurb: "", time_label: "", card_ids: [], step_count: 1,
    coverUrl: "", category: null, introduction: null, featuredRank: null,
    audience: [],
    progress: learnedAt ? { status: "in-progress", completedSteps: 1, updatedAt: learnedAt } : null,
  };
}

describe("selectHomeCourses", () => {
  it("puts most-recently-learned first, then fills with the rest in catalog order", () => {
    const all = [
      c("a"),
      c("b", "2026-08-20T10:00:00Z"),
      c("c"),
      c("d", "2026-08-22T10:00:00Z"),
    ];
    const out = selectHomeCourses(all, 6).map((x) => x.slug);
    // d then b (newest-first by progress), then the unlearned rest in order.
    expect(out).toEqual(["d", "b", "a", "c"]);
  });

  it("falls back to the first `limit` catalog courses when nothing was learned", () => {
    const all = Array.from({ length: 10 }, (_, i) => c(String(i)));
    expect(selectHomeCourses(all, 6).map((x) => x.slug)).toEqual(["0", "1", "2", "3", "4", "5"]);
  });

  it("caps at the limit", () => {
    const all = Array.from({ length: 10 }, (_, i) => c(String(i)));
    expect(selectHomeCourses(all, 6)).toHaveLength(6);
  });

  it("returns all when fewer than the limit", () => {
    expect(selectHomeCourses([c("a")], 6)).toHaveLength(1);
  });
});
