// shortDate: ISO timestamp → YYYY-MM-DD. Locale-free and deterministic.
export function shortDate(iso: string): string {
  return iso.slice(0, 10);
}
