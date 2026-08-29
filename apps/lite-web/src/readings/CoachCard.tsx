import { useEffect, useId, useRef, useState } from "react";

/**
 * CoachCard — 印记 把这一步递到她手上，让她点。
 *
 * A card is a question 印记 wrote into its own reply, answered by tapping
 * instead of by typing a paragraph. The product ruling it exists to serve:
 *
 *   > the questions are ladders, we use these to support students to dive to
 *   > think deep to connect his own interests. we do use them as gate
 *   > sometimes to avoid students' 胡乱阅读, but we do not serve as an exam,
 *   > 不要变成标准化考试, we think each child's thoughts are precious.
 *
 * 🚨 **这张卡片上永远不会出现对错。** 没有 ✓、没有 ✗、没有分数、没有连胜
 * （铁律②）。这不是「暂时没做」——服务端根本不发 answer key，因为**不存在**
 * 一个正确答案：`validateCoachCard`（`reading_coach_card.go`）校验的是
 * 「选项是不是文章里真有的一句话」，不是「哪一句对」，而 prompt 里写死了
 * 「问她的判断，不能有唯一正解」。她点下去的是一个**选择**，不是一次作答尝试。
 * 所以这里没有任何地方需要「判分」的信息，也不许有人后来补上。
 *
 * ## 为什么不用 `@/cards/CardRenderer`
 *
 * 那是工具卡的渲染器，`CardSpec` 的 Zod 要求
 * `id/category/name/purpose/trigger_condition/steps/rubric_tags` 全齐，
 * 并且 `card_id` 必须在 `CARD_REGISTRY` 里解析得到——模型现写的一张三选一
 * 卡片永远满足不了，为它去编这些字段等于把契约弄假。lite 自己写一个窄组件。
 *
 * ## 三种形状
 *
 * - `choose_span`     文章里 2–4 句原话，点一句。
 * - `pick_in_article` 不给选项：这一步要她自己回文章里去指。卡片只说话，
 *                     指的动作发生在文章上（房间的引用条），Task 8 接线。
 * - `short_text`      她用自己的话写一句。
 *
 * 已作答的卡片仍然留在对话里显示她选了什么——那是她说过的话，不该在刷新之后
 * 消失——但**不再可点**：第二次点击只会变成第二条学生消息。
 *
 * ## `stale`：一次只问一个（铁律③）
 *
 * 印记 给一张卡 → 她不理，直接打字 → 印记 又给一张。两张都敞开的话，她得先猜
 * 房间到底要哪一个——这正是铁律③ 要消除的认知负担。旧的那张收成一行问题。
 *
 * 🚨 **不是置灰。** 一排死掉的灰色 UI 读起来就是「这是你没做完的所有事」，
 * 计分板的情绪从后门溜进来了，而这个房间恰恰最不能有计分板。折叠保留了记录
 * （印记 确实问过），同时只有一张可操作，读起来是「我们往下走了」。
 *
 * 🚨 **必须能点开。** 配对循环（ReadingCoachPanel `cards`）特意支持「新卡出现
 * 之后仍然回去答旧卡」，折叠不许把它弄丢；而且**由她选择重新打开，房间不替她
 * 关死**——这条是铁律②。
 */

export type CoachCardType = "choose_span" | "pick_in_article" | "short_text";

/** 卡片上的一个选项：文章里某一段（blockId）的某一句原话（quote）。 */
export type CoachCardOption = { blockId: string; quote: string };

/** 服务端 `coachCard` 键上那张卡片（`reading_coach.go` 的响应）。 */
export type CoachCardSpec = {
  type: CoachCardType;
  prompt: string;
  /** 只有 `choose_span` 有；另外两种服务端会清空。 */
  options?: CoachCardOption[];
};

/**
 * 她的作答，原样发给 `POST /readings/{id}/coach` 的 `cardAnswer`。
 *
 * 🚨 `choice` 必须是她点中的那句**原文照抄**——不要 trim 掉标点、不要做任何
 * 改写。服务端拿它回文章里做字面子串核对来分辨「原文」和「她自己的话」
 * （`quoteIsArticleText`）；核不上就当成她的话处理，「她指了」这件事就没了，
 * hunt 步也就落不了地。
 */
export type CoachCardAnswer = {
  type: CoachCardType;
  prompt: string;
  choice: string;
  blockId?: string;
};

