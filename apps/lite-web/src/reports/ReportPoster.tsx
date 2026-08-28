import { forwardRef } from "react";
import type { LiteReport } from "@lite/api/reports";

/**
 * ReportPoster — the picture she can send someone, not a shrunk copy of
 * `ReportView`. The product owner's own words: "should not be verbose, but
 * be good looking … like lark meeting notes they would conclude some 金句
 * … with student's name and effort be noted." So the content is a fixed,
 * short list — her name, the title, the date, the stats as large numerals,
 * and up to three 金句 given real room — and nothing else: no kind label,
 * no brand mark, no share chrome, no 收获 list. Restraint is the point.
 *
 * ## Offscreen, not hidden
 *
 * The root node itself carries `position:fixed; left:-99999px` — it is
 * ALWAYS rendered this way, since this component only ever exists to be
 * rasterized by `exportPoster`, never to be seen on the page. That is
 * deliberately not `display:none`: a `display:none` node has no layout box
 * at all, so html-to-image (which walks real geometry) would rasterize it
 * to a blank image. `position:fixed` off past the left edge keeps a real,
 * measured 1080×1440 box that just never enters the visible viewport.
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

    return (
      <div
        ref={ref}
        aria-hidden="true"
        style={{
          position: "fixed",
          left: -99999,
          top: 0,
          width: 1080,
          height: 1440,
          overflow: "hidden",
          boxSizing: "border-box",
          display: "flex",
          flexDirection: "column",
          gap: 44,
          padding: 76,
          background: PAPER,
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

        {report.stats.length > 0 && (
          <div style={{ display: "flex", flexWrap: "wrap", gap: 22 }}>
            {report.stats.map((stat, i) => {
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
                  <span style={{ fontSize: 54, lineHeight: 1, fontWeight: 700, color: fg }}>
                    {stat.value}
                    {stat.unit && (
                      <span style={{ fontSize: 26, marginLeft: 4, fontWeight: 500 }}>{stat.unit}</span>
                    )}
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
    );
  },
);
