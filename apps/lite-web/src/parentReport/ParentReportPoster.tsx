import { forwardRef } from "react";
import type { ParentReport } from "@lite/api/parentReports";
import { publishedMonthDay, rangeLabel } from "./range";
import { statTiles, visibleSections } from "./view";

/**
 * ParentReportPoster — the picture a parent saves from `/r/:token`.
 *
 * ## Offscreen, and the offset goes on the WRAPPER
 *
 * 🚨 Two elements: an outer wrapper with `position:fixed; left:-99999px`, and
 * the poster itself, `position:static`, which the ref points at. Never merge
 * them. html-to-image clones the rasterized node with its computed style into
 * an SVG `<foreignObject>`; an offset on that node moves the clone out of its
 * own viewport and the PNG comes out correctly sized and completely blank
 * (2026-08-30). See `reports/ReportPoster.tsx`.
 *
 * ## Explicit hex colours and system fonts
 *
 * The page it is exported from follows the viewer's theme, and a parent in
 * dark mode must still get the same light picture. So every colour here is a
 * literal (the lite light palette and the macaron `-bg`/`-fg` pairs), never a
 * `var(--mk-…)`. A `<foreignObject>` never loads a web font, so the stack is
 * system fonts only.
 *
 * Same content rules as `ParentReportView`: only non-blank sections, only
 * tiles with a source, 金句 labelled 学生原话, the teacher's text unlabelled as
 * hers, no chat.
 */

const FONT_STACK =
  '-apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif';

const INK = "#263342";
const MUTED = "#667587";
const PAPER = "#f7fafa";
const SURFACE = "#ffffff";
const BORDER = "#e3ebee";
const ACCENT = "#065f69";

const MACARON = [
  { bg: "#FDE7D3", fg: "#9A5A22" },
  { bg: "#E0F0EC", fg: "#177368" },
  { bg: "#FCE7EB", fg: "#A63A50" },
  { bg: "#E7F1DD", fg: "#4D6B3A" },
  { bg: "#F0EAF6", fg: "#5B4A80" },
  { bg: "#FAF3DE", fg: "#8A6320" },
  { bg: "#E6EEF9", fg: "#3C5A86" },
] as const;

function macaron(i: number) {
  return MACARON[i % MACARON.length] ?? MACARON[0];
}

/** At most three quotes, given room rather than shrunk. */
const MAX_POSTER_MOMENTS = 3;

export const ParentReportPoster = forwardRef<HTMLDivElement, { report: ParentReport }>(function ParentReportPoster(
  { report },
  ref,
) {
  const sections = visibleSections(report.sections, report.body);
  const tiles = statTiles(report.facts);
  const moments = report.facts.moments.filter((m) => m.quote.trim()).slice(0, MAX_POSTER_MOMENTS);
  const range = rangeLabel(report.rangeStart, report.rangeEnd);
  const published = report.publishedAt ? publishedMonthDay(report.publishedAt) : "";

  return (
    // The wrapper holds the offscreen offset; the poster below is static and
    // is what gets rasterized. Merging the two produces a blank PNG.
    <div aria-hidden="true" style={{ position: "fixed", left: -99999, top: 0 }}>
      <div
        ref={ref}
        style={{
          position: "static",
          width: 1080,
          boxSizing: "border-box",
          display: "flex",
          flexDirection: "column",
          gap: 40,
          padding: 72,
          background: PAPER,
          color: INK,
          fontFamily: FONT_STACK,
        }}
      >
        <header style={{ display: "flex", flexDirection: "column", gap: 14, paddingBottom: 28, borderBottom: `2px solid ${INK}` }}>
          <span style={{ fontSize: 24, letterSpacing: 4, color: ACCENT, fontWeight: 600 }}>学习报告</span>
          <h1 style={{ margin: 0, fontSize: 60, lineHeight: 1.25, fontWeight: 700, color: INK }}>{report.studentName}</h1>
          <p style={{ margin: 0, fontSize: 28, color: MUTED }}>
            {[report.className, range].filter(Boolean).join(" · ")}
          </p>
        </header>

        {tiles.length > 0 && (
          <div style={{ display: "flex", flexWrap: "wrap", gap: 20 }}>
            {tiles.map((tile, i) => {
              const { bg, fg } = macaron(i);
              return (
                <div
                  key={tile.key}
                  style={{
                    flex: tile.key === "assignments" ? "1 1 100%" : "1 1 200px",
                    boxSizing: "border-box",
                    borderRadius: 24,
                    background: bg,
                    padding: "26px 30px",
                    display: "flex",
                    flexDirection: "column",
                    gap: 8,
                  }}
                >
                  <span style={{ display: "flex", flexWrap: "wrap", alignItems: "baseline", columnGap: 24, rowGap: 6, color: fg }}>
                    {tile.parts.map((part, pi) => (
                      <span key={pi} style={{ display: "flex", alignItems: "baseline", gap: 4, whiteSpace: "nowrap" }}>
                        {part.lead && <span style={{ fontSize: 24, fontWeight: 600, marginRight: 6 }}>{part.lead}</span>}
                        <span style={{ fontSize: 50, lineHeight: 1, fontWeight: 700 }}>{part.n}</span>
                        {part.unit && <span style={{ fontSize: 24, fontWeight: 500 }}>{part.unit}</span>}
                      </span>
                    ))}
                  </span>
                  <span style={{ fontSize: 22, color: fg }}>{tile.label}</span>
                </div>
              );
            })}
          </div>
        )}

        {sections.map((section) => (
          <section key={section.key} style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            <h2 style={{ margin: 0, fontSize: 26, fontWeight: 600, color: ACCENT }}>{section.label}</h2>
            <p
              style={{
                margin: 0,
                padding: "28px 32px",
                borderRadius: 24,
                background: SURFACE,
                border: `1px solid ${BORDER}`,
                fontSize: 30,
                lineHeight: 1.8,
                color: INK,
                whiteSpace: "pre-wrap",
              }}
            >
              {section.text}
            </p>
          </section>
        ))}

        {moments.length > 0 && (
          <section style={{ display: "flex", flexDirection: "column", gap: 18 }}>
            <h2 style={{ margin: 0, fontSize: 26, fontWeight: 600, color: ACCENT }}>学生原话</h2>
            {moments.map((m, i) => {
              const { bg, fg } = macaron(i + 2);
              return (
                <blockquote
                  key={`${m.itemTitle}-${i}`}
                  style={{ margin: 0, borderRadius: 24, background: bg, padding: "30px 36px", display: "flex", flexDirection: "column", gap: 12 }}
                >
                  <p style={{ margin: 0, fontSize: 34, lineHeight: 1.6, fontWeight: 500, color: INK }}>“{m.quote}”</p>
                  {m.itemTitle && <footer style={{ fontSize: 24, color: fg }}>《{m.itemTitle}》</footer>}
                </blockquote>
              );
            })}
          </section>
        )}

        <footer
          style={{
            display: "flex",
            justifyContent: "space-between",
            gap: 24,
            paddingTop: 24,
            borderTop: `1px solid ${BORDER}`,
            fontSize: 22,
            color: MUTED,
          }}
        >
          <span>
            由 {report.teacherName} 发布{published ? ` · ${published}` : ""}
          </span>
          <span>思维印记</span>
        </footer>
      </div>
    </div>
  );
});
