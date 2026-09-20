import { useState } from "react";
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

/**
 * 分级标签的字和颜色。
 *
 * 用「已／待」那套成对词（AGENTS.md 文案规则 4），不写成句子。
 *
 * 🚨 **`polish` 不能用 danger。** 它的意思是「不挡着她往下走」，
 * 长得像错误就等于没分级 —— 那正是同事指出的那个毛病。
 */
// 🚨 只用**真的存在**的 mk token。`--mk-surface-2` 在 lite 的 index.css 里
// 被用过两次却从来没有被定义 —— 那一类幽灵变量解析成空、背景直接没有，
// 而且不报错（2026-09-02 一次清查里揪出过四个）。
const VERDICT_CHIP: Record<string, { label: string; bg: string; fg: string; border: string }> = {
  pass: {
    label: "已通过",
    bg: "var(--mk-accent-100)",
    fg: "var(--mk-accent-700)",
    border: "var(--mk-accent-300)",
  },
  polish: {
    label: "可优化",
    bg: "var(--mk-surface)",
    fg: "var(--mk-muted)",
    border: "var(--mk-border)",
  },
  revise: {
    label: "需修改",
    bg: "var(--mk-danger-bg)",
    fg: "var(--mk-danger)",
    border: "var(--mk-danger)",
  },
};

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
  // 🚨 **做完的那几条要从眼前拿走，不是压暗留在那儿。**
  //
  // 上一版给它们加了「上一版」的标、压到 55% 不透明度，以为「说清楚」就够了。
  // 第九轮线上走查一个学生连着八步在说同一件事，而且越说越烦：
  //
  //   「那三张卡片还是显示『上一版』，看着很烦，明明我已经改了」
  //   「卡片19标着『上一版』但还在列表里，看着像没刷新，不知道它到底还要不要我改」
  //   「希望按了之后能更新」
  //
  // 她把「还摆在列表里」读成「没刷新」，不是「你已经做完了」。摆着不动的东西
  // 看起来就是待办 —— 压暗改变不了这一点。而这几条她**已经做完了**：
  // 被引的那句话在她正文里逐字都不在了。
  //
  // 所以收起来，不是删掉：折在一行后面，她想回头看还能翻开。
  const chip = comment.verdict ? VERDICT_CHIP[comment.verdict] : undefined;
  const verdictLabel = chip?.label ?? "";
  const verdictStyle = chip
    ? { background: chip.bg, color: chip.fg, border: `1px solid ${chip.border}` }
    : undefined;

  const livePoints = comment.points.filter((p) => !stale(p.quote));
  const stalePoints = comment.points.filter((p) => stale(p.quote));
  const [showStale, setShowStale] = useState(false);

  return (
    <div className="flex flex-col gap-4 rounded-mk-md border border-mk-border bg-mk-paper p-4">
      {/* 🚨 **这一行要排在总评上面。**
          原来它在整块面板的最下面，而她是从上往下读的：第一眼看到的是那句
          总评，而总评里往往**逐字引着她已经改掉的句子**（点评逐条会折起来，
          总评是一句人话，没法逐条过滤）。
          2026-09-11 第十一轮线上走查，她为这个卡了三步：

            「印记最新的反馈说最后一句是『大家真的应该重视起来』，
              但框0里明明已经不是这句了」
            「不知道是不是我已经改过了它还没刷新，还是我该在现有基础上再加点东西」
            「不知道该不该再点一下请印记看看确认改好了」

          先告诉她「这是对着上一版说的」，她再往下读那句总评就对得上了。 */}
      {(staleCount > 0 || edited) && onRecheck && (
        <div className="flex flex-wrap items-center gap-2 rounded-mk-sm border border-mk-border px-3 py-2">
          <span className="text-mk-body text-mk-muted">
            {/* 🚨 数数这件事交给下面那一行，这里不重复。
                原来这句写的是「下面那 N 条你已经做完」—— 两处毛病：
                做完的那几条已经**折起来**了、不在「下面」（这是我把横幅从面板
                底部挪到顶部时顺手改坏的，位置变了指代没跟着变）；而且它和折叠
                那一行说的是同一句话，同一块面板上出现两遍。
                2026-09-12 第二十一轮她照着这句去数可见的卡片，数不上：
                「提示说『下面那1条你已经做完』，但我数了一下只有两条建议卡片，
                  不知道第三条是什么、在哪里改的」。
                横幅只说一件事：这一段动过了。几条、在哪儿，下面那一行自己会说。

                🚨 **但「总评说的是上一版」这半句不能跟着一起删掉。**
                上一版我把它砍成了「这一段改过了。」—— 为的是不和折叠那一行
                重复数数，顺手把唯一指着总评的那半句也砍了。第二十八轮她为这个
                卡了三步，中文那一路八条卡壳里占四条：

                  「正文里明明已经有『低头刷短视频』和『像扔掉一张用过的纸巾
                    一样自然』了，它还说我缺这些，搞不懂是不是没刷新还是嫌我
                    写得不够透」
                  「印记的反馈跟我正文里已经写好的内容对不上」

                逐条点评会折起来，**总评不会** —— 它是一句人话，没法逐条过滤，
                于是它始终以满字重挂在最上面，对着一版旧正文下判断。
                她的原话给出了判据：她分不清「没刷新」和「嫌我写得不够透」。
                横幅要替她答的就是这一个问题，所以两支都得指着总评说。
                数数仍然不在这里 —— 那是折叠那一行的事。 */}
            {/* 🚨 **说的是「下面所有的」，不是只有总评。**
                上一版这句只点了总评（那一轮她卡的正是总评）。第三十五轮中文那一路
                五条卡壳全在说下面那几条**意见**：

                  「印记说这两个问题还没改，但我看正文里明明已经加了」
                  「李浩那条意见我明明在第三段加了好几句解释了，它还显示在没改的里面」
                  「我在正文里明明已经按它说的改过了，但它下面还显示着旧的提示，感觉没更新」

                为什么逐条折叠救不了这几条：折叠的判据是「被引的那句话不在了」，
                而她的做法是**另加一句**，被引的那句原封不动 ⇒ 判据说「还算数」，
                于是它留在没折起来的那一堆里，读起来就是「你还没做」。
                判错的方向恰好是最伤的那个：她做完了，产品说她没做。

                不改成「做完了就藏起来」—— 我们并不知道她加的那句够不够
                （[[feedback-staleness-asymmetry]]：把一条还有效的意见标成做完，
                等于告诉她一件没做完的事不用做了）。能诚实说的只有版本：
                这些话读的是哪一版。剩下的交给她按那颗按钮。 */}
            {/* 🚨 说完版本还要说**下一步**。
                第三十九轮她读完这句之后的原话：「那我现在看到的这些意见到底
                还有没有用？我还要不要再改？没说清楚。」—— 我们告诉了她这些话
                是对着哪一版说的，却没说她该拿它怎么办，于是她卡在原地。
                答案本来就在旁边那颗按钮上，只是没有一句话把两者连起来。

                🚨 指的时候要**叫它的名字**，不要说方位。上一版写的是「点右边」，
                第四十轮她就卡在这儿：「它说『点右边让它重看一遍』，但我只看到
                『请印记再看一遍』这个按钮，不确定是不是就是它说的那个」——
                屏幕上东西一多，「右边」就不是一个地址。 */}
            这一段改过了。下面的总评和意见，读的都是你改之前那一版 —— 还没改的照样可以按着做；都改完了就按「请印记再看一遍」。
          </span>
          <Button variant="secondary" size="sm" onClick={onRecheck} loading={rechecking}>
            请印记再看一遍
          </Button>
        </div>
      )}

      {/* 🚨 分级标签排在总评**上面**：她第一眼要看到的是「这一段算什么」，
          而不是一句读不出轻重的评语（同事 2026-09-20 的意见 10：分级反馈）。
          老评论没有 verdict ⇒ 整行不渲染，不补一个等级上去。 */}
      {verdictLabel && (
        <span
          className="w-fit rounded-mk-full px-2.5 py-0.5 text-mk-label font-semibold"
          style={verdictStyle}
        >
          {verdictLabel}
        </span>
      )}

      <p className="text-mk-body-lg font-semibold text-mk-ink">{comment.summary}</p>

      {livePoints.length > 0 && (
        <ol className="flex list-none flex-col gap-3">
          {livePoints.map((point, i) => (
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

      {/* 改完之后那一下。没有它，这一轮就永远不结束 ——
          她照着改了，屏幕上还是那几条旧话，只好反复问印记是不是没看见。 */}
      {/* 已经改过的那几条，折起来。她想回头看还翻得开 —— 收起不等于删掉。 */}
      {stalePoints.length > 0 && (
        <div className="flex flex-col gap-2">
          <button
            type="button"
            onClick={() => setShowStale((v) => !v)}
            className="w-fit text-mk-small text-mk-muted underline hover:text-mk-accent-700"
          >
            {showStale ? "收起" : `你已经改过的 ${stalePoints.length} 条`}
          </button>
          {showStale && (
            <ol className="flex list-none flex-col gap-2">
              {stalePoints.map((point, i) => (
                <li
                  key={i}
                  data-comment-stale
                  className="rounded-mk-sm border border-mk-border p-2.5 text-mk-small text-mk-muted"
                >
                  <span className="block text-mk-ink">{point.text}</span>
                  <span className="mt-1 flex items-start gap-1.5">
                    <Icon icon={Quote} size={12} className="mt-[3px] shrink-0" />
                    <span>{point.quote}</span>
                  </span>
                </li>
              ))}
            </ol>
          )}
        </div>
      )}

    </div>
  );
}
