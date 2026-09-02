import { apiFetch } from "./client";

// api/lookback.ts —— 复盘，和上线之后。
// 形状读自 apps/api/internal/api/pbl_lookback.go 和 pbl_keep.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

export type ReviewSection = "what" | "how" | "moment" | "praise" | "improve" | "with_ai";

/**
 * 复盘的六段（产品负责人 2026-09-02）。段是固定的，段里的问题由印记按这个项目
 * 真发生过的事现写——见 apps/api/internal/pbl/lookback.go。
 */
export const REVIEW_SECTIONS: { key: ReviewSection; title: string }[] = [
  { key: "what", title: "做了什么" },
  { key: "how", title: "感受如何" },
  { key: "moment", title: "印象最深的一件事" },
  { key: "praise", title: "值得肯定的地方" },
  { key: "improve", title: "还能更好的地方" },
  { key: "with_ai", title: "和 AI 的协作" },
];

export interface LookbackPrompt {
  id: string;
  section: ReviewSection;
  prompt: string;
  answer: string;
  ordinal: number;
}

/** 按六段分组，空段不出现。 */
export function bySection(
  prompts: LookbackPrompt[],
): { key: ReviewSection; title: string; prompts: LookbackPrompt[] }[] {
  return REVIEW_SECTIONS.map((s) => ({
    ...s,
    prompts: prompts.filter((p) => p.section === s.key),
  })).filter((g) => g.prompts.length > 0);
}

export function getLookback(projectId: string): Promise<LookbackPrompt[]> {
  return apiFetch<LookbackPrompt[]>(`${base(projectId)}/lookback`);
}

export function answerLookback(
  projectId: string,
  promptId: string,
  answer: string,
): Promise<LookbackPrompt> {
  return apiFetch<LookbackPrompt>(`${base(projectId)}/lookback/${promptId}`, {
    method: "PATCH",
    body: JSON.stringify({ answer }),
  });
}

export function lookbackTodo(prompts: LookbackPrompt[]): string {
  const left = prompts.filter((p) => !p.answer.trim()).length;
  return left ? `还有 ${left} 问没答` : "";
}

/* ── 上线之后 ───────────────────────────────────────────────────────────── */

export type KeepKind = "stat" | "feedback" | "thought";
export type KeepStage = "ship" | "observe" | "interpret" | "change";

export interface KeepEntry {
  id: string;
  kind: KeepKind;
  body: string;
  stage: KeepStage;
  /** 这条数据长出来的那一轮思考。 */
  sessionId: string | null;
  createdAt: string;
}

/** 循环里的四步。颜色是为了让她一眼看出自己停在哪一步。 */
export const KEEP_STAGES: { stage: KeepStage; label: string; hint: string; hue: string }[] = [
  { stage: "ship", label: "发布上线", hint: "让真实用户开始使用你的成果", hue: "#3B82F6" },
  { stage: "observe", label: "收集数据", hint: "整理访谈、问卷、后台数据，拿到真实反馈", hue: "#10B981" },
  { stage: "interpret", label: "数据分析", hint: "从这些数据里提取对成果有用的洞察", hue: "#F59E0B" },
  { stage: "change", label: "产品迭代", hint: "形成迭代方案，并把它做出来", hue: "#8B5CF6" },
];

export const KEEP_KINDS: { kind: KeepKind; label: string; placeholder: string }[] = [
  { kind: "stat", label: "产品数据", placeholder: "这周的一个数字，以及它是从哪看到的" },
  { kind: "feedback", label: "访谈 / 调查结果", placeholder: "针对谁的访谈，ta 表达了什么？" },
  { kind: "thought", label: "我的灵感", placeholder: "你的灵感是什么，为什么？" },
];

/**
 * 常见的几个产品数据，点开看它是什么。
 *
 * 产品负责人 2026-09-02：「停留时长、点击率、留存率(with each can click to see
 * what are they)」。直接甩三个词给一个中学生，等于没说——她多半会跳过这一档，
 * 而这一档恰恰是"用数据说话"这件事的入口。
 */
export const KEEP_METRICS: { name: string; what: string }[] = [
  { name: "停留时长", what: "一个人打开之后待了多久。太短通常说明他没找到想看的东西。" },
  { name: "点击率", what: "看到入口的人里，有多少真的点进来了。" },
  { name: "留存率", what: "上次来过的人，这次还回来的比例。" },
];

export function listKeepEntries(projectId: string): Promise<KeepEntry[]> {
  return apiFetch<KeepEntry[]>(`${base(projectId)}/keep`);
}

export function addKeepEntry(
  projectId: string,
  body: { kind: KeepKind; body: string; stage: KeepStage },
): Promise<KeepEntry> {
  return apiFetch<KeepEntry>(`${base(projectId)}/keep`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

/** 让这一条长出一轮新的思考。已经有的话，回到原来那一轮。 */
export function openKeepSession(
  projectId: string,
  entryId: string,
): Promise<{ sessionId: string }> {
  return apiFetch<{ sessionId: string }>(`${base(projectId)}/keep/${entryId}/session`, {
    method: "POST",
  });
}

/**
 * 她走到循环的哪一步了。
 *
 * 「改一件事」之后回到「放出去」——这是一个圈，不是一条队列。停在"看数据"上
 * 三个月和还没上线是两种完全不同的状态，而这行字是唯一说得出区别的地方。
 */
export function keepStage(entries: KeepEntry[]): KeepStage {
  if (entries.length === 0) return "ship";
  const latest = entries[0]!; // 服务端按时间倒序
  if (latest.stage === "change") return "ship";
  return latest.stage;
}
