import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";

/**
 * SplitPane — a horizontal two-pane layout with a draggable divider
 * (design-system primitive; studio writing stage, spec §2/§6).
 *
 * The left pane's width is a RATIO of the container (0–1), so the split holds
 * its proportion as the window resizes. Drag the divider (pointer) or focus it
 * and use ←/→ (keyboard, 2% steps) — the keyboard path is what jsdom tests
 * exercise, since jsdom has no layout for real pointer geometry. The ratio
 * persists to localStorage under `storageKey` when given.
 *
 * GOTCHA (design-system convention): Tailwind emits same-CSS-property utilities
 * in ALPHABETICAL class order, not className order — so every element applies
 * exactly ONE class per competing property. Widths here are inline styles, not
 * utilities, so they're exempt.
 */

/** Join truthy class fragments with a single space; drops falsy/empty ones
 * (local copy, per the ui/ convention — no shared `cx` export). */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export interface SplitPaneProps {
  left: ReactNode;
  right: ReactNode;
  /** Left-pane width as a fraction of the container, 0–1. Default 0.4. */
  defaultRatio?: number;
  /** Minimum px width enforced on BOTH panes while dragging. Default 220. */
  minPx?: number;
  /** localStorage key to persist the ratio across mounts. Omit → no persist. */
  storageKey?: string;
  className?: string;
  /** Accessible label for the divider handle. Default "调整栏宽". */
  dividerLabel?: string;
}

function clampRatio(r: number): number {
  if (Number.isNaN(r)) return 0.4;
  return Math.min(0.8, Math.max(0.2, r));
}

export function SplitPane({
  left,
  right,
  defaultRatio = 0.4,
  minPx = 220,
  storageKey,
  className,
  dividerLabel = "调整栏宽",
}: SplitPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [ratio, setRatio] = useState<number>(() => {
    if (storageKey) {
      try {
        const v = localStorage.getItem(storageKey);
        if (v != null) return clampRatio(parseFloat(v));
      } catch {
        /* best-effort; a blocked storage must never break the layout */
      }
    }
    return clampRatio(defaultRatio);
  });
  const [dragging, setDragging] = useState(false);

  const persist = useCallback(
    (r: number) => {
      if (!storageKey) return;
      try {
        localStorage.setItem(storageKey, r.toFixed(4));
      } catch {
        /* best-effort */
      }
    },
    [storageKey],
  );

  const commit = useCallback(
    (next: number) => {
      const c = clampRatio(next);
      setRatio(c);
      persist(c);
    },
    [persist],
  );

  // Pointer drag: translate the pointer's x within the container to a ratio,
  // enforcing minPx on both sides. Listeners live on window so a fast drag that
  // outruns the divider still tracks.
  useEffect(() => {
    if (!dragging) return;
    function onMove(e: PointerEvent) {
      const el = containerRef.current;
      if (!el) return;
      const rect = el.getBoundingClientRect();
      if (rect.width <= 0) return;
      const min = Math.min(0.8, Math.max(0.2, minPx / rect.width));
      const max = 1 - min;
      const raw = (e.clientX - rect.left) / rect.width;
      setRatio(Math.min(max, Math.max(min, raw)));
    }
    function onUp() {
      setDragging(false);
    }
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
    return () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
    };
  }, [dragging, minPx]);

  // Persist once a pointer drag ends (avoids a write per move frame).
  const wasDragging = useRef(false);
  useEffect(() => {
    if (wasDragging.current && !dragging) persist(ratio);
    wasDragging.current = dragging;
  }, [dragging, ratio, persist]);

  function onKeyDown(e: React.KeyboardEvent) {
    if (e.key === "ArrowLeft") {
      e.preventDefault();
      commit(ratio - 0.02);
    } else if (e.key === "ArrowRight") {
      e.preventDefault();
      commit(ratio + 0.02);
    }
  }

  const pct = `${(ratio * 100).toFixed(2)}%`;

  return (
    <div ref={containerRef} className={cx("flex h-full min-h-0 w-full", dragging && "select-none", className)}>
      <div className="flex min-h-0 min-w-0 flex-col" style={{ width: pct }}>
        {left}
      </div>
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label={dividerLabel}
        aria-valuenow={Math.round(ratio * 100)}
        aria-valuemin={20}
        aria-valuemax={80}
        tabIndex={0}
        onPointerDown={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onKeyDown={onKeyDown}
        className={cx(
          "group relative flex w-1.5 shrink-0 cursor-col-resize items-center justify-center",
          "focus-visible:outline-none",
        )}
      >
        {/* the visible hairline; thickens + turns accent on hover/drag/focus */}
        <span
          aria-hidden="true"
          className={cx(
            "h-full w-px transition-colors duration-[120ms] ease-mk",
            dragging ? "bg-mk-accent" : "bg-mk-border group-hover:bg-mk-accent group-focus-visible:bg-mk-accent",
          )}
        />
      </div>
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">{right}</div>
    </div>
  );
}
