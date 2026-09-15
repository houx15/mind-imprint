import { ReportVisualSummary } from "./ReportVisualSummary";
import { studentArtwork } from "../learning/StudentArtwork";
import { forwardRef } from "react";
import type { LiteReport } from "@lite/api/reports";
import { displayStat } from "./statLabels";

/**
 * ReportPoster — the picture she can send someone, not a shrunk copy of
 * `ReportView`. The product owner's own words: "should not be verbose, but
 * be good looking … like lark meeting notes they would conclude some 金句
 * … with student's name and effort be noted." So the content is a concise,
 * short list — her name, the title, the date, the stats as large numerals,
 * and up to three 金句 given real room. Reading also includes the student's
 * takeaway with attribution and grows vertically to fit its contents.
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
 * would differ from whatever she previewed. FONT_STACK uses system fonts.
 * Colors inherit the student's shared design tokens from the themed body.
 * html-to-image resolves their computed values before rasterization; the
 * browser export check verifies both actual content and the resulting colors.
 */

// Both journals grow with their contents. All quoted/generated text keeps attribution.
const FONT_STACK =
  '-apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif';

const INK = "var(--mk-ink)";
const MUTED = "var(--mk-muted)";
const PAPER = "var(--mk-paper)";

/** Semantic report colors stay shared with the on-screen record. */
const MACARON = ["peach", "lake", "berry", "matcha", "taro", "butter", "mist"].map(
  tone => ({ bg: `var(--mk-${tone}-bg)`, fg: `var(--mk-${tone}-fg)` }),
);

function macaron(i: number) {
  // `i % MACARON.length` is always a valid index into a non-empty literal
  // array — the `?? MACARON[0]` only satisfies noUncheckedIndexedAccess.
  return MACARON[i % MACARON.length] ?? MACARON[0]!;
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
    // Keep all recorded statistics; the image grows to fit the visual summary.
    // The full report page continues to show every supplied stat.
    // Same client-side label resolution as the page — a stored report carries
    // whatever wording it was generated with (see statLabels.ts).
    const stats = report.stats
      .filter((stat) => stat.value !== 0)
      .map((stat) => displayStat(stat, report.kind));

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
          height: "auto",
          overflow: "hidden",
          boxSizing: "border-box",
          display: "flex",
          flexDirection: "column",
          gap: 32,
          padding: 64,
          background: PAPER,
          fontFamily: FONT_STACK,
        }}
      >
        <header style={{ display: "flex", flexDirection: "column", gap: 18 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", height: 260 }}>
            <span style={{ fontSize: 24, letterSpacing: 3, color: MUTED }}>{report.kind === "reading" ? "READING JOURNAL" : "WRITING JOURNAL"}</span>
            <img src={report.kind === "reading" ? studentArtwork.keepsake : studentArtwork.writing} alt="" style={{ width: 340, height: 260, objectFit: "contain" }} />
          </div>
          <h1
            style={{
              margin: 0,
              fontSize: 50,
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

        <div className="report-poster-visual"><ReportVisualSummary stats={stats} /></div>

        {report.keep && <section style={{ padding: "30px 0", borderTop: "1px solid var(--mk-border)" }}>
          <p style={{ fontSize: 22, color: MUTED, margin: "0 0 18px" }}>{report.keep.label}</p>
          <p style={{ fontSize: 38, lineHeight: 1.65, color: INK, margin: 0 }}>{report.keep.text}</p>
          <p style={{ fontSize: 20, color: MUTED, margin: "18px 0 0" }}>{report.keep.source === "student" ? report.studentName : `印记根据这次${report.kind === "reading" ? "阅读" : "写作"}整理`}</p>
        </section>}

        {moments.length > 0 && (
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              gap: 26,
              flex: "none",
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
                    flex: "none",
                    minHeight: 0,
                    borderRadius: report.kind === "reading" ? 0 : 28,
                    background: report.kind === "reading" ? "transparent" : bg,
                    padding: report.kind === "reading" ? "34px 0" : "34px 42px",
                    borderTop: report.kind === "reading" ? "1px solid var(--mk-border)" : undefined,
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
