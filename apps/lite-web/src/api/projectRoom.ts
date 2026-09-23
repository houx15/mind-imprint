import { apiFetch } from "./client";

// api/projectRoom.ts — the 项目 workbench's client: thread, turn, sessions, plan.
// Shapes read off apps/api/internal/api/pbl_sessions.go, pbl_turn.go and
// pbl_plan.go, not guessed.

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

/* ── thread ─────────────────────────────────────────────────────────────── */

/** What an AI message may carry alongside its prose. Today only a hook; the
 *  column is jsonb, so an artifact or a tool lands here later without a
 *  migration. */
export interface HookPayload {
  kind: "hook";
  hook: string;
  hookKind: SessionKind;
}

/** Marks a conclusion that came back from a closed session. */
export interface WriteBackPayload {
  kind: "session_writeback";
  sessionId: string;
  sessionKind: SessionKind;
}

export type MessagePayload = HookPayload | WriteBackPayload;

export interface ThreadMessage {
  seq: number;
  role: "student" | "ai" | "system";
  content: string;
  payload?: MessagePayload;
  createdAt: string;
}

export function getThread(projectId: string, sessionId?: string): Promise<ThreadMessage[]> {
  const q = sessionId ? `?session=${encodeURIComponent(sessionId)}` : "";
  return apiFetch<ThreadMessage[]>(`${base(projectId)}/thread${q}`);
}

export interface TurnResult {
  reply: string;
  hook?: string;
  hookKind?: SessionKind;
  /** 这一轮印记递了一件工具。有值就去刷新工具列表。 */
  tool?: string;
  toolId?: string;
}

export function postTurn(projectId: string, text: string, sessionId?: string, completedToolId?: string, observation?: {toolId: string; revision: number}): Promise<TurnResult> {
  return apiFetch<TurnResult>(`${base(projectId)}/turn`, {
    method: "POST",
    body: JSON.stringify({ text, sessionId: sessionId ?? "", completedToolId, observation }),
  });
}

/* ── sessions ───────────────────────────────────────────────────────────── */

export const SESSION_KINDS = ["free", "observation", "reframe", "brainstorm", "plan_check", "review", "keeping"] as const;
export type SessionKind = (typeof SESSION_KINDS)[number];

export const SESSION_KIND_LABELS: Record<SessionKind, string> = {
  free: "深挖一层",
  observation: "去看看",
  reframe: "重新看这个问题",
  brainstorm: "多想几种做法",
  plan_check: "一起看计划",
  review: "成果审阅",
  keeping: "迭代讨论",
};

export interface Session {
  id: string;
  kind: SessionKind;
  parentId: string | null;
  depth: number;
  anchorKind: string;
  anchorRef: string;
  question: string;
  takeaway: string;
  writeBack?: Record<string, string>;
  closedAt: string | null;
  createdAt: string;
}

export function listSessions(projectId: string): Promise<Session[]> {
  return apiFetch<Session[]>(`${base(projectId)}/sessions`);
}

