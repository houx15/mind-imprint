// teacher/controls/dateFieldLogic.ts — pure logic for `DateField`: parsing
// and building the two value shapes the teacher forms already use
// (`YYYY-MM-DD` and the datetime-local `YYYY-MM-DDTHH:mm`), the month grid,
// the label on the closed field, and the deadline shortcuts.
//
// Every "today" is Beijing's, from a fixed +08:00 offset (shared/deadline.ts
// explains why there is no time zone lookup).

const BEIJING_OFFSET_MS = 8 * 60 * 60 * 1000;
const WEEKDAYS = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"] as const;

export interface Day {
  y: number;
  /** 1–12 */
  m: number;
  d: number;
}

export interface Parts extends Day {
  hh: number;
  mi: number;
}

const pad2 = (n: number) => (n < 10 ? `0${n}` : `${n}`);

export function dayKey({ y, m, d }: Day): string {
  return `${y}-${pad2(m)}-${pad2(d)}`;
}

export function daysInMonth(y: number, m: number): number {
  return new Date(Date.UTC(y, m, 0)).getUTCDate();
}

/** `YYYY-MM-DD` or `YYYY-MM-DDTHH:mm` → its parts; a date alone reads as
 * 00:00. Anything else, or an impossible date (2月30日), → null. */
export function parseValue(v: string): Parts | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})(?:T(\d{2}):(\d{2}))?$/.exec(v.trim());
  if (!m) return null;
  const y = Number(m[1]);
  const mo = Number(m[2]);
  const d = Number(m[3]);
  const hh = m[4] === undefined ? 0 : Number(m[4]);
  const mi = m[5] === undefined ? 0 : Number(m[5]);
  if (mo < 1 || mo > 12 || d < 1 || d > daysInMonth(y, mo) || hh > 23 || mi > 59) return null;
  return { y, m: mo, d, hh, mi };
}

export function formatValue(p: Parts, withTime: boolean): string {
  return withTime ? `${dayKey(p)}T${pad2(p.hh)}:${pad2(p.mi)}` : dayKey(p);
}

export function weekday({ y, m, d }: Day): string {
  return WEEKDAYS[new Date(Date.UTC(y, m - 1, d)).getUTCDay()]!;
}

export function todayBeijing(nowMs: number): Day {
  const bj = new Date(nowMs + BEIJING_OFFSET_MS);
  return { y: bj.getUTCFullYear(), m: bj.getUTCMonth() + 1, d: bj.getUTCDate() };
}

export function addDays(day: Day, n: number): Day {
  const t = new Date(Date.UTC(day.y, day.m - 1, day.d + n));
  return { y: t.getUTCFullYear(), m: t.getUTCMonth() + 1, d: t.getUTCDate() };
}

/** The month before/after (`delta` = ±1). */
export function shiftMonth(y: number, m: number, delta: number): { y: number; m: number } {
  const t = new Date(Date.UTC(y, m - 1 + delta, 1));
  return { y: t.getUTCFullYear(), m: t.getUTCMonth() + 1 };
}

export interface GridCell extends Day {
  inMonth: boolean;
}

/** Six Monday-first weeks covering the month, with the neighbouring months'
 * days filling the first and last rows. */
export function monthGrid(y: number, m: number): GridCell[] {
  const firstDow = new Date(Date.UTC(y, m - 1, 1)).getUTCDay(); // 0 = Sunday
  const lead = (firstDow + 6) % 7;
  const start = addDays({ y, m, d: 1 }, -lead);
  return Array.from({ length: 42 }, (_, i) => {
    const day = addDays(start, i);
    return { ...day, inMonth: day.m === m && day.y === y };
  });
}

/** Whether a day may be picked: `min`/`max` are `YYYY-MM-DD` (either may be
 * empty), compared as strings, which orders correctly in this shape. */
export function dayAllowed(day: Day, min?: string, max?: string): boolean {
  const k = dayKey(day);
  if (min && k < min.slice(0, 10)) return false;
  if (max && k > max.slice(0, 10)) return false;
  return true;
}

/** The closed field's text: 「9月18日 周五 21:00」, with the year when it is
 * not this year. "" for an empty or unreadable value. */
export function valueLabel(v: string, withTime: boolean, today: Day): string {
  const p = parseValue(v);
  if (!p) return "";
  const date = `${p.y === today.y ? "" : `${p.y}年`}${p.m}月${p.d}日 ${weekday(p)}`;
  return withTime ? `${date} ${pad2(p.hh)}:${pad2(p.mi)}` : date;
}

/** Picking a day keeps the time already set, or uses `defaultTime`. */
export function withDay(v: string, day: Day, defaultTime: { hh: number; mi: number }): Parts {
  const p = parseValue(v);
  return { ...day, hh: p ? p.hh : defaultTime.hh, mi: p ? p.mi : defaultTime.mi };
}

export const DEFAULT_DUE_TIME = { hh: 21, mi: 0 };

export interface Shortcut {
  label: string;
  value: string;
}

/**
 * Deadline shortcuts, all at 21:00 Beijing, weeks starting on Monday:
 * - 今天, while 21:00 is still ahead;
 * - 明天;
 * - the coming Friday when it is not today or tomorrow — 本周五 from Monday
 *   to Wednesday, 下周五 on Saturday and Sunday;
 * - 下周一, unless that is tomorrow (Sunday).
 */
export function dueShortcuts(nowMs: number): Shortcut[] {
  const today = todayBeijing(nowMs);
  const at = (day: Day) => formatValue({ ...day, ...DEFAULT_DUE_TIME }, true);
  const out: Shortcut[] = [];
  const bj = new Date(nowMs + BEIJING_OFFSET_MS);
  if (bj.getUTCHours() < DEFAULT_DUE_TIME.hh) out.push({ label: "今天 21:00", value: at(today) });
  const tomorrow = addDays(today, 1);
  out.push({ label: `明天 ${weekday(tomorrow)}`, value: at(tomorrow) });
  const dow = new Date(Date.UTC(today.y, today.m - 1, today.d)).getUTCDay();
  const toFriday = (5 - dow + 7) % 7;
  if (toFriday >= 2) {
    out.push({ label: dow === 0 || dow === 6 ? "下周五" : "本周五", value: at(addDays(today, toFriday)) });
  }
  const toNextMonday = (1 - dow + 7) % 7 || 7;
  if (toNextMonday > 1) out.push({ label: "下周一", value: at(addDays(today, toNextMonday)) });
  return out;
}

/** Hours 00–23 and minutes on the quarter hour, plus the value's own minute
 * when it is not on one (an AI-set 21:10 stays selectable as it is). */
export function minuteOptions(current: number | null): number[] {
  const base = [0, 15, 30, 45];
  if (current === null || base.includes(current)) return base;
  return [...base, current].sort((a, b) => a - b);
}

export const HOURS = Array.from({ length: 24 }, (_, i) => i);

export { pad2 };
