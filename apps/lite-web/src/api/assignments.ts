// api/assignments.ts — lite teacher assignment CRUD + extraction, and the
// student-side inbox/seen/start/for-atom clients. Field names below are
// verified against the Go handlers (Tasks 1, 3-6), not guessed from the plan:
//
// - `apps/api/internal/api/lite_teacher_assignments.go`: AssignmentDTO,
//   AssignmentSummaryDTO, RecipientDTO, create/list/get/patch/archive.
// - `apps/api/internal/api/lite_assignment_extract.go`: extract.
// - `apps/api/internal/api/lite_student_assignments.go`: InboxItemDTO,
//   inbox/seen/start/for-atom.
// - `apps/api/internal/api/lite_personalized_reading.go`: preview,
//   RecipientReadingDTO.
// - `apps/api/internal/liteassign/payload.go`: ReadingPayload's `fileName`
//   /`disciplines`/`picks` and PersonalPick.

import { apiFetch } from "./client";
import type { AssignmentStatus } from "../shared/deadline";
import type { Rubric } from "./gradings";

export type AssignmentKind = "reading" | "writing" | "project";

// ---- payload shapes (mirrors apps/api/internal/liteassign/payload.go) ----

/** One student's article, confirmed by the teacher on a personalized reading
 * homework (liteassign.PersonalPick). */
export interface PersonalPick {
  slug: string;
  /** null: the student's own level when she starts. */
  tier: number | null;
}

export interface ReadingAssignmentPayload {
  source: "library" | "url" | "text" | "personalized";
  slug?: string;
  tier?: number | null;
  url?: string;
  text?: string;
  /** Set when the text came from an uploaded document. */
  fileName?: string;
  disciplines?: string[];
  picks?: Record<string, PersonalPick>;
}

export interface WritingAssignmentPayload {
  prompt: string;
  targetWords: number;
  lang: "zh" | "en";
  /** The effective rubric (stored one, else the Go default for `lang`) — the
   * server always bakes this in, so a writing homework's payload never omits
   * it. See Task 1's DTO ruling. */
  rubric?: Rubric;
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

/** One student's article on a personalized reading homework
 * (lite_personalized_reading.go RecipientReadingDTO). `tier: null` in
 * `picked` means her own level, not a computed suggestion — never look one
 * up to display in its place. */
export interface RecipientReading {
  slug: string;
  title: string;
  tier: number | null;
  state: "started" | "picked" | "pending";
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
  /** Set when the teacher returned this writing (退回修改). */
  returnedAt: string | null;
  returnDueAt: string | null;
  returnNote: string | null;
  versionCount: number;
  /** This student's article on a personalized reading homework; null on any
   * other homework. */
  reading: RecipientReading | null;
}

/** One row of POST …/classes/{id}/personalized-reading/preview
 * (personalizedPreviewRowDTO). */
export interface PreviewRow {
  userId: string;
  name: string;
  slug: string;
  title: string;
  tier: number;
  suggestedTier: number;
  reason: string;
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
  returnDueAt: string | null;
  returnNote: string | null;
}

/** A sent 批改 of one of her writings. */
export interface GradingInboxItem {
  type: "grading";
  id: string;
  atomId: string;
  writingTitle: string;
  sentAt: string;
  unread: boolean;
}

// The inbox holds assignments and sent gradings. Parent reports were
// delivered here once (plan 4); since 2026-09-15 a teacher exports them
// instead.
export type InboxItemDTO = AssignmentInboxItem | GradingInboxItem;

export interface StartAssignmentResult {
  kind: AssignmentKind;
  atomId: string;
  projectId: string | null;
}

export interface AssignmentForAtom {
  id: string;
  kind: AssignmentKind;
  title: string;
  dueAt: string;
  /** Set when the teacher returned this writing (退回修改). */
  returnedAt: string | null;
  returnDueAt: string | null;
  returnNote: string | null;
  /** A version was submitted after the return. */
  resubmitted: boolean;
}

export interface ReturnRecipientInput {
  /** RFC3339, in the future. */
  dueAt: string;
  note?: string;
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
  /** Top-level; editable after students start. `null` resets to the default. */
  rubric?: Rubric | null;
}

// ---- normalizers ----

const s = (v: unknown): string => (typeof v === "string" ? v : "");
const arr = <T>(v: unknown): T[] => (Array.isArray(v) ? (v as T[]) : []);
const obj = (v: unknown): Record<string, unknown> =>
  v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
const nullableString = (v: unknown): string | null => (typeof v === "string" ? v : null);

const KNOWN_STATUSES: readonly AssignmentStatus[] = [
  "not_started",
  "in_progress",
  "done",
  "done_late",
  "overdue",
  "returned",
  "resubmitted",
];

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

export const tierOrNull = (v: unknown): number | null =>
  typeof v === "number" && Number.isInteger(v) && v >= 1 && v <= 5 ? v : null;

/** An unrecognised `state` (a server from before this shipped, or a future
 * value) is treated the same as no reading at all. */
export function normalizeRecipientReading(raw: unknown): RecipientReading | null {
  if (!raw || typeof raw !== "object") return null;
  const r = obj(raw);
  if (r.state !== "started" && r.state !== "picked" && r.state !== "pending") return null;
  return { slug: s(r.slug), title: s(r.title), tier: tierOrNull(r.tier), state: r.state };
}

/** Rows without a student or a picked article are dropped; a tier outside
 * 1..5 falls back to the suggested tier (which itself defaults to 2 when
 * malformed) rather than showing a nonsense difficulty. */
export function normalizePreviewRows(raw: unknown): PreviewRow[] {
  return arr<unknown>(obj(raw).rows)
    .map(obj)
    .filter((r) => s(r.userId) !== "" && s(r.slug) !== "")
    .map((r) => {
      const suggestedTier = tierOrNull(r.suggestedTier) ?? 2;
      return {
        userId: s(r.userId),
        name: s(r.name),
        slug: s(r.slug),
        title: s(r.title),
        tier: tierOrNull(r.tier) ?? suggestedTier,
        suggestedTier,
        reason: s(r.reason),
      };
    });
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
    returnedAt: nullableString(raw.returnedAt),
    returnDueAt: nullableString(raw.returnDueAt),
    returnNote: nullableString(raw.returnNote),
    versionCount: typeof raw.versionCount === "number" ? raw.versionCount : 0,
    reading: normalizeRecipientReading(raw.reading),
  };
}

