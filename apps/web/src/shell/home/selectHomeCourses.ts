import type { CourseSummary } from "@mind-imprint/contracts";

// selectHomeCourses picks the up-to-`limit` courses the home page shows.
// Featured courses (featuredRank != null) come first, ascending by rank; the
// remaining slots are filled from the rest in random order. When NOTHING is
// featured (the pre-curation state), this degrades to "random `limit`" — so
// "random for now, curated later" is one code path, flipped by data alone.
// `rng` is injectable for deterministic tests; defaults to Math.random.
export function selectHomeCourses(
  courses: CourseSummary[],
  limit: number,
  rng: () => number = Math.random,
): CourseSummary[] {
  const shuffle = (xs: CourseSummary[]): CourseSummary[] => {
    const a = [...xs];
    for (let i = a.length - 1; i > 0; i--) {
      const j = Math.floor(rng() * (i + 1));
      const tmp = a[i]!;
      a[i] = a[j]!;
      a[j] = tmp;
    }
    return a;
  };
  const ranked = courses
    .filter((c) => c.featuredRank != null)
    .sort((x, y) => (x.featuredRank as number) - (y.featuredRank as number));
  const rest = shuffle(courses.filter((c) => c.featuredRank == null));
  return [...ranked, ...rest].slice(0, limit);
}
