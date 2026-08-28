import { Icon } from "@/ui";
import { Quote } from "lucide-react";
import type { Comment } from "../api/writingRoom";

/**
 * CommentPanel — 印记's structured critique, rendered so it survives past
 * one screenful of prose.
 *
 * Before Task 5, `POST /review` answered `{"feedback": "<prose>"}` — one
 * paragraph, rendered once, gone the moment she navigated away. The server
 * now hands back a `Comment`: one summary line, then 2–4 concrete points,
 * each anchored to a sentence she actually wrote (`quote`, validated
 * server-side — see writing_comment.go's validateCommentPoints). This
 * component is the other half of that guarantee: it makes the anchor
 * visible and clickable, not just present in the payload.
 *
 * This component only renders and calls back — `onTrace(quote)` is lifted to
 * whichever stage hosts it, which passes the quote into `ProseSurface`'s
 * `highlight` prop. No DOM reaching-in here, no scrolling logic: that is the
 * host's job (it owns the surface being highlighted), not this panel's.
 *
 * Every point's clickable control carries `data-comment-point` — a plain
 * DOM attribute, not a design choice — because the later e2e walk selects
 * `[data-comment-point]` and nothing else in this plan creates it.
 */
export function CommentPanel({ comment, onTrace }: { comment: Comment; onTrace: (quote: string) => void }) {
  return (
    <div className="flex flex-col gap-4 rounded-mk-md border border-mk-border bg-mk-paper p-4">
      <p className="text-mk-body-lg font-semibold text-mk-ink">{comment.summary}</p>

      {comment.points.length > 0 && (
        <ol className="flex list-none flex-col gap-3">
          {comment.points.map((point, i) => (
            <li key={i}>
              <button
                type="button"
                data-comment-point
                onClick={() => onTrace(point.quote)}
                className="flex w-full flex-col items-start gap-1.5 rounded-mk-md border p-3 text-left transition-colors hover:border-mk-accent"
                style={{
                  borderColor: "var(--mk-border)",
                  background: "color-mix(in srgb, var(--mk-accent-500) 5%, transparent)",
                }}
              >
                <span className="text-mk-body-lg text-mk-ink">{point.text}</span>
                <span className="flex items-start gap-1.5 text-mk-body text-mk-muted">
                  <Icon icon={Quote} size={14} className="mt-[3px] shrink-0" />
                  <span>{point.quote}</span>
                </span>
              </button>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