export function CoachCard({
  card,
  onAnswer,
  answered,
  busy = false,
  stale = false,
}: {
  card: CoachCardSpec;
  /** 她点了/写了。调用方负责把它发出去并把 `answered` 传回来。 */
  onAnswer: (answer: CoachCardAnswer) => void;
  /** 已经答过了：显示她的选择，整张卡片停止响应。 */
  answered?: CoachCardAnswer | null;
  /** 这一轮还在飞——不接第二次作答。 */
  busy?: boolean;
  /** 后面又来了一张还没答的卡：这张收起来，点一下能重新展开。 */
  stale?: boolean;
}) {
  const [draft, setDraft] = useState("");
  const [reopened, setReopened] = useState(false);
  const promptId = useId();
  const options = card.options ?? [];
  const done = Boolean(answered);
  // 已作答的卡片永远不折叠：她说过的话不该缩回去。
  const collapsed = stale && !done && !reopened;
  // 挂载时就已经答过 → 这是刷新回来的旧卡，别抢焦点（她刚进房间，不该被拽到
  // 某张旧卡上）。只有「在她眼前从未答变成已答」才需要有人接住焦点。
  const answeredAtMount = useRef(done);

  function answer(choice: string, blockId?: string) {
    if (done || busy || collapsed) return;
    onAnswer({ type: card.type, prompt: card.prompt, choice, ...(blockId ? { blockId } : {}) });
  }

  if (collapsed) {
    // 一行问题 + 一句邀请。没有「未完成」、没有计数、没有任何在记账的字眼——
    // 这是一扇还开着的门，不是一条对她的记账。
    return (
      <button
        type="button"
        aria-expanded={false}
        onClick={() => setReopened(true)}
        className="mk-coachcard my-1.5 flex w-full flex-col gap-0.5 rounded-mk-lg border border-mk-border px-3 py-2 text-left transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
        style={{
          // mk-* 是裸 CSS 变量，alpha 语法一个字节都不生成：只能 color-mix。
          background: "color-mix(in srgb, var(--mk-accent-50) 35%, var(--mk-surface))",
        }}
      >
        <span className="text-mk-small leading-relaxed text-mk-muted">{card.prompt}</span>
        <span className="text-mk-small text-mk-faint">想回来答这个，点一下就行。</span>
      </button>
    );
  }

  return (
    <div
      role="group"
      aria-labelledby={promptId}
      className="mk-coachcard my-1.5 flex flex-col gap-2 rounded-mk-lg border border-mk-accent-200 p-3"
      style={{
        // mk-* 是裸 CSS 变量：`bg-mk-accent-50/60` 这类 alpha 语法一个字节的
        // CSS 都不会生成（静默透明）。半透明只能走 color-mix。
        background: "color-mix(in srgb, var(--mk-accent-50) 70%, var(--mk-surface))",
      }}
    >
      <p id={promptId} className="text-mk-body leading-relaxed text-mk-ink">
        {card.prompt}
      </p>

      {done ? (
        <HerAnswer answer={answered!} takeFocus={!answeredAtMount.current} />
      ) : card.type === "choose_span" ? (
        <>
          <ul className="flex flex-col gap-1.5">
            {options.map((o, i) => (
              <li key={`${o.blockId}-${i}`}>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => answer(o.quote, o.blockId)}
                  className="w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-left text-mk-small leading-relaxed text-mk-ink transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {o.quote}
                </button>
              </li>
            ))}
          </ul>
          {/* 说出来，免得她当成一道题：这句话就是这张卡片的立场。 */}
          <p className="text-mk-small text-mk-faint">点一句就行，怎么想都算你的。</p>
        </>
      ) : card.type === "pick_in_article" ? (
        <p className="text-mk-small text-mk-faint">在左边文章里点出那一句，点了就会出现在输入框上面。</p>
      ) : (
        <div className="flex flex-col gap-2">
          <textarea
            // placeholder 不是可及名称的归宿：读屏用户要听见的是 印记 问的那
            // 个问题，不是「写一句你自己的话就够了」。
            aria-label={card.prompt}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            rows={2}
            disabled={busy}
            placeholder="写一句你自己的话就够了"
            className="mk-scroll w-full resize-none rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-small leading-relaxed text-mk-ink placeholder:text-mk-faint focus:border-mk-accent-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:opacity-60"
          />
          <div className="flex justify-end">
            <button
              type="button"
              disabled={busy}
              onClick={() => {
                const text = draft.trim();
                // 一句空白不是作答：发出去只会变成一条空的学生消息。
                if (!text) return;
                setDraft("");
                answer(text);
              }}
              className="rounded-mk-full px-3 py-1 text-mk-small text-white transition-opacity duration-[120ms] ease-mk hover:opacity-90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:cursor-not-allowed disabled:opacity-60"
              style={{ background: "var(--mk-accent-500)" }}
            >
              说说看
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * 她答过之后卡片剩下的东西：**只有她的选择**。
 *
 * 故意不把没被选的几个选项一起留在这里。把它们并排摆着，中间还有一个被标出来
 * 的，屏幕上读起来就是「答案对照表」——而这张卡片从头到尾没有一个正确答案。
 * 留下的这一句是她说过的话，不是她的成绩。
 */
function HerAnswer({ answer, takeFocus = false }: { answer: CoachCardAnswer; takeFocus?: boolean }) {
  const hersInHerOwnWords = answer.type === "short_text";
  // 🚨 她按的那个按钮在作答之后被卸载了。什么都不接手的话，焦点掉回 `<body>`
  // ——用键盘或读屏的学生答完一题，回到文档顶部，而且没有任何提示告诉她刚才
  // 发生了什么。`role="status"` 让这块被念出来，`tabIndex={-1}` 让它能被聚焦
  // 而不进 tab 序列。
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (takeFocus) ref.current?.focus?.();
  }, [takeFocus]);
  return (
    <div
      ref={ref}
      role="status"
      tabIndex={-1}
      className="flex flex-col gap-1 rounded-mk-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <span className="text-mk-small text-mk-faint">{hersInHerOwnWords ? "你写的" : "你选的"}</span>
      <p
        className="rounded-mk-md border-l-2 px-3 py-2 text-mk-small leading-relaxed text-mk-ink"
        style={{
          borderLeftColor: "var(--mk-accent-400)",
          background: "color-mix(in srgb, var(--mk-surface) 80%, transparent)",
        }}
      >
        {hersInHerOwnWords ? answer.choice : `“${answer.choice}”`}
      </p>
    </div>
  );
}
