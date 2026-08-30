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
  /** 我写的 — her finished piece, read start to finish rather than glanced at.
   *  Body weight and generous leading, matching `PROSE_TYPOGRAPHY`'s 17px/1.9
   *  in the writing room.
   *
   *  🚨 NOT the writing room's 17px/1.9. This is the read-it-through size, and
   *  it is set the way a newspaper or a good blog sets long-form: bigger than
   *  chrome, with air between the lines. 19px at 2.0 gives roughly 33 Chinese
   *  characters a line at this measure — the band CJK body text is actually
   *  comfortable in. The composition surface stays at 17px because writing and
   *  reading are different jobs. */
  "mk-report-piece": ["19px", { lineHeight: "2", fontWeight: "400" }],
  /** An article's own title, on the page that presents her piece as an
   *  article. Distinct from `mk-report-hero`: that one sits inside the
   *  report's coloured masthead and is built to be screenshotted, this one is
   *  just type on a page — 「don't make the title a block」 — so it is set in
   *  the same serif as the prose it belongs to. */
  "mk-report-title": ["34px", { lineHeight: "1.35", fontWeight: "700" }],
} as const;

/**
 * The reading face for her finished piece — the ONE place in either app that
 * sets long-form prose someone reads start to finish.
 *
 * Serif, because that is what long-form reading looks like everywhere it is
 * done well, and because it separates HER ARTICLE from the product's UI at a
 * glance: everything else on the page is the interface talking, this is her.
 *
 * The stack is ordered for CJK first, then Latin, then the generic — a
 * Chinese essay must not fall back to a Latin serif rendering Chinese in the
 * system sans (which is what a bare `Georgia, serif` does). Songti SC ships on
 * macOS/iOS and SimSun on Windows, so the common cases are covered without a
 * webfont; Noto/Source Han are named first for anyone who has them.
 */
const pieceSerif =
  '"Noto Serif SC", "Source Han Serif SC", "Songti SC", "SimSun", "Georgia", "Times New Roman", serif';

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
      fontFamily: {
        ...(base.theme?.extend?.fontFamily ?? {}),
        "mk-piece": [pieceSerif],
      },
    },
  },
} satisfies Config;
