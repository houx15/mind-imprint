// api/assignments.ts — lite teacher assignment CRUD + extraction, and the
// student-side inbox/seen/start/for-atom clients. Field names below are
// verified against the Go handlers (Tasks 4-6), not guessed from the plan:
//
// - `apps/api/internal/api/lite_teacher_assignments.go`: AssignmentDTO,
//   AssignmentSummaryDTO, RecipientDTO, create/list/get/patch/archive.
// - `apps/api/internal/api/lite_assignment_extract.go`: extract.
// - `apps/api/internal/api/lite_student_assignments.go`: InboxItemDTO,
//   inbox/seen/start/for-atom.

import { apiFetch } from "./client";
import type { AssignmentStatus } from "../shared/deadline";

export type AssignmentKind = "reading" | "writing" | "project";

// ---- payload shapes (mirrors apps/api/internal/liteassign/payload.go) ----

export interface ReadingAssignmentPayload {
  source: "library" | "url" | "text";
  slug?: string;
  tier?: number | null;
  url?: string;
  text?: string;
}

export interface WritingAssignmentPayload {
  prompt: string;
  targetWords: number;
  lang: "zh" | "en";
}

export interface ProjectAssignmentPayload {
  drivingQuestion: string;
  description?: string;
}

export type AssignmentPayload = ReadingAssignmentPayload | WritingAssignmentPayload | ProjectAssignmentPayload;

// ---- DTOs ----

export interface AssignmentDTO {
  id: string;
  classId: string;
  kind: AssignmentKind;
  title: string;
  instructions: string;
  payload: Record<string, unknown>;
  dueAt: string;
  createdAt: string;
}

export interface AssignmentSummaryDTO extends AssignmentDTO {
  counts: Record<AssignmentStatus, number>;
}

export interface RecipientDTO {
  userId: string;
  displayName: string;
  avatarColor: string;
  status: AssignmentStatus;
  statusLabel: string;
  atomId: string | null;
  startedAt: string | null;
  finishedAt: string | null;
  seenAt: string | null;
}

export interface AssignmentInboxItem {
  type: "assignment";
  id: string;
  kind: AssignmentKind;
  title: string;
  instructions: string;
  className: string;
  dueAt: string;
  status: AssignmentStatus;
  statusLabel: string;
  atomId: string | null;
  unread: boolean;
}

// The inbox holds assignments only. Parent reports were delivered here once
// (plan 4); since 2026-09-15 a teacher exports them instead.
export type InboxItemDTO = AssignmentInboxItem;

export interface StartAssignmentResult {
  kind: AssignmentKind;
  atomId: string;
  projectId: string | null;
}

export interface AssignmentForAtom {
  id: string;
  title: string;
  dueAt: string;
}

export interface ExtractResult {
  prompt: string;
  targetWords: number | null;
  lang: "zh" | "en";
}

// ---- create/patch inputs ----

export interface CreateAssignmentInput {
  kind: AssignmentKind;
  title: string;
  instructions?: string;
  payload: AssignmentPayload;
  dueAt: string;
  userIds: string[];
}

export interface PatchAssignmentInput {
  title?: string;
  instructions?: string;
  dueAt?: string;
  kind?: AssignmentKind;
  payload?: AssignmentPayload;
  addUserIds?: string[];
  removeUserIds?: string[];
}

// ---- normalizers ----

const s = (v: unknown): string => (typeof v === "string" ? v : "");
const arr = <T>(v: unknown): T[] => (Array.isArray(v) ? (v as T[]) : []);
const obj = (v: unknown): Record<string, unknown> =>
  v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : {};

const KNOWN_STATUSES: readonly AssignmentStatus[] = ["not_started", "in_progress", "done", "done_late", "overdue"];

/** An unrecognised wire status (a future addition the client has not shipped
 * for yet) falls back to `not_started` rather than throwing or leaking a raw
 * string into a `Record<AssignmentStatus, string>` lookup. */
function normalizeStatus(raw: unknown): AssignmentStatus {
  return typeof raw === "string" && (KNOWN_STATUSES as readonly string[]).includes(raw)
    ? (raw as AssignmentStatus)
    : "not_started";
}

function normalizeKind(raw: unknown): AssignmentKind {
  return raw === "writing" || raw === "project" ? raw : "reading";
}

export function normalizeAssignmentDTO(raw: Record<string, unknown>): AssignmentDTO {
  return {
    id: s(raw.id),
    classId: s(raw.classId),
    kind: normalizeKind(raw.kind),
    title: s(raw.title),
    instructions: s(raw.instructions),
    payload: obj(raw.payload),
    dueAt: s(raw.dueAt),
    createdAt: s(raw.createdAt),
  };
}

function normalizeCounts(raw: unknown): Record<AssignmentStatus, number> {
  const r = obj(raw);
  const counts = {} as Record<AssignmentStatus, number>;
  for (const status of KNOWN_STATUSES) {
    const v = r[status];
    counts[status] = typeof v === "number" ? v : 0;
  }
  return counts;
}

export function normalizeAssignmentSummaryDTO(raw: Record<string, unknown>): AssignmentSummaryDTO {
  return { ...normalizeAssignmentDTO(raw), counts: normalizeCounts(raw.counts) };
}

export function normalizeRecipientDTO(raw: Record<string, unknown>): RecipientDTO {
  return {
    userId: s(raw.userId),
    displayName: s(raw.displayName),
    avatarColor: s(raw.avatarColor),
    status: normalizeStatus(raw.status),
    statusLabel: s(raw.statusLabel),
    atomId: typeof raw.atomId === "string" ? raw.atomId : null,
    startedAt: typeof raw.startedAt === "string" ? raw.startedAt : null,
    finishedAt: typeof raw.finishedAt === "string" ? raw.finishedAt : null,
    seenAt: typeof raw.seenAt === "string" ? raw.seenAt : null,
  };
}

