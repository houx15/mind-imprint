import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Play } from "lucide-react";
import { Button, Icon, Pebble } from "@/ui";
import { Composer } from "@/studio/ai/Composer";
import type { ReadingCoachSlot } from "./ReadingRoom";
import { CoachCard, type CoachCardAnswer, type CoachCardSpec } from "./CoachCard";
import { LiteChatMarkdown } from "./LiteChatMarkdown";
import { ThinkingFold } from "./ThinkingFold";
import { ApiError } from "../api/client";
import {
  coachAnswerOf,
  coachCardOf,
  postReadingCoachTurn,
  type ReadingLensDone,
  type ReadingTask,
} from "../api/readingRoom";
import type { LiteMessage } from "../api/readingRoom";
import { apiErrorText } from "../api/errorText";

/**
 * ReadingCoachPanel — 带读: 印记 leads, she doesn't manage stages.
 *
 * The product call that replaced the checklist:
 *
 *   > merge the tasks with the AI bar. at start the AI begins with 让我来带你
 *   > 详细阅读这篇文章吧, clicks 开始. then AI generates the plan, introduces
 *   > the plan, then we will enter a stage directly. student doesn't handle the
 *   > stages themselves, but the AI directs these.
 *
 * So there are no 做完了 / 跳过 buttons here. She reads and she answers; the
 * coach decides whether that counted and says what is next. The steps live on
 * the floating dial in the room's corner (ReadingPlanDial) as **progress she
 * can see** rather than controls she operates.
 *
 * Skipping did not disappear. It moved into language: she says 这步跳过吧, and
 * the coach records it (铁律④) without arguing. A student who wants out of a
 * step should not have to hunt for the button that admits it.
 *
 * ## This panel IS the room's chat (2026-08-28)
 *
 * It used to live in a rail of its own, beside a room that ran its own AI
 * chat — two composers, two logs, 印记 talking in both:
 *
 *   > we don't have two AIs. only one AI talks.
 *
 * It is now mounted INSIDE `ReadingRoom`'s coach column through that
 * component's `renderCoach` slot, and it is the only conversation in the
 * room. The slot hands back the two things a composer cannot do for itself:
 * the sentences she picked out of the article, and whether a lens is open on
 * the article (in which case talking should wait). 透镜库 moved to the
 * article's own toolbar, beside 完成这篇 — it acts on the article, and putting
 * it there keeps it reachable before 带读 has even started.
 *
 * Both this endpoint and the room's own turn endpoint have always written to
 * the SAME `atom_message` table — which is why merging them cost no migration
 * and why `initialMessages` restores a conversation started either way.
 */
