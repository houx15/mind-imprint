/** The − / + result for a number box holding `value`, or null when the button
 * has nothing to do. An empty or unreadable box starts at `min` (or 0). A
 * value outside [min, max] moves to the nearer end, and only in the
 * direction pressed. */
export function stepNumber(value: string, delta: number, min?: number, max?: number): string | null {
  const n = Number(value);
  const lo = min ?? -Infinity;
  const hi = max ?? Infinity;
  if (value.trim() === "" || !Number.isFinite(n)) return String(min ?? 0);
  if (n < lo) return delta > 0 ? String(lo) : null;
  if (n > hi) return delta < 0 ? String(hi) : null;
  const next = Math.min(Math.max(n + delta, lo), hi);
  return next === n ? null : String(next);
}
