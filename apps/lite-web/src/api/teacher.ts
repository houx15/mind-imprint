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
    lenses: { cardId: string; fields: Record<string, unknown> }[];
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
    status: string;
    stepsDone: number;
    stepsTotal: number;
    steps: { title: string; status: string }[] | null;
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
  };
}

const base = (classId: string) => `/api/v1/lite/teacher/classes/${encodeURIComponent(classId)}`;

export async function getRoster(classId: string): Promise<RosterRow[]> {
  const r = await apiFetch<{ roster?: Record<string, unknown>[] }>(`${base(classId)}/roster`);
  return (r.roster ?? []).map(normalizeRosterRow);
}

export async function getStudentPage(classId: string, userId: string): Promise<StudentPage> {
  const r = await apiFetch<{ student: Record<string, unknown>; items?: ItemRow[] }>(
    `${base(classId)}/students/${encodeURIComponent(userId)}`,
  );
  return { student: normalizeRosterRow(r.student ?? {}), items: r.items ?? [] };
}

export async function getItem(classId: string, userId: string, atomId: string): Promise<ItemDetail> {
  return apiFetch<ItemDetail>(
    `${base(classId)}/students/${encodeURIComponent(userId)}/items/${encodeURIComponent(atomId)}`,
  );
}

export async function getStudentTree(classId: string, userId: string): Promise<InterestTree> {
  return fetchInterestTreeFrom(`${base(classId)}/students/${encodeURIComponent(userId)}/tree`);
}