export function ReadingCoachPanel({
  readingId,
  tasks,
  initialMessages,
  slot,
  onTasks,
  onFocusBlock,
  lensDone,
  onLensDoneSent,
  onFinish,
}: {
  readingId: string;
  tasks: ReadingTask[];
  /** The persisted transcript. Without it a reload showed her the 开始
   *  invitation again on a reading she was halfway through. */
  initialMessages: LiteMessage[];
  slot: ReadingCoachSlot;
  onTasks: (next: ReadingTask[]) => void;
  /** `tool` is set when 印记 reached for a paragraph tool this turn. */
  onFocusBlock: (blockId: string, tool?: string) => void;
  /** A lens the room just watched her finish. Non-null for exactly as long as
   *  it takes this panel to turn it into one coach turn; the room clears it
   *  through `onLensDoneSent`. See the effect below. */
  lensDone: ReadingLensDone | null;
  /** Called once the turn for `lensDone` has been ATTEMPTED — success or
   *  failure. Failure must still clear it: a lens she finished is not worth
   *  retrying forever against a server that is down, and the outcome itself is
   *  already saved either way. */
  onLensDoneSent: () => void;
  /** Opens the room's 完成这篇 confirmation. Called from the panel because
   *  「每一步都做完了」 is something only this panel can see (it owns `tasks`),
   *  while the confirm dialog and the finish call belong to the room. */
  onFinish: () => void;
}) {
  const [messages, setMessages] = useState<LiteMessage[]>(initialMessages);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  /** 上一轮**带着她的作答**却没送出去的那一份，原样留着好重发。 */
  const [failed, setFailed] = useState<null | {
    text: string;
    picks: { blockId: string; quote: string }[];
    cardAnswer: CoachCardAnswer;
    mine: LiteMessage;
  }>(null);
  const localSeq = useRef(-1);
  /** 这一轮的思考过程，按 seq 存在内存里。
   *
   *  刻意不写进 messages：思考过程**不入库**（服务端也不写进 transcript），
   *  所以刷新之后它就没有了。把它塞进消息记录会让人以为它是被保存的，
   *  于是有人开始拿它当过程证据用——模型的草稿不是她的记录，过程树才是。 */
  const [thinkingBySeq, setThinkingBySeq] = useState<Record<number, string>>({});

  const started = messages.length > 0;
  // Everything settled → the walk is over. Derived from the plan rather than
  // remembered from the last turn's flag, so a reload lands in the same state.
  const finished = tasks.length > 0 && tasks.every((t) => t.status !== "pending");

  // Which card each 印记 message carried, and which of them she has already
  // answered. Derived from the transcript rather than remembered in a state of
  // its own, so a card that arrived three sessions ago comes back exactly as
  // it was: the transcript IS the storage (atom_message.payload, 0106).
  //
  // An answer is paired to the card whose question it repeats, falling back to
  // the newest still-open card. Prompt-first, because she is free to answer an
  // older card after a newer one has appeared, and position alone would then
  // hang her answer on the wrong question.
  const cards = useMemo(() => {
    const cardBySeq = new Map<number, CoachCardSpec>();
    const answerBySeq = new Map<number, CoachCardAnswer>();
    const open: { seq: number; card: CoachCardSpec }[] = [];
    let newest: number | null = null;
    for (const m of messages) {
      if (m.role === "ai") {
        const card = coachCardOf(m);
        if (card) {
          cardBySeq.set(m.seq, card);
          open.push({ seq: m.seq, card });
          newest = m.seq;
        }
        continue;
      }
      const answer = coachAnswerOf(m);
      if (!answer || open.length === 0) continue;
      let i = open.length - 1;
      for (let k = open.length - 1; k >= 0; k--) {
        if (open[k]!.card.prompt === answer.prompt) {
          i = k;
          break;
        }
      }
      answerBySeq.set(open[i]!.seq, answer);
      open.splice(i, 1);
    }
    // 铁律③ 一次只问一个：只有**对话里最后到达的那张**卡片是敞开的，它之前的
    // 都收起来（CoachCard `stale`）。折叠 ≠ 关死——她点一下就能回去答，配对循环
    // 上面按 prompt 认卡，就是为了这件事。
    //
    // 🚨 判据是「后面还有没有更新的卡片」，不是「它是不是最新的那张未答卡片」。
    // 按后者算的话（`open.slice(0, -1)`），她答掉当前这张之后，一张早就折起来的
    // 旧卡片会因为顶上了「最新未答」的位置而**自己弹开**，下一条回复到达时又折
    // 回去——真实走查的 `05-turn1-card-AFTER-tap.png` 拍到的就是这一下抖动。
    // 折叠状态该跟着「她是不是已经往下走了」，而这件事一旦发生就不会倒退。
    const stale = new Set(open.filter((o) => newest !== null && o.seq < newest).map((o) => o.seq));
    // 敞开的那张 = 最后到达的那张，且她还没答。她答完之后没有卡片自动接班：
    // 一张折起来的旧卡片不该在背后悄悄接住她下一次在文章里点的那一句。
    const last = open.at(-1) ?? null;
    return { cardBySeq, answerBySeq, open: last && last.seq === newest ? last : null, stale };
  }, [messages]);

  async function turn(
    text: string,
    picks: { blockId: string; quote: string }[] = [],
    cardAnswer: CoachCardAnswer | null = null,
    finishedLens: ReadingLensDone | null = null,
  ) {
    if (busy) return;
    setBusy(true);
    setError(null);
    // Her side of this turn, shown before the server has spoken. A tap carries
    // its payload the same way the stored row will, so the optimistic state and
    // the reloaded state are the SAME state — the card above renders answered
    // either way, and `content` holds only what is genuinely hers to say.
    const mine: LiteMessage | null =
      text || cardAnswer
        ? {
            seq: --localSeq.current,
            role: "student",
            content: text,
            createdAt: "",
            ...(cardAnswer ? { payload: { answer: cardAnswer } } : {}),
          }
        : null;
    if (mine) setMessages((prev) => [...prev, mine]);
    setFailed(null);
    try {
      const res = await postReadingCoachTurn(readingId, text, picks, cardAnswer, finishedLens);
      const seq = --localSeq.current;
      if (res.thinking) setThinkingBySeq((prev) => ({ ...prev, [seq]: res.thinking }));
      setMessages((prev) => [
        ...prev,
        {
          seq,
          role: "ai",
          content: res.reply,
          createdAt: "",
          ...(res.coachCard ? { payload: { card: res.coachCard } } : {}),
        },
      ]);
      onTasks(res.tasks);
      // The coach names the paragraph this step is about; jumping there is
      // part of leading her, not a separate thing she has to do.
      if (res.focusBlock) onFocusBlock(res.focusBlock, res.tool || undefined);
      // A lens landed on the article from THIS endpoint, not from the room's
      // own turn/summon flow — the room's card state has no way to have
      // picked it up on its own, so it needs telling.
      if (res.card) slot.onCardSummoned?.();
    } catch (err) {
      setError(apiErrorText(err));
      // 🚨 一次 502 不许把她点过的答案偷偷取消掉。
      //
      // 这条乐观消息**带着 `payload.answer`**，所以把它撤掉等于把卡片恢复成未答：
      // 选项全部回来，屏幕上没有任何地方还记得她刚才点的是哪一句，她得自己猜。
      // 真实走查里中过一次。她的选择是她说过的话，一个服务端的坏心情不该抹掉它。
      //
      // 所以带着作答的那一轮**留在原地**（卡片继续显示她选的那一句），失败只表现为
      // 一行错误 + 一个「重试」——她重发的是同一份东西，不用重新回忆。
      // 只打了字的那一轮仍然照旧退回输入框：那句话在输入框里她还能改，
      // 而卡片上的选择改不了。
      if (mine && cardAnswer) {
        setFailed({ text, picks, cardAnswer, mine });
      } else {
        // By identity, not by position: the failed turn's message is not
        // necessarily the last one any more once a card answer can add a row of
        // its own, and slicing the tail off would eat somebody else's turn.
        if (mine) setMessages((prev) => prev.filter((m) => m !== mine));
        setDraft(text);
      }
    } finally {
      setBusy(false);
    }
  }

  /**
   * 她把一副透镜做完了 → 印记 必须回应它，并且推进这一步。
   *
   * 透镜循环是和 pro 共用的（`apps/web/src/studio/reading/readingLoop.ts`），
   * 它的 `confirm()` 把「已保存」写进 `loop.messages`——而 lite 从来不渲染
   * 那个数组（lite 渲染的是这个面板，同一张 `atom_message` 表上的另一条线）。
   * 于是「透镜应用完毕之后，没有响应，没有推进到下一步」：确实一个字都没有，
   * 因为根本没有一轮被送出去。房间现在盯着 `loop.outcomes` 长出新的一条，
   * 把它交给这里，这里把它变成一轮真的对话。
   *
   * 🚨 `text` 是空的，`lensDone` 不是。服务端靠这一点分辨这一轮不是「她刚点了
   * 开始」——空文本那条分支会让 印记 从头介绍一遍读法清单，这正是 PBL 房间
   * 踩过的那颗雷（做完工具之后的空轮让 印记 原话重复、还把刚做完的工具又召
   * 唤了一遍）。见 `reading_coach.go` 的 `readingLensDone`。
   *
   * 🚨 用 `sentRef` 记住「这一条已经送过了」，而不是只依赖 `lensDone` 变 null：
   * StrictMode 会把这个 effect 跑两遍（挂载 → 清理 → 再挂载），而 `busy` 在
   * 第一遍的 `await` 之前就已经是 true 了——但第二遍是在同一个 render 的
   * 闭包里跑的，读到的 `busy` 还是旧值 false。没有这个 ref 就是两次
   * 旗舰调用、两条回复。（`useAlive` 的注释讲的是同一族陷阱的另一半。）
   */
  const lensSentRef = useRef<ReadingLensDone | null>(null);
  useEffect(() => {
    if (!lensDone) {
      lensSentRef.current = null;
      return;
    }
    if (lensSentRef.current === lensDone) return;
    // 🚨 上一轮还在飞的时候不要标记成「送过了」。`turn` 开头的 `if (busy)`
    // 会直接返回，而如果这里已经调了 `onLensDoneSent`，这副她真的做完了的
    // 透镜就被静默丢掉了——她动了手，屏幕上却依然什么都没有，正是这次要修的
    // 那个 bug 的另一条路径。`busy` 进依赖，这一轮落地之后再送。
    if (busy) return;
    lensSentRef.current = lensDone;
    void turn("", [], null, lensDone).finally(onLensDoneSent);
    // `turn` 和 `onLensDoneSent` 每次 render 都会重建，挂进依赖只会让这个
    // effect 每次 render 都重跑；真正的触发条件只有「来了一条新的 lensDone」
    // 和「刚腾出手」，而重复发送由上面的 ref 把住。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lensDone, busy]);

  /** 重发上一轮失败的作答，原样。她点过的那一句一直留在卡片上，这里只是把它
   *  再送一次——先把留着的那条乐观消息撤掉，`turn` 会重新放一条一样的。 */
  function retryFailed() {
    const f = failed;
    if (!f || busy) return;
    setFailed(null);
    setMessages((prev) => prev.filter((m) => m !== f.mine));
    void turn(f.text, f.picks, f.cardAnswer);
  }

  /** Her message, with whatever she quoted out of the article carried in
   *  front of it. The coach endpoint still takes one text field — the quotes
   *  ride inside it as blockquotes, unchanged — and ALSO takes them as
   *  structured `picks` (one per quote that came from a real paragraph),
   *  which is what tells "点了" apart from "打字说了".
   *
   *  `tapped` is set when this turn started on a card instead of in the
   *  composer. It goes through the SAME function on purpose: a tap is a turn
   *  like any other, and routing it around send() would leave the guard below
   *  as a trap the next card type walks into. */
  function send(tapped?: CoachCardAnswer) {
    const text = draft.trim();
    // pick_in_article is the one card shape that has no button of its own: it
    // sends her back to the article, and the tap happens on the paragraph.
    // The quote chip that comes back is her answer — so it is lifted OUT of
    // the quote block and into `cardAnswer`, and left out of `picks`, because
    // the server rebuilds both from `choice` (composeCardAnswerMessage +
    // validateReadingPicks). Sending it twice would put the sentence in the
    // transcript twice and hand the coach a duplicate pick.
    let cardAnswer: CoachCardAnswer | null = tapped ?? null;
    let spent: string | null = null;
    const open = cards.open;
    const pointed = slot.quotes[0];
    if (!cardAnswer && open?.card.type === "pick_in_article" && pointed) {
      const q = pointed;
      cardAnswer = {
        type: "pick_in_article",
        prompt: open.card.prompt,
        choice: q.quote,
        ...(q.blockId ? { blockId: q.blockId } : {}),
      };
      spent = q.key;
    }
    // Pointing counts on its own: a quote chip with nothing typed must still
    // send. So does a tap on a card — she may answer without typing a
    // character, and this guard is the only thing standing between that tap
    // and the server, which accepts an empty `text` beside a `cardAnswer`.
    // Only refuse when there is truly nothing at all — no text AND no quote
    // AND no answer — or while a lens holds the composer locked.
    if ((!text && slot.quotes.length === 0 && !cardAnswer) || slot.locked) return;
    const carried = slot.quotes.filter((q) => q.key !== spent);
    // F1: prefix EVERY line of a quote, not just its first. A drag-selection
    // across a hard-wrapped paragraph (no blank line between its lines —
    // SplitBlocks on the server only splits on "\n\n") returns a quote whose
    // text contains internal "\n"s; prefixing only the first line left the
    // article's later lines sitting in the stored message with no "> " at
    // all, where report_facts.go's stripQuotedLines (which only strips lines
    // that already start with "> ") could not catch them. See
    // report_facts.go's stripArticleLines for the server-side half of this
    // fix, which also repairs transcripts already stored under this bug.
    const quoted = carried
      .map((q) => q.quote.split("\n").map((line) => `> ${line}`).join("\n"))
      .join("\n");
    const picks = carried
      .filter((q) => Boolean(q.blockId))
      .map((q) => ({ blockId: q.blockId as string, quote: q.quote }));
    setDraft("");
    slot.clearQuotes();
    // An empty composer must not leave a bare trailing blank line once the
    // quote block is prepended — quoted alone is already a coherent payload.
    const payload = quoted ? (text ? `${quoted}\n\n${text}` : quoted) : text;
    void turn(payload, picks, cardAnswer);
  }

  // Deliberately NOT memoized: every card's props depend on `busy`, on the
  // lens lock and on `send`, which is rebuilt each render anyway — a useMemo
  // here would either be a lie or never hit.
  const chatMessages: CoachRow[] = [];
  for (const m of messages) {
    if (m.role === "ai") {
      // 印记's turn is markdown; the student's (below) is not. Her literal `*`
      // and `#` are hers to keep. `LiteChatMarkdown` is the shared renderer
      // with accent-coloured bold — see that file for why lite has its own.
      chatMessages.push({
        id: `c${m.seq}`,
        kind: "ai",
        node: (
          <>
            <LiteChatMarkdown text={m.content} />
            <ThinkingFold text={thinkingBySeq[m.seq] ?? ""} />
          </>
        ),
      });
      const card = cards.cardBySeq.get(m.seq);
      if (card) {
        chatMessages.push({
          id: `card${m.seq}`,
          // 卡片不是一句话，所以它不进气泡：它自带边框，也要这一列的整个宽度，
          // 而不是气泡里的 85%。
          kind: "card",
          node: (
            <div className="text-left">
              <CoachCard
                card={card}
                answered={cards.answerBySeq.get(m.seq) ?? null}
                // A lens open on the article counts as busy here for the same
                // reason it locks the composer: 一次只问一个. The server already
                // refuses to mint both in one turn, but an OLD card sitting
                // above a fresh lens could still be tapped.
                busy={busy || slot.locked}
                stale={cards.stale.has(m.seq)}
                onAnswer={send}
              />
            </div>
          ),
        });
      }
      continue;
    }
    const answer = coachAnswerOf(m);
    if (answer) {
      // Her answer is already on the card above — rendering the stored message
      // too would say the same sentence twice, and say it in the raw `> ` form
      // composeCardAnswerMessage stores it in. What is left after the quoted
      // lines are dropped is whatever she typed alongside the tap, which is
      // hers and belongs in the log.
      //
      // 🚨 …except for `short_text`, where her sentence is stored BARE (the
      // server's `case choice != "": own = choice` — those words have to reach
      // her corpus unquoted). Dropping `> ` lines alone leaves exactly her
      // answer standing, and the card above is already showing it: she writes
      // one sentence, refreshes, and sees it twice. `choice` is therefore
      // passed in and peeled off the front.
      const own = ownWords(m.content, answer.choice);
      if (own) chatMessages.push({ id: `c${m.seq}`, kind: "student", node: own });
      continue;
    }
    chatMessages.push({ id: `c${m.seq}`, kind: "student", node: m.content });
  }

  // Scroll the newest turn into view without dragging the whole page.
  const endRef = useRef<HTMLDivElement | null>(null);
  // 🚨 `answered` is a dependency in its own right. A card GROWS IN PLACE when
  // she answers it — the options collapse into her sentence — and her tap adds
  // no visible message (the words live on the card). So the number of rows in
  // the log does not change, and neither ChatLog's own count-driven autoscroll
  // nor a `[messages.length]` effect here would fire: the answered card would
  // quietly grow off the bottom of the panel.
  const answeredCards = cards.answerBySeq.size;
  useEffect(() => {
    // Guarded because jsdom has no scrollIntoView, and an autoscroll must
    // never be the thing that takes the conversation down with it.
    endRef.current?.scrollIntoView?.({ behavior: "smooth", block: "nearest" });
  }, [chatMessages.length, answeredCards]);

  if (!started) {
    return (
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-4 px-6 text-center">
        <Pebble state="idle" size={52} />
        <div className="flex flex-col gap-1.5">
          <p className="text-mk-h2 text-mk-ink">让我来带你详细读一遍这篇文章。</p>
          <p className="text-mk-body leading-relaxed text-mk-muted">
            我先看一遍，排一条阅读路线，然后逐步带你读。想跳过哪一步，随时告诉我。
          </p>
        </div>
        <Button onClick={() => void turn("")} loading={busy} iconStart={<Icon icon={Play} size={14} />}>
          {/* 🚨 排一条读法要跑一次旗舰模型，四十秒起。这段时间里按钮是禁用的，
              而在这之前它上面写的仍然是「开始」—— 屏幕上唯一的动静是一个转圈。
              模拟学生的走查在这里当场卡死：它读到的是「一个按不动的『开始』」，
              于是判定自己没路可走了。真人看得见那个转圈，所以不至于此，但四十秒
              里一个字都不给还是太少。
              「处理中」是 ui-copy-style 第 4 条的那个词（状态用「已/待/中」）。 */}
          {busy ? "处理中" : "开始"}
        </Button>
        {busy && (
          <p className="text-mk-small text-mk-muted">正在通读全文，排一条读法。这一步要花几十秒。</p>
        )}
        {error && <p className="text-mk-small text-mk-danger">{error}</p>}
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2 pt-1">
      {/* `data-coach-log` marks the conversation's own scroll box: a card must
          render INSIDE it, right under the words 印记 asked it with — the
          position IS the design claim (Task 8), and a test that only asks
          「prompt 在某处」 would pass on a rail above the log too. */}
      <div data-coach-log className="mk-scroll min-h-0 flex-1 overflow-y-auto pr-1">
        <CoachLog rows={chatMessages} thinking={busy} />
        <div ref={endRef} data-scroll-anchor="coach-end" />
      </div>

      {error && (
        <div className="shrink-0 flex flex-wrap items-center gap-2">
          <p className="text-mk-small text-mk-danger">{error}</p>
          {/* 只有「她的作答没送出去」那一种失败给重试按钮：她点过的那一句还在
              卡片上，重试送的就是同一份，她不用重新回忆刚才点了什么。
              只打了字的那一轮，字已经回到输入框里了，再放一个按钮反而是两条路。 */}
          {failed && (
            <button
              type="button"
              onClick={retryFailed}
              disabled={busy}
              className="rounded-mk-full border border-mk-accent-200 px-3 py-0.5 text-mk-small text-mk-accent-700 transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:opacity-60"
            >
              重试
            </button>
          )}
        </div>
      )}

      {/* R4 (4)：这里曾经常驻一条面板提示「在文章里点出那一句，点了就会出现在
          这里」。它跟 pick_in_article 卡片自己的脚注「回文章里点出那一句，点完
          它就会出现在对话里」几乎一字不差，而且**上下叠着**——同一句话说了两遍。
          更糟的是它跟着 hunt 这一步走、不跟着卡片走：她答完卡片、屏幕上一张
          敞开的卡片都没有了，这条指令还赖在输入框上面，指着一个此刻没人要她做
          的动作。留卡片上那句（它属于那张卡，卡片答完就跟着收走），面板这句去掉。 */}
      <div className="shrink-0 flex flex-col gap-2">
        {slot.quotes.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5">
            {slot.quotes.map((q) => (
              <span
                key={q.key}
                title={q.quote}
                className="inline-flex max-w-full items-center gap-1 rounded-mk-full border border-mk-accent-200 bg-mk-accent-50 px-2 py-0.5 text-mk-small text-mk-accent-700"
              >
                <span className="truncate">“{q.quote.length > 40 ? `${q.quote.slice(0, 40)}…` : q.quote}”</span>
                <button
                  type="button"
                  aria-label="取消引用这一处"
                  onClick={() => slot.removeQuote(q.key)}
                  className="shrink-0 text-mk-accent-600 hover:text-mk-accent-700"
                >
                  ✕
                </button>
              </span>
            ))}
          </div>
        )}

        {/*
          读法走完之后的那一步。

          🚨 同事试用：「全部完成之后没有引导」。在这之前，「每一步都做完了」
          在屏幕上的全部体现是**输入框的 placeholder 换了一句话**
          （「读完了，还想聊点什么？」）——而 完成这篇 是文章工具栏上一颗常驻
          的小按钮，从第一秒起就在那儿，没有任何时刻会指向它。

          线上数据说明了代价：23 篇阅读里 17 篇状态还是 active，
          lite_report 最后一次触发是 08-31。**没有人走到「完成」**，
          所以报告那一整条链路在生产里根本没被走通过——报告的两个 bug
          之所以一直没被发现，也是因为这个。

          文案按 AGENTS.md §界面文案怎么写：标签是名词（「读法已全部完成」，
          不是「你把这一趟都走完啦」），先说这件事为什么值得做再请她做，
          按钮写「做什么」。感叹号留给真正的节点——这是其中一个。
        */}
        {finished && (
          <div
            className="flex flex-col gap-2 rounded-mk-lg border p-3"
            style={{
              // mk-* 是裸 CSS 变量，Tailwind 的 alpha 语法对它们一个字节的
              // CSS 都不生成：半透明只能走 color-mix。
              background: "color-mix(in srgb, var(--mk-accent-50) 80%, var(--mk-surface))",
              borderColor: "color-mix(in srgb, var(--mk-accent-500) 30%, transparent)",
            }}
          >
            <p className="text-mk-label text-mk-accent-700">读法已全部完成</p>
            <p className="text-mk-small leading-relaxed text-mk-ink">
              报告会汇总这一篇的阅读时长、划线、笔记与透镜发现。完成后本篇不可再修改。
            </p>
            <div className="flex justify-end">
              <Button onClick={onFinish}>完成阅读，生成报告</Button>
            </div>
          </div>
        )}

        <Composer
          value={draft}
          onChange={setDraft}
          // Wrapped, NOT passed by reference: Composer calls onSend with its
          // click/key event, and send()'s first parameter is now a card
          // answer — handing it a SyntheticEvent would ship one to the server.
          onSend={() => send()}
          // Composer (shared, apps/web) derives "empty" vs "typing" from
          // `value` alone when `state` is left undefined — it has no way to
          // know a quote chip exists. Left to that default, the send button
          // stays disabled on an empty box even with a chip queued, and
          // send() never gets called at all. Passing state explicitly here
          // (rather than touching the shared component) is what actually
          // lets pointing alone send.
          state={busy ? "replying" : draft.trim() || slot.quotes.length > 0 ? "typing" : "empty"}
          placeholder={
            slot.locked
              ? "先完成文章里的这副透镜…"
              : finished
                ? "读完了，还想聊点什么？"
                : "读完这一步，跟印记说一声"
          }
        />

      </div>
    </div>
  );
}

/** 日志里的一行：印记 说的一句话、她说的一句话，或者一张卡片。 */
type CoachRow = { id: string; kind: "ai" | "student" | "card"; node: ReactNode };

// 气泡的圆角是 spec §13 的字面值（印记 的尾巴在起头一侧，她的在另一侧），和
// `@/studio/ai/ChatLog` 保持一致 —— 这个房间只是把头像挂到了气泡**外面**。
const AI_RADIUS = "rounded-[4px_13px_13px_13px]";
const HER_RADIUS = "rounded-[13px_4px_13px_13px]";
const BUBBLE = "inline-block max-w-[85%] px-4 py-3 text-mk-body text-mk-ink";

/**
 * CoachLog — 带读 房间自己的对话日志。
 *
 * 产品负责人对着真实截图说的：*"currently in the box ai's chat box and task
 * card is not very clear. task card. AI avatar is necessary."*
 * （`02-first-reply-with-card.png`：印记 的话是一个浅色气泡，卡片是**另一个**
 * 几乎一样浅的框，她分不出「这是在跟我说话」和「这是要我动手的东西」。）
 *
 * 两件事都要在**气泡之外**发生，所以这里没有复用共享的 `ChatLog`：
 *
 *   1. **头像挂在气泡旁边**，不是塞在气泡里。共享 `ChatLog` 的一条消息只有
 *      「气泡里的内容」这一个插口，头像只能进气泡内部——而 `apps/web` 不归这个
 *      任务改（改了会波及 pro 的四个房间）。这里是 lite 自己的一列对话，
 *      多这二十行比去动共享组件安全得多。
 *   2. **卡片根本不是一个气泡**：它自带边框、要整列宽度，`data-chat-row="card"`
 *      让它在 DOM 上就和「谁在说话」分得开。
 *
 * 其余（圆角、白气泡 + 极淡阴影、思考中的三个点）和共享 `ChatLog` 逐字一致：
 * 这是同一个 印记，不该在 lite 里换一套长相。
 */
function CoachLog({ rows, thinking = false }: { rows: CoachRow[]; thinking?: boolean }) {
  return (
    <div className="flex flex-col gap-3">
      {rows.map((row) =>
        row.kind === "card" ? (
          // `data-role="system"` 保留下来：它标的是「这一行不是谁在说话」。
          <div key={row.id} data-chat-row="card" data-role="system" className="text-left">
            {row.node}
          </div>
        ) : row.kind === "ai" ? (
          <div key={row.id} data-chat-row="ai" className="flex items-start justify-start gap-2">
            <span className="mt-0.5 shrink-0">
              <Pebble state="idle" size={24} />
            </span>
            <div data-role="assistant" className={`${BUBBLE} ${AI_RADIUS} bg-mk-surface shadow-mk-xs`}>
              {row.node}
            </div>
          </div>
        ) : (
          // 她那一侧不挂头像：房间里只有一个角色需要被认出来。
          <div key={row.id} data-chat-row="student" className="flex justify-end">
            {/* 🚨 `whitespace-pre-wrap` 只给她这一侧。她那一轮是原样字符串
                （印记 那一轮走 LiteChatMarkdown），而 HTML 会把她敲的换行折掉
                ——同事试用里报的「换行的话发送给AI就不换行了」。印记 那一侧
                绝对不能加：markdown 已经产出块级元素，pre-wrap 会把源码里每个
                换行变成看得见的空行。 */}
            <div data-role="student" className={`${BUBBLE} ${HER_RADIUS} whitespace-pre-wrap bg-mk-accent-50`}>
              {row.node}
            </div>
          </div>
        ),
      )}
      {thinking && (
        <div data-chat-row="ai" className="flex items-start justify-start gap-2" aria-label="印记正在打字">
          <span className="mt-0.5 shrink-0">
            <Pebble state="thinking" size={24} />
          </span>
          <div data-role="assistant" className={`${BUBBLE} ${AI_RADIUS} flex items-center gap-1 bg-mk-surface`}>
            <span className="mk-think-dot" />
            <span className="mk-think-dot [animation-delay:0.15s]" />
            <span className="mk-think-dot [animation-delay:0.3s]" />
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * What is HERS in a stored card-answer message. `composeCardAnswerMessage`
 * puts 印记's question and the article sentence she pointed at behind `> `
 * prefixes and leaves only her own words bare — which is the same rule
 * report_facts.go's stripQuotedLines reads the transcript by. Reusing it here
 * means the log shows her exactly the words that will ever be quoted back to
 * her as hers.
 */
function ownWords(content: string, choice = ""): string {
  const rest = content
    .split("\n")
    .filter((line) => !line.trimStart().startsWith(">"))
    .join("\n")
    .trim();
  const answered = choice.trim();
  if (!answered || !rest) return rest;
  // A `short_text` answer is stored bare, so it survives the `> ` filter and
  // would be said twice (the card is already showing it). Both stored shapes
  // are the same shape: `choice`, optionally followed by whatever she typed
  // alongside the tap after a blank line.
  if (rest === answered) return "";
  if (rest.startsWith(answered)) {
    const tail = rest.slice(answered.length);
    // Only when `choice` is a whole leading LINE — otherwise a typed sentence
    // that happens to open with the same words would get its head cut off.
    if (tail.startsWith("\n")) return tail.trim();
  }
  return rest;
}
