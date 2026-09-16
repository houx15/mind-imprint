import { readableText } from "./readableText";
import type { SiteContent, SiteTheme } from "./types";

/**
 * 她的网站的公共零件。
 *
 * 🚨 `site/` 里不许 import 这个 app 的 UI kit。一旦用了 `Btn` / `Panel` / 任何
 * `mk-*` token，这一页就开始长得像做出它的那个产品。连 `cx` 都是在这里自己写
 * 一行，而不是从 `@/ui` 拿——那一个 import 就是这条规矩破的口子。
 */

export function cx(...xs: (string | false | null | undefined)[]): string {
  return xs.filter(Boolean).join(" ");
}

export interface LayoutProps {
  site: SiteContent;
  theme: SiteTheme;
  narrow: boolean;
  /** 她自己在预览 = true；访客打开公开链接 = false。只影响「这里还没写」这类
   *  提示要不要出现，不影响任何内容。 */
  editing: boolean;
  /** 她第三关生成的头图。空 = 她没要，版式各自决定这时候画什么。 */
  heroUrl?: string;
}

export const MONO = {
  fontFamily: 'ui-monospace,"SF Mono","PingFang SC",monospace',
} as const;

/** 墨色往纸色里兑：正文 45–85%，分隔线和淡底 20% 以下。
 *  🚨 只能用 `color-mix`，不能用 Tailwind 的透明度写法——`--st-ink` 是一个裸
 *  CSS 变量，`text-[var(--st-ink)]/60` 一行 CSS 都不会生成。 */
export function mix(amount: number): string {
  return `color-mix(in srgb, var(--st-ink) ${Math.round(amount * 100)}%, var(--st-paper))`;
}

export function textMix(amount: number): string {
  return `color-mix(in srgb, var(--st-readable-ink) max(${Math.round(amount * 100)}%, var(--st-text-minimum)), var(--st-paper))`;
}

/** 同上，但透明——给压在淡底上的线用。 */
export function hair(amount: number): string {
  return `color-mix(in srgb, var(--st-ink) ${Math.round(amount * 100)}%, transparent)`;
}

/**
 * 代替照片的双色版。
 *
 * 🚨 刻意不是「app 的封面渐变 + 中间一个图标」。中间带图标的方块是产品家具，
 * 也是原型那一版最像产品截图的一处。
 *
 * 它也不假装是一张照片，所以没有「这里还缺一张照片」的虚线框——那句话在原型里
 * 铺满整页，把一个还没做图片功能的产品说成一个她没交作业的页面。
 */
