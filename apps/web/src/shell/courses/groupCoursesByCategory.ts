import type { CourseSummary } from "@mind-imprint/contracts";
import { COURSE_CATEGORIES } from "@mind-imprint/contracts";

export interface CourseGroup {
  slug: string;
  label: string;
  courses: CourseSummary[];
}

// groupCoursesByCategory buckets the catalog into the 7 categories (in their
// declared order), dropping any empty bucket, and appends a trailing 未分类
// section for courses whose category is null or not one of the 7 (the
// pre-backfill state). Order within a bucket is the input order (the list
// endpoint already sorts by branch/title).
export function groupCoursesByCategory(courses: CourseSummary[]): CourseGroup[] {
  const groups: CourseGroup[] = [];
  const known = new Set<string>(COURSE_CATEGORIES.map((c) => c.slug));
  for (const { slug, label } of COURSE_CATEGORIES) {
    const inCat = courses.filter((c) => c.category === slug);
    if (inCat.length > 0) groups.push({ slug, label, courses: inCat });
  }
  const uncategorized = courses.filter((c) => c.category == null || !known.has(c.category));
  if (uncategorized.length > 0) {
    groups.push({ slug: "__uncategorized__", label: "未分类", courses: uncategorized });
  }
  return groups;
}
