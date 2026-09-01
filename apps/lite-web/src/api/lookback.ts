import { apiFetch } from "./client";

// api/lookback.ts —— 复盘，和上线之后。
// 形状读自 apps/api/internal/api/pbl_lookback.go 和 pbl_keep.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

export interface LookbackPrompt {
  id: string;
  prompt: string;
  /** 这一问是从哪件真事上长出来的。 */
  anchorKind: "reframe" | "decision" | "artifact" | "free";
  anchorRef: string;
  answer: string;
  ordinal: number;
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
  { stage: "ship", label: "放出去", hint: "东西到了真实的人手里", hue: "#3B82F6" },
  { stage: "observe", label: "看数据", hint: "发生了什么，先只记事实", hue: "#10B981" },
  { stage: "interpret", label: "读出意思", hint: "这些说明了什么", hue: "#F59E0B" },
  { stage: "change", label: "改一件事", hint: "只改一件，才知道是它起的作用", hue: "#8B5CF6" },
];

export const KEEP_KINDS: { kind: KeepKind; label: string }[] = [
  { kind: "stat", label: "数字" },
  { kind: "feedback", label: "别人说的" },
  { kind: "thought", label: "我的想法" },
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
