type BeanProps = { color?: string; size?: number };

// Ported from the Claude Design Bean.dc.html: a rounded "bean" body with two
// blinking eyes; eye color contrasts with the fill (light eyes on dark beans).
//
// `color` defaults to the design system's default accent (--mk-accent-500,
// vermilion #EA5140) instead of the old-blue #2A3B7A — a literal hex, not
// `var(--mk-accent-500)`, because the luminance contrast math right below
// needs a concrete color to sample; a CSS var string wouldn't parse and
// would silently fall through to the dark-eye fallback. Callers who want the
// bean to follow a different theme (e.g. a resolved per-branch color) pass
// their own hex via `color`.
//
// `eye`'s two contrast outcomes ("#FFFFFF" / "#17223B") stay plain hex
// literals rather than `var(--mk-surface)`/`var(--mk-ink)` — same call as
// `ui/Pebble.tsx`'s `EYE_FILL`: these are computed contrast values, not
// theme colors, and Bean.test.tsx asserts the exact fill string.
export function Bean({ color = "#EA5140", size = 38 }: BeanProps) {
  const m = /^#?([0-9a-f]{6})$/i.exec(color.trim());
  let eye = "#17223B";
  if (m) {
    const n = parseInt(m[1]!, 16);
    const r = (n >> 16) & 255, g = (n >> 8) & 255, b = n & 255;
    const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
    eye = lum < 0.55 ? "#FFFFFF" : "#17223B";
  }
  return (
    <svg viewBox="0 0 120 120" width={size} height={size} style={{ display: "block", overflow: "visible" }} aria-hidden="true">
      <g style={{ transformOrigin: "60px 60px" }}>
        <path
          d="M59.5 24.8 C75.7 23.7 93.3 31.9 98.9 47.2 C105.1 64.1 96.2 82.8 80.5 91.2 C64.6 99.7 42.4 96.4 30.2 83.4 C18.5 71.0 18.8 51.2 30.9 38.7 C37.8 31.6 48.1 25.6 59.5 24.8Z"
          fill={color}
        />
        <ellipse cx="49.4" cy="58.2" rx="2.6" ry="3.9" fill={eye} />
        <ellipse cx="68.3" cy="58.2" rx="2.6" ry="3.9" fill={eye} />
      </g>
    </svg>
  );
}
