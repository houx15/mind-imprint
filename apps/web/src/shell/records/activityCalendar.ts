const SWATCHES = ["#EDEFF4", "#C9D0E8", "#97A3D2", "#5C6CB0", "#2A3B7A"];
const WEEKS = 17;
const DAYS = WEEKS * 7; // 119

function dayKey(d: Date): string {
  return d.toISOString().slice(0, 10);
}
function intensity(count: number): number {
  if (count <= 0) return 0;
  if (count === 1) return 1;
  if (count <= 3) return 2;
  if (count <= 5) return 3;
  return 4;
}

export interface CalendarCell { style: string; }
export interface ActivityCalendar { cells: CalendarCell[]; activeDays: number; weeks: number; }

export function deriveActivityCalendar(
  events: { created_at: string }[],
  now: Date,
): ActivityCalendar {
  const counts = new Map<string, number>();
  for (const e of events) {
    const k = e.created_at.slice(0, 10);
    counts.set(k, (counts.get(k) ?? 0) + 1);
  }
  const cells: CalendarCell[] = [];
  let activeDays = 0;
  const today = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  // Oldest cell first so column-major fill matches the design grid.
  for (let i = DAYS - 1; i >= 0; i--) {
    const d = new Date(today);
    d.setUTCDate(today.getUTCDate() - i);
    const c = counts.get(dayKey(d)) ?? 0;
    if (c > 0) activeDays++;
    const color = SWATCHES[intensity(c)]!;
    cells.push({ style: `width:13px; height:13px; border-radius:3px; background:${color};` });
  }
  return { cells, activeDays, weeks: WEEKS };
}
