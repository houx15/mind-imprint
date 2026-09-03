import { apiFetch } from "./client";

/**
 * api/interestQuiz — 觉醒协议（兴趣测试）。
 *
 * 形状逐字段抄自 `apps/api/internal/api/interest_quiz.go`，对着那份 Go 源码核过。
 * 和 `api/interest.ts` 一样，每个读取都带 `?? []` / `?? ""`：少一个字段绝不让
 * 一次渲染崩掉。
 *
 * 三个调用对应三件事：
 *
 *   fetchQuizStatus  树上那条邀请要不要出现（她做完过没有）
 *   startQuiz        开一次作答，拿到 id —— **在第一屏就调**，这样一次中途
 *                    退出也留下痕迹（过程即数据）
 *   finishQuiz       交卷。**这一个会慢**：服务端同步跑一次采集调用，因为结果
 *                    页要显示的就是那几个词。界面必须为此显示「正在生成」。
 */

export type QuizHook = "character" | "craft" | "society";

export interface QuizAttempt {
  id: string;
  navigator: string;
  anchorWork: string;
  anchorReason: string;
  hook: string;
  challengeChoice: string;
  challengeAttempts: number;
  finishedAt: string;
}

export interface QuizStatus {
  /** 只在她**做完过**至少一次时为真。中途退出不算。 */
  taken: boolean;
  latest: QuizAttempt | null;
}

export interface QuizSyllabusRef {
  board: string;
  code: string;
  label: string;
  level?: string;
}

/** 结果页上的一片学科透镜 —— 真学科，带考纲投影。 */
export interface QuizLens {
  id: string;
  zh: string;
  en: string;
  field: string;
  asks: string;
  method: string;
  exemplar: string;
  syllabus: QuizSyllabusRef[];
}

export interface QuizPlanted {
  textZh: string;
  textEn: string;
  field: string;
  note: string;
  evidence: string;
}

export interface QuizResult {
  attempt: QuizAttempt;
  lenses: QuizLens[];
  /** 这次真的种到树上的词。**可能是空的**，结果页必须照实说。 */
  keywords: QuizPlanted[];
  /**
   * 采集到底跑没跑。
   *
   * `false` 表示她写得太短，我们**根本没发那次调用** —— 这和「跑了但一个词都
   * 没长出来」是两回事，结果页对这两种情况说的话也不一样。
   */
  harvested: boolean;
}

export interface FinishQuizInput {
  navigator: string;
  anchorWork: string;
  anchorReason: string;
  hook: QuizHook | "";
  challengeChoice: string;
  challengeAttempts: number;
}

function normalizeAttempt(raw: Partial<QuizAttempt> | null | undefined): QuizAttempt {
  return {
    id: raw?.id ?? "",
    navigator: raw?.navigator ?? "",
    anchorWork: raw?.anchorWork ?? "",
    anchorReason: raw?.anchorReason ?? "",
    hook: raw?.hook ?? "",
    challengeChoice: raw?.challengeChoice ?? "",
    challengeAttempts: raw?.challengeAttempts ?? 0,
    finishedAt: raw?.finishedAt ?? "",
  };
}

export async function fetchQuizStatus(): Promise<QuizStatus> {
  const raw = await apiFetch<Partial<QuizStatus>>("/api/v1/interest/quiz");
  return {
    taken: raw.taken ?? false,
    latest: raw.latest ? normalizeAttempt(raw.latest) : null,
  };
}

export async function startQuiz(): Promise<QuizAttempt> {
  return normalizeAttempt(await apiFetch<Partial<QuizAttempt>>("/api/v1/interest/quiz", {
    method: "POST",
  }));
}

/** ⏳ 会慢：服务端同步跑一次采集调用。调用处必须显示「正在生成」。 */
export async function finishQuiz(id: string, input: FinishQuizInput): Promise<QuizResult> {
  const raw = await apiFetch<Partial<QuizResult>>(`/api/v1/interest/quiz/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
  return {
    attempt: normalizeAttempt(raw.attempt),
    lenses: (raw.lenses ?? []).map((l) => ({
      id: l.id ?? "",
      zh: l.zh ?? "",
      en: l.en ?? "",
      field: l.field ?? "",
      asks: l.asks ?? "",
      method: l.method ?? "",
      exemplar: l.exemplar ?? "",
      syllabus: l.syllabus ?? [],
    })),
    keywords: (raw.keywords ?? []) as QuizPlanted[],
    harvested: raw.harvested ?? false,
  };
}
