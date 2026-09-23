// api/gradings.ts — AI 批改 clients and normalizers. Shapes follow the real
// backend (Tasks 5-7: apps/api/internal/api/lite_teacher_gradings.go,
// lite_student_gradings.go), not the Task 9 brief's pre-backend guesses.
// See task-9-report.md for every place the wire shape differed from the
// brief; the notable ones:
//   - POST .../assignments/{aid}/gradings (一键AI批改) returns
//     {queued, failed, error}, not a bare count; a total failure is a 503
//     `grading_enqueue_failed` (thrown as ApiError), not a 200.
//   - POST .../assignments/{aid}/gradings/send (发送全部已审阅) returns
//     {sent, skipped}, not a bare count.
//   - No frontend copy of liteassign.DefaultRubric's dimension names/notes:
//     the server always bakes the effective rubric into every payload it
//     sends (Task 1's DTO ruling; task-5-report.md's gradingRubricFor), so
//     the fallback rubric here is a generic, non-duplicating placeholder
//     that a real response should never trigger.

import { apiFetch } from "./client";

export type GradingStatus = "queued" | "running" | "draft" | "failed" | "sent";
export type PointKind = "good" | "issue";
export type GradingSource = "ai" | "teacher";
export const LETTER_GRADES = ["A+", "A", "A-", "B+", "B", "B-", "C+", "C", "C-", "D"] as const;

export interface RubricDimension {
  name: string;
  note: string;
}
export interface Rubric {
  scale: "letter" | "points";
  max?: number;
  dimensions: RubricDimension[];
  focus: string;
}
export interface GradingPoint {
  kind: PointKind;
  quote: string | null;
  text: string;
  action: string | null;
  source: "ai" | "teacher";
  /** Which rubric dimension this point belongs to (exact `RubricDimension.name`), or "" when
   *  the model didn't give one or gave one the server couldn't match. */
  dimension: string;
  /** Which symptom in the writing room's closed table this point matched, already resolved to
   *  its teacher-facing name server-side (never a raw id) — or "" when there isn't one. */
  symptom: string;
}
export interface GradingContent {
  overall: { grade: string; comment: string };
  dimensions: { name: string; grade: string; comment: string }[];
  points: GradingPoint[];
}
export interface GradingSummary {
  id: string;
  status: GradingStatus;
  overallGrade: string | null;
  error: string | null;
  reviewedAt: string | null;
  sentAt: string | null;
}
export interface GradingRow {
  userId: string;
  displayName: string;
  atomId: string | null;
  version: { number: number; submittedAt: string } | null;
  grading: GradingSummary | null;
}
export interface TeacherGrading {
  id: string;
  classId: string;
  assignmentId: string | null;
  userId: string;
  displayName: string;
  atomId: string;
  versionNumber: number;
  latestVersionNumber: number;
  title: string;
  body: string;
  lang: "zh" | "en";
  rubric: Rubric;
  status: GradingStatus;
  content: GradingContent | null;
  error: string | null;
  /** "ai" when the model drafted it, "teacher" for a 人工批改. */
  source: GradingSource;
  reviewedAt: string | null;
  sentAt: string | null;
  studentSeenAt: string | null;
  updatedAt: string;
}
export interface StudentGrading {
  id: string;
  versionNumber: number;
  rubric: Rubric;
  content: GradingContent;
  sentAt: string;
  seen: boolean;
  source: GradingSource;
}

/**
 * POST .../assignments/{aid}/gradings (一键AI批改) response. `queued` is the
 * count that actually committed; `failed` is how many eligible recipients'
 * enqueue attempts rolled back; `error` is the first backend failure text
 * (already reads `入队失败：…`), null when `failed === 0`. When EVERY
 * eligible recipient fails, the server answers 503 `grading_enqueue_failed`
 * instead of this 200 shape — that surfaces as a thrown `ApiError`, the
 * caller must catch it separately from reading this result.
 */
export interface QueueGradingsResult {
  queued: number;
  failed: number;
  error: string | null;
}

/** POST .../assignments/{aid}/gradings/send (发送全部已审阅) response.
 * `skipped` = ids.length - sent (not a reviewed draft of this assignment, or
 * the student left the class). */
export interface SendGradingsResult {
  sent: number;
  skipped: number;
}

const s = (v: unknown): string => (typeof v === "string" ? v : "");
const ns = (v: unknown): string | null => (typeof v === "string" ? v : null);
const n = (v: unknown): number => (typeof v === "number" ? v : 0);
const obj = (v: unknown): Record<string, unknown> =>
  v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
const arr = (v: unknown): Record<string, unknown>[] => (Array.isArray(v) ? v.map(obj) : []);

