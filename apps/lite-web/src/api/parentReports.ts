// api/parentReports.ts — the teacher's parent reports. There is no parent end:
// a teacher generates a report, edits it and exports it as a picture that she
// sends to parents herself. Nothing here is public and the student never
// receives it.
//
// Shapes verified against apps/api/internal/api/lite_parent_report.go
// (`ParentReportDTO`, `ParentReportSummaryDTO`) and
// apps/api/internal/liteparent/facts.go (`Facts`, `Hidden`, `HiddenMentions`):
//
// - The teacher DTO carries names only inside `facts`, frozen at generate.
// - `facts` is the FULL frozen facts, hidden items included, so the editor can
//   list every 金句 and keyword with a toggle. The preview and the poster filter
//   them with `visibleFacts` (parentReport/view.ts).
// - `sections` is computed from the VISIBLE facts: with every keyword hidden,
//   `interests` is absent while `body.interests` may still hold text.
// - `hiddenMentions` names the visible sections whose stored body still quotes
//   a hidden 金句 or keyword.
//
// No payload carries chat text. 金句 (`facts.moments`) are her own sentences;
// `body` is the teacher's text.

import { apiFetch } from "./client";

export interface ParentReportItem {
  kind: string;
  title: string;
  /** YYYY-MM-DD, Beijing. */
  finishedAt: string;
}

export interface ParentReportMoment {
  quote: string;
  itemTitle: string;
}

export interface ParentReportKeyword {
  text: string;
  field: string;
  fieldLabel: string;
}

export interface ParentReportFacts {
  studentName: string;
  className: string;
  teacherName: string;
  rangeStart: string;
  rangeEnd: string;
  days: number;
  activeDays: number;
  /** -1 when the range has no time buckets at all (no record). */
  minutes: number;
  turns: number;
  readings: ParentReportItem[];
  writings: ParentReportItem[];
  projects: ParentReportItem[];
  moments: ParentReportMoment[];
  assignmentsTotal: number;
  assignmentsOnTime: number;
  assignmentsLate: number;
  assignmentsMissed: number;
  keywords: ParentReportKeyword[];
}

/** What the view and the poster render. */
export interface ParentReport {
  studentName: string;
  className: string;
  teacherName: string;
  rangeStart: string;
  rangeEnd: string;
  /** RFC3339; the byline date. */
  createdAt: string;
  facts: ParentReportFacts;
  /** Section keys in display order (`liteparent.SectionsWithFacts`). */
  sections: string[];
  /** Section key → the teacher's text. A key may be missing or blank. */
  body: Record<string, string>;
}

/** Hidden items, addressed by exact text: a moment by `quote`, a keyword by
 * `text`. */
export interface ParentReportHidden {
  moments: string[];
  keywords: string[];
}

const s = (v: unknown): string => (typeof v === "string" ? v : "");
const n = (v: unknown, fallback = 0): number => (typeof v === "number" && Number.isFinite(v) ? v : fallback);
const obj = (v: unknown): Record<string, unknown> =>
  v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
const list = (v: unknown): Record<string, unknown>[] => (Array.isArray(v) ? v.map(obj) : []);
const strings = (v: unknown): string[] => (Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : []);

export function emptyFacts(): ParentReportFacts {
  return {
    studentName: "",
    className: "",
    teacherName: "",
    rangeStart: "",
    rangeEnd: "",
    days: 0,
    activeDays: 0,
    minutes: -1,
    turns: 0,
    readings: [],
    writings: [],
    projects: [],
    moments: [],
    assignmentsTotal: 0,
    assignmentsOnTime: 0,
    assignmentsLate: 0,
    assignmentsMissed: 0,
    keywords: [],
  };
}

function normalizeItems(raw: unknown): ParentReportItem[] {
  return list(raw).map((it) => ({ kind: s(it.kind), title: s(it.title), finishedAt: s(it.finishedAt) }));
}

export function normalizeFacts(raw: unknown): ParentReportFacts {
  const f = obj(raw);
  return {
    studentName: s(f.studentName),
    className: s(f.className),
    teacherName: s(f.teacherName),
    rangeStart: s(f.rangeStart),
    rangeEnd: s(f.rangeEnd),
    days: n(f.days),
    activeDays: n(f.activeDays),
    minutes: n(f.minutes, -1),
    turns: n(f.turns),
    readings: normalizeItems(f.readings),
    writings: normalizeItems(f.writings),
    projects: normalizeItems(f.projects),
    moments: list(f.moments).map((m) => ({ quote: s(m.quote), itemTitle: s(m.itemTitle) })),
    assignmentsTotal: n(f.assignmentsTotal),
    assignmentsOnTime: n(f.assignmentsOnTime),
    assignmentsLate: n(f.assignmentsLate),
    assignmentsMissed: n(f.assignmentsMissed),
    keywords: list(f.keywords).map((k) => ({ text: s(k.text), field: s(k.field), fieldLabel: s(k.fieldLabel) })),
  };
}

/** Reads the report part of the teacher DTO. Top-level names fall back to the
 * frozen facts, which is where the teacher DTO carries them. */
export function normalizeParentReport(raw: unknown): ParentReport {
  const r = obj(raw);
  const facts = normalizeFacts(r.facts);
  const body: Record<string, string> = {};
  for (const [key, value] of Object.entries(obj(r.body))) {
    if (typeof value === "string") body[key] = value;
  }
  return {
    studentName: s(r.studentName) || facts.studentName,
    className: s(r.className) || facts.className,
    teacherName: s(r.teacherName) || facts.teacherName,
    rangeStart: s(r.rangeStart) || facts.rangeStart,
    rangeEnd: s(r.rangeEnd) || facts.rangeEnd,
    createdAt: s(r.createdAt),
    facts,
    sections: strings(r.sections),
    body,
  };
}

