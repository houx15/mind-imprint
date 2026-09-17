import type { StudentGrading } from "../api/gradings";
import type { WritingVersion } from "../api/writings";
import { formatDeadline } from "../shared/deadline";
import { determinedUnmarkedQuotes } from "../shared/gradingText";
import { gradingAttribution, gradingVersionLine } from "./finishedWriting";

/**
 * 老师批改 in the finished page's rail: the gradings the teacher has sent,
 * newest version first (the server's own order). Absent — not an empty
 * placeholder — when there are none (controller ruling 1).
 *
 * A point's quote is a button: clicking it switches the left column to that
 * grading's version and highlights that quote there (`FinishedWritingPage`'s
 * `onQuote`, which highlights the clicked quote on its own with
 * `rangeForQuote`, so two points quoting overlapping sentences both
 * highlight). A quote that is not found in that version's text shows
 * 「未在正文中标出」 underneath (`determinedUnmarkedQuotes`: not found only;
 * the teacher's `GradingPage` editor also flags overlaps, because the
 * teacher can pick a different sentence). That check needs the graded version's own
 * body, not whatever version is currently on screen — `bodies` is
 * `FinishedWritingPage`'s version cache, keyed by version number; a grading
 * whose version hasn't loaded (or failed to load) shows no "unmarked" flag
 * until it has (`determinedUnmarkedQuotes` returns `null` for both).
 *
 * `onQuote`/`clickable`/`heading`: the writing room
 * reuses this same panel while she revises, where clicking a quote must jump
 * into the CURRENT DRAFT rather than a submitted version, and the room's own
 * collapsible wrapper already carries the 「老师批改」 label. `onQuote` is
 * optional so a caller with no jump target at all (none needed here yet, but
 * keeps the type honest) still renders plain quotes; `clickable` lets a
 * caller gate which quotes get a button without touching `unmarked` (still
 * computed from `bodies`, which the room leaves empty — it has no submitted
 * version to check a quote against, so "未在正文中标出" never applies there).
 * Both default to FinishedWritingPage's exact original behaviour: `onQuote`
 * always called, every quote clickable.
 */
const EMPTY_SET: ReadonlySet<number> = new Set();

export function TeacherGradingPanel({
  gradings,
  error,
  shownVersion,
  bodies,
  onQuote,
  clickable,
  heading = true,
}: {
  gradings: StudentGrading[];
  error: string | null;
  shownVersion: number | null;
  bodies: Record<number, WritingVersion>;
  onQuote?: (versionNumber: number, quotes: readonly (string | null)[], index: number) => void;
  /** Whether a point's quote gets a clickable button. Omit to make every
   *  quote clickable (as long as `onQuote` is given) — FinishedWritingPage's
   *  original rule. */
  clickable?: (quote: string, versionNumber: number, index: number) => boolean;
  /** Set false to omit the internal 「老师批改」 heading — for a caller (the
   *  writing room) whose own collapsible wrapper already labels it. */
  heading?: boolean;
}) {
  if (!error && gradings.length === 0) return null;
  return (
    <section className="flex flex-col gap-3">
      {heading && <h2 className="text-mk-small font-semibold text-mk-secondary">老师批改</h2>}
      {error && (
        <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
          加载失败：{error}
        </p>
      )}
      {gradings.map((g) => {
        const forLine = gradingVersionLine(g.versionNumber, shownVersion);
        const quotes = g.content.points.map((p) => p.quote);
        const unmarked = determinedUnmarkedQuotes(bodies[g.versionNumber]?.body, quotes) ?? EMPTY_SET;
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
                    {p.quote && onQuote && (!clickable || clickable(p.quote, g.versionNumber, i)) ? (
                      <button
                        type="button"
                        onClick={() => onQuote(g.versionNumber, quotes, i)}
                        className="text-left text-mk-secondary underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                      >
                        「{p.quote}」
                      </button>
                    ) : (
                      p.quote && <span className="text-left text-mk-secondary">「{p.quote}」</span>
                    )}
                    {p.quote && unmarked.has(i) && <span className="text-mk-label text-mk-danger">未在正文中标出</span>}
                    <span className="text-mk-ink">{p.text}</span>
                    {p.action && <span className="text-mk-ink">修改建议：{p.action}</span>}
                  </li>
                ))}
              </ul>
            )}
            <p className="text-mk-label text-mk-muted">{gradingAttribution(g.source)}</p>
          </article>
        );
      })}
    </section>
  );
}
