import { useEffect, useMemo, useRef, useState } from "react";
import { Play } from "lucide-react";
import { Button, Icon, Pebble } from "@/ui";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { Composer } from "@/studio/ai/Composer";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
import type { ReadingCoachSlot } from "./ReadingRoom";
import { CoachCard, type CoachCardAnswer, type CoachCardSpec } from "./CoachCard";
import { ApiError } from "../api/client";
import { coachAnswerOf, coachCardOf, postReadingCoachTurn, type ReadingTask } from "../api/readingRoom";
import type { LiteMessage } from "../api/readingRoom";

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
 * coach decides whether that counted and says what is next. The steps live in
 * the rail beside the article (ReadingPlanRail) as **progress she can see**
 * rather than controls she operates.
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
}) {
  const [messages, setMessages] = useState<LiteMessage[]>(initialMessages);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const localSeq = useRef(-1);

  const started = messages.length > 0;
  // Everything settled → the walk is over. Derived from the plan rather than
  // remembered from the last turn's flag, so a reload lands in the same state.
  const finished = tasks.length > 0 && tasks.every((t) => t.status !== "pending");
  // 找一找 (hunt): the current step only settles when she POINTS at a
  // sentence, not when she describes one — the hint says so, right above
  // wherever her quote chips are about to appear.
  const hunting = tasks.find((t) => t.status === "pending")?.kind === "hunt";

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
    for (const m of messages) {
      if (m.role === "ai") {
        const card = coachCardOf(m);
        if (card) {
          cardBySeq.set(m.seq, card);
          open.push({ seq: m.seq, card });
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
    return { cardBySeq, answerBySeq, open: open.at(-1) ?? null };
  }, [messages]);

  async function turn(
    text: string,
    picks: { blockId: string; quote: string }[] = [],
    cardAnswer: CoachCardAnswer | null = null,
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
    try {
      const res = await postReadingCoachTurn(readingId, text, picks, cardAnswer);
      setMessages((prev) => [
        ...prev,
        {
          seq: --localSeq.current,
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
      setError(err instanceof ApiError ? err.message : "印记这次没接上，再试一次。");
      // By identity, not by position: the failed turn's message is not
      // necessarily the last one any more once a card answer can add a row of
      // its own, and slicing the tail off would eat somebody else's turn.
      if (mine) setMessages((prev) => prev.filter((m) => m !== mine));
      setDraft(text);
    } finally {
      setBusy(false);
    }
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
  const chatMessages: ChatMessage[] = [];
  for (const m of messages) {
    if (m.role === "ai") {
      chatMessages.push({ id: `c${m.seq}`, role: "assistant", node: <ChatMarkdown text={m.content} /> });
      const card = cards.cardBySeq.get(m.seq);
      if (card) {
        chatMessages.push({
          id: `card${m.seq}`,
          // `system` is the one role ChatBubble renders with NO bubble chrome
          // — the card brings its own frame and wants the column's full width,
          // not 85% of it inside a speech bubble. The wrapper undoes that
          // row's `text-center`, which is meant for 「印记 summoned a card」
          // one-liners, not for something she reads and answers.
          role: "system",
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
                onAnswer={send}
              />
            </div>
          ),
        });
      }
      continue;
    }
    if (coachAnswerOf(m)) {
      // Her answer is already on the card above — rendering the stored message
      // too would say the same sentence twice, and say it in the raw `> ` form
      // composeCardAnswerMessage stores it in. What is left after the quoted
      // lines are dropped is whatever she typed alongside the tap, which is
      // hers and belongs in the log.
      const own = ownWords(m.content);
      if (own) chatMessages.push({ id: `c${m.seq}`, role: "student", node: own });
      continue;
    }
    chatMessages.push({ id: `c${m.seq}`, role: "student", node: m.content });
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
          <p className="text-mk-h2 text-mk-ink">让我来带你详细读一遍这篇文章吧。</p>
          <p className="text-mk-body leading-relaxed text-mk-muted">
            我先看看这篇，排一条路线，然后一步一步带你走。中途想跳过哪一步，说一声就行。
          </p>
        </div>
        <Button onClick={() => void turn("")} loading={busy} iconStart={<Icon icon={Play} size={14} />}>
          开始
        </Button>
        {error && <p className="text-mk-small text-mk-danger">{error}</p>}
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2 pt-1">
      <div className="mk-scroll min-h-0 flex-1 overflow-y-auto pr-1">
        <ChatLog messages={chatMessages} thinking={busy} />
        <div ref={endRef} data-scroll-anchor="coach-end" />
      </div>

      {error && <p className="shrink-0 text-mk-small text-mk-danger">{error}</p>}

      <div className="shrink-0 flex flex-col gap-2">
        {hunting && !slot.locked && (
          <p className="text-mk-small text-mk-muted">在文章里点出那一句，点了就会出现在这里</p>
        )}
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

/**
 * What is HERS in a stored card-answer message. `composeCardAnswerMessage`
 * puts 印记's question and the article sentence she pointed at behind `> `
 * prefixes and leaves only her own words bare — which is the same rule
 * report_facts.go's stripQuotedLines reads the transcript by. Reusing it here
 * means the log shows her exactly the words that will ever be quoted back to
 * her as hers.
 */
function ownWords(content: string): string {
  return content
    .split("\n")
    .filter((line) => !line.trimStart().startsWith(">"))
    .join("\n")
    .trim();
}
