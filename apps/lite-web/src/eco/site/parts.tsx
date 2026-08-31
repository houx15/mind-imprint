import type { SiteContent, SiteTheme } from "../data/site";

/**
 * Shared bits of 她的网站.
 *
 * 🚨 Nothing in `site/` may import the app's UI kit. The moment this page uses
 * `Btn`, `Panel` or an `mk-*` token it starts looking like the product that
 * made it, which is the one thing a personal site must not look like. Colours
 * come only from the four `--st-*` variables the chosen 方案 sets.
 */

export interface LayoutProps {
  site: SiteContent;
  theme: SiteTheme;
  /** 1 · 2 · 3 — 印记's build rounds, made visible. See `BuiltSite`. */
  stage: 1 | 2 | 3;
  narrow: boolean;
}

export const MONO = {
  fontFamily: 'ui-monospace,"SF Mono","PingFang SC",monospace',
} as const;

/** Ink mixed toward paper: text at 45–85%, hairlines and washes below 20%.
 *  🚨 `color-mix`, never a Tailwind alpha modifier — `--st-ink` is a bare CSS
 *  variable, and `text-[var(--st-ink)]/60` silently emits no CSS at all. */
export function mix(amount: number): string {
  return `color-mix(in srgb, var(--st-ink) ${Math.round(amount * 100)}%, var(--st-paper))`;
}

/** Same, but transparent — for rules that sit over a wash. */
export function hair(amount: number): string {
  return `color-mix(in srgb, var(--st-ink) ${Math.round(amount * 100)}%, transparent)`;
}

/**
 * A photograph she has not taken yet.
 *
 * Before the last round these are honestly empty: 印记 admits 「那张照片得你拍，
 * 我没有」, and an empty frame on the page is what makes that admission
 * checkable rather than a sentence she has to trust.
 *
 * The filled version is a two-colour wash with a soft light — deliberately not
 * a tile with a glyph in the middle, which is app furniture and was the single
 * biggest reason this page used to read as a product screen.
 */
export function Plate({
  plate,
  height,
  filled,
  radius = 0,
  label,
}: {
  plate: [string, string];
  height: number | string;
  filled: boolean;
  radius?: number;
  label?: string;
}) {
  if (!filled) {
    return (
      <div
        className="flex items-center justify-center text-[12px]"
        style={{
          height,
          borderRadius: radius,
          border: `1px dashed ${hair(0.26)}`,
          color: mix(0.45),
        }}
      >
        {label ?? "这里还缺一张照片"}
      </div>
    );
  }
  return (
    <div
      style={{
        height,
        borderRadius: radius,
        background: `radial-gradient(120% 100% at 22% 12%, ${plate[0]} 0%, ${plate[1]} 72%)`,
      }}
      aria-hidden
    />
  );
}

/**
 * 头图 — the banner.
 *
 * 🚨 This is **the site's own artwork**, not a crop of one of her projects.
 * On every classic personal blog the banner is a picture chosen or drawn for
 * the site — a cat in a purple galaxy, a canyon at dusk — and it is doing a
 * different job from a portfolio thumbnail: it is the thing that says whose
 * page this is before a single word is read. Putting a project image there
 * makes the top of the page look like the first row of a list.
 *
 * Drawn rather than photographed, because she has no photograph and a stock
 * one would be somebody else's. It is her subject matter: a lamp still lit,
 * loose screws around it, in a dusk sky.
 */
