// api/parentReports.ts — a published parent report, read by a parent (public
// link, no session) and by the student herself.
//
// Shapes verified against apps/api/internal/api/lite_parent_report_read.go
// (`PublicParentReportDTO`, `StudentParentReportDTO`) and
// apps/api/internal/liteparent/facts.go (`Facts`):
//
// - The public DTO has no id, no share token and no draft.
// - The student DTO is the same object plus `id`.
// - Student, class and teacher names come from the facts frozen at generate
//   time, so the top-level names always equal `facts.*Name`.
//
// Neither payload carries chat text. 金句 (`facts.moments`) are her own
// sentences; `body` is the teacher's text.

import { API_BASE, ApiError, apiFetch } from "./client";

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

export interface ParentReport {
  /** Present on the student route only. */
  id: string | null;
  studentName: string;
  className: string;
  teacherName: string;
  rangeStart: string;
  rangeEnd: string;
  /** RFC3339; null only for a teacher preview of an unpublished report. */
  publishedAt: string | null;
  facts: ParentReportFacts;
  /** Section keys in display order (`liteparent.SectionsWithFacts`). */
  sections: string[];
  /** Section key → the teacher's text. A key may be missing or blank. */
  body: Record<string, string>;
}

const s = (v: unknown): string => (typeof v === "string" ? v : "");
const n = (v: unknown, fallback = 0): number => (typeof v === "number" && Number.isFinite(v) ? v : fallback);
const obj = (v: unknown): Record<string, unknown> =>
  v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
const list = (v: unknown): Record<string, unknown>[] => (Array.isArray(v) ? v.map(obj) : []);

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

/** Reads either DTO. Top-level names fall back to the frozen facts, so the
 * teacher DTO (which carries names only inside `facts`) reads the same way. */
export function normalizeParentReport(raw: unknown): ParentReport {
  const r = obj(raw);
  const facts = normalizeFacts(r.facts);
  const body: Record<string, string> = {};
  for (const [key, value] of Object.entries(obj(r.body))) {
    if (typeof value === "string") body[key] = value;
  }
  return {
    id: typeof r.id === "string" ? r.id : null,
    studentName: s(r.studentName) || facts.studentName,
    className: s(r.className) || facts.className,
    teacherName: s(r.teacherName) || facts.teacherName,
    rangeStart: s(r.rangeStart) || facts.rangeStart,
    rangeEnd: s(r.rangeEnd) || facts.rangeEnd,
    publishedAt: typeof r.publishedAt === "string" && r.publishedAt ? r.publishedAt : null,
    facts,
    sections: Array.isArray(r.sections) ? r.sections.filter((k): k is string => typeof k === "string") : [],
    body,
  };
}

/** Thrown for an unknown or revoked link, so the page can tell it apart from
 * a network failure. */
export class ParentReportNotFoundError extends Error {
  constructor() {
    super("parent_report_not_found");
    this.name = "ParentReportNotFoundError";
  }
}

/**
 * `GET /api/v1/public/parent-reports/{token}`, from a page with no session.
 * Not `apiFetch`: that sends `credentials:"include"`, and this request must
 * carry no cookie. A 404 raises `ParentReportNotFoundError`; any other failure
 * raises an `ApiError` or the network error, with its message.
 */
export async function getPublicParentReport(token: string): Promise<ParentReport> {
  const res = await fetch(`${API_BASE}/api/v1/public/parent-reports/${encodeURIComponent(token)}`, {
    credentials: "omit",
  });
  if (res.status === 404) throw new ParentReportNotFoundError();
  if (!res.ok) {
    let code = "internal_error";
    let message = `HTTP ${res.status}`;
    try {
      const body = await res.json();
      code = body?.error?.code ?? code;
      message = body?.error?.message ?? message;
    } catch {
      /* non-JSON error body */
    }
    throw new ApiError(code, message, res.status);
  }
  const body = (await res.json()) as { report?: unknown };
  return normalizeParentReport(body.report);
}

/** `GET /api/v1/lite/parent-reports/{id}` — her own published report. */
export async function getStudentParentReport(id: string): Promise<ParentReport> {
  const r = await apiFetch<{ report?: unknown }>(`/api/v1/lite/parent-reports/${encodeURIComponent(id)}`);
  return normalizeParentReport(r.report);
}

/** `POST …/seen` — 204. Callers ignore a failure. */
export async function markParentReportSeen(id: string): Promise<void> {
  await apiFetch<void>(`/api/v1/lite/parent-reports/${encodeURIComponent(id)}/seen`, { method: "POST" });
}
