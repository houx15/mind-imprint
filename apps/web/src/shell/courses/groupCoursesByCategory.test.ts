import { describe, it, expect } from "vitest";
import { groupCoursesByCategory } from "./groupCoursesByCategory";
import type { CourseSummary } from "@mind-imprint/contracts";

function c(slug: string, category: CourseSummary["category"]): CourseSummary {
  return { slug, branch: "A", title: slug, blurb: "", time_label: "", card_ids: [], step_count: 1, coverUrl: "", category, introduction: null, featuredRank: null, audience: [], progress: null};
}

describe("groupCoursesByCategory", () => {
  it("groups in declared category order and drops empty sections", () => {
    const groups = groupCoursesByCategory([c("a", "source-check"), c("b", "stance-value")]);
    expect(groups.map((g) => g.slug)).toEqual(["stance-value", "source-check"]); // declared order
    expect(groups[0]?.courses.map((x) => x.slug)).toEqual(["b"]);
  });

  it("puts null / unknown categories in a trailing 未分类 section", () => {
    const groups = groupCoursesByCategory([c("a", null), c("b", "source-check")]);
    const last = groups[groups.length - 1];
    expect(last?.slug).toBe("__uncategorized__");
    expect(last?.label).toBe("未分类");
    expect(last?.courses.map((x) => x.slug)).toEqual(["a"]);
  });

  it("returns no 未分类 section when every course is categorized", () => {
    const groups = groupCoursesByCategory([c("a", "data-literacy")]);
    expect(groups.some((g) => g.slug === "__uncategorized__")).toBe(false);
  });
});
