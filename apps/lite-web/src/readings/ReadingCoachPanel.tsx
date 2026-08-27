import { useEffect, useMemo, useRef, useState } from "react";
import { Check, Play, SkipForward } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { Composer } from "@/studio/ai/Composer";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
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
 * coach decides whether that counted and says what is next. The steps are
 * still on screen, but as **progress she can see** rather than controls she
 * operates — which is also why they are rendered small and above the
 * conversation rather than as the main event.
 *
 * Skipping did not disappear. It moved into language: she says 这步跳过吧, and
 * the coach records it (铁律④) without arguing. A student who wants out of a
 * step should not have to hunt for the button that admits it.
 */
export function ReadingCoachPanel({
  readingId,
  tasks,
  onTasks,
  onFocusBlock,
}: {
  readingId: string;
  tasks: ReadingTask[];
  onTasks: (next: ReadingTask[]) => void;
  onFocusBlock: (blockId: string) => void;
}) {
  const [messages, setMessages] = useState<LiteMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [finished, setFinished] = useState(false);
  const localSeq = useRef(-1);

  const started = messages.length > 0;
  const current = tasks.find((t) => t.status === "pending") ?? null;
  const settled = tasks.filter((t) => t.status !== "pending").length;

  async function turn(text: string) {
    if (busy) return;
    setBusy(true);
    setError(null);
    if (text) {
      setMessages((prev) => [...prev, { seq: --localSeq.current, role: "student", content: text, createdAt: "" }]);
    }
    try {
      const res = await postReadingCoachTurn(readingId, text);
      setMessages((prev) => [...prev, { seq: --localSeq.current, role: "ai", content: res.reply, createdAt: "" }]);
      onTasks(res.tasks);
      setFinished(res.finished);
      // The coach names the paragraph this step is about; jumping there is
      // part of leading her, not a separate thing she has to do.
      if (res.focusBlock) onFocusBlock(res.focusBlock);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "印记这次没接上，再试一次。");
      if (text) setMessages((prev) => prev.slice(0, -1));
      setDraft(text);
    } finally {
      setBusy(false);
    }
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
    endRef.current?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [messages.length]);

  if (!started) {
    return (
      <div className="flex flex-col gap-3 rounded-mk-md border border-mk-border bg-mk-surface p-4">
        <p className="text-mk-body text-mk-ink">让我来带你详细读一遍这篇文章吧。</p>
        <p className="text-mk-small text-mk-muted">我先看看这篇，排一条路线，然后一步一步带你走。中途想跳过哪一步，说一声就行。</p>
        <Button onClick={() => void turn("")} loading={busy} iconStart={<Icon icon={Play} size={14} />}>
          开始
        </Button>
        {error && <p className="text-mk-small text-mk-danger">{error}</p>}
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-3">
      {/* Progress, not controls. Nothing here is clickable: 印记 moves her. */}
      {tasks.length > 0 && (
        <div className="shrink-0 rounded-mk-md border border-mk-border bg-mk-surface p-3">
          <div className="flex items-center justify-between gap-2">
            <span className="text-mk-label text-mk-faint">带读进度</span>
            <span className="text-mk-small text-mk-muted">
              {settled} / {tasks.length}
            </span>
          </div>
          <ol className="mt-2 flex list-none flex-col gap-1">
            {tasks.map((task, i) => {
              const isCurrent = current?.id === task.id;
              return (
                <li key={task.id} className="flex items-start gap-2">
                  <span
                    aria-hidden="true"
                    className="mt-[3px] flex h-4 w-4 shrink-0 items-center justify-center rounded-mk-full text-[10px]"
                    style={
                      task.status === "done"
                        ? { background: "var(--mk-accent-500)", color: "white" }
                        : task.status === "skipped"
                          ? { border: "1px dashed var(--mk-border)", color: "var(--mk-faint)" }
                          : isCurrent
                            ? { border: "1px solid var(--mk-accent-500)", color: "var(--mk-accent-700)" }
                            : { border: "1px solid var(--mk-border)", color: "var(--mk-faint)" }
                    }
                  >
                    {task.status === "done" ? (
                      <Icon icon={Check} size={10} />
                    ) : task.status === "skipped" ? (
                      <Icon icon={SkipForward} size={9} />
                    ) : (
                      i + 1
                    )}
                  </span>
                  <span
                    className="text-mk-small"
                    style={{
                      color: isCurrent
                        ? "var(--mk-ink)"
                        : task.status === "pending"
                          ? "var(--mk-faint)"
                          : "var(--mk-muted)",
                      fontWeight: isCurrent ? 600 : 400,
                    }}
                  >
                    {task.label}
                  </span>
                </li>
              );
            })}
          </ol>
        </div>
      )}

      <div className="mk-scroll min-h-0 flex-1 overflow-y-auto rounded-mk-md border border-mk-border bg-mk-surface p-3">
        <ChatLog messages={chatMessages} thinking={busy} />
        <div ref={endRef} />
      </div>

      {error && <p className="shrink-0 text-mk-small text-mk-danger">{error}</p>}

      <div className="shrink-0">
        <Composer
          value={draft}
          onChange={setDraft}
          onSend={() => {
            const text = draft.trim();
            if (!text) return;
            setDraft("");
            void turn(text);
          }}
          state={busy ? "replying" : undefined}
          placeholder={finished ? "读完了，还想聊点什么？" : "读完这一步，跟印记说一声"}
        />
      </div>
    </div>
  );
}
