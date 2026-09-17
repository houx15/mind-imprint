// parentReport/range.ts — date helpers for a parent report's range.
//
// Dates are calendar days (`YYYY-MM-DD`, Beijing), matching
// `liteparent.ParseRange` / `DefaultRange` on the Go side. All math is done on
// UTC epoch values, never on the running machine's zone: a parent's phone may
// be set to any zone, and the report covers Beijing days.

const BEIJING_OFFSET_MS = 8 * 60 * 60 * 1000;
const DAY_MS = 24 * 60 * 60 * 1000;
const DATE_RE = /^(\d{4})-(\d{2})-(\d{2})$/;

/** Days in a default report range, same as `liteparent.DefaultRangeDays`. */
export const DEFAULT_RANGE_DAYS = 28;

function parseDate(date: string): { y: number; m: number; d: number } | null {
  const match = DATE_RE.exec(date);
  if (!match) return null;
  return { y: Number(match[1]), m: Number(match[2]), d: Number(match[3]) };
}

function isoDay(ms: number): string {
  return new Date(ms).toISOString().slice(0, 10);
}

/** Today's Beijing calendar date for an epoch instant. */
export function todayBeijing(nowMs: number): string {
  return isoDay(nowMs + BEIJING_OFFSET_MS);
}

/** `date` moved by `days` calendar days. Empty for a malformed date. */
export function addDays(date: string, days: number): string {
  const p = parseDate(date);
  if (!p) return "";
  return isoDay(Date.UTC(p.y, p.m - 1, p.d) + days * DAY_MS);
}

/** The 28 days ending today (Beijing), same as `liteparent.DefaultRange`:
 * work finished today belongs in a report written today. */
export function defaultRange(today: string): { start: string; end: string } {
  const end = today;
  return { start: addDays(end, -(DEFAULT_RANGE_DAYS - 1)), end };
}

/** `2026-09-03` → `9月3日`. Empty for a malformed date. */
export function monthDay(date: string): string {
  const p = parseDate(date);
  return p ? `${p.m}月${p.d}日` : "";
}

/** `8月17日–9月13日`; both years are named when the range spans two. */
export function rangeLabel(start: string, end: string): string {
  const s = parseDate(start);
  const e = parseDate(end);
  if (!s || !e) return "";
  if (s.y === e.y) return `${monthDay(start)}–${monthDay(end)}`;
  return `${s.y}年${monthDay(start)}–${e.y}年${monthDay(end)}`;
}

/** An RFC3339 instant → its Beijing `M月D日`. Empty when unparsable. */
export function publishedMonthDay(iso: string): string {
  const ms = Date.parse(iso);
  if (Number.isNaN(ms)) return "";
  return monthDay(todayBeijing(ms));
}