export function openSession(
  projectId: string,
  body: { kind: SessionKind; question?: string; parentId?: string; anchorKind?: string; anchorRef?: string },
): Promise<Session> {
  return apiFetch<Session>(`${base(projectId)}/sessions`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

/** Closing needs the write-back. The server refuses without it, and so does the
 *  UI — a disabled button plus a server gate, because a gate only in the button
 *  is decoration. */
export function closeSession(
  projectId: string,
  sessionId: string,
  body: { takeaway?: string; writeBack?: Record<string, string> },
): Promise<Session> {
  return apiFetch<Session>(`${base(projectId)}/sessions/${sessionId}/close`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

/** Which field a kind must produce to close. Mirrors pbl.requiredFields — the
 *  client uses it to disable the button; the server is what actually enforces. */
export const SESSION_REQUIRED_FIELD: Record<SessionKind, string | null> = {
  free: null, // the takeaway itself
  observation: "observation",
  reframe: "frame",
  brainstorm: "next_bet",
  plan_check: "resolution",
  review: null,
  keeping: "reading",
};

export const SESSION_WRITEBACK_PROMPT: Record<SessionKind, string> = {
  free: "这一层，你想明白了什么？",
  observation: "你去看了之后，看到了什么？",
  reframe: "现在这个问题，你会怎么重新说一遍？",
  brainstorm: "下一步先试哪个？",
  plan_check: "你决定怎么办？",
  review: "请记录审阅结论",
  keeping: "请记录你的判断和下一步，区分已验证的发现与待验证的想法",
};

/* ── plan ───────────────────────────────────────────────────────────────── */

export const STEP_STATUSES = [
  "settled", "tentative", "awaiting_evidence", "awaiting_decision",
  "done", "revised", "cancelled",
] as const;
export type StepStatus = (typeof STEP_STATUSES)[number];

export const STEP_STATUS_LABELS: Record<StepStatus, string> = {
  settled: "已确定",
  tentative: "暂时确定",
  awaiting_evidence: "处理中",
  awaiting_decision: "需要确定",
  done: "已完成",
  revised: "已修改",
  cancelled: "跳过",
};

/** The two waiting states are the reason this vocabulary exists: they let the
 *  plan say WHY nothing is moving, which is the thing a stuck student cannot
 *  usually put into words. They are marked so they read differently. */
export const WAITING_STATUSES: StepStatus[] = ["awaiting_evidence", "awaiting_decision"];

export interface PlanStep {
  id: string;
  ordinal: number;
  title: string;
  blurb: string;
  goal: string;
  youBring: string;
  iBring: string;
  decide: string;
  thenBring: string;
  status: StepStatus;
  progress?: "todo" | "doing" | "done";
  submission?: StepSubmission;
}

export interface StepSubmission {
  id: string;
  note: string;
  url: string;
  confirmedAt: string;
}

export interface Plan {
  versionId: string;
  version: number;
  summary: string;
  reason: string;
  approvedAt: string | null;
  steps: PlanStep[];
}

export interface PendingChange {
  id: string;
  kind: "add" | "modify" | "remove" | "defer";
  diff: Record<string, unknown>;
  evidence: string;
  resolution: string | null;
  reason: string;
  createdAt: string;
}

export interface PlanState {
  plan: Plan | null;
  pending: PendingChange[];
}

export function getPlan(projectId: string): Promise<PlanState> {
  return apiFetch<PlanState>(`${base(projectId)}/plan`);
}

export function approvePlan(projectId: string, versionId: string): Promise<{ version: number }> {
  return apiFetch<{ version: number }>(`${base(projectId)}/plan/approve`, {
    method: "POST",
    body: JSON.stringify({ versionId }),
  });
}

export function setStepStatus(projectId: string, stepId: string, status: StepStatus): Promise<PlanStep> {
  return apiFetch<PlanStep>(`${base(projectId)}/plan/steps/${stepId}`, {
    method: "PATCH",
    body: JSON.stringify({ status }),
  });
}

export function submitStepDeliverable(
  projectId: string,
  stepId: string,
  body: { note: string; url: string; confirmed: boolean },
): Promise<{ submission: StepSubmission; status: "done" }> {
  return apiFetch(`${base(projectId)}/plan/steps/${stepId}/submission`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export const PLAN_RESOLUTIONS = ["accepted", "edited", "kept", "forked", "deferred"] as const;
export type PlanResolution = (typeof PLAN_RESOLUTIONS)[number];

export const PLAN_RESOLUTION_LABELS: Record<PlanResolution, string> = {
  accepted: "确认",
  edited: "修改",
  kept: "保持现状",
  forked: "保留两者",
  deferred: "稍后再改",
};

/**
 * 界面上真正给她的两个选择。
 *
 * 产品负责人 2026-09-02：「this page has five buttons - this is ridiculous.
 * only modify, or confirm」。五个并列的按钮把一个本来很简单的判断（这版改动
 * 行不行）变成了一道选择题。
 *
 * 另外三个值仍然留在 PLAN_RESOLUTIONS 和数据库的 CHECK 里：历史数据里有它们，
 * 而且删一个枚举值要一次迁移，换不来任何东西。
 */
export const PLAN_DECISIONS: PlanResolution[] = ["accepted", "edited"];

export function resolveChange(
  projectId: string,
  changeId: string,
  resolution: PlanResolution,
  reason: string,
): Promise<PendingChange> {
  return apiFetch<PendingChange>(`${base(projectId)}/plan/changes/${changeId}/resolve`, {
    method: "POST",
    body: JSON.stringify({ resolution, reason }),
  });
}

/* ── derived ────────────────────────────────────────────────────────────── */

/**
 * The breadcrumb from the main thread down to `sessionId`.
 *
 * Returns the chain oldest-first. An unknown id, or a parent chain that does
 * not resolve, yields what it could walk rather than throwing — a broken
 * breadcrumb must not take the room down with it.
 */
export function sessionTrail(sessions: Session[], sessionId: string | null): Session[] {
  if (!sessionId) return [];
  const byId = new Map(sessions.map((s) => [s.id, s]));
  const out: Session[] = [];
  let cur = byId.get(sessionId);
  // Bounded by the server's depth cap; the guard is against a cycle that
  // should be impossible rather than one we expect.
  let guard = 0;
  while (cur && guard++ < 8) {
    out.unshift(cur);
    cur = cur.parentId ? byId.get(cur.parentId) : undefined;
  }
  return out;
}
