import type { ReactNode } from "react";
import type { ParentReport } from "@lite/api/parentReports";
import { Hero, macaron, rise, SectionTitle } from "@lite/reports/ReportView";
import { monthDay, publishedMonthDay, rangeLabel } from "./range";
import { finishedLists, keywordGroups, statTiles, visibleSections, type StatTile } from "./view";

/**
 * ParentReportView — a published parent report, presentational only.
 *
 * Same page as the refreshed public reading/writing report (`ReportView`): the
 * `mk-rp` measure, its hero band, section titles, stat strip and quote cards,
 * so a parent who has opened both links sees one product.
 *
 * Whose words are whose:
 * - Each section's text is the TEACHER's report text. It carries the section
 *   label only and is never presented as the student's words.
 * - 学生原话 lists `facts.moments`: her own sentences, each with the title of
 *   the work it came from.
 * - Nothing from her chat with 印记 is in the payload, so nothing renders.
 *
 * Every block renders only when it has content; a section whose body is blank
 * is dropped by `visibleSections` (see its comment).
 */
export function ParentReportView({
  report,
  variant,
  actions,
}: {
  report: ParentReport;
  variant: "public" | "student" | "teacherPreview";
  /** Icon buttons in the hero's upper-right corner (保存为图片). */
  actions?: ReactNode;
}) {
  const { facts } = report;
  const sections = visibleSections(report.sections, report.body);
  const tiles = statTiles(facts);
  const lists = finishedLists(facts);
  const groups = keywordGroups(facts.keywords);
  const moments = facts.moments.filter((m) => m.quote.trim());
  const published = report.publishedAt ? publishedMonthDay(report.publishedAt) : "";
  const byline =
    variant === "teacherPreview" && !report.publishedAt
      ? `${report.teacherName} · 待发布`
      : `由 ${report.teacherName} 发布${published ? ` · ${published}` : ""}`;

  return (
    <article className="mk-rp mk-rp-measure flex flex-col gap-8 py-8 sm:py-12">
      <Hero
        kind="keepsake"
        kindLabel={report.className || "学习报告"}
        title="学习报告"
        name={report.studentName}
        date={rangeLabel(report.rangeStart, report.rangeEnd)}
        actions={actions}
      />

      {tiles.length > 0 && <StatTiles tiles={tiles} />}

      {sections.map((section) => (
        <section key={section.key} className="flex flex-col gap-4">
          <SectionTitle>{section.label}</SectionTitle>
          <div className="mk-rp-card rounded-mk-lg p-5 sm:p-7">
            <p className="whitespace-pre-wrap text-mk-body-lg text-mk-ink" style={{ lineHeight: 1.9 }}>
              {section.text}
            </p>
          </div>
        </section>
      ))}

      {moments.length > 0 && (
        <section className="flex flex-col gap-4">
          <SectionTitle>学生原话</SectionTitle>
          <div className="mk-rp-moments">
            {moments.map((m, i) => {
              const { bg, fg } = macaron(i + 2);
              return (
                <blockquote
                  key={`${m.itemTitle}-${i}`}
                  className="mk-rp-moment mk-rp-rise relative overflow-hidden rounded-mk-lg py-7 pl-10 pr-6"
                  style={{ background: bg, ...rise(i + 3) }}
                >
                  <span aria-hidden="true" className="mk-rp-moment__mark" style={{ color: fg }}>
                    “
                  </span>
                  <p className="text-mk-report-quote text-mk-ink">{m.quote}</p>
                  {m.itemTitle && (
                    <footer className="mt-4 text-mk-small" style={{ color: fg }}>
                      《{m.itemTitle}》
                    </footer>
                  )}
                </blockquote>
              );
            })}
          </div>
        </section>
      )}

      {lists.length > 0 && (
        <div className="mk-rp-columns">
          {lists.map((list) => (
            <section key={list.key} className="flex flex-col gap-4">
              <SectionTitle>{list.label}</SectionTitle>
              <ul className="flex flex-col gap-2">
                {list.items.map((item, i) => (
                  <li
                    key={`${item.title}-${i}`}
                    className="mk-rp-card flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 rounded-mk-lg px-5 py-3"
                  >
                    <span className="min-w-0 text-mk-body-lg text-mk-ink" style={{ overflowWrap: "anywhere" }}>
                      《{item.title}》
                    </span>
                    {monthDay(item.finishedAt) && (
                      <span className="shrink-0 text-mk-small text-mk-muted">{monthDay(item.finishedAt)}</span>
                    )}
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </div>
      )}

      {groups.length > 0 && (
        <section className="flex flex-col gap-4">
          <SectionTitle>兴趣关键词</SectionTitle>
          <div className="flex flex-col gap-4">
            {groups.map((group, gi) => {
              const { bg, fg } = macaron(gi);
              return (
                <div key={group.label} className="flex flex-wrap items-center gap-2">
                  <span className="mr-1 text-mk-small text-mk-muted">{group.label}</span>
                  {group.words.map((word) => (
                    <span
                      key={word}
                      className="rounded-mk-full px-3 py-1 text-mk-body"
                      style={{ background: bg, color: fg }}
                    >
                      {word}
                    </span>
                  ))}
                </div>
              );
            })}
          </div>
        </section>
      )}

      <footer className="text-mk-small text-mk-muted">{byline}</footer>
    </article>
  );
}

/** The `mk-rp-stats` strip with the same tile treatment as `ReportView`. A
 * tile can hold several numbers (学习时长, 作业): each number keeps its unit on
 * one line, and the numbers wrap between themselves. The 作业 tile spans two
 * grid columns so its three numbers have room. */
function StatTiles({ tiles }: { tiles: StatTile[] }) {
  return (
    <section className="mk-rp-stats" aria-label="数据">
      {tiles.map((tile, i) => {
        const { bg, fg } = macaron(i);
        return (
          <div
            key={tile.key}
            className="mk-rp-stat mk-rp-rise rounded-mk-lg px-4 py-4 sm:px-5 sm:py-5"
            style={{ background: bg, ...rise(i + 1), ...(tile.key === "assignments" ? { gridColumn: "span 2" } : {}) }}
          >
            <span
              className="mk-rp-stat__value text-mk-report-stat"
              style={{ color: fg, flexWrap: "wrap", columnGap: 12, rowGap: 4 }}
            >
              {tile.parts.map((part, pi) => (
                <span key={pi} className="inline-flex items-baseline gap-0.5 whitespace-nowrap">
                  {part.lead && <span className="mr-1 text-mk-small font-semibold">{part.lead}</span>}
                  {part.n}
                  {part.unit && <span className="text-mk-h3">{part.unit}</span>}
                </span>
              ))}
            </span>
            <span className="mt-2 block text-mk-label" style={{ color: fg }}>
              {tile.label}
            </span>
          </div>
        );
      })}
    </section>
  );
}