const STATUSES: readonly GradingStatus[] = ["queued", "running", "draft", "failed", "sent"];
const status = (v: unknown): GradingStatus =>
  (STATUSES as readonly unknown[]).includes(v) ? (v as GradingStatus) : "failed";

/**
 * Defensive-only fallback: every real `lite_grading` row is created with an
 * effective rubric snapshot already resolved server-side (homework's own
 * rubric, or `liteassign.DefaultRubric`), so a live response should never
 * hit this. It deliberately does NOT reproduce the Go default's dimension
 * names/notes (binding decision: no frontend copy of that text) — just a
 * generic, clearly-a-fallback placeholder.
 */
const FALLBACK_RUBRIC: Rubric = { scale: "letter", dimensions: [{ name: "总评", note: "" }], focus: "" };

export function normalizeRubric(raw: unknown): Rubric | null {
  const r = obj(raw);
  const dimensions = arr(r.dimensions)
    .map((d) => ({ name: s(d.name), note: s(d.note) }))
    .filter((d) => d.name !== "");
  if (dimensions.length === 0) return null;
  const scale = r.scale === "points" ? "points" : "letter";
  return scale === "points" ? { scale, max: n(r.max), dimensions, focus: s(r.focus) } : { scale, dimensions, focus: s(r.focus) };
}

export function normalizeGradingContent(raw: unknown): GradingContent | null {
  if (!raw || typeof raw !== "object") return null;
  const r = obj(raw);
  const overall = obj(r.overall);
  return {
    overall: { grade: s(overall.grade), comment: s(overall.comment) },
    dimensions: arr(r.dimensions).map((d) => ({ name: s(d.name), grade: s(d.grade), comment: s(d.comment) })),
    points: arr(r.points).map((p) => ({
      kind: p.kind === "good" ? "good" : "issue",
      quote: ns(p.quote),
      text: s(p.text),
      action: ns(p.action),
      source: p.source === "ai" ? "ai" : "teacher",
      dimension: s(p.dimension),
      symptom: s(p.symptom),
    })),
  };
}

export function normalizeGradingSummary(raw: unknown): GradingSummary | null {
  if (!raw || typeof raw !== "object") return null;
  const r = obj(raw);
  return {
    id: s(r.id),
    status: status(r.status),
    overallGrade: ns(r.overallGrade),
    error: ns(r.error),
    reviewedAt: ns(r.reviewedAt),
    sentAt: ns(r.sentAt),
  };
}

export function normalizeGradingRow(raw: unknown): GradingRow {
  const r = obj(raw);
  const v = r.version && typeof r.version === "object" ? obj(r.version) : null;
  return {
    userId: s(r.userId),
    displayName: s(r.displayName),
    atomId: ns(r.atomId),
    version: v ? { number: n(v.number), submittedAt: s(v.submittedAt) } : null,
    grading: normalizeGradingSummary(r.grading),
  };
}

export function normalizeTeacherGrading(raw: unknown): TeacherGrading {
  const r = obj(raw);
  const lang = r.lang === "en" ? "en" : "zh";
  return {
    id: s(r.id),
    classId: s(r.classId),
    assignmentId: ns(r.assignmentId),
    userId: s(r.userId),
    displayName: s(r.displayName),
    atomId: s(r.atomId),
    versionNumber: n(r.versionNumber),
    latestVersionNumber: n(r.latestVersionNumber),
    title: s(r.title),
    body: s(r.body),
    lang,
    rubric: normalizeRubric(r.rubric) ?? FALLBACK_RUBRIC,
    status: status(r.status),
    content: normalizeGradingContent(r.content),
    error: ns(r.error),
    source: r.source === "teacher" ? "teacher" : "ai",
    reviewedAt: ns(r.reviewedAt),
    sentAt: ns(r.sentAt),
    studentSeenAt: ns(r.studentSeenAt),
    updatedAt: s(r.updatedAt),
  };
}

export function normalizeStudentGrading(raw: unknown): StudentGrading | null {
  const r = obj(raw);
  const content = normalizeGradingContent(r.content);
  if (!content) return null;
  return {
    id: s(r.id),
    versionNumber: n(r.versionNumber),
    rubric: normalizeRubric(r.rubric) ?? FALLBACK_RUBRIC,
    content,
    sentAt: s(r.sentAt),
    seen: r.seen === true,
    source: r.source === "teacher" ? "teacher" : "ai",
  };
}

const teacherBase = "/api/v1/lite/teacher";
const enc = encodeURIComponent;

/** GET .../assignments/{aid}/gradings — a writing homework's recipients,
 * each one's latest submitted version and that version's grading, if any. */
export async function listAssignmentGradings(aid: string): Promise<GradingRow[]> {
  const r = await apiFetch<{ rows?: unknown[] }>(`${teacherBase}/assignments/${enc(aid)}/gradings`);
  return (Array.isArray(r.rows) ? r.rows : []).map(normalizeGradingRow);
}

