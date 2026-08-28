import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowRight } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { Composer } from "@/studio/ai/Composer";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
import { ApiError } from "../api/client";
import {
  postWritingPlanTurn,
  postWritingOpening,
  putWritingOutline,
  type WritingOutlineItem,
} from "../api/writingRoom";
import type { LiteMessage } from "../api/readingRoom";
import type { Writing } from "../api/writings";
import { EditableTitle } from "./EditableTitle";
import { MindMap } from "./MindMap";

/**
 * PlanningView — 结构, as a full-screen planning conversation.
 *
 * This replaced a template picker on 2026-08-27. The product call:
 *
 *   > 结构 is like planning, is not a fixed one, but guide students to think,
 *   > then compose the structure.
 *
 * So: 印记 asks one question at a time, she answers, and what she says grows
 * onto a mind map at the right. The map is `writing_outline` rendered as a
 * tree, so when she goes to write, the thing she planned IS the outline — no
 * conversion, nothing lost in between.
 *
 * LAYOUT, as specified: full screen while planning. The chat starts centred
 * and alone; the moment the map has anything on it the view splits, chat left
 * and map right — the same shape as Claude opening an artifact panel. An empty
 * panel sitting there from the first second would be a promise the screen
 * hasn't kept yet.
 *
 * NOT A GATE. 「去写」 is available from the first render. A student who
 * already knows what she wants to say should not have to talk her way past a
 * planning screen to reach the page — 铁律②, and the reason planning is a
 * surface rather than a checkpoint.
 */
