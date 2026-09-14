// api/weekly.ts — the teacher's weekly summary: one student's week and one
// class's week. Shapes mirror apps/api/internal/api/lite_weekly.go as built
// (Task 5): `liteStudentWeeklyDTO`, `liteClassWeeklyDTO`, and the POST
// variants that add `proseError`.
//
// - GET never calls a model. It returns facts, rule cards and any stored prose.
// - POST …/prose returns the stored prose, or asks the model once. A failed
//   validation or transport error is a 200 with `prose: null` and
//   `proseError` set to the backend's raw message. A routing failure is a
//   non-200 and arrives as an ApiError from `apiFetch`.
// - `title` is built by the server (上周表现总结 · … for the latest week) and
//   rendered as is.

import { ApiError, apiFetch } from "./client";

export interface WeekItem {
  kind: string;
  title: string;
}

export interface WeekMoment {
  quote: string;
  itemTitle: string;
}

export interface StudentWeekFacts {
  activeDays: number;
  minutes: number; // -1 = no time records in or before this week
  turns: number;
  prevActiveDays: number;
  finished: WeekItem[];
  assignmentsDone: number;
  assignmentsLate: number;
  assignmentsOverdue: number;
  stalled: WeekItem[];
  newKeywords: string[];
  moments: WeekMoment[];
}

export interface WeekCard {
  kind: string; // "praise" | "watch"
  code: string;
  label: string;
  evidence: string;
}

export interface StudentWeeklyProse {
  summary: string;
  suggestions: { text: string; evidenceCode: string }[];
}

export interface StudentWeekly {
  weekStart: string;
  weekLabel: string;
  title: string;
  isLatest: boolean;
  facts: StudentWeekFacts;
  cards: WeekCard[];
  prose: StudentWeeklyProse | null;
  proseReady: boolean;
}

export interface ClassWeekStats {
  classSize: number;
  activeStudents: number;
  minutes: number;
  turns: number;
  finished: number;
  assignmentRate: number; // integer percent; -1 when nothing was due
}

export interface ClassWeekCard extends WeekCard {
  userId: string;
  name: string;
}

export interface ClassCardProse {
  userId: string;
  lead: string;
  action: string;
}

export interface ClassWeeklyProse {
  comment: string;
  cards: ClassCardProse[];
}

export interface ClassWeekly {
  weekStart: string;
  weekLabel: string;
  title: string;
  isLatest: boolean;
  stats: ClassWeekStats;
  praise: ClassWeekCard[];
  watch: ClassWeekCard[];
  prose: ClassWeeklyProse | null;
  proseReady: boolean;
}

/** A POST …/prose result: the week as it now stands, plus the failure text
 * when no prose came back. */
export interface ProseResult<T> {
  week: T;
  proseError: string | null;
}

type Raw = Record<string, unknown>;

const n = (v: unknown) => (typeof v === "number" ? v : 0);
const s = (v: unknown) => (typeof v === "string" ? v : "");
const obj = (v: unknown): Raw => (v && typeof v === "object" && !Array.isArray(v) ? (v as Raw) : {});
const list = (v: unknown): Raw[] => (Array.isArray(v) ? v.map(obj) : []);

function item(raw: Raw): WeekItem {
  return { kind: s(raw.kind), title: s(raw.title) };
}

function card(raw: Raw): WeekCard {
  return { kind: s(raw.kind), code: s(raw.code), label: s(raw.label), evidence: s(raw.evidence) };
}

function studentProse(raw: unknown): StudentWeeklyProse | null {
  if (!raw || typeof raw !== "object") return null;
  const p = raw as Raw;
  const summary = s(p.summary);
  if (summary.trim() === "") return null;
  return {
    summary,
    suggestions: list(p.suggestions).map((x) => ({ text: s(x.text), evidenceCode: s(x.evidenceCode) })),
  };
}

function classProse(raw: unknown): ClassWeeklyProse | null {
  if (!raw || typeof raw !== "object") return null;
  const p = raw as Raw;
  const comment = s(p.comment);
  if (comment.trim() === "") return null;
  return {
    comment,
    cards: list(p.cards).map((x) => ({ userId: s(x.userId), lead: s(x.lead), action: s(x.action) })),
  };
}

export function normalizeStudentWeekly(raw: Raw): StudentWeekly {
  const f = obj(raw.facts);
  const prose = studentProse(raw.prose);
  return {
    weekStart: s(raw.weekStart),
    weekLabel: s(raw.weekLabel),
    title: s(raw.title),
    isLatest: raw.isLatest === true,
    facts: {
      activeDays: n(f.activeDays),
      minutes: typeof f.minutes === "number" ? f.minutes : -1,
      turns: n(f.turns),
      prevActiveDays: n(f.prevActiveDays),
      finished: list(f.finished).map(item),
      assignmentsDone: n(f.assignmentsDone),
      assignmentsLate: n(f.assignmentsLate),
      assignmentsOverdue: n(f.assignmentsOverdue),
      stalled: list(f.stalled).map(item),
      newKeywords: Array.isArray(f.newKeywords) ? f.newKeywords.filter((k): k is string => typeof k === "string") : [],
      moments: list(f.moments).map((m) => ({ quote: s(m.quote), itemTitle: s(m.itemTitle) })),
    },
    cards: list(raw.cards).map(card),
    prose,
    proseReady: raw.proseReady === true && prose !== null,
  };
}

