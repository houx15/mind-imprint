// teacher/weekNav.ts — week navigation for the weekly summary pages.
//
// A week is named by its Beijing Monday as a calendar date (`YYYY-MM-DD`, the
// API's `weekStart`). Shifting is arithmetic on that date only: it is built in
// UTC and read back in UTC, so the browser's own time zone never enters.

/** `weekStart` moved by `days` calendar days (±7 for the previous/next week). */
export function shiftWeek(weekStart: string, days: number): string {
  const [y, m, d] = weekStart.split("-").map(Number);
  const t = new Date(Date.UTC(y ?? 1970, (m ?? 1) - 1, d ?? 1));
  t.setUTCDate(t.getUTCDate() + days);
  const mm = String(t.getUTCMonth() + 1).padStart(2, "0");
  const dd = String(t.getUTCDate()).padStart(2, "0");
  return `${t.getUTCFullYear()}-${mm}-${dd}`;
}

/** The server's title split so the week label can be kept on one line:
 * `上周班级周报 · 第 37 周（9.7–9.13）` → head `上周班级周报 · `, label
 * `第 37 周（9.7–9.13）`. The title is never rebuilt here; a title that does
 * not end with the label comes back whole as `head`. */
export function splitWeekTitle(title: string, weekLabel: string): { head: string; label: string } {
  if (weekLabel !== "" && title.endsWith(weekLabel)) {
    return { head: title.slice(0, title.length - weekLabel.length), label: weekLabel };
  }
  return { head: title, label: "" };
}

/** Whether 下一周 is available. The latest completed week has no next week:
 * the week after it has not ended, and the API rejects it. */
export function canGoNext(weekStart: string, isLatest: boolean): boolean {
  return weekStart !== "" && !isLatest;
}
