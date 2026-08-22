import type { CourseSummary } from "@mind-imprint/contracts";

// selectHomeCourses picks the up-to-`limit` courses the 首页 最近课程 strip shows:
// the courses the student most recently learned come first, newest-first by
// their own `progress.updatedAt`; any remaining slots — or the whole strip when
// nothing has been learned yet — are filled from the rest of the catalog in its
// given order. Deterministic (no shuffle) so the cards don't jump between
// renders, and 铁律②-safe: the only order is the student's own recency, never a
// popularity ranking.
export function selectHomeCourses(courses: CourseSummary[], limit: number): CourseSummary[] {
  const learned = courses
    .filter((c) => c.progress != null)
    // RFC3339 timestamps sort correctly lexicographically; y-vs-x = descending.
    .sort((x, y) => y.progress!.updatedAt.localeCompare(x.progress!.updatedAt));
  const learnedSlugs = new Set(learned.map((c) => c.slug));
  const rest = courses.filter((c) => !learnedSlugs.has(c.slug));
  return [...learned, ...rest].slice(0, limit);
}
