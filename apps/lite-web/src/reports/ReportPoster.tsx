import { forwardRef } from "react";
import type { LiteReport } from "@lite/api/reports";
import { displayStat } from "./statLabels";

/**
 * ReportPoster — the picture she can send someone, not a shrunk copy of
 * `ReportView`. The product owner's own words: "should not be verbose, but
 * be good looking … like lark meeting notes they would conclude some 金句
 * … with student's name and effort be noted." So the content is a fixed,
 * short list — her name, the title, the date, the stats as large numerals,
 * and up to three 金句 given real room — and nothing else: no kind label,
 * no brand mark, no share chrome, no 收获 list. Restraint is the point.
 *
 * ## Offscreen, not hidden — and the offset goes on the WRAPPER
 *
 * 🚨 This component renders TWO elements: an outer wrapper that carries
 * `position:fixed; left:-99999px`, and the poster itself, which is
 * `position:static` and is what the ref (and therefore `exportPoster`) points
 * at. Do not collapse them back into one node.
 *
 * The offscreen offset used to live on the rasterized node itself, and it
 * produced a correctly-sized, **completely blank** PNG. html-to-image works by
 * cloning the node, inlining its computed style, and dropping the clone into
 * an SVG `<foreignObject>` sized to the node — so `left:-99999px` came along
 * for the ride and positioned the clone 99999px outside its own viewport.
 * Nothing painted, `toPng` resolved successfully, and the browser downloaded
 * a blank sheet. A wrapper keeps the page-level offset off the clone.
 *
 * The offset is still `position:fixed`, not `display:none`: a `display:none`
 * node has no layout box, and html-to-image walks real geometry, so that
 * would rasterize blank too — for a different reason.
 *
 * ## System fonts, explicit colours
 *
 * html-to-image rasterizes through an SVG `<foreignObject>`, which never
 * loads a web font — it would fall back silently mid-export and the picture
 * would differ from whatever she previewed. `FONT_STACK` below is the
 * platform CJK stack only. Colours are explicit hex, copied from the same
 * `mk-*` macaron palette `apps/web/src/index.css` defines (so the poster
 * still reads as the same product), rather than `var(--mk-…)` custom
 * properties: this is a separate rasterization context from the page's own
 * cascade, and an explicit value removes a class of "did the variable
 * actually resolve at capture time" failure for no visible cost. This also
 * sidesteps the unrelated `mk-*`-as-Tailwind-alpha trap (`bg-mk-x/NN` emits
 * no CSS at all, since these are bare custom properties) by never going
 * through a Tailwind class for colour on this component at all.
 */

const FONT_STACK =
  '-apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif';

const INK = "#33302E";
const MUTED = "#8A827A";
const PAPER = "#FBF8F4";

/** Copied from `apps/web/src/index.css`'s `--mk-*` macaron tokens — see the
 *  file-level comment above for why these are hex literals, not `var()`. */
const MACARON = [
  { bg: "#FDE7D3", fg: "#9A5A22" }, // peach
  { bg: "#E0F0EC", fg: "#177368" }, // lake
  { bg: "#FCE7EB", fg: "#A63A50" }, // berry
  { bg: "#E7F1DD", fg: "#4D6B3A" }, // matcha
  { bg: "#F0EAF6", fg: "#5B4A80" }, // taro
  { bg: "#FAF3DE", fg: "#8A6320" }, // butter
  { bg: "#E6EEF9", fg: "#3C5A86" }, // mist
] as const;

/** How many stat tiles fit on one row of a 1080-wide picture. See the
 *  `slice` in the component for why exceeding it is not merely ugly. */
const MAX_POSTER_STATS = 4;

function macaron(i: number) {
  // `i % MACARON.length` is always a valid index into a non-empty literal
  // array — the `?? MACARON[0]` only satisfies noUncheckedIndexedAccess.
  return MACARON[i % MACARON.length] ?? MACARON[0];
}

/** Absolute date, matching `ReportView`'s own `formatDate` — this picture
 *  can be read weeks later by someone who wasn't there. */
