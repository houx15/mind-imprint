// shortDate: ISO timestamp → YYYY-MM-DD. Locale-free and deterministic.
export function shortDate(iso: string): string {
  return iso.slice(0, 10);
}

// relativeTime: humanized activity relative to `now` (ms epoch). null → 从未.
// Caller passes Date.now(); the helper stays pure for deterministic tests.
export function relativeTime(iso: string | null, now: number): string {
  if (iso == null) return "从未";
  const then = Date.parse(iso);
  const diff = now - then;
  const MIN = 60_000, HOUR = 60 * MIN, DAY = 24 * HOUR;
  if (diff < MIN) return "刚刚";
  if (diff < HOUR) return `${Math.floor(diff / MIN)} 分钟前`;
  if (diff < DAY) return `${Math.floor(diff / HOUR)} 小时前`;
  if (diff < 7 * DAY) return `${Math.floor(diff / DAY)} 天前`;
  return iso.slice(0, 10);
}