function normalizeInboxItem(raw: Record<string, unknown>): InboxItemDTO {
  return {
    type: "assignment",
    id: s(raw.id),
    kind: normalizeKind(raw.kind),
    title: s(raw.title),
    instructions: s(raw.instructions),
    className: s(raw.className),
    dueAt: s(raw.dueAt),
    status: normalizeStatus(raw.status),
    statusLabel: s(raw.statusLabel),
    atomId: typeof raw.atomId === "string" ? raw.atomId : null,
    unread: raw.unread === true,
  };
}

/** `unread` is the server's own count, but a client must not trust a field
 * that could be missing on an older/partial response: falling back to
 * counting `unread: true` items keeps the badge honest either way. */
export function normalizeInboxResponse(raw: unknown): { items: InboxItemDTO[]; unread: number } {
  const r = obj(raw);
  const items = arr<unknown>(r.items).map((it) => normalizeInboxItem(obj(it)));
  const unread = typeof r.unread === "number" ? r.unread : items.filter((it) => it.unread).length;
  return { items, unread };
}

/**
 * 🚨 `targetWords` must stay `null`, never become `0` — see
 * `lite_assignment_extract.go`'s `normalizeExtraction`: a missing count IS
 * `null` on the wire, and `0` there would silently mean "aim for a
 * zero-word essay" instead of "the teacher didn't say".
 */
export function normalizeExtractResult(raw: unknown): ExtractResult {
  const r = obj(raw);
  return {
    prompt: s(r.prompt),
    targetWords: typeof r.targetWords === "number" ? r.targetWords : null,
    lang: r.lang === "en" ? "en" : "zh",
  };
}

export function normalizeStartResult(raw: unknown): StartAssignmentResult {
  const r = obj(raw);
  return { kind: normalizeKind(r.kind), atomId: s(r.atomId), projectId: typeof r.projectId === "string" ? r.projectId : null };
}

export function normalizeAssignmentForAtom(raw: unknown): AssignmentForAtom | null {
  const r = obj(raw);
  if (typeof r.id !== "string") return null;
  return { id: r.id, title: s(r.title), dueAt: s(r.dueAt) };
}

// ---- teacher client ----

const teacherBase = "/api/v1/lite/teacher";
const studentBase = "/api/v1/lite";

export async function listAssignments(classId: string): Promise<AssignmentSummaryDTO[]> {
  const r = await apiFetch<{ assignments?: unknown[] }>(
    `${teacherBase}/classes/${encodeURIComponent(classId)}/assignments`,
  );
  return arr<unknown>(r.assignments).map((row) => normalizeAssignmentSummaryDTO(obj(row)));
}

export async function createAssignment(classId: string, input: CreateAssignmentInput): Promise<AssignmentDTO> {
  const r = await apiFetch<{ assignment: unknown }>(
    `${teacherBase}/classes/${encodeURIComponent(classId)}/assignments`,
    { method: "POST", body: JSON.stringify(input) },
  );
  return normalizeAssignmentDTO(obj(r.assignment));
}

export async function getAssignment(aid: string): Promise<{ assignment: AssignmentDTO; recipients: RecipientDTO[] }> {
  const r = await apiFetch<{ assignment: unknown; recipients?: unknown[] }>(
    `${teacherBase}/assignments/${encodeURIComponent(aid)}`,
  );
  return {
    assignment: normalizeAssignmentDTO(obj(r.assignment)),
    recipients: arr<unknown>(r.recipients).map((row) => normalizeRecipientDTO(obj(row))),
  };
}

export async function patchAssignment(aid: string, patch: PatchAssignmentInput): Promise<AssignmentDTO> {
  const r = await apiFetch<{ assignment: unknown }>(`${teacherBase}/assignments/${encodeURIComponent(aid)}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
  return normalizeAssignmentDTO(obj(r.assignment));
}

/**
 * Resolves on 204. A repeat archive call on an already-archived assignment
 * 404s (controller Ruling P2-4, Task 4 review) and that surfaces here as a
 * thrown `ApiError` — Task 8's UI decides whether a 404 right after a
 * confirmed archive should read as success.
 */
export async function archiveAssignment(aid: string): Promise<void> {
  await apiFetch<void>(`${teacherBase}/assignments/${encodeURIComponent(aid)}`, { method: "DELETE" });
}

export async function extractWritingFields(text: string): Promise<ExtractResult> {
  const r = await apiFetch<unknown>(`${teacherBase}/assignments/extract`, {
    method: "POST",
    body: JSON.stringify({ text }),
  });
  return normalizeExtractResult(r);
}

// ---- student client ----

export async function getInbox(): Promise<{ items: InboxItemDTO[]; unread: number }> {
  const r = await apiFetch<unknown>(`${studentBase}/inbox`);
  return normalizeInboxResponse(r);
}

export async function markSeen(aid: string): Promise<void> {
  await apiFetch<void>(`${studentBase}/assignments/${encodeURIComponent(aid)}/seen`, { method: "POST" });
}

export async function startAssignment(aid: string): Promise<StartAssignmentResult> {
  const r = await apiFetch<unknown>(`${studentBase}/assignments/${encodeURIComponent(aid)}/start`, {
    method: "POST",
  });
  return normalizeStartResult(r);
}

export async function getAssignmentForAtom(atomId: string): Promise<AssignmentForAtom | null> {
  const r = await apiFetch<{ assignment: unknown }>(
    `${studentBase}/assignments/for-atom/${encodeURIComponent(atomId)}`,
  );
  return normalizeAssignmentForAtom(r.assignment);
}
