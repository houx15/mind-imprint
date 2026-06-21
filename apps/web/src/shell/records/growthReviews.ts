import type { Evaluation } from "@mind-imprint/contracts";

export interface GrowthReview {
  period: string;
  text: string;
}

function fmt(iso: string): string {
  const d = new Date(iso);
  return `${d.getUTCFullYear()} 年 ${d.getUTCMonth() + 1} 月 ${d.getUTCDate()} 日`;
}

export function deriveGrowthReviews(
  evaluations: Evaluation[]
): GrowthReview[] {
  return [...evaluations]
    .sort((a, b) => (a.created_at < b.created_at ? 1 : -1))
    .map((e) => ({ period: fmt(e.created_at), text: e.narrative }));
}
