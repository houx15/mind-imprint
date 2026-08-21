import { describe, it, expect } from "vitest";
import type { CourseSummary } from "@mind-imprint/contracts";
import { ALL_CATEGORIES, UNCATEGORIZED, selectCourseList } from "./selectCourseList";

function c(slug: string, title: string, category: CourseSummary["category"] = "source-check"): CourseSummary {
  return {
    slug,
    branch: "A",
    title,
    blurb: "",
    time_label: "",
    card_ids: [],
    step_count: 1,
    coverUrl: "",
    category,
    introduction: null,
    featuredRank: null,
  };
}

// Catalog order as the list endpoint returns it.
const CATALOG = [
  c("a", "追踪信息背后的利益链"),
  c("b", "生成式 AI 时代的多模态信息甄别"),
  c("c", "把争议放回证据里"),
  c("d", "元认知：给自己的思维体检", "self-knowledge"),
];

describe("selectCourseList", () => {
  it("puts recently-learnt courses on top, newest first, and keeps the rest in catalog order", () => {
    const out = selectCourseList({
      courses: CATALOG,
      category: ALL_CATEGORIES,
      query: "",
      sort: "recent",
      lastLearnedBySlug: { c: "2026-08-19T10:00:00Z", b: "2026-08-21T09:00:00Z" },
    });
    expect(out.map((x) => x.slug)).toEqual(["b", "c", "a", "d"]);
  });

  it("leaves the catalog order untouched when the student has learnt nothing yet", () => {
    const out = selectCourseList({
      courses: CATALOG,
      category: ALL_CATEGORIES,
      query: "",
      sort: "recent",
      lastLearnedBySlug: {},
    });
    expect(out.map((x) => x.slug)).toEqual(["a", "b", "c", "d"]);
  });

  it("sorts by name when asked, ignoring history entirely", () => {
    const out = selectCourseList({
      courses: CATALOG,
      category: ALL_CATEGORIES,
      query: "",
      sort: "name",
      lastLearnedBySlug: { a: "2026-08-21T09:00:00Z" },
    });
    const byName = [...CATALOG].sort((x, y) => x.title.localeCompare(y.title, "zh-Hans-CN")).map((x) => x.slug);
    expect(out.map((x) => x.slug)).toEqual(byName);
  });

  it("searches the course NAME, case- and width-insensitively", () => {
    const hit = selectCourseList({
      courses: CATALOG,
      category: ALL_CATEGORIES,
      query: "  ai ",
      sort: "recent",
      lastLearnedBySlug: {},
    });
    expect(hit.map((x) => x.slug)).toEqual(["b"]);

    const wide = selectCourseList({
      courses: CATALOG,
      category: ALL_CATEGORIES,
      query: "ＡＩ",
      sort: "recent",
      lastLearnedBySlug: {},
    });
    expect(wide.map((x) => x.slug)).toEqual(["b"]);
  });

  it("returns nothing when the query matches no name (an empty state, not the whole catalog)", () => {
    const out = selectCourseList({
      courses: CATALOG,
      category: ALL_CATEGORIES,
      query: "量子力学",
      sort: "recent",
      lastLearnedBySlug: {},
    });
    expect(out).toEqual([]);
  });

  it("narrows to one category, and combines that with search + recency", () => {
    const out = selectCourseList({
      courses: CATALOG,
      category: "source-check",
      query: "",
      sort: "recent",
      lastLearnedBySlug: { c: "2026-08-19T10:00:00Z" },
    });
    expect(out.map((x) => x.slug)).toEqual(["c", "a", "b"]);
  });

  it("collects null / unknown categories under the 未分类 chip", () => {
    const courses = [c("x", "无分类课", null), c("y", "怪分类课", "not-a-real-category" as CourseSummary["category"]), c("z", "有分类课")];
    const out = selectCourseList({
      courses,
      category: UNCATEGORIZED,
      query: "",
      sort: "recent",
      lastLearnedBySlug: {},
    });
    expect(out.map((x) => x.slug)).toEqual(["x", "y"]);
  });

  it("breaks a recency tie by name so the order is total, never arbitrary", () => {
    const out = selectCourseList({
      courses: CATALOG,
      category: ALL_CATEGORIES,
      query: "",
      sort: "recent",
      lastLearnedBySlug: { a: "2026-08-21T09:00:00Z", d: "2026-08-21T09:00:00Z" },
    });
    const tied = out.slice(0, 2).map((x) => x.title);
    expect(tied).toEqual([...tied].sort((x, y) => x.localeCompare(y, "zh-Hans-CN")));
  });
});
