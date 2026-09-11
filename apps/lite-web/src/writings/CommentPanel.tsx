import { Icon } from "@/ui";
import { ArrowRight, Quote } from "lucide-react";
import { COMMENT_LAYER_NAMES, type Comment } from "../api/writingRoom";

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
                {/* 已经用对的那一条排在最前面，给它一个看得出来的标。
                    要改的那些带一个层名（立意/材料/结构/字句）——她因此知道
                    印记这一轮在管哪一层，而不是随口挑了一句。 */}
                <span className="flex items-center gap-2">
                  {point.kind === "good" ? (
                    <span
                      className="rounded-mk-full px-2 py-0.5 text-mk-small"
                      style={{
                        background: "color-mix(in srgb, var(--mk-accent-500) 16%, transparent)",
                        color: "var(--mk-accent-700)",
                      }}
                    >
                      已经用对
                    </span>
                  ) : point.layer && COMMENT_LAYER_NAMES[point.layer] ? (
                    <span className="rounded-mk-full border border-mk-border px-2 py-0.5 text-mk-small text-mk-muted">
                      {COMMENT_LAYER_NAMES[point.layer]}
                    </span>
                  ) : null}
                </span>

                <span className="text-mk-body-lg text-mk-ink">{point.text}</span>
                <span className="flex items-start gap-1.5 text-mk-body text-mk-muted">
                  <Icon icon={Quote} size={14} className="mt-[3px] shrink-0" />
                  <span>{point.quote}</span>
                </span>

                {/* 🚨 这一行是整条意见里最要紧的东西：她接下来要做的那件事。
                    服务端保证 issue 一定带着它（action 为空的整条丢掉），
                    所以这里不需要兜底文案——没有 action 的只可能是
                    2026-09-11 之前存下来的老评论，那时候本来就没有这个字段。 */}
                {point.action && (
                  <span
                    className="mt-1 flex items-start gap-1.5 rounded-mk-sm px-2.5 py-1.5 text-mk-body text-mk-ink"
                    style={{ background: "color-mix(in srgb, var(--mk-accent-500) 10%, transparent)" }}
                  >
                    <Icon icon={ArrowRight} size={14} className="mt-[3px] shrink-0" />
                    <span>{point.action}</span>
                  </span>
                )}
              </button>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