export function normalizeHidden(raw: unknown): ParentReportHidden {
  const h = obj(raw);
  return { moments: strings(h.moments), keywords: strings(h.keywords) };
}

/** `{section: [text]}`. A non-list value and an empty list are dropped, so a
 * key present means that section has at least one mention. */
export function normalizeHiddenMentions(raw: unknown): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const [key, value] of Object.entries(obj(raw))) {
    const texts = strings(value);
    if (texts.length > 0) out[key] = texts;
  }
  return out;
}

export interface TeacherParentReport {
  id: string;
  studentId: string;
  classId: string;
  /** Whether a draft has ever been stored (`draft` is null until then). */
  hasDraft: boolean;
  createdAt: string;
  hidden: ParentReportHidden;
  hiddenMentions: Record<string, string[]>;
  /** The report with the FULL facts and the stored body. */
  view: ParentReport;
}

export interface ParentReportSummary {
  id: string;
  studentId: string;
  studentName: string;
  rangeStart: string;
  rangeEnd: string;
  createdAt: string;
}

const optString = (v: unknown): string | null => (typeof v === "string" && v ? v : null);

export function normalizeTeacherParentReport(raw: unknown): TeacherParentReport {
  const r = obj(raw);
  return {
    id: s(r.id),
    studentId: s(r.studentId),
    classId: s(r.classId),
    hasDraft: r.draft !== null && typeof r.draft === "object",
    createdAt: s(r.createdAt),
    hidden: normalizeHidden(r.hidden),
    hiddenMentions: normalizeHiddenMentions(r.hiddenMentions),
    view: normalizeParentReport(raw),
  };
}

export function normalizeParentReportSummary(raw: unknown): ParentReportSummary {
  const r = obj(raw);
  return {
    id: s(r.id),
    studentId: s(r.studentId),
    studentName: s(r.studentName),
    rangeStart: s(r.rangeStart),
    rangeEnd: s(r.rangeEnd),
    createdAt: s(r.createdAt),
  };
}

const TEACHER = "/api/v1/lite/teacher";
const reportPath = (id: string) => `${TEACHER}/parent-reports/${encodeURIComponent(id)}`;
const studentReportsPath = (classId: string, userId: string) =>
  `${TEACHER}/classes/${encodeURIComponent(classId)}/students/${encodeURIComponent(userId)}/parent-reports`;

/** A generate or redraft result. `draftError` is set when the report row
 * exists but its draft could not be written. */
export interface ParentReportDraftResult {
  report: TeacherParentReport;
  draftError: string | null;
}

function draftResult(r: { report?: unknown; draftError?: unknown }): ParentReportDraftResult {
  return { report: normalizeTeacherParentReport(r.report), draftError: optString(r.draftError) };
}

/** `POST …/classes/{id}/students/{userId}/parent-reports` → 201. */
export async function createParentReport(
  classId: string,
  userId: string,
  range: { rangeStart: string; rangeEnd: string },
): Promise<ParentReportDraftResult> {
  const r = await apiFetch<{ report?: unknown; draftError?: unknown }>(studentReportsPath(classId, userId), {
    method: "POST",
    body: JSON.stringify(range),
  });
  return draftResult(r);
}

export async function listStudentParentReports(classId: string, userId: string): Promise<ParentReportSummary[]> {
  const r = await apiFetch<{ reports?: unknown }>(studentReportsPath(classId, userId));
  return list(r.reports).map(normalizeParentReportSummary);
}

export async function listClassParentReports(classId: string): Promise<ParentReportSummary[]> {
  const r = await apiFetch<{ reports?: unknown }>(`${TEACHER}/classes/${encodeURIComponent(classId)}/parent-reports`);
  return list(r.reports).map(normalizeParentReportSummary);
}

export async function getTeacherParentReport(id: string): Promise<TeacherParentReport> {
  const r = await apiFetch<{ report?: unknown }>(reportPath(id));
  return normalizeTeacherParentReport(r.report);
}

/** PATCH merges: only the section sent is changed, and `""` clears it. The key
 * must be in the report's current `sections` (400 `invalid_section`). */
export async function patchParentReportSection(id: string, key: string, text: string): Promise<TeacherParentReport> {
  const r = await apiFetch<{ report?: unknown }>(reportPath(id), {
    method: "PATCH",
    body: JSON.stringify({ body: { [key]: text } }),
  });
  return normalizeTeacherParentReport(r.report);
}

/** PATCH `hidden`: the object replaces the whole stored set, so send all of it. */
export async function patchParentReportHidden(id: string, hidden: ParentReportHidden): Promise<TeacherParentReport> {
  const r = await apiFetch<{ report?: unknown }>(reportPath(id), {
    method: "PATCH",
    body: JSON.stringify({ hidden: { moments: hidden.moments, keywords: hidden.keywords } }),
  });
  return normalizeTeacherParentReport(r.report);
}

export async function redraftParentReport(id: string, replaceBody: boolean): Promise<ParentReportDraftResult> {
  const r = await apiFetch<{ report?: unknown; draftError?: unknown }>(`${reportPath(id)}/redraft`, {
    method: "POST",
    body: JSON.stringify({ replaceBody }),
  });
  return draftResult(r);
}
