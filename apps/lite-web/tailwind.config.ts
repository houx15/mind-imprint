import type { Config } from "tailwindcss";
import base from "../web/tailwind.config";

/**
 * Lite's Tailwind config = the shared `mk` theme, plus the few tokens that
 * only lite has a surface for.
 *
 * ## Why the extra fontSize entries live HERE and not in the base config
 *
 * `apps/web/tailwind.config.ts` is pro's config; lite must never change how
 * pro renders. Adding a token there would be additive and probably harmless,
 * but "probably harmless" is not the standard for a file two products
 * compile against. Spreading base and extending here gets lite its own
 * tokens with zero reach into pro.
 *
 * ## Why report type needed tokens at all
 *
 * The base scale tops out at `mk-display` (32px) and is built for CHROME —
 * panel headings, labels, body copy. The report is a poster: a title meant to
 * be screenshotted, numerals meant to be read across a room, a 收获 meant to
 * be the loudest sentence on the page. Those sizes were briefly written as
 * raw `font-size` in `.mk-rp-*` CSS and one arbitrary `text-[42px]`, which is
 * exactly the drift the token system exists to prevent — so they are named
 * here, and `.mk-rp-*` in `index.css` went back to owning layout and motion
 * only.
 *
 * The precedent is `mk-prose` in the base config, added for composition
 * surfaces with the same reasoning: page type is not chrome type.
 */
const reportType = {
  /** The report's own title. Screenshotted, so it outranks `mk-display`. */
  "mk-report-hero": ["40px", { lineHeight: "1.2", fontWeight: "700" }],
  /** A stat tile's numeral. `lineHeight: 1` — the tile packs it against its label. */
  "mk-report-stat": ["32px", { lineHeight: "1", fontWeight: "700" }],
  /** 我的收获 — the loudest sentence on the page. */
  "mk-report-lede": ["24px", { lineHeight: "1.7", fontWeight: "600" }],
  /** A 金句 pull-quote, and the questions that grow out of a finished reading. */
  "mk-report-quote": ["19px", { lineHeight: "1.75", fontWeight: "600" }],
  /** Decorative numerals: a gain's index, a quote mark. */
  "mk-report-numeral": ["26px", { lineHeight: "1.1", fontWeight: "700" }],
} as const;

export default {
  ...base,
  // 房间组件的 class 写在 apps/web 里。漏掉这一条，阅读室会渲染成没有样式的裸 DOM。
  content: ["./index.html", "./src/**/*.{ts,tsx}", "../web/src/**/*.{ts,tsx}"],
  theme: {
    ...base.theme,
    extend: {
      ...base.theme?.extend,
      fontSize: {
        ...(base.theme?.extend?.fontSize ?? {}),
        ...reportType,
      },
    },
  },
} satisfies Config;
