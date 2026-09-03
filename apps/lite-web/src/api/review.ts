import { apiFetch } from "./client";

// api/review.ts —— 审一遍。形状读自 apps/api/internal/api/pbl_review.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

export interface ReviewMark {
  id: string;
  /** 属于哪一部分，以及这一部分该关心什么。 */
  part: string;
  partNote: string;
  /** 被划出来的那句话。 */
  quote: string;
  question: string;
  answer: string;
  sessionId: string | null;
  ordinal: number;
  /** 她自己选中问出来的（不是印记划的）。 */
  mine: boolean;
}

export interface ReviewDimension {
  id: string;
  prompt: string;
  why: string;
  answer: string;
  ordinal: number;
}

export interface ReviewPlan {
  marks: ReviewMark[];
  dimensions: ReviewDimension[];
}

export function getReview(projectId: string, artifactId: string): Promise<ReviewPlan> {
  return apiFetch<ReviewPlan>(`${base(projectId)}/artifacts/${artifactId}/review`);
}

export function createReviewPlan(
  projectId: string,
  artifactId: string,
  body: {
    marks?: { part?: string; partNote?: string; quote?: string; question: string }[];
    dimensions?: { prompt: string; why?: string }[];
  },
): Promise<ReviewPlan> {
  return apiFetch<ReviewPlan>(`${base(projectId)}/artifacts/${artifactId}/review`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

/** 选中一段就地问：划一条、开一条会话线、接上，一次请求做完。 */
export function askAbout(
  projectId: string,
  artifactId: string,
  quote: string,
  question?: string,
): Promise<{ mark: ReviewMark; sessionId: string }> {
  return apiFetch<{ mark: ReviewMark; sessionId: string }>(
    `${base(projectId)}/artifacts/${artifactId}/ask`,
    { method: "POST", body: JSON.stringify({ quote, question: question ?? "" }) },
  );
}

export function answerMark(projectId: string, markId: string, answer: string): Promise<ReviewMark> {
  return apiFetch<ReviewMark>(`${base(projectId)}/marks/${markId}`, {
    method: "PATCH",
    body: JSON.stringify({ answer }),
  });
}

export function answerDimension(
  projectId: string,
  dimensionId: string,
  answer: string,
): Promise<ReviewDimension> {
  return apiFetch<ReviewDimension>(`${base(projectId)}/dimensions/${dimensionId}`, {
    method: "PATCH",
    body: JSON.stringify({ answer }),
  });
}

/**
 * 一段文字被划出来的地方切成几截。
 *
 * 高亮必须落在原文里，不能把原文换成"引用 + 问题"的列表——一句话离开上下文
 * 就没法判断它对不对，而判断它对不对正是审阅这件事本身。
 *
 * 重叠的划线只认最先出现的那一条：两条线叠在同一句上时，把文字切碎成两半
 * 会让两条都读不通。
 */
export function splitByMarks(
  text: string,
  marks: ReviewMark[],
): { text: string; mark: ReviewMark | null }[] {
  const hits = marks
    .filter((m) => m.quote.trim() && text.includes(m.quote))
    .map((m) => ({ mark: m, at: text.indexOf(m.quote) }))
    .sort((a, b) => a.at - b.at);

  const out: { text: string; mark: ReviewMark | null }[] = [];
  let cursor = 0;
  for (const hit of hits) {
    if (hit.at < cursor) continue; // 和前一条重叠，跳过
    if (hit.at > cursor) out.push({ text: text.slice(cursor, hit.at), mark: null });
    out.push({ text: hit.mark.quote, mark: hit.mark });
    cursor = hit.at + hit.mark.quote.length;
  }
  if (cursor < text.length) out.push({ text: text.slice(cursor), mark: null });
  return out;
}
