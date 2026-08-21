import type { CourseSummary } from "@mind-imprint/contracts";
import { COURSE_CATEGORIES } from "@mind-imprint/contracts";

/** The two orderings the 课程 list offers. `recent` is the default. */
export type CourseSort = "recent" | "name";

/** The slug the category chips use for "everything". */
export const ALL_CATEGORIES = "all";
/** The synthetic chip slug for courses with no (or an unknown) category. */
export const UNCATEGORIZED = "__uncategorized__";

export interface SelectCourseListInput {
  courses: CourseSummary[];
  /** Category chip: `ALL_CATEGORIES`, a category slug, or `UNCATEGORIZED`. */
  category: string;
  /** Free-text query matched against the course NAME (title). Trimmed here. */
  query: string;
  sort: CourseSort;
  /** slug → ISO timestamp of the student's last activity on that course. */
  lastLearnedBySlug: Record<string, string>;
}

const KNOWN_CATEGORIES = new Set<string>(COURSE_CATEGORIES.map((c) => c.slug));

/** Case- and width-insensitive so "AI" finds "ＡＩ" and "ai" alike. */
function normalize(text: string): string {
  return text.normalize("NFKC").toLowerCase();
}

function matchesCategory(course: CourseSummary, category: string): boolean {
  if (category === ALL_CATEGORIES) return true;
  if (category === UNCATEGORIZED) return course.category == null || !KNOWN_CATEGORIES.has(course.category);
  return course.category === category;
}

/**
 * The 课程 list's single ordering/filtering rule (课程 tab, 全部 + per-category
 * chips alike). Pure so the ordering is unit-testable without the network.
 *
 * `recent` puts the courses the student has actually touched at the top, newest
 * activity first, and leaves every untouched course below them in catalog order
 * (the list endpoint's own branch/title sort) — a student coming back lands on
 * what they were in the middle of, without the catalog reshuffling around it.
 * `name` ignores history entirely and sorts by title under a Chinese collation.
 *
 * Ordering is by NAME/recency only — never by popularity, streak, or anything
 * that would rank students against each other (铁律②).
 */
export function selectCourseList({
  courses,
  category,
  query,
  sort,
  lastLearnedBySlug,
}: SelectCourseListInput): CourseSummary[] {
  const needle = normalize(query.trim());
  const filtered = courses.filter(
    (c) => matchesCategory(c, category) && (needle === "" || normalize(c.title).includes(needle)),
  );

  if (sort === "name") {
    return [...filtered].sort((a, b) => a.title.localeCompare(b.title, "zh-Hans-CN"));
  }

  // `recent`: touched courses first (newest activity first), then the rest in
  // the catalog order they arrived in. Both halves are stable, so an untouched
  // course never jumps around as unrelated progress lands.
  const touched: CourseSummary[] = [];
  const untouched: CourseSummary[] = [];
  for (const c of filtered) {
    (lastLearnedBySlug[c.slug] ? touched : untouched).push(c);
  }
  touched.sort((a, b) => {
    const at = lastLearnedBySlug[a.slug]!;
    const bt = lastLearnedBySlug[b.slug]!;
    // Descending ISO-8601 string compare == descending time; equal timestamps
    // fall back to the title so the order is total, never arbitrary.
    if (at !== bt) return at < bt ? 1 : -1;
    return a.title.localeCompare(b.title, "zh-Hans-CN");
  });
  return [...touched, ...untouched];
}