function normalizeInboxItem(raw: Record<string, unknown>): AssignmentInboxItem {
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
    returnDueAt: nullableString(raw.returnDueAt),
    returnNote: nullableString(raw.returnNote),
  };
}

function normalizeGradingInboxItem(raw: Record<string, unknown>): GradingInboxItem {
  return {
    type: "grading",
    id: s(raw.id),
    atomId: s(raw.atomId),
    writingTitle: s(raw.writingTitle),
    sentAt: s(raw.sentAt),
    unread: raw.unread === true,
  };
}

/** `unread` is the server's own count, but a client must not trust a field
 * that could be missing on an older/partial response: falling back to
 * counting `unread: true` items keeps the badge honest either way.
 *
 * Rows of type `assignment` and `grading` are kept; any other type (a
 * `parent_report` row from a server before 2026-09-15) is dropped, and the
 * badge then counts the kept rows. */
export function normalizeInboxResponse(raw: unknown): { items: InboxItemDTO[]; unread: number } {
  const r = obj(raw);
  const rows = arr<unknown>(r.items).map(obj);
  const items: InboxItemDTO[] = [];
  for (const it of rows) {
    if (it.type === "assignment") items.push(normalizeInboxItem(it));
    else if (it.type === "grading") items.push(normalizeGradingInboxItem(it));
  }
  const dropped = rows.length !== items.length;
  const unread = typeof r.unread === "number" && !dropped ? r.unread : items.filter((it) => it.unread).length;
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

/** `kind` defaults to `"writing"` here (not `normalizeKind`'s own `"reading"`
 * default) because the only caller that needs it is a writing page and an
 * older server omits the field entirely; an explicit but unrecognised kind
 * still falls through to `normalizeKind`'s own default. */
export function normalizeAssignmentForAtom(raw: unknown): AssignmentForAtom | null {
  const r = obj(raw);
  if (typeof r.id !== "string") return null;
  return {
    id: r.id,
    kind: r.kind === undefined ? "writing" : normalizeKind(r.kind),
    title: s(r.title),
    dueAt: s(r.dueAt),
    returnedAt: nullableString(r.returnedAt),
    returnDueAt: nullableString(r.returnDueAt),
    returnNote: nullableString(r.returnNote),
    resubmitted: r.resubmitted === true,
  };
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
 * 404s and throws `ApiError`; the caller decides whether a 404 right after a
 * confirmed archive reads as success.
 */
export async function archiveAssignment(aid: string): Promise<void> {
  await apiFetch<void>(`${teacherBase}/assignments/${encodeURIComponent(aid)}`, { method: "DELETE" });
}

/** 退回修改: POST …/assignments/{aid}/recipients/{userId}/return. */
export async function returnRecipient(aid: string, userId: string, input: ReturnRecipientInput): Promise<RecipientDTO> {
  const r = await apiFetch<{ recipient: unknown }>(
    `${teacherBase}/assignments/${encodeURIComponent(aid)}/recipients/${encodeURIComponent(userId)}/return`,
    { method: "POST", body: JSON.stringify(input) },
  );
  return normalizeRecipientDTO(obj(r.recipient));
}

export async function extractWritingFields(text: string): Promise<ExtractResult> {
  const r = await apiFetch<unknown>(`${teacherBase}/assignments/extract`, {
    method: "POST",
    body: JSON.stringify({ text }),
  });
  return normalizeExtractResult(r);
}

/** POST …/classes/{id}/personalized-reading/preview: one row per enrolled
 * student, the article she'd be picked for right now under the given
 * filter. An empty `disciplines` and a null `tier` are left out of the
 * body — the server reads their absence as "no filter"/"her own level". */
export async function previewPersonalizedReading(
  classId: string,
  input: { disciplines: string[]; tier: number | null },
): Promise<PreviewRow[]> {
  const body = {
    ...(input.disciplines.length ? { disciplines: input.disciplines } : {}),
    ...(input.tier !== null ? { tier: input.tier } : {}),
  };
  const r = await apiFetch<unknown>(
    `${teacherBase}/classes/${encodeURIComponent(classId)}/personalized-reading/preview`,
    { method: "POST", body: JSON.stringify(body) },
  );
  return normalizePreviewRows(r);
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
