import { apiFetch } from "./client";

// api/classSummary.ts — the one-sentence summary on each class card (§12.5).
// Shape read from apps/api/internal/api/lite_class_summary.go
// (liteClassSummaryDTO). POST because it calls a model; the server caches it
// per class per Beijing day. A failed check is a 502 whose message already
// starts with 「摘要生成失败：」, surfaced by `apiFetch` as an ApiError.

export interface ClassSummaryText {
  summary: string;
  generatedAt: string;
  cached: boolean;
}

export async function postClassSummary(classId: string): Promise<ClassSummaryText> {
  const raw = await apiFetch<Partial<ClassSummaryText>>(
    `/api/v1/lite/teacher/classes/${encodeURIComponent(classId)}/summary`,
    { method: "POST" },
  );
  return { summary: raw.summary ?? "", generatedAt: raw.generatedAt ?? "", cached: raw.cached === true };
}