function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日`;
}

export const ReportPoster = forwardRef<HTMLDivElement, { report: LiteReport }>(
  function ReportPoster({ report }, ref) {
    const date = formatDate(report.finishedAt);
    // At most three 金句 — "given real space", not shrunk to fit more in.
    const moments = report.moments.slice(0, 3);
    // Same "absent rather than empty" rule as ReportView's StatStrip: a stat
    // of 0 is absence, not a fact worth putting in the picture she sends to
    // a parent. No row at all when nothing survives.
    //
    // Capped at MAX_POSTER_STATS, unlike the page, which shows every one. The
    // page can grow downwards; this is a fixed 1080×1440 box, and the server
    // now sends seven reading stats — enough to wrap the strip onto a second
    // row and push the 金句 off the bottom edge, silently, with `overflow:
    // hidden` swallowing the evidence. The first four are the first four the
    // server emits (time, volume, conversation, marks), which is the order
    // that survives a crop best.
    // Same client-side label resolution as the page — a stored report carries
    // whatever wording it was generated with (see statLabels.ts).
    const stats = report.stats
      .filter((stat) => stat.value !== 0)
      .slice(0, MAX_POSTER_STATS)
      .map(displayStat);

    return (
      // The wrapper holds the offscreen offset; the poster below is static and
      // is what gets rasterized. See this file's "Offscreen" section — merging
      // these two nodes is what produced a blank PNG.
      <div aria-hidden="true" style={{ position: "fixed", left: -99999, top: 0 }}>
      <div
        ref={ref}
        style={{
          position: "static",
          width: 1080,
          height: 1440,
          overflow: "hidden",
          boxSizing: "border-box",
          display: "flex",
          flexDirection: "column",
          gap: 44,
          padding: 76,
          // Layered radial washes over paper rather than a separate banner
          // element: same warm light as the page's own hero, with zero effect
          // on layout inside a box whose height is fixed and whose overflow is
          // hidden. Explicit hex, no `var()` and no `color-mix()` — see the
          // file comment on why this rasterization context takes literals.
          background: `radial-gradient(90% 60% at 6% 0%, #FDE7D3 0%, rgba(253,231,211,0) 60%),
            radial-gradient(80% 55% at 96% 4%, #E0F0EC 0%, rgba(224,240,236,0) 62%), ${PAPER}`,
          fontFamily: FONT_STACK,
        }}
      >
        <header style={{ display: "flex", flexDirection: "column", gap: 18 }}>
          <h1
            style={{
              margin: 0,
              fontSize: 58,
              lineHeight: 1.35,
              fontWeight: 700,
              color: INK,
            }}
          >
            {report.title}
          </h1>
          <p style={{ margin: 0, fontSize: 30, color: MUTED }}>
            {report.studentName}
            {date && ` · ${date}`}
          </p>
        </header>

        {stats.length > 0 && (
          <div style={{ display: "flex", flexWrap: "wrap", gap: 22 }}>
            {stats.map((stat, i) => {
              const { bg, fg } = macaron(i);
              return (
                <div
                  key={stat.key}
                  style={{
                    flex: "1 1 200px",
                    minWidth: 200,
                    borderRadius: 28,
                    background: bg,
                    padding: "30px 34px",
                    display: "flex",
                    flexDirection: "column",
                    gap: 8,
                  }}
                >
                  {/* Baseline flex with nowrap, and grouped digits — the same
                      two fixes the page's own tiles needed. Without them
                      「3428 字」 rendered as a bare 3428 with 字 dropped onto a
                      second line, reading as two unrelated facts stacked. */}
                  <span
                    style={{
                      display: "flex",
                      alignItems: "baseline",
                      gap: 4,
                      flexWrap: "nowrap",
                      fontSize: 54,
                      lineHeight: 1,
                      fontWeight: 700,
                      color: fg,
                    }}
                  >
                    {stat.value.toLocaleString("zh-CN")}
                    {stat.unit && <span style={{ fontSize: 26, fontWeight: 500 }}>{stat.unit}</span>}
                  </span>
                  <span style={{ fontSize: 24, color: fg }}>{stat.label}</span>
                </div>
              );
            })}
          </div>
        )}

        {moments.length > 0 && (
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              gap: 26,
              flex: 1,
              minHeight: 0,
            }}
          >
            {moments.map((m, i) => {
              const { bg, fg } = macaron(i + 2);
              return (
                <blockquote
                  key={`${m.where}-${i}`}
                  style={{
                    margin: 0,
                    flex: 1,
                    minHeight: 0,
                    borderRadius: 28,
                    background: bg,
                    padding: "34px 42px",
                    display: "flex",
                    flexDirection: "column",
                    justifyContent: "center",
                    gap: 14,
                  }}
                >
                  <p style={{ margin: 0, fontSize: 36, lineHeight: 1.6, fontWeight: 500, color: INK }}>
                    “{m.quote}”
                  </p>
                  <footer style={{ margin: 0, fontSize: 24, color: fg }}>来自：{m.where}</footer>
                </blockquote>
              );
            })}
          </div>
        )}
      </div>
      </div>
    );
  },
);
