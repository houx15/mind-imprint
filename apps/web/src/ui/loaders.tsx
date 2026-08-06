import { Pebble } from "@/ui/Pebble";

/**
 * Pebble loaders (design-system foundation, Part 1 Task 11, spec §15).
 *
 * Three loading affordances built on the 豆豆 Pebble body
 * (`M20 6 C...Z`, always `var(--mk-accent-500)`, never a hardcoded color).
 * SVG markup, positions, and `@keyframes` are copied verbatim (see
 * `index.css`) from the authoritative mockups — do not re-derive the motion:
 * - `PebbleProgress` ← docs/design/design-system-mockups/pebble-progress-v2.html
 *   (`mk-pebble-fill` / `mk-pebble-ride`, rider rides ON TOP of the fill bar).
 * - `RabbitHoleLoader` ← docs/design/design-system-mockups/pebble-rabbithole-v2.html
 *   (`mk-pebble-hop`, ground hole rim/dark/豆豆/lip z-order 1/2/3/4 + carrot).
 * - `PebbleInlineSpinner` reuses the `mk-pebble-spin` keyframe already
 *   defined for the Pebble's `processing` ring (Task 10) — just a plain
 *   `rotate(360deg)` — at a slower linear 1s cadence via a dedicated class,
 *   rather than adding a near-duplicate keyframe.
 */

function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

const EYE_FILL = "#2A2724";

/** The rider/dou face: identical in the progress-v2 and rabbithole-v2 mockups
 * at full size (rx 2 / ry 2.6, with catchlights). The slim progress variant
 * uses its own slightly larger, catchlight-less eyes (rx 2.1 / ry 2.7). */
function RiderFace({ slim = false }: { slim?: boolean }) {
  return slim ? (
    <>
      <ellipse cx="16" cy="21" rx="2.1" ry="2.7" fill={EYE_FILL} />
      <ellipse cx="25" cy="20.4" rx="2.1" ry="2.7" fill={EYE_FILL} />
    </>
  ) : (
    <>
      <ellipse cx="16" cy="21" rx="2" ry="2.6" fill={EYE_FILL} />
      <circle cx="15.4" cy="20" r=".8" fill="#fff" />
      <ellipse cx="25" cy="20.4" rx="2" ry="2.6" fill={EYE_FILL} />
      <circle cx="24.4" cy="19.4" r=".8" fill="#fff" />
    </>
  );
}

const BODY_D =
  "M20 6 C 30 5 36.5 11 35.5 21 C 34.5 30 27 35.5 18.5 34.5 C 9.5 33.5 4.5 27 5.5 18 C 6.5 10 12 7 20 6 Z";

function RiderSvg({ slim = false }: { slim?: boolean }) {
  return (
    <svg viewBox="0 0 40 40" style={{ width: "100%", height: "100%", position: "relative" }} aria-hidden="true">
      <path d={BODY_D} fill="var(--mk-accent-500)" />
      <RiderFace slim={slim} />
    </svg>
  );
}

function clampPercent(value: number): number {
  if (Number.isNaN(value)) return 0;
  return Math.min(100, Math.max(0, value));
}

export interface PebbleProgressProps {
  /** 0–100 (clamped). Omit for indeterminate — the fill/rider loop forever. */
  value?: number;
  /** Thinner page-top variant: smaller bar + smaller 豆豆. */
  slim?: boolean;
}

/**
 * Accent fill bar with 豆豆 riding on top at the fill head, PvZ-style.
 * No `value` → indeterminate looping animation. With `value` → static fill
 * width + rider parked at the corresponding position, no animation classes.
 */
export function PebbleProgress({ value, slim = false }: PebbleProgressProps) {
  const indeterminate = value === undefined;
  const pct = indeterminate ? undefined : clampPercent(value);
  // Static rider position: interpolate between the mockup ride keyframe's
  // settled endpoints (v=0 → left:-4px, v=100 → left:calc(100% - riderW - 2px)).
  const riderWidth = slim ? 24 : 32;
  const offset0 = -4;
  const offset100 = -(riderWidth - 2);
  const riderLeft =
    pct === undefined
      ? undefined
      : `calc(${pct}% + ${(offset0 + (pct / 100) * (offset100 - offset0)).toFixed(2)}px)`;

  return (
    <div className={cx("mk-pebble-progress", slim && "mk-pebble-progress-slim")}>
      <div className="mk-pebble-progress-rail" />
      <div
        className={cx("mk-pebble-progress-fill", indeterminate && "mk-pebble-indeterminate")}
        style={indeterminate ? undefined : { width: `${pct}%` }}
        data-testid="pebble-progress-fill"
      />
      <div
        className={cx("mk-pebble-progress-rider", indeterminate && "mk-pebble-indeterminate")}
        style={indeterminate ? undefined : { left: riderLeft }}
        data-testid="pebble-progress-rider"
      >
        <div className="mk-pebble-progress-rider-shadow" />
        <RiderSvg slim={slim} />
      </div>
    </div>
  );
}

export interface PebbleInlineSpinnerProps {
  /** Diameter in px; 20–26, default 22. */
  size?: number;
}

/** 豆豆 rolling in place — a plain rotating wrapper around the shared `Pebble`. */
export function PebbleInlineSpinner({ size = 22 }: PebbleInlineSpinnerProps) {
  return (
    <span
      className="mk-pebble-inline-spin"
      style={{ width: size, height: size }}
      role="status"
      aria-label="加载中"
    >
      <Pebble size={size} />
    </span>
  );
}

export interface RabbitHoleLoaderProps {
  /** Caption under the scene; defaults to the mockup's copy. */
  caption?: string;
}

/** Ground hole + carrot + hopping 豆豆 that dives in and is occluded by the front lip. */
export function RabbitHoleLoader({ caption = "正在钻兔子洞…" }: RabbitHoleLoaderProps) {
  return (
    <div>
      <div className="mk-pebble-hole-scene">
        <div className="mk-pebble-hole-ground" />
        <span className="mk-pebble-hole-grass mk-pebble-hole-g1" />
        <span className="mk-pebble-hole-grass mk-pebble-hole-g2" />

        <svg className="mk-pebble-hole-carrot" viewBox="0 0 24 40" aria-hidden="true">
          <path d="M11 14 C 9 7 6 4 3.5 2.5 C 4.5 8 6.5 11.5 11 14 Z" fill="#6FB06A" />
          <path d="M11 14 C 11 6 11 2.5 11 0.5 C 13 4.5 13 10 11 14 Z" fill="#5FA35C" />
          <path d="M11 14 C 13 7 16.5 4 19 2.5 C 18 8 16 11.5 11 14 Z" fill="#7FB87A" />
          <path d="M6 14 L16 14 L12.4 36 C 11.4 39 10.6 39 9.6 36 Z" fill="#EE7A2E" />
          <path
            d="M8.4 19 L13.2 19 M8.9 24 L12.6 24 M9.4 29 L12 29"
            stroke="#D9631D"
            strokeWidth="1"
            strokeLinecap="round"
          />
        </svg>

        <div className="mk-pebble-hole-rim" />
        <div className="mk-pebble-hole-dark" />

        <div className="mk-pebble-hole-dou">
          <svg viewBox="0 0 40 40" style={{ width: "100%", height: "100%" }} aria-hidden="true">
            <path d={BODY_D} fill="var(--mk-accent-500)" />
            <RiderFace />
          </svg>
        </div>

        <div className="mk-pebble-hole-lip" />
      </div>
      <div className="mk-pebble-hole-caption">{caption}</div>
    </div>
  );
}
