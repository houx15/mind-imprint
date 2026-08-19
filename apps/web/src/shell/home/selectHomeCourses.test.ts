import { describe, it, expect } from "vitest";
import { selectHomeCourses } from "./selectHomeCourses";
import type { CourseSummary } from "@mind-imprint/contracts";

function c(slug: string, featuredRank: number | null): CourseSummary {
  return { slug, branch: "A", title: slug, blurb: "", time_label: "", card_ids: [], step_count: 1, coverUrl: "", category: null, introduction: null, featuredRank };
}

describe("selectHomeCourses", () => {
  it("orders featured courses by rank ascending, then fills with the rest", () => {
    const all = [c("a", null), c("b", 2), c("c", 1), c("d", null)];
    const rng = () => 0; // deterministic: stable order for the unranked fill
    const out = selectHomeCourses(all, 6, rng);
    expect(out.slice(0, 2).map((x) => x.slug)).toEqual(["c", "b"]); // rank 1 then rank 2
    expect(out.map((x) => x.slug).sort()).toEqual(["a", "b", "c", "d"]);
  });

  it("caps at the limit", () => {
    const all = Array.from({ length: 10 }, (_, i) => c(String(i), null));
    expect(selectHomeCourses(all, 6, () => 0)).toHaveLength(6);
  });

  it("returns all when fewer than the limit", () => {
    expect(selectHomeCourses([c("a", null)], 6, () => 0)).toHaveLength(1);
  });
});
