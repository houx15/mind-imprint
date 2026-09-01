import { useEffect, useMemo, useRef, useState } from "react";
import { X } from "lucide-react";
import { Icon } from "@/ui";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { Composer } from "@/studio/ai/Composer";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
import { ApiError } from "../api/client";
import { getWritingBlockThread, postWritingBlockDeepen } from "../api/writingRoom";
import type { LiteMessage } from "../api/readingRoom";
import { apiErrorText } from "../api/errorText";

/**
 * DeepenDrawer — 深入一层, the side conversation about ONE block.
 *
 * ## The one rule that outranks everything else here
 *
 * **She meets no second character.** The drawer is headed 「印记 · 关于这一块」:
 * the same name, no new name, no avatar, no 「助手」/「小老师」 label, and no
 * copy anywhere in this file that implies someone else is talking. This
 * product spent an entire session collapsing the reading room's two AIs into
 * one; 深入一层 must not quietly undo that. It is a context-isolated
 * sub-agent in the SERVER's implementation (writing_deepen.go) and nowhere
 * else — the isolation buys focus and cost, not a persona.
 *
 * Because it is the same 印记, it looks like 印记: the same `ChatLog` bubbles
 * and the same `Composer` the room and 结构 already use, not a bespoke bubble
 * style invented for this drawer.
 *
 * ## 铁律①
 *
 * There is NO "把这句放进去" button, and there never should be. The endpoint
 * has no write path to her snippets (structural), and this component adds no
 * affordance that would let her one-tap a model sentence into her paragraph
 * (deliberate). She can of course select and copy text, the same as any
 * chat — the line we hold is that the product never hands it over for her.
 *
 * ## Why the thread is fetched rather than passed in
 *
 * It persists per block server-side, so reopening a block she talked about
 * last week returns to that conversation. Holding it in the parent would mean
 * 段落 carrying a thread per block it never renders.
 */
export function DeepenDrawer({
  writingId,
  outlineId,
  heading,
  onClose,
}: {
  writingId: string;
  outlineId: string;
  /** The block this conversation is about, shown under the heading so she
   *  can always see WHICH block she opened — the drawer is scoped, and a
   *  scoped conversation that doesn't say what it is scoped to is confusing. */
  heading: string;
  onClose: () => void;
}) {
  const [messages, setMessages] = useState<LiteMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Optimistic local turns only need ids that can never collide with a
  // persisted seq — the same negative-counter trick PlanningView uses.
  const localSeq = useRef(-1);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void getWritingBlockThread(writingId, outlineId)
      .then((rows) => {
        if (!cancelled) setMessages(rows);
      })
      .catch(() => {
        // An unreadable thread is not worth an error banner: she can still
        // talk, and the turn she sends lands in the same persisted thread.
        if (!cancelled) setMessages([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [writingId, outlineId]);

  // Esc closes. A drawer that can only be dismissed by hitting a small × is
  // a trap on a laptop, and this one opens over the paragraph she is writing.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const chatMessages: ChatMessage[] = useMemo(
    () =>
      messages
        .filter((m) => m.role === "student" || m.role === "ai")
        .map((m) => ({
          id: `d${m.seq}`,
          role: m.role === "ai" ? "assistant" : "student",
          node: m.role === "ai" ? <ChatMarkdown text={m.content} /> : m.content,
        })),
    [messages],
  );

  async function send() {
    const text = draft.trim();
    if (!text || sending) return;
    setDraft("");
    setSending(true);
    setError(null);
    const optimistic: LiteMessage = { seq: --localSeq.current, role: "student", content: text, createdAt: "" };
    const withStudent = [...messages, optimistic];
    setMessages(withStudent);
    try {
      const reply = (await postWritingBlockDeepen(writingId, outlineId, text)).trim();
      if (reply) {
        setMessages([...withStudent, { seq: --localSeq.current, role: "ai", content: reply, createdAt: "" }]);
      }
    } catch (err) {
      // Her turn IS persisted server-side before the model is ever called
      // (writing_deepen.go), so rolling the bubble back locally would show
      // her something the reload will contradict. Keep it, say what failed.
      setError(apiErrorText(err));
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <div
        aria-hidden="true"
        onClick={onClose}
        className="absolute inset-0"
        style={{ background: "color-mix(in srgb, var(--mk-ink) 32%, transparent)" }}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="印记 · 关于这一块"
        className="relative z-10 flex h-full w-full max-w-[460px] flex-col border-l border-mk-border bg-mk-surface shadow-mk-lg"
      >
        <header className="flex shrink-0 items-start justify-between gap-3 border-b border-mk-border px-4 py-3">
          <div className="flex min-w-0 flex-col gap-0.5">
            {/* Same 印记. The heading names the SCOPE of the conversation,
                never a second speaker. */}
            <h2 className="text-mk-body font-semibold text-mk-ink">印记 · 关于这一块</h2>
            <p className="truncate text-mk-small text-mk-muted">{heading || "这一段"}</p>
          </div>
          <button
            type="button"
            aria-label="关上"
            onClick={onClose}
            className="shrink-0 rounded-mk-sm p-1 text-mk-faint hover:text-mk-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          >
            <Icon icon={X} size={16} />
          </button>
        </header>

        {error && (
          <div
            role="alert"
            className="shrink-0 px-4 py-2 text-mk-small text-mk-danger"
            style={{ background: "var(--mk-danger-bg)" }}
          >
            {error}
            <button type="button" className="ml-3 underline" onClick={() => setError(null)}>
              知道了
            </button>
          </div>
        )}

        {/* The empty state is prose at reading size, not a 11px hint strip:
            it is the first thing she reads in here, and it has to say what
            this conversation is for and what it will not do. */}
        {!loading && chatMessages.length === 0 && (
          <div className="shrink-0 px-4 pt-4">
            <p className="text-mk-body-lg text-mk-ink">
              这里只聊这一块。你卡在哪儿、想到什么、拿不准哪句话——都可以直接说。
            </p>
            <p className="mt-2 text-mk-body text-mk-muted">我会一直问下去，帮你把想说的挖出来，但这一段还是你自己写。</p>
          </div>
        )}

        <ChatLog messages={chatMessages} thinking={sending || loading} className="min-h-0 flex-1 px-4 py-4" />

        <div className="shrink-0 px-4 pb-4">
          <Composer
            value={draft}
            onChange={setDraft}
            onSend={() => void send()}
            state={sending ? "replying" : undefined}
            placeholder="说说你卡在哪儿"
          />
        </div>
      </div>
    </div>
  );
}
