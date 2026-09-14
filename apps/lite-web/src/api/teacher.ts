// api/teacher.ts — the lite teacher end's read-only view of one class's
// students. Every shape here mirrors what apps/api/internal/api/lite_teacher*
// actually returns (verified in Tasks 3-6), not what the plan guessed before
// the backend was built:
//
// - `GET .../roster` → `{"roster": [RosterRow]}`, camelCase fields.
// - `GET .../students/{userId}` → `{"student": RosterRow, "items": [ItemRow]}`.
// - `GET .../students/{userId}/items/{atomId}` → `ItemDetail`.
// - `GET .../students/{userId}/tree` → exactly `GET /api/v1/interest/tree`'s
//   shape, fetched via `fetchInterestTreeFrom` so the normalizer is shared
//   with the student-facing tree instead of duplicated here.

import { apiFetch } from "./client";
import { fetchInterestTreeFrom, type InterestTree } from "./interest";

export interface RosterRow {
  id: string;
  displayName: string;
  avatarColor: string;
  lastActiveAt: string | null;
  activeDaysThisWeek: number;
  minutesTotal: number;
  minutesThisWeek: number; // -1 = no time recorded yet
  turns: number;
  readingsDone: number;
  readingsTotal: number;
  writingsDone: number;
  writingsTotal: number;
  projectsDone: number;
  projectsTotal: number;
  // Task 6 (lite_teacher_roster.go): how many of the class's assignments are
  // `overdue` for this student. Additive to plan 1's RosterRow.
  overdueAssignments: number;
}

// One assignment on the teacher's student page, in one class. Mirrors
// `StudentAssignmentDTO` (apps/api/internal/api/lite_teacher_roster.go,
// Task 6) — status is derived server-side, never stored.
export interface StudentAssignmentRow {
  id: string;
  kind: string;
  title: string;
  dueAt: string;
  status: string;
  statusLabel: string;
  atomId: string | null;
}

export interface ItemRow {
  atomId: string;
  kind: "reading" | "writing" | "project";
  title: string;
  status: string;
  level: number | null;
  minutes: number;
  turns: number;
  createdAt: string;
  lastActiveAt: string;
  finishedAt: string | null;
}

export interface StudentPage {
  student: RosterRow;
  items: ItemRow[];
  // Task 6: her assignments in this class. Additive to plan 1's StudentPage.
  assignments: StudentAssignmentRow[];
}

export interface ReportSlice {
  stats: { key: string; value: number; unit: string }[];
  moments: { quote: string; where: string }[];
  keep: { text: string; source: "student" | "coach"; label?: string } | null;
  prosePending: boolean;
}

export interface ItemDetail {
  item: ItemRow;
  reading: {
    source: { librarySlug: string | null; level: number | null; url: string | null } | null;
    highlights: { quote: string; note: string }[];
    takeaway: string | null;
    // 🚨 Go 端（`liteLensDTO`, lite_teacher_item.go）发的是 `{title, fields}`
    // ——`title` 是卡 spec 的 name，线上没有 `cardId` 这个字段。这里的类型
    // 原来写的是 `cardId`，是一个从没被任何真实响应喂到过的死字段。
    lenses: { title: string; fields: Record<string, unknown> }[];
  } | null;
  writing: {
    targetWords: number | null;
    lang: string;
    structureKey: string;
    outline: { role: string; text: string }[];
    snippets: { position: number; text: string }[];
    draft: string | null;
    comments: { scope: string; summary: string; points: unknown[] }[];
  } | null;
  project: {
    idea: string;
    /** true: `idea` is the driving question the teacher assigned. */
    assigned: boolean;
    status: string;
    stepsDone: number;
    stepsTotal: number;
    // 🚨 服务端（`ListPblPlanSteps` 为空时 `version == nil`）发的是 `null`，
    // 不是 `[]`——`normalizeItemDetail` 把它收口成空数组，这里就不再是
    // nullable：调用方不用在每个读点自己写一次 `?? []`。
    steps: { title: string; status: string }[];
    tools: { key: string; status: string; result: unknown }[];
    artifacts: { title: string; payload: unknown }[];
    keeps: { text: string }[];
    courses: { slug: string; why: string; takeaway: string; finishedAt: string | null }[];
    siteToken: string | null;
  } | null;
  report: ReportSlice | null;
  reportError: string | null;
}

const n = (v: unknown) => (typeof v === "number" ? v : 0);
const s = (v: unknown) => (typeof v === "string" ? v : "");
const arr = <T>(v: unknown): T[] => (Array.isArray(v) ? (v as T[]) : []);
const obj = (v: unknown): Record<string, unknown> =>
  v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : {};

export function normalizeRosterRow(raw: Record<string, unknown>): RosterRow {
  return {
    id: s(raw.id),
    displayName: s(raw.displayName),
    avatarColor: s(raw.avatarColor),
    lastActiveAt: typeof raw.lastActiveAt === "string" ? raw.lastActiveAt : null,
    activeDaysThisWeek: n(raw.activeDaysThisWeek),
    minutesTotal: n(raw.minutesTotal),
    minutesThisWeek: typeof raw.minutesThisWeek === "number" ? raw.minutesThisWeek : -1,
    turns: n(raw.turns),
    readingsDone: n(raw.readingsDone),
    readingsTotal: n(raw.readingsTotal),
    writingsDone: n(raw.writingsDone),
    writingsTotal: n(raw.writingsTotal),
    projectsDone: n(raw.projectsDone),
    projectsTotal: n(raw.projectsTotal),
    overdueAssignments: n(raw.overdueAssignments),
  };
}

