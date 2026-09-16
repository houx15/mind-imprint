import { useCallback, useEffect, useState } from "react";
import { Button } from "@/ui";
import { apiErrorText } from "../api/errorText";
import { listWritingGradings, type StudentGrading } from "../api/gradings";
import { useAlive } from "../shared/useAlive";
import { draftQuoteMatch, quoteClickableInRoom } from "./draftQuote";
import { TeacherGradingPanel } from "./TeacherGradingPanel";

/**
 * RoomTeacherFeedback — 老师批改, now visible while she revises.
 *
 * Before this, a sent grading only ever showed on the finished page: the
 * moment she pressed 修改 and reopened the room, it fell off screen even
 * though it is exactly the thing she is meant to be acting on. This mounts
 * the SAME panel (`TeacherGradingPanel`) the finished page uses, collapsed
 * under one toggle — open by default, because that first sight is the point,
 * but collapsible so it never permanently eats space from 印记's rail or the
 * draft.
 *
 * A quote here is clickable only when it can still be found in the CURRENT
 * DRAFT AND the room is actually showing 成稿 right now
 * (`quoteClickableInRoom` — `stage === "draft"`; 段落/结构 have no single
 * draft surface to scroll a highlight into), not the frozen submitted
 * version the grading was written against: she opened 修改 precisely
 * because that text may already differ. A quote she has since rewritten (or
 * one she'd have to leave this stage to reach) renders as plain text instead
 * of a dead button (`TeacherGradingPanel`'s `clickable` gate). `bodies={{}}`
 * deliberately suppresses the finished page's separate "未在正文中标出"
 * label — that check is about the GRADED version's own text and does not
 * apply here; whether a quote is worth clicking is already communicated by
 * whether it is a button at all.
 *
 * A click hands `onQuote` the ACTUAL matched substring of `draftBody`
 * (`draftQuoteMatch`), not the teacher's raw quote text — see that
 * function's doc comment: `ProseSurface`'s highlight is a literal
 * `text.indexOf`, and a teacher's quote is routinely not byte-for-byte
 * identical to the draft even when it normalising-matches.
 *
 * Hidden entirely (renders nothing) under the same rule `TeacherGradingPanel`
 * itself already uses: no error and no sent gradings. The caller additionally
 * only mounts this at all once the essay has been finished once
 * (`isWritingFinished`) — a brand-new essay can have no sent grading, so
 * there is nothing here worth an unconditional fetch for.
 */
export function RoomTeacherFeedback({
  writingId,
  stage,
  draftBody,
  onQuote,
}: {
  writingId: string;
  /** `writing.stage` — only 成稿 (`"draft"`) has a draft surface to jump a
   *  quote into; see `quoteClickableInRoom`. */
  stage: string;
  /** The live draft to match quotes against, not any submitted version —
   *  `WritingRoomHost`'s own `state.draft.body`. */
  draftBody: string;
  /** A clickable quote was picked: the matched substring of `draftBody` to
   *  scroll to (and highlight, where the room already has a way to). */
  onQuote: (matchedText: string) => void;
}) {
  const alive = useAlive();
  const [gradings, setGradings] = useState<StudentGrading[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [reloadNonce, setReloadNonce] = useState(0);
  const [open, setOpen] = useState(true);

  const load = useCallback(() => {
    setError(null);
    listWritingGradings(writingId)
      .then((g) => {
        if (alive.current) setGradings(g);
      })
      .catch((e: unknown) => {
        if (alive.current) setError(apiErrorText(e));
      });
  }, [writingId, alive]);

  useEffect(() => {
    setGradings([]);
    load();
    // `load` is stable per `writingId` (its own `useCallback` dep) — this
    // effect's job is "fetch again for this writing, or when retried",
    // not "re-fetch whenever `load` is recreated".
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [writingId, reloadNonce]);

  // Same visibility rule `TeacherGradingPanel` itself uses: absent, not an
  // empty placeholder, when there is nothing to show and nothing went wrong.
  if (!error && gradings.length === 0) return null;

  return (
    <section className="flex flex-col gap-2 rounded-mk-md border border-mk-border bg-mk-surface p-3">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex items-center justify-between gap-2 text-left text-mk-small font-semibold text-mk-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      >
        老师批改
        <span className="text-mk-label font-normal text-mk-muted">{open ? "收起" : "展开"}</span>
      </button>
      {open &&
        (error ? (
          <div className="flex flex-col items-start gap-2">
            <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
              加载失败：{error}
            </p>
            <Button variant="secondary" size="sm" onClick={() => setReloadNonce((n) => n + 1)}>
              重试
            </Button>
          </div>
        ) : (
          <div className="mk-scroll max-h-[min(320px,33vh)] overflow-y-auto">
            <TeacherGradingPanel
              gradings={gradings}
              error={null}
              shownVersion={null}
              bodies={{}}
              heading={false}
              onQuote={(_version, quotes, index) => {
                const quote = quotes[index];
                const match = quote ? draftQuoteMatch(draftBody, quote) : null;
                if (match) onQuote(match);
              }}
              clickable={(quote) => quoteClickableInRoom(stage, draftBody, quote)}
            />
          </div>
        ))}
    </section>
  );
}