export function Plate({
  plate,
  height,
  radius = 0,
}: {
  plate: [string, string];
  height: number | string;
  radius?: number;
}) {
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

/* ── 头图 ─────────────────────────────────────────────────────────────── */

/** mulberry32——一个小得能读完的确定性随机数发生器。
 *  用它而不是 `Math.random()`：头图不能在她每敲一个字时重新洗一次牌。 */
function rng(seed: number) {
  let a = (seed >>> 0) + 0x6d2b79f5;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/**
 * 头图 — **这一页自己的画**。
 *
 * 🚨 由她的 `seed` 生成，而 seed 由她的名字派生。原型这里画的是一盏台灯和散在
 * 天上的螺丝——那是林知遥的故事，却画在每一个学生的页顶上。头图的职责是「在读
 * 到第一个字之前就说清这是谁的页面」，所以它必须随人不同；一张所有人共用的画
 * 做的恰恰是反面的事。
 *
 * 画的是抽象的夜空：一道地平线、一片星、几道弧、地平线上一组高低不同的形。每
 * 一样的数量、位置、色相都从 seed 来，所以同一个人每次都一样，两个人不会一样，
 * 而且它不讲任何人的具体故事——一个人的故事该由她自己写在页面上，不该由我们替
 * 她画在头顶。
 */
export function Banner({ height, seed }: { height: number; seed: number }) {
  const r = rng(seed || 1);

  // 色相在夜空的范围里挑，避免撞上白天的颜色。
  const h1 = Math.floor(200 + r() * 90); // 深蓝到紫
  const h2 = (h1 + 30 + Math.floor(r() * 40)) % 360;
  const warm = Math.floor(18 + r() * 26); // 地平线上的暖光
  const sky1 = `hsl(${h1} 38% 12%)`;
  const sky2 = `hsl(${h2} 32% 24%)`;
  const sky3 = `hsl(${warm} 42% 30%)`;
  const glow = `hsl(${warm} 78% 68%)`;

  const stars = Array.from({ length: 34 + Math.floor(r() * 26) }, () => ({
    x: (r() * 180).toFixed(2),
    y: (2 + r() * 42).toFixed(2),
    rad: [0.16, 0.24, 0.36][Math.floor(r() * 3)],
    o: (0.25 + r() * 0.55).toFixed(2),
  }));

  const arcs = Array.from({ length: 2 + Math.floor(r() * 3) }, () => {
    const cx0 = 20 + r() * 140;
    const rad = 18 + r() * 26;
    return { d: `M${(cx0 - rad).toFixed(1)} 44 A ${rad.toFixed(1)} ${rad.toFixed(1)} 0 0 1 ${(cx0 + rad).toFixed(1)} 44`, o: (0.14 + r() * 0.3).toFixed(2) };
  });

  // 地平线上一组高低不同的形。抽象，不是任何具体的东西。
  const shapes = Array.from({ length: 3 + Math.floor(r() * 4) }, () => {
    const w = 2 + r() * 7;
    return {
      x: 8 + r() * 160,
      w: w.toFixed(2),
      h: (4 + r() * 16).toFixed(2),
      round: r() > 0.6,
    };
  }).sort((a, b) => a.x - b.x);

  // 光源落在其中一个形上，给整张图一个重心。
  const anchor = shapes[Math.floor(r() * shapes.length)] ?? { x: 120 };

  return (
    <svg
      viewBox="0 0 180 60"
      // 贴着底边：地平线和站在上面的那些形是构图，任何裁切都得保住它们。
      preserveAspectRatio="xMidYMax slice"
      style={{ display: "block", width: "100%", height }}
      aria-hidden
    >
      <defs>
        <linearGradient id={`sky-${seed}`} x1="0" y1="0" x2="0.25" y2="1">
          <stop offset="0%" stopColor={sky1} />
          <stop offset="52%" stopColor={sky2} />
          <stop offset="100%" stopColor={sky3} />
        </linearGradient>
        <radialGradient id={`glow-${seed}`} cx={(anchor.x / 180).toFixed(3)} cy="0.72" r="0.46">
          <stop offset="0%" stopColor={glow} stopOpacity="0.72" />
          <stop offset="38%" stopColor={glow} stopOpacity="0.24" />
          <stop offset="100%" stopColor={glow} stopOpacity="0" />
        </radialGradient>
        <linearGradient id={`ground-${seed}`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#000" stopOpacity="0.18" />
          <stop offset="100%" stopColor="#000" stopOpacity="0.46" />
        </linearGradient>
      </defs>

      <rect width="180" height="60" fill={`url(#sky-${seed})`} />
      <rect width="180" height="60" fill={`url(#glow-${seed})`} />

      {stars.map((s, i) => (
        <circle key={i} cx={s.x} cy={s.y} r={s.rad} fill="#FFF3DE" opacity={s.o} />
      ))}

      <g stroke="#FFE8C4" fill="none" strokeWidth="0.18">
        {arcs.map((a, i) => (
          <path key={i} d={a.d} opacity={a.o} />
        ))}
      </g>

      <rect y="47.6" width="180" height="12.4" fill={`url(#ground-${seed})`} />

      <g fill="#0B0A12" opacity="0.72">
        {shapes.map((s, i) => (
          <rect
            key={i}
            x={s.x.toFixed(2)}
            y={(48 - Number(s.h)).toFixed(2)}
            width={s.w}
            height={s.h}
            rx={s.round ? Number(s.w) / 2 : 0.3}
          />
        ))}
      </g>
    </svg>
  );
}

/** 页面的底。四个变量在这里设一次，所有版式都只读它们。 */
export function Ground({
  theme,
  children,
}: {
  theme: SiteTheme;
  children: React.ReactNode;
}) {
  const text = readableText(theme.ink, theme.paper);
  return (
    <div
      className="mk-site min-h-full"
      style={
        {
          "--st-paper": theme.paper,
          "--st-ink": theme.ink,
          "--st-readable-ink": text.ink,
          "--st-text-minimum": `${text.minimum}%`,
          "--st-accent": theme.accent,
          background: theme.paper,
          color: text.ink,
          fontFamily: theme.font,
        } as React.CSSProperties
      }
    >
      {children}
    </div>
  );
}

/**
 * 她还没写的地方。
 *
 * 只在**她自己**预览时出现（`editing`），访客看到的公开页面上永远不会有——
 * 一个公开页面上写着「这里还没写」，是把她的页面变成一张待办清单。
 *
 * 这是「不假装」那条规则的另一半：空的地方明说是空的，不用示例内容填上。
 */
export function Blank({ what, editing }: { what: string; editing: boolean }) {
  if (!editing) return null;
  return (
    <span
      className="text-[12px]"
      style={{ ...MONO, color: textMix(0.38), border: `1px dashed ${hair(0.24)}`, padding: "1px 6px" }}
    >
      {what}
    </span>
  );
}