export function normalizeStudentAssignmentRow(raw: Record<string, unknown>): StudentAssignmentRow {
  return {
    id: s(raw.id),
    kind: s(raw.kind),
    title: s(raw.title),
    dueAt: s(raw.dueAt),
    status: s(raw.status),
    statusLabel: s(raw.statusLabel),
    atomId: typeof raw.atomId === "string" ? raw.atomId : null,
  };
}

export function normalizeStudentPage(raw: { student?: Record<string, unknown>; items?: ItemRow[]; assignments?: Record<string, unknown>[] }): StudentPage {
  return {
    student: normalizeRosterRow(raw.student ?? {}),
    items: raw.items ?? [],
    assignments: (raw.assignments ?? []).map(normalizeStudentAssignmentRow),
  };
}

const base = (classId: string) => `/api/v1/lite/teacher/classes/${encodeURIComponent(classId)}`;

export async function getRoster(classId: string): Promise<RosterRow[]> {
  const r = await apiFetch<{ roster?: Record<string, unknown>[] }>(`${base(classId)}/roster`);
  return (r.roster ?? []).map(normalizeRosterRow);
}

export async function getStudentPage(classId: string, userId: string): Promise<StudentPage> {
  const r = await apiFetch<{ student: Record<string, unknown>; items?: ItemRow[]; assignments?: Record<string, unknown>[] }>(
    `${base(classId)}/students/${encodeURIComponent(userId)}`,
  );
  return normalizeStudentPage(r);
}

/**
 * normalizeItemDetail — guards every array/object field the server can send
 * as `null` so `ItemPage` never has to null-check before `.length`, `.map`
 * or `Object.entries` at each call site.
 *
 * This is a real, hit-on-every-load bug, not a defensive-programming
 * exercise: `atom_report.go`'s `Moments`/`Stats`/`Keep` fields are all
 * `json:",omitempty"`, and `liteTeacherReport` (lite_teacher_item.go)
 * copies them straight through as `json.RawMessage` — a nil slice/map
 * marshals to the JSON literal `null`, not `[]`/`{}`. So `report.moments`
 * arrives `null` on EVERY phase-1 report (`prosePending: true`, i.e. the
 * common case right after an item finishes), and `project.steps` arrives
 * `null` whenever a project has no live plan version yet
 * (`GetPblLivePlan` → `pgx.ErrNoRows` → `version == nil`, lite_teacher_item.go).
 * The student-facing report already normalizes this exact shape of bug —
 * see `normalizeReport` in `api/reports.ts` (`raw.moments ?? []`).
 */
export function normalizeItemDetail(raw: Record<string, unknown>): ItemDetail {
  return {
    item: (raw.item ?? {}) as ItemRow,
    reading: normalizeReading(raw.reading),
    writing: normalizeWriting(raw.writing),
    project: normalizeProject(raw.project),
    report: normalizeReportSlice(raw.report),
    reportError: typeof raw.reportError === "string" ? raw.reportError : null,
  };
}

function normalizeReportSlice(raw: unknown): ReportSlice | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as Record<string, unknown>;
  return {
    stats: arr(r.stats),
    moments: arr(r.moments),
    keep: (r.keep ?? null) as ReportSlice["keep"],
    prosePending: r.prosePending === true,
  };
}

function normalizeReading(raw: unknown): NonNullable<ItemDetail["reading"]> | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as Record<string, unknown>;
  return {
    source: (r.source ?? null) as NonNullable<ItemDetail["reading"]>["source"],
    highlights: arr(r.highlights),
    takeaway: typeof r.takeaway === "string" ? r.takeaway : null,
    lenses: arr<Record<string, unknown>>(r.lenses).map((l) => ({
      title: s(l.title),
      fields: obj(l.fields),
    })),
  };
}

function normalizeWriting(raw: unknown): NonNullable<ItemDetail["writing"]> | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as Record<string, unknown>;
  return {
    targetWords: typeof r.targetWords === "number" ? r.targetWords : null,
    lang: s(r.lang),
    structureKey: s(r.structureKey),
    outline: arr(r.outline),
    snippets: arr(r.snippets),
    draft: typeof r.draft === "string" ? r.draft : null,
    comments: arr<Record<string, unknown>>(r.comments).map((c) => ({
      scope: s(c.scope),
      summary: s(c.summary),
      points: arr(c.points),
    })),
  };
}

function normalizeProject(raw: unknown): NonNullable<ItemDetail["project"]> | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as Record<string, unknown>;
  return {
    idea: s(r.idea),
    assigned: r.assigned === true,
    status: s(r.status),
    stepsDone: n(r.stepsDone),
    stepsTotal: n(r.stepsTotal),
    steps: arr(r.steps),
    tools: arr(r.tools),
    artifacts: arr(r.artifacts),
    keeps: arr(r.keeps),
    courses: arr(r.courses),
    siteToken: typeof r.siteToken === "string" ? r.siteToken : null,
  };
}

/**
 * `prose: true` lets the server generate a report's pending prose on this
 * request (a flagship model call). The first load must not pass it, so the
 * page never waits on that call; only the page's one follow-up re-fetch does.
 */
export async function getItem(
  classId: string,
  userId: string,
  atomId: string,
  opts?: { prose?: boolean },
): Promise<ItemDetail> {
  const r = await apiFetch<Record<string, unknown>>(
    `${base(classId)}/students/${encodeURIComponent(userId)}/items/${encodeURIComponent(atomId)}${opts?.prose ? "?prose=1" : ""}`,
  );
  return normalizeItemDetail(r);
}

export async function getStudentTree(classId: string, userId: string): Promise<InterestTree> {
  return fetchInterestTreeFrom(`${base(classId)}/students/${encodeURIComponent(userId)}/tree`);
}