export function normalizeClassWeekly(raw: Raw): ClassWeekly {
  const st = obj(raw.stats);
  const classCard = (x: Raw): ClassWeekCard => ({ ...card(x), userId: s(x.userId), name: s(x.name) });
  const prose = classProse(raw.prose);
  return {
    weekStart: s(raw.weekStart),
    weekLabel: s(raw.weekLabel),
    title: s(raw.title),
    isLatest: raw.isLatest === true,
    stats: {
      classSize: n(st.classSize),
      activeStudents: n(st.activeStudents),
      minutes: n(st.minutes),
      turns: n(st.turns),
      finished: n(st.finished),
      assignmentRate: typeof st.assignmentRate === "number" ? st.assignmentRate : -1,
    },
    praise: list(raw.praise).map(classCard),
    watch: list(raw.watch).map(classCard),
    prose,
    proseReady: raw.proseReady === true && prose !== null,
  };
}

/** The error text of a POST …/prose 200. A 200 that carries neither prose nor
 * an error still counts as a failure, so the page can offer 重试. */
function proseErrorOf(raw: Raw, hasProse: boolean): string | null {
  const e = typeof raw.proseError === "string" ? raw.proseError.trim() : "";
  if (e !== "") return e;
  return hasProse ? null : "没有返回总结";
}

export function normalizeStudentWeeklyProse(raw: Raw): ProseResult<StudentWeekly> {
  const week = normalizeStudentWeekly(raw);
  return { week, proseError: proseErrorOf(raw, week.prose !== null) };
}

export function normalizeClassWeeklyProse(raw: Raw): ProseResult<ClassWeekly> {
  const week = normalizeClassWeekly(raw);
  return { week, proseError: proseErrorOf(raw, week.prose !== null) };
}

/** The text after 总结生成失败： for a POST that threw. The message, plus the
 * envelope's `details` when it is a string: a routing failure's message is
 * only 「AI 响应错误」, and the part that says what failed
 * (`lite_student_weekly route: no LLM provider configured`) is in `details`. */
export function proseErrorText(e: unknown): string {
  const msg = e instanceof Error ? e.message : String(e);
  const details = e instanceof ApiError && typeof e.details === "string" ? e.details.trim() : "";
  return details ? `${msg}（${details}）` : msg;
}

/** No rule card and no activity of any kind: the page says 该周没有学习记录
 * and asks for no prose. */
export function studentWeekIsEmpty(w: StudentWeekly): boolean {
  const f = w.facts;
  return (
    w.cards.length === 0 &&
    f.activeDays === 0 &&
    f.turns === 0 &&
    f.finished.length === 0 &&
    f.assignmentsDone + f.assignmentsLate + f.assignmentsOverdue === 0 &&
    f.stalled.length === 0 &&
    f.newKeywords.length === 0 &&
    f.moments.length === 0
  );
}

/** The label of the card a suggestion rests on, or "" when no card has that code. */
export function suggestionLabel(cards: WeekCard[], code: string): string {
  return cards.find((c) => c.code === code)?.label ?? "";
}

/** The lead and action the class prose wrote for one student, if any. */
export function classCardProse(prose: ClassWeeklyProse | null, userId: string): ClassCardProse | null {
  return prose?.cards.find((c) => c.userId === userId) ?? null;
}

const base = (classId: string) => `/api/v1/lite/teacher/classes/${encodeURIComponent(classId)}`;
const weekQuery = (weekStart: string | null) => (weekStart ? `?weekStart=${encodeURIComponent(weekStart)}` : "");

/** `weekStart` null = the latest completed week. */
export async function getStudentWeekly(classId: string, userId: string, weekStart: string | null): Promise<StudentWeekly> {
  const r = await apiFetch<Raw>(`${base(classId)}/students/${encodeURIComponent(userId)}/weekly${weekQuery(weekStart)}`);
  return normalizeStudentWeekly(r);
}

export async function postStudentWeeklyProse(
  classId: string,
  userId: string,
  weekStart: string,
): Promise<ProseResult<StudentWeekly>> {
  const r = await apiFetch<Raw>(`${base(classId)}/students/${encodeURIComponent(userId)}/weekly/prose${weekQuery(weekStart)}`, {
    method: "POST",
  });
  return normalizeStudentWeeklyProse(r);
}

export async function getClassWeekly(classId: string, weekStart: string | null): Promise<ClassWeekly> {
  const r = await apiFetch<Raw>(`${base(classId)}/weekly${weekQuery(weekStart)}`);
  return normalizeClassWeekly(r);
}

export async function postClassWeeklyProse(classId: string, weekStart: string): Promise<ProseResult<ClassWeekly>> {
  const r = await apiFetch<Raw>(`${base(classId)}/weekly/prose${weekQuery(weekStart)}`, { method: "POST" });
  return normalizeClassWeeklyProse(r);
}