export function PlanningView({
  writing,
  messages,
  outline,
  onRenamed,
  onMessages,
  onOutline,
  onDone,
  onBack,
}: {
  writing: Writing;
  messages: LiteMessage[];
  outline: WritingOutlineItem[];
  /** The title is editable in every step, 结构 included — this is how the new
   *  one reaches the rest of the room. */
  onRenamed?: (next: Writing) => void;
  onMessages: (next: LiteMessage[]) => void;
  onOutline: (next: WritingOutlineItem[]) => void;
  /** Leaves planning for 段落. */
  onDone: () => void;
  onBack: () => void;
}) {
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [opening, setOpening] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [justAdded, setJustAdded] = useState<string[]>([]);
  // Seq numbers for optimistic local turns. Real rows come back from the
  // server on the next load; these only need to be unique and negative so
  // they can never collide with a persisted seq.
  const localSeq = useRef(-1);

  const hasMap = outline.length > 0;

  /**
   * 印记 opens. Same idempotent endpoint the room uses — calling it twice
   * replays rather than greeting her again, so a refresh mid-flight is free.
   */
  const openingNeeded = writing.setupAt !== null && !messages.some((m) => m.role === "ai");
  useEffect(() => {
    if (!openingNeeded) return;
    let cancelled = false;
    setOpening(true);
    void postWritingOpening(writing.id)
      .then((res) => {
        if (cancelled) return;
        const reply = res.reply.trim();
        if (reply) onMessages([...messages, { seq: --localSeq.current, role: "ai", content: reply, createdAt: "" }]);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof ApiError ? err.message : "印记这次没接上，你可以直接开始说。");
      })
      .finally(() => {
        if (!cancelled) setOpening(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openingNeeded, writing.id]);

  const chatMessages: ChatMessage[] = useMemo(
    () =>
      messages
        .filter((m) => m.role === "student" || m.role === "ai")
        .map((m) => ({
          id: `p${m.seq}`,
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
    onMessages(withStudent);
    try {
      const turn = await postWritingPlanTurn(writing.id, text);
      onMessages([...withStudent, { seq: --localSeq.current, role: "ai", content: turn.reply, createdAt: "" }]);
      onOutline(turn.outline);
      setJustAdded(turn.addedIds);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "印记暂时没接上，请重试。");
      onMessages(messages);
      setDraft(text);
    } finally {
      setSending(false);
    }
  }

  /**
   * Her own edits to the map. These go through the full-replace PUT — the
   * one path that CAN change or remove a node, and it is only ever reachable
   * from her hands. The planning turn has no such call.
   */
  async function mutate(next: { text: string; role: string; depth: number }[]) {
    try {
      onOutline(await putWritingOutline(writing.id, next));
      setJustAdded([]);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "改这张图失败，请重试。");
    }
  }

  function removeNode(id: string) {
    const ordered = outline.slice().sort((a, b) => a.position - b.position);
    const target = ordered.find((n) => n.id === id);
    if (!target) return;
    // Removing a node takes its whole subtree with it: the rows that follow
    // it while staying deeper than it ARE its children (the flattened-tree
    // convention), and leaving them behind would reparent her material under
    // whatever happened to precede it.
    const keep: typeof ordered = [];
    let skipping = false;
    for (const n of ordered) {
      if (n.id === id) {
        skipping = true;
        continue;
      }
      if (skipping) {
        if (n.depth > target.depth) continue;
        skipping = false;
      }
      keep.push(n);
    }
    void mutate(keep.map((n) => ({ text: n.text, role: n.role, depth: n.depth })));
  }

  function editNode(id: string, text: string) {
    void mutate(
      outline
        .slice()
        .sort((a, b) => a.position - b.position)
        .map((n) => ({ text: n.id === id ? text : n.text, role: n.role, depth: n.depth })),
    );
  }

  return (
    <div className="flex h-full w-full flex-col bg-mk-paper">
      <header className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-mk-border px-5 py-3">
        <div className="flex min-w-0 flex-col">
          <button type="button" onClick={onBack} className="w-fit text-mk-small text-mk-muted hover:text-mk-accent-700">
            ← 我的写作
          </button>
          <EditableTitle writingId={writing.id} title={writing.title} onRenamed={onRenamed} />
        </div>
        <div className="flex items-center gap-3">
          <span className="text-mk-small text-mk-muted">先想清楚，再动笔</span>
          <Button onClick={onDone} iconEnd={<Icon icon={ArrowRight} size={14} />}>
            去写
          </Button>
        </div>
      </header>

      {error && (
        <div role="alert" className="shrink-0 px-5 py-2 text-mk-small text-mk-danger" style={{ background: "var(--mk-danger-bg)" }}>
          {error}
          <button type="button" className="ml-3 underline" onClick={() => setError(null)}>
            知道了
          </button>
        </div>
      )}

      <div className={hasMap ? "grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_minmax(440px,44%)]" : "flex min-h-0 flex-1 justify-center"}>
        <div className={hasMap ? "flex min-h-0 flex-col" : "flex min-h-0 w-full max-w-[720px] flex-col"}>
          <ChatLog messages={chatMessages} thinking={sending || opening} className="mk-scroll min-h-0 flex-1 px-5 py-4" />
          <div className="shrink-0 px-5 pb-5">
            <Composer
              value={draft}
              onChange={setDraft}
              onSend={() => void send()}
              state={sending ? "replying" : undefined}
              placeholder="说说你的想法"
            />
          </div>
        </div>

        {hasMap && (
          <aside className="relative flex min-h-0 flex-col border-t border-mk-border lg:border-l lg:border-t-0">
            {/* Floats over the canvas rather than sitting above it: a header
                band would cut the drawing surface in two, and the point of
                this panel is that it reads as one continuous sheet. */}
            <div className="pointer-events-none absolute inset-x-0 top-0 z-10 flex items-center justify-between px-4 py-2">
              <span
                className="rounded-mk-full px-2 py-0.5 text-mk-label text-mk-faint"
                style={{ background: "color-mix(in srgb, var(--mk-paper) 88%, transparent)" }}
              >
                你的思路
              </span>
              <span
                className="rounded-mk-full px-2 py-0.5 text-mk-small text-mk-faint"
                style={{ background: "color-mix(in srgb, var(--mk-paper) 88%, transparent)" }}
              >
                点一条可以改，也能删
              </span>
            </div>
            <MindMap items={outline} justAdded={justAdded} onRemove={removeNode} onEdit={editNode} />
          </aside>
        )}
      </div>
    </div>
  );
}
