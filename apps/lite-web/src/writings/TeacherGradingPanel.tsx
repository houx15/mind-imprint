import type { StudentGrading } from "../api/gradings";
import type { WritingVersion } from "../api/writings";
import { formatDeadline } from "../shared/deadline";
import { quoteRanges, unmarkedPointQuotes } from "../shared/gradingText";
import { AI_ATTRIBUTION, gradingVersionLine } from "./finishedWriting";

/**
 * 老师批改 in the finished page's rail: the gradings the teacher has sent,
 * newest version first (the server's own order). Absent — not an empty
 * placeholder — when there are none (controller ruling 1).
 *
 * A point's quote is a button: clicking it switches the left column to that
 * grading's version and highlights the sentence there (`FinishedWritingPage`'s
 * `onQuote`). A quote that would not actually highlight in that version's
 * text — not found, or only overlapping another point's already-taken span —
 * also shows 「未在正文中标出」 underneath, the same rule and the same
 * `unmarkedPointQuotes` helper the teacher's own `GradingPage` editor uses,
 * so a point never silently looks fine when it isn't backed by real text.
 * That check needs the graded version's own body, not whatever version is
 * currently on screen — `bodies` is `FinishedWritingPage`'s version cache,
 * keyed by version number; a grading whose version hasn't loaded yet (still
 * in flight) simply shows no "unmarked" flag until it has.
 */
export function TeacherGradingPanel({
  gradings,
  error,
  shownVersion,
  bodies,
  onQuote,
}: {
  gradings: StudentGrading[];
  error: string | null;
  shownVersion: number | null;
  bodies: Record<number, WritingVersion>;
  onQuote: (versionNumber: number, quote: string) => void;
}) {
  if (!error && gradings.length === 0) return null;
  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-mk-small font-semibold text-mk-secondary">老师批改</h2>
      {error && (
        <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
          加载失败：{error}
        </p>
      )}
      {gradings.map((g) => {
        const forLine = gradingVersionLine(g.versionNumber, shownVersion);
        const body = bodies[g.versionNumber]?.body ?? "";
        const quotes = g.content.points.map((p) => p.quote);
        const unmarked = new Set(unmarkedPointQuotes(quotes, quoteRanges(body, quotes)));
        return (
          <article key={g.id} className="flex flex-col gap-3 rounded-mk-md border border-mk-border bg-mk-surface p-4">
            <div className="flex flex-wrap items-baseline gap-2">
              <span className="text-mk-h3 font-bold text-mk-ink">{g.content.overall.grade}</span>
              <span className="text-mk-small text-mk-muted">总评</span>
              {forLine && <span className="text-mk-small text-mk-muted">{forLine}</span>}
              <span className="ml-auto text-mk-label text-mk-muted">{formatDeadline(g.sentAt)}</span>
            </div>
            {g.content.overall.comment && (
              <p className="whitespace-pre-wrap text-mk-small text-mk-ink">{g.content.overall.comment}</p>
            )}
            {g.content.dimensions.length > 0 && (
              <dl className="grid grid-cols-[auto_auto_1fr] gap-x-3 gap-y-1.5 text-mk-small">
                {g.content.dimensions.map((d) => (
                  <div key={d.name} className="contents">
                    <dt className="text-mk-muted">{d.name}</dt>
                    <dd className="font-bold text-mk-ink">{d.grade}</dd>
                    <dd className="text-mk-ink">{d.comment}</dd>
                  </div>
                ))}
              </dl>
            )}
            {g.content.points.length > 0 && (
              <ul className="flex flex-col gap-2">
                {g.content.points.map((p, i) => (
                  <li key={i} className="flex flex-col gap-1 text-mk-small">
                    <span className="text-mk-label font-bold text-mk-muted">{p.kind === "good" ? "优点" : "问题"}</span>
                    {p.quote && (
                      <button
                        type="button"
                        onClick={() => onQuote(g.versionNumber, p.quote ?? "")}
                        className="text-left text-mk-secondary underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                      >
                        「{p.quote}」
                      </button>
                    )}
                    {p.quote && unmarked.has(i) && <span className="text-mk-label text-mk-danger">未在正文中标出</span>}
                    <span className="text-mk-ink">{p.text}</span>
                    {p.action && <span className="text-mk-ink">修改建议：{p.action}</span>}
                  </li>
                ))}
              </ul>
            )}
            <p className="text-mk-label text-mk-muted">{AI_ATTRIBUTION}</p>
          </article>
        );
      })}
    </section>
  );
}
