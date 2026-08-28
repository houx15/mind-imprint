import { useEffect, useMemo, useRef, useState } from "react";
import { Play } from "lucide-react";
import { Button, Icon, Pebble } from "@/ui";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { Composer } from "@/studio/ai/Composer";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
import type { ReadingCoachSlot } from "@/studio/reading/ReadingRoom";
import { ApiError } from "../api/client";
import { postReadingCoachTurn, type ReadingTask } from "../api/readingRoom";
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

  async function turn(text: string, picks: { blockId: string; quote: string }[] = []) {
    if (busy) return;
    setBusy(true);
    setError(null);
    if (text) {
      setMessages((prev) => [...prev, { seq: --localSeq.current, role: "student", content: text, createdAt: "" }]);
    }
    try {
      const res = await postReadingCoachTurn(readingId, text, picks);
      setMessages((prev) => [...prev, { seq: --localSeq.current, role: "ai", content: res.reply, createdAt: "" }]);
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
      if (text) setMessages((prev) => prev.slice(0, -1));
      setDraft(text);
    } finally {
      setBusy(false);
    }
  }

  /** Her message, with whatever she quoted out of the article carried in
   *  front of it. The coach endpoint still takes one text field — the quotes
   *  ride inside it as blockquotes, unchanged — and ALSO takes them as
   *  structured `picks` (one per quote that came from a real paragraph),
   *  which is what tells "点了" apart from "打字说了". */
  function send() {
    const text = draft.trim();
    if (!text || slot.locked) return;
    const quoted = slot.quotes.map((q) => `> ${q.quote}`).join("\n");
    const picks = slot.quotes
      .filter((q) => Boolean(q.blockId))
      .map((q) => ({ blockId: q.blockId as string, quote: q.quote }));
    setDraft("");
    slot.clearQuotes();
    void turn(quoted ? `${quoted}\n\n${text}` : text, picks);
  }

  const chatMessages: ChatMessage[] = useMemo(
    () =>
      messages.map((m) => ({
        id: `c${m.seq}`,
        role: m.role === "ai" ? "assistant" : "student",
        node: m.role === "ai" ? <ChatMarkdown text={m.content} /> : m.content,
      })),
    [messages],
  );

  // Scroll the newest turn into view without dragging the whole page.
  const endRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    // Guarded because jsdom has no scrollIntoView, and an autoscroll must
    // never be the thing that takes the conversation down with it.
    endRef.current?.scrollIntoView?.({ behavior: "smooth", block: "nearest" });
  }, [messages.length]);

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
        <div ref={endRef} />
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
          onSend={send}
          state={busy ? "replying" : undefined}
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