export function Banner({ height, ink }: { height: number; ink: string }) {
  // Deterministic scatter — `Math.random()` here would reshuffle the sky on
  // every keystroke in the workbench.
  const screws = Array.from({ length: 46 }, (_, i) => {
    const a = Math.sin(i * 12.9898) * 43758.5453;
    const b = Math.sin(i * 78.233) * 12345.6789;
    return {
      x: ((a - Math.floor(a)) * 180).toFixed(2),
      y: (2 + (b - Math.floor(b)) * 44).toFixed(2),
      r: [0.18, 0.26, 0.38][i % 3],
      o: (0.3 + ((i * 13) % 6) * 0.1).toFixed(2),
    };
  });

  return (
    <svg
      viewBox="0 0 180 60"
      // Anchored to the bottom: the horizon and the lamp standing on it are
      // the composition, and they must survive every crop.
      preserveAspectRatio="xMidYMax slice"
      style={{ display: "block", width: "100%", height }}
      aria-hidden
    >
      <defs>
        <linearGradient id="sky" x1="0" y1="0" x2="0.25" y2="1">
          <stop offset="0%" stopColor="#1B1B33" />
          <stop offset="46%" stopColor="#382C4C" />
          <stop offset="100%" stopColor="#6E4536" />
        </linearGradient>
        <radialGradient id="lamp" cx="0.79" cy="0.42" r="0.42">
          <stop offset="0%" stopColor="#FFD79A" stopOpacity="0.85" />
          <stop offset="34%" stopColor="#E8A15C" stopOpacity="0.3" />
          <stop offset="100%" stopColor="#E8A15C" stopOpacity="0" />
        </radialGradient>
        <linearGradient id="ground" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={ink} stopOpacity="0.05" />
          <stop offset="26%" stopColor={ink} stopOpacity="0.2" />
          <stop offset="100%" stopColor={ink} stopOpacity="0.3" />
        </linearGradient>
        <radialGradient id="pool" cx="0.5" cy="0.5" r="0.5">
          <stop offset="0%" stopColor="#FFD79A" stopOpacity="0.3" />
          <stop offset="100%" stopColor="#FFD79A" stopOpacity="0" />
        </radialGradient>
      </defs>

      <rect width="180" height="60" fill="url(#sky)" />
      <rect width="180" height="60" fill="url(#lamp)" />

      {/* 螺丝散在天上 */}
      {screws.map((s, i) => (
        <circle key={i} cx={s.x} cy={s.y} r={s.r} fill="#FFF3DE" opacity={s.o} />
      ))}

      {/* 中间：一枚垫圈和两道弧，让手机上的裁切里也有东西 */}
      <g stroke="#FFE8C4" fill="none" opacity="0.34">
        <circle cx="62" cy="26" r="5.4" strokeWidth="0.34" />
        <circle cx="62" cy="26" r="2.1" strokeWidth="0.34" />
        <path d="M40 40 A 26 26 0 0 1 78 12" strokeWidth="0.2" opacity="0.55" />
        <path d="M34 44 A 32 32 0 0 1 82 8" strokeWidth="0.16" opacity="0.34" />
      </g>

      {/* 地面 */}
      <rect y="48.2" width="180" height="11.8" fill="url(#ground)" />
      <ellipse cx="136" cy="50.6" rx="20" ry="3.4" fill="url(#pool)" />

      {/* 台灯：罩、杆、底座，只画轮廓，站在地平线上 */}
      <g
        stroke="#FFE8C4"
        strokeWidth="0.4"
        fill="none"
        opacity="0.9"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M124.4 20.6 L138.6 16.6 L142.4 25.4 L127.8 29.2 Z" />
        <path d="M135.6 28 L139.4 49.4" />
        <path d="M133.6 49.6 L146.6 49.6" />
        <path d="M128.8 30.4 L124.2 39" strokeWidth="0.2" opacity="0.5" />
        <path d="M137.4 30 L139 40" strokeWidth="0.2" opacity="0.42" />
      </g>
    </svg>
  );
}

/** The page ground. Sets the four variables every layout reads. */
export function Ground({
  theme,
  children,
}: {
  theme: SiteTheme;
  children: React.ReactNode;
}) {
  return (
    <div
      className="eco-site min-h-full"
      style={
        {
          "--st-paper": theme.paper,
          "--st-ink": theme.ink,
          "--st-accent": theme.accent,
          background: theme.paper,
          color: theme.ink,
          fontFamily: theme.font,
        } as React.CSSProperties
      }
    >
      {children}
    </div>
  );
}
