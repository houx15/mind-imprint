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

/**
 * 她自己标出来的那一处，question 固定是这一句。
 *
 * 🚨 必须和服务端的 `pblSpotQuestion` 一字不差（apps/api/internal/api/
 * pbl_review.go）——界面靠它把「她找的」和「印记划的」分开。
 */
export const SPOT_QUESTION = "你标出来的地方";

/**
 * 「找茬」：她自己划出一处觉得有问题的地方，写一句哪里不对。
 *
 * 🚨 和 askAbout 的区别是**不开会话线**。这一轮她在自己判断，不是在问印记——
 * 每划一处就把印记拉进来，她会顺着印记的话走，而这件事整个的意思恰恰是让她
 * 先于印记看出问题（铁律①）。想问，旁边一直有「问问这一句」。
 */
export function spotProblem(
  projectId: string,
  artifactId: string,
  quote: string,
  why: string,
): Promise<ReviewMark> {
  return apiFetch<ReviewMark>(`${base(projectId)}/artifacts/${artifactId}/spot`, {
    method: "POST",
    body: JSON.stringify({ quote, why }),
  });
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

/**
 * 一段一段地进去看，而不是摊开一整篇。
 *
 * 🚨 产品负责人 2026-09-03：「we must go into texts, instead of presenting a
 * large text」。一整篇铺在那里，她能做的只有从头划到尾——那是"读过了"，不是
 * "审过了"。
 *
 * docs/2026-09-01-pbl-detail.md 要的是「explanations for each part so that we
 * know what we should care about in each part」。marks 上的 part / partNote
 * 两个字段就是为这件事留的：印记交东西时顺手说清楚每一部分该看什么。字段一直
 * 在库里、在 DTO 里，界面一次也没用过。
 *
 * 分段的依据是划线在原文里出现的位置：一条划线带了新的 part 名，就从它那一段
 * 开始算新的一部分。印记没给 part 的时候退回一整段，界面照常能用。
 */
export interface ReviewPart {
  /** 这一部分叫什么。空 = 印记没分段。 */
  name: string;
  /** 这一部分要留意什么。 */
  note: string;
  /** 属于这一部分的自然段。 */
  paragraphs: string[];
  /** 落在这一部分里的划线。 */
  marks: ReviewMark[];
}

export function splitIntoParts(paragraphs: string[], marks: ReviewMark[]): ReviewPart[] {
  // 每一段里第一条划线（按它在这一段里的位置）决定这一段属于谁。
  const partOfParagraph = paragraphs.map((text) => {
    const hits = marks
      .filter((m) => m.quote.trim() && text.includes(m.quote))
      .map((m) => ({ m, at: text.indexOf(m.quote) }))
      .sort((a, b) => a.at - b.at);
    return hits[0]?.m ?? null;
  });

  const parts: ReviewPart[] = [];
  let current: ReviewPart | null = null;
  paragraphs.forEach((text, i) => {
    const lead = partOfParagraph[i];
    const name = lead?.part.trim() ?? "";
    // 新的一部分：这一段的划线带了一个和当前不同的 part 名。
    if (current === null || (name !== "" && name !== current.name)) {
      current = { name, note: lead?.partNote.trim() ?? "", paragraphs: [], marks: [] };
      parts.push(current);
    }
    current.paragraphs.push(text);
  });

  // 划线归到它所在的那一部分。找不到出处的（原文里没有这句）归到第一部分，
  // 免得整条问题消失——她仍然该看见印记问了什么。
  for (const m of marks) {
    const owner =
      parts.find((p) => p.paragraphs.some((t) => m.quote.trim() && t.includes(m.quote))) ??
      parts[0];
    owner?.marks.push(m);
  }
  return parts;
}

/** 这一部分答完了几条。 */
export function partProgress(part: ReviewPart): { done: number; total: number } {
  return {
    done: part.marks.filter((m) => m.answer.trim() !== "").length,
    total: part.marks.length,
  };
}
