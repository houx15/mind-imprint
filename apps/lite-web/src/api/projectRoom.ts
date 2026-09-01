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

export function postTurn(projectId: string, text: string, sessionId?: string): Promise<TurnResult> {
  return apiFetch<TurnResult>(`${base(projectId)}/turn`, {
    method: "POST",
    body: JSON.stringify({ text, sessionId: sessionId ?? "" }),
  });
}

/* ── sessions ───────────────────────────────────────────────────────────── */

export const SESSION_KINDS = ["free", "observation", "reframe", "brainstorm", "plan_check"] as const;
export type SessionKind = (typeof SESSION_KINDS)[number];

export const SESSION_KIND_LABELS: Record<SessionKind, string> = {
  free: "深挖一层",
  observation: "去看看",
  reframe: "重新看这个问题",
  brainstorm: "多想几种做法",
  plan_check: "一起看计划",
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
};

export const SESSION_WRITEBACK_PROMPT: Record<SessionKind, string> = {
  free: "这一层，你想明白了什么？",
  observation: "你真的看见了什么？",
  reframe: "现在你觉得这个问题该怎么问？",
  brainstorm: "下一步先试哪个？",
  plan_check: "你决定怎么办？",
};

/* ── plan ───────────────────────────────────────────────────────────────── */

export const STEP_STATUSES = [
  "settled", "tentative", "awaiting_evidence", "awaiting_decision",
  "done", "revised", "cancelled",
] as const;
export type StepStatus = (typeof STEP_STATUSES)[number];

export const STEP_STATUS_LABELS: Record<StepStatus, string> = {
  settled: "定了",
  tentative: "暂定",
  awaiting_evidence: "等结果",
  awaiting_decision: "等你定",
  done: "做完了",
  revised: "改过了",
  cancelled: "不做了",
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

export const PLAN_RESOLUTIONS = ["accepted", "edited", "kept", "forked", "deferred"] as const;
export type PlanResolution = (typeof PLAN_RESOLUTIONS)[number];

/** 「保留原计划」 sits among the others as an equal, not as a decline. */
export const PLAN_RESOLUTION_LABELS: Record<PlanResolution, string> = {
  accepted: "就这么改",
  edited: "改一下再用",
  kept: "还是按原来的",
  forked: "两条都试试",
  deferred: "先补证据再说",
};

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
