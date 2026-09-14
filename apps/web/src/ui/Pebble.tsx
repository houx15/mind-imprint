import { useCompanionImage } from "./CompanionAppearance";
/**
 * 豆豆 Pebble — the AI role's character (design-system foundation, spec §14).
 *
 * One inline SVG; the body follows the student's accent (`var(--mk-accent-500)`,
 * never a hardcoded color). Geometry, eye placement, and the four state
 * keyframes are copied verbatim from the authoritative mockup
 * `docs/design/design-system-mockups/pebble-states.html` — do not re-derive
 * the motion values here (see `index.css` for `mk-pebble-blink` /
 * `mk-pebble-pulse` / `mk-pebble-dart` / `mk-pebble-spin` / `mk-pebble-pop`).
 *
 * Kept as a component (not a static .svg) because a static file would lose:
 * (1) accent theming, (2) state switching via props, (3) future data-driven
 * variants (progress rider, etc).
 */

export type PebbleState = "idle" | "thinking" | "generating" | "processing" | "done";

export interface PebbleProps {
  state?: PebbleState;
  /** Diameter in px. At <=28px eyes enlarge for legibility and drop catchlights (spec §14). */
  size?: number;
}

const BODY_D =
  "M20 6 C 30 5 36.5 11 35.5 21 C 34.5 30 27 35.5 18.5 34.5 C 9.5 33.5 4.5 27 5.5 18 C 6.5 10 12 7 20 6 Z";

const EYE_FILL = "#2A2724";

function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export function Pebble({ state = "idle", size = 28 }: PebbleProps) {
  const image = useCompanionImage();
  if (image) return <img src={image} alt="" aria-hidden="true" className={`mk-companion mk-companion--${state}`} width={size} height={size} style={{ width: size, height: size, objectFit: "contain", flexShrink: 0 }} />;
  const compact = size <= 28;
  const rx = compact ? 2.1 : 1.9;
  const ry = compact ? 2.7 : 2.5;
  const withCatchlights = !compact;

  return (
    <svg
      className={cx("mk-pebble", state === "done" && "mk-pebble-done")}
      width={size}
      height={size}
      viewBox="0 0 40 40"
      style={{ overflow: "visible" }}
      aria-hidden="true"
    >
      {state === "thinking" && (
        <circle
          className="mk-pebble-ring"
          cx="20"
          cy="20"
          r="17.5"
          fill="none"
          stroke="var(--mk-accent-500)"
          strokeWidth="2"
        />
      )}

      <path d={BODY_D} fill="var(--mk-accent-500)" />

      {state === "processing" && (
        <circle
          className="mk-pebble-ring2"
          cx="20"
          cy="20"
          r="18.5"
          fill="none"
          stroke="var(--mk-accent-500)"
          strokeWidth="2.4"
          strokeLinecap="round"
          strokeDasharray="26 92"
        />
      )}

      {state === "done" ? (
        <>
          <path
            d="M13.6 21.5 Q16 18.8 18.4 21.5"
            stroke={EYE_FILL}
            strokeWidth="1.7"
            fill="none"
            strokeLinecap="round"
          />
          <path
            d="M22.6 21 Q25 18.3 27.4 21"
            stroke={EYE_FILL}
            strokeWidth="1.7"
            fill="none"
            strokeLinecap="round"
          />
          <path
            d="M31 8 l1 2.6 2.6 1 -2.6 1 -1 2.6 -1 -2.6 -2.6 -1 2.6 -1 z"
            fill="#E0A63A"
          />
        </>
      ) : state === "generating" ? (
        <g className="mk-pebble-eyes">
          <ellipse cx="16" cy="21" rx={rx} ry={ry} fill={EYE_FILL} />
          {withCatchlights && <circle cx="15.4" cy="20" r=".75" fill="#fff" />}
          <ellipse cx="25" cy="20.4" rx={rx} ry={ry} fill={EYE_FILL} />
          {withCatchlights && <circle cx="24.4" cy="19.4" r=".75" fill="#fff" />}
        </g>
      ) : state === "processing" ? (
        <>
          {/* Mockup's proc block omits catchlights at every size (unlike
              idle/thinking/generating, which keep them above 28px). */}
          <ellipse cx="16" cy="21" rx={rx} ry={ry} fill={EYE_FILL} />
          <ellipse cx="25" cy="20.4" rx={rx} ry={ry} fill={EYE_FILL} />
        </>
      ) : state === "thinking" ? (
        <>
          <ellipse className="mk-pebble-eye" cx="16" cy="19" rx={rx} ry={ry} fill={EYE_FILL} />
          {withCatchlights && <circle cx="15.4" cy="18" r=".75" fill="#fff" />}
          <ellipse className="mk-pebble-eye" cx="25" cy="18.6" rx={rx} ry={ry} fill={EYE_FILL} />
          {withCatchlights && <circle cx="24.4" cy="17.6" r=".75" fill="#fff" />}
        </>
      ) : (
        <>
          <ellipse cx="16" cy="21" rx={rx} ry={ry} fill={EYE_FILL} />
          {withCatchlights && <circle cx="15.4" cy="20" r=".75" fill="#fff" />}
          <ellipse cx="25" cy="20.4" rx={rx} ry={ry} fill={EYE_FILL} />
          {withCatchlights && <circle cx="24.4" cy="19.4" r=".75" fill="#fff" />}
        </>
      )}
    </svg>
  );
}
