// shared/deadline.ts — Beijing-time deadline helpers for the lite teacher
// end's assignment forms, plus the assignment status vocabulary.
//
// 🚨 No `Intl` time zone lookup and no reliance on the running machine's
// zone: the distroless production image has no tzdata (see
// memory/distroless-has-no-tzdata-2026-09-11.md — the same bug, once already
// paid for on the Go side). Beijing is a fixed +08:00 offset with no DST, so
// every conversion here is UTC epoch ± 8 hours read back with `getUTC*`.

const BEIJING_OFFSET_MS = 8 * 60 * 60 * 1000;

function pad2(n: number): string {
  return n < 10 ? `0${n}` : `${n}`;
}

const INPUT_RE = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/;

/**
 * `"2026-09-20T22:00"` (an HTML `datetime-local` value, always naive — no
 * offset, no seconds) → `"2026-09-20T22:00:00+08:00"`, read as Beijing time.
 * Malformed input (missing the time part, an empty string, an out-of-range
 * field) → `null`; the caller never sends a half-built ISO string to the
 * server.
 */
export function beijingInputToISO(v: string): string | null {
  const m = INPUT_RE.exec(v);
  if (!m) return null;
  const year = m[1] ?? "";
  const month = m[2] ?? "";
  const day = m[3] ?? "";
  const hour = m[4] ?? "";
  const minute = m[5] ?? "";
  const mo = Number(month);
  const d = Number(day);
  const h = Number(hour);
  const mi = Number(minute);
  if (mo < 1 || mo > 12 || d < 1 || d > 31 || h > 23 || mi > 59) return null;
  return `${year}-${month}-${day}T${hour}:${minute}:00+08:00`;
}

/**
 * The inverse direction, for pre-filling a `datetime-local` input from a
 * stored ISO deadline: any parseable ISO instant → the Beijing wall-clock
 * reading in `datetime-local` shape (`"YYYY-MM-DDTHH:mm"`).
 */
export function isoToBeijingInput(iso: string): string {
  const bj = new Date(new Date(iso).getTime() + BEIJING_OFFSET_MS);
  const yyyy = bj.getUTCFullYear();
  const mm = pad2(bj.getUTCMonth() + 1);
  const dd = pad2(bj.getUTCDate());
  const hh = pad2(bj.getUTCHours());
  const mi = pad2(bj.getUTCMinutes());
  return `${yyyy}-${mm}-${dd}T${hh}:${mi}`;
}

/** A deadline for display: `M月D日 HH:mm`, in Beijing time regardless of the
 * viewer's browser zone. */
export function formatDeadline(iso: string): string {
  const bj = new Date(new Date(iso).getTime() + BEIJING_OFFSET_MS);
  const month = bj.getUTCMonth() + 1;
  const day = bj.getUTCDate();
  const hh = pad2(bj.getUTCHours());
  const mi = pad2(bj.getUTCMinutes());
  return `${month}月${day}日 ${hh}:${mi}`;
}

// The wire statuses `liteassign.StatusWithReturn` (apps/api/internal/liteassign/status.go)
// derives — never stored, always computed from started/finished/dueAt/return/now.
export type AssignmentStatus =
  | "not_started"
  | "in_progress"
  | "done"
  | "done_late"
  | "overdue"
  | "returned"
  | "resubmitted";

/** UI copy per AGENTS.md 界面文案 rule 4 (已/待/中 pairs, not folksy prose) —
 * kept in lockstep with `liteassign.StatusLabel` on the Go side. */
export const STATUS_LABEL: Record<AssignmentStatus, string> = {
  not_started: "未开始",
  in_progress: "进行中",
  done: "已完成",
  done_late: "逾期完成",
  overdue: "已逾期",
  returned: "已退回",
  resubmitted: "已重新提交",
};
