export interface ResolveOpts { timeoutMs?: number; intervalMs?: number; }

/** Resolve a tour anchor selector, polling briefly since the element may mount after
 *  navigation. Resolves null on timeout — callers degrade gracefully (never throw). */
export function resolveAnchor(selector: string, opts: ResolveOpts = {}): Promise<HTMLElement | null> {
  const { timeoutMs = 2000, intervalMs = 50 } = opts;
  const immediate = document.querySelector<HTMLElement>(selector);
  if (immediate) return Promise.resolve(immediate);
  return new Promise((resolve) => {
    let elapsed = 0;
    const timer = setInterval(() => {
      const el = document.querySelector<HTMLElement>(selector);
      if (el) { clearInterval(timer); resolve(el); return; }
      elapsed += intervalMs;
      if (elapsed >= timeoutMs) { clearInterval(timer); resolve(null); }
    }, intervalMs);
  });
}
