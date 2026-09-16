import { apiFetch } from "./client";
import { tierOrNull } from "./assignments";
import type { LibraryArticle, LibraryLevel, LibraryTag } from "./library";

// api/teacherLibrary.ts — the class-wide reading recommendation.
// Shape read from apps/api/internal/api/lite_teacher_library.go
// (libraryGroupRecommendedDTO): each article is the same shape GET /library
// gives (libraryArticleDTO) plus `why` and `readCount`.

export interface ClassRecommendedArticle extends LibraryArticle {
  /** Chinese discipline names the class's interests matched. Empty when the
   *  article was added as filler rather than for the class's interests. */
  why: string[];
  /** How many students in the class have already opened this article. */
  readCount: number;
}

export interface ClassRecommendations {
  articles: ClassRecommendedArticle[];
  /** The class's shared difficulty (1..5); null when the server sent none. */
  tier: number | null;
}

const s = (v: unknown): string => (typeof v === "string" ? v : "");
const arr = (v: unknown): unknown[] => (Array.isArray(v) ? v : []);
const obj = (v: unknown): Record<string, unknown> =>
  v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
const count = (v: unknown): number => (typeof v === "number" && Number.isInteger(v) && v > 0 ? v : 0);

function normalizeTag(raw: unknown): LibraryTag {
  const t = obj(raw);
  return { id: s(t.id), zh: s(t.zh), field: s(t.field) };
}

function normalizeLevel(raw: unknown): LibraryLevel | null {
  const l = obj(raw);
  const tier = tierOrNull(l.tier);
  if (tier === null) return null;
  return { tier, name: s(l.name), lexile: count(l.lexile), words: count(l.words), minutes: count(l.minutes) };
}

/** Articles without a slug are dropped: the picker selects by slug. */
export function normalizeClassRecommendations(raw: unknown): ClassRecommendations {
  const r = obj(raw);
  const articles = arr(r.articles)
    .map(obj)
    .filter((a) => s(a.slug) !== "")
    .map((a): ClassRecommendedArticle => ({
      slug: s(a.slug),
      title: s(a.title),
      zhTitle: s(a.zhTitle),
      reason: s(a.reason),
      field: s(a.field),
      tags: arr(a.tags).map(normalizeTag).filter((t) => t.id !== ""),
      coverUrl: s(a.coverUrl),
      levels: arr(a.levels)
        .map(normalizeLevel)
        .filter((l): l is LibraryLevel => l !== null),
      finished: a.finished === true,
      why: arr(a.why).filter((w): w is string => typeof w === "string" && w !== ""),
      readCount: count(a.readCount),
    }));
  return { articles, tier: tierOrNull(r.tier) };
}

/** GET /api/v1/lite/teacher/classes/{id}/library/recommended?limit=N */
export async function getClassRecommendations(classId: string, limit = 8): Promise<ClassRecommendations> {
  const r = await apiFetch<unknown>(
    `/api/v1/lite/teacher/classes/${encodeURIComponent(classId)}/library/recommended?limit=${limit}`,
  );
  return normalizeClassRecommendations(r);
}