/** POST .../assignments/{aid}/gradings — 一键AI批改. `retryFailed` also
 * requeues that version's `failed` rows. Throws (503 `grading_enqueue_failed`)
 * when every eligible recipient's enqueue attempt fails. */
export async function queueAssignmentGradings(aid: string, retryFailed: boolean): Promise<QueueGradingsResult> {
  const r = await apiFetch<{ queued?: number; failed?: number; error?: string | null }>(
    `${teacherBase}/assignments/${enc(aid)}/gradings`,
    { method: "POST", body: JSON.stringify({ retryFailed }) },
  );
  return { queued: n(r.queued), failed: n(r.failed), error: typeof r.error === "string" ? r.error : null };
}

/** POST .../assignments/{aid}/gradings/send — 发送全部已审阅. */
export async function sendReviewedGradings(aid: string, ids: string[]): Promise<SendGradingsResult> {
  const r = await apiFetch<{ sent?: number; skipped?: number }>(
    `${teacherBase}/assignments/${enc(aid)}/gradings/send`,
    { method: "POST", body: JSON.stringify({ ids }) },
  );
  return { sent: n(r.sent), skipped: n(r.skipped) };
}

/** POST .../classes/{id}/students/{userId}/items/{atomId}/gradings — grades
 * one writing's latest version. `mode: "manual"` is 人工批改: a blank draft
 * the teacher fills in, with no model call. 409 `grading_exists` if that
 * version already has a draft (use `regradeGrading` instead); 409
 * `grading_sent` / `grading_in_progress`; 503 `grading_queue_unavailable`
 * (AI only). */
export async function queueWritingGrading(
  classId: string,
  userId: string,
  atomId: string,
  mode: "ai" | "manual" = "ai",
): Promise<TeacherGrading> {
  const r = await apiFetch<{ grading: unknown }>(
    `${teacherBase}/classes/${enc(classId)}/students/${enc(userId)}/items/${enc(atomId)}/gradings`,
    { method: "POST", ...(mode === "manual" ? { body: JSON.stringify({ mode }) } : {}) },
  );
  return normalizeTeacherGrading(r.grading);
}

export async function getGrading(gid: string): Promise<TeacherGrading> {
  const r = await apiFetch<{ grading: unknown }>(`${teacherBase}/gradings/${enc(gid)}`);
  return normalizeTeacherGrading(r.grading);
}

/** PATCH .../gradings/{gid}. Without `content` this is 标记已审阅. 409
 * `grading_not_editable` when the row is queued/running (or failed with no
 * content). Editing an already-`sent` row re-sends it. */
export async function patchGrading(gid: string, content?: GradingContent): Promise<TeacherGrading> {
  const r = await apiFetch<{ grading: unknown }>(`${teacherBase}/gradings/${enc(gid)}`, {
    method: "PATCH",
    body: JSON.stringify(content ? { content } : {}),
  });
  return normalizeTeacherGrading(r.grading);
}

/** POST .../gradings/{gid}/send. 409 `grading_not_sendable` unless the row is
 * a draft or already sent, with content. */
export async function sendGrading(gid: string): Promise<TeacherGrading> {
  const r = await apiFetch<{ grading: unknown }>(`${teacherBase}/gradings/${enc(gid)}/send`, { method: "POST" });
  return normalizeTeacherGrading(r.grading);
}

/** POST .../gradings/{gid}/regrade — 重新批改, the only route allowed to
 * overwrite an existing draft (「重新批改会覆盖当前修改」). */
export async function regradeGrading(gid: string): Promise<TeacherGrading> {
  const r = await apiFetch<{ grading: unknown }>(`${teacherBase}/gradings/${enc(gid)}/regrade`, { method: "POST" });
  return normalizeTeacherGrading(r.grading);
}

/** GET /api/v1/writings/{atomId}/gradings — her own sent gradings, newest
 * version first. A row without content (should never happen — the server
 * only ever sends `sent` rows here) is dropped rather than rendered blank. */
export async function listWritingGradings(atomId: string): Promise<StudentGrading[]> {
  const r = await apiFetch<{ gradings?: unknown[] }>(`/api/v1/writings/${enc(atomId)}/gradings`);
  return (Array.isArray(r.gradings) ? r.gradings : [])
    .map(normalizeStudentGrading)
    .filter((g): g is StudentGrading => g !== null);
}

/** POST /api/v1/lite/inbox/gradings/{gid}/seen. 404 if it isn't hers or isn't sent. */
export async function markGradingSeen(gid: string): Promise<void> {
  await apiFetch<void>(`/api/v1/lite/inbox/gradings/${enc(gid)}/seen`, { method: "POST" });
}
