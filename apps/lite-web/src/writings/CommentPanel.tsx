import { Button, Icon } from "@/ui";
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
/**
 * 这条意见说的是不是上一版。
 *
 * 判据和服务端存进来时用的是同一条（writing_comment.go 的
 * validateCommentPoints：quote 必须逐字出现在她写的东西里）。当时成立、
 * 现在不成立，只可能是她把那句话改掉了——也就是她照着这条做了。
 *
 * 🚨 两条边界都是「拿不准就当还算数」，而且方向是故意的：
 * 把一条还有效的意见标成「上一版」，等于告诉她一件没做完的事不用做了；
 * 反过来只是多留一条旧话在屏幕上，她点一下「再看一遍」就清掉。
 *
 *   - `currentText` 没给（成稿那一步还没接）→ 不算上一版。
 *   - `quote` 是空的（2026-09-11 之前存下来的老评论可能没有）→ 不算。
 *     空串是任何字符串的子串，不挡这一下的话，`includes("")` 永远为真，
 *     结论会反过来。
 */
export function commentPointIsStale(quote: string, currentText?: string): boolean {
  if (currentText === undefined) return false;
  if (quote.trim() === "") return false;
  return !currentText.includes(quote);
}

export function CommentPanel({
  comment,
  onTrace,
  currentText,
  onRecheck,
  rechecking = false,
}: {
  comment: Comment;
  onTrace: (quote: string) => void;
  /**
   * 她现在框里的字。给了的话，这块面板就能认出哪几条说的已经是上一版。
   * 不给（成稿那一步暂时没给）就照旧全都当成还算数。
   */
  currentText?: string;
  /** 「请印记再看一遍」。不给就不摆那颗按钮。 */
  onRecheck?: () => void;
  rechecking?: boolean;
}) {
  // 🚨 **一条意见指着的那句话不在了，这条意见就是上一版的。**
  //
  // 判据和服务端存进来时用的是同一条（writing_comment.go 的
  // validateCommentPoints：quote 必须逐字出现在她写的东西里）。当时成立，
  // 现在不成立，只可能是她把那句话改掉了 —— 也就是她照着这条做了。
  //
  // 2026-09-11 第三轮线上走查，三条卡壳说的都是这一件事：
  //   「框[0]里同时存在我新写的带 So 的版本和它标黄的旧版本，看着像没保存好」
  //   「框1里明明已经有那句话了，但下面还在叫我加。是不是要重新提交它才看得到？」
  //   「印记的回复好像还在说旧版的顺序问题」
  // 她照着意见改完，意见还挂在那儿说着改之前的话，于是她读成了
  //「印记没看见我改了」。改完之后**没有任何东西告诉她这一轮结束了**。
  //
  // 这里不调模型、不猜她改得好不好：只诚实地说「这条说的是上一版」，
  // 再把下一步递给她。判她改得对不对是 onRecheck 那一下的事，由她决定。
  const stale = (quote: string) => commentPointIsStale(quote, currentText);
  const staleCount = comment.points.filter((p) => stale(p.quote)).length;
  // 🚨 **quote 还在，不等于这条意见还说得准。**
  //
  // 第一版只看 quote 在不在，理由是「她把那句改了才算照着做了」。线上第五轮
  // 打脸得很干脆 —— 一个学生连着四步说同一件事：
  //
  //   「我已经按它说的加了让步句，但下面还是显示缺，不知道是不是没刷新」
  //   「框2里面明明已经有让步的句子了，但下面材料那块还是说缺让步」
  //
  // 那条意见说的正是「缺让步」，而她的做法是**新加一句**，被引的那句原封不动。
  // 于是 quote 判据说「还算数」，屏幕接着说她没做 —— 判错的方向恰好是最伤的
  // 那一个：她做完了，产品说她没做。
  //
  // 所以再问一个 quote 答不了的问题：**这一段在这条意见之后动过没有。**
  // 这个只有服务端知道（它存下了当时读的那一版），比对是逐字的，没有猜测。
  const edited = comment.sourceText !== "" && currentText !== undefined && currentText !== comment.sourceText;

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
                data-comment-stale={stale(point.quote) ? "" : undefined}
                onClick={() => onTrace(point.quote)}
                className={
                  "flex w-full flex-col items-start gap-1.5 rounded-mk-md border p-3 text-left transition-colors hover:border-mk-accent" +
                  (stale(point.quote) ? " opacity-55" : "")
                }
                style={{
                  borderColor: "var(--mk-border)",
                  background: stale(point.quote)
                    ? "transparent"
                    : "color-mix(in srgb, var(--mk-accent-500) 5%, transparent)",
                }}
              >
                {/* 已经用对的那一条排在最前面，给它一个看得出来的标。
                    要改的那些带一个层名（立意/材料/结构/字句）——她因此知道
                    印记这一轮在管哪一层，而不是随口挑了一句。 */}
                <span className="flex items-center gap-2">
                  {/* 这条说的是上一版 —— 摆在最前面，她一眼就知道不用再照着做。 */}
                  {stale(point.quote) && (
                    <span className="rounded-mk-full border border-mk-border px-2 py-0.5 text-mk-small text-mk-muted">
                      上一版
                    </span>
                  )}
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

      {/* 改完之后那一下。没有它，这一轮就永远不结束 ——
          她照着改了，屏幕上还是那几条旧话，只好反复问印记是不是没看见。 */}
      {(staleCount > 0 || edited) && onRecheck && (
        <div className="flex flex-wrap items-center gap-2 border-t border-mk-border pt-3">
          <span className="text-mk-body text-mk-muted">
            {staleCount > 0
              ? `这一段改过了，上面有 ${staleCount} 条说的是上一版。`
              : "这一段在这条意见之后改过了。"}
          </span>
          <Button variant="secondary" size="sm" onClick={onRecheck} loading={rechecking}>
            请印记再看一遍
          </Button>
        </div>
      )}
    </div>
  );
}
