import { useEffect, useRef, useState } from "react";
import { questionCardTurn, commitQuestionCard, type QuestionCardMsg } from "../api/questionCard";

// QuestionCardModal — slice 3a · the 提问卡 adaptive sub-agent as a chat modal
// (all-statuses.md §2), the first sub-agent card renderer. It guides the student
// from a vague/empty 目标 to a focused, personal research question, one question
// at a time (铁律③), never writing it for them (铁律①). On confirm it fills
// proposal.objective and closes.

export type QuestionCardDeps = {
  turn: typeof questionCardTurn;
  commit: typeof commitQuestionCard;
};

const realDeps: QuestionCardDeps = { turn: questionCardTurn, commit: commitQuestionCard };

export function QuestionCardModal({
  projectId,
  onClose,
  onCommitted,
  deps = realDeps,
}: {
  projectId: string;
  onClose: () => void;
  // Called after the objective is committed, so the shell can refresh + drop a
  // card-used chip into the coach thread.
  onCommitted: (objective: string) => void;
  deps?: QuestionCardDeps;
}) {
  const [messages, setMessages] = useState<QuestionCardMsg[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [done, setDone] = useState(false);
  const [objectiveDraft, setObjectiveDraft] = useState<string | null>(null);
  const [committing, setCommitting] = useState(false);
  const kickedOff = useRef(false);

  // Kick off the opening question once.
  useEffect(() => {
    if (kickedOff.current) return;
    kickedOff.current = true;
    void runTurn([]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function runTurn(history: QuestionCardMsg[]) {
    setSending(true);
    try {
      const reply = await deps.turn(projectId, history);
      setMessages([...history, { role: "ai", text: reply.narrate }]);
      if (reply.done && reply.suggestedObjective) {
        setDone(true);
        setObjectiveDraft(reply.suggestedObjective);
      }
    } catch {
      setMessages([...history, { role: "ai", text: "刚才没接上，我们再试一次——用你自己的话说说你对题目的理解？" }]);
    } finally {
      setSending(false);
    }
  }

  function send() {
    const text = input.trim();
    if (!text || sending) return;
    setInput("");
    void runTurn([...messages, { role: "student", text }]);
  }

  async function confirm() {
    const obj = (objectiveDraft ?? "").trim();
    if (!obj || committing) return;
    setCommitting(true);
    try {
      await deps.commit(projectId, obj);
      onCommitted(obj);
      onClose();
    } catch {
      setCommitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" onClick={onClose}>
      <div
        className="flex max-h-[88vh] w-full max-w-lg flex-col overflow-hidden rounded-mk-lg bg-mk-surface shadow-mk-lg"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 border-b border-mk-border px-5 py-3">
          <span className="rounded-full bg-mk-accent px-2 py-0.5 text-[12px] font-bold text-white">提问卡</span>
          <h2 className="font-sans text-[15px] font-bold text-mk-ink">先听听你自己</h2>
          <button type="button" onClick={onClose} className="ml-auto text-[14px] font-bold text-mk-faint hover:text-mk-ink">
            ✕
          </button>
        </div>

        <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-5 py-4">
          {messages.map((m, i) => (
            <div key={i} className={m.role === "student" ? "self-end" : "self-start"}>
              <div
                className={`max-w-[85%] whitespace-pre-wrap rounded-mk-md px-3 py-2 text-[14px] leading-relaxed ${
                  m.role === "student" ? "bg-mk-accent text-white" : "bg-mk-paper text-mk-ink"
                }`}
              >
                {m.text}
              </div>
            </div>
          ))}
          {sending && <p className="self-start text-[13px] text-mk-muted">印记在想……</p>}
        </div>

        {done && objectiveDraft !== null ? (
          <div className="border-t border-mk-border bg-mk-paper px-5 py-4">
            <p className="text-[13px] font-bold text-mk-muted">这就是你的研究问题吗？（可以再改）</p>
            <textarea
              value={objectiveDraft}
              onChange={(e) => setObjectiveDraft(e.target.value)}
              rows={2}
              className="mt-2 w-full resize-none rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-[14px] text-mk-ink outline-none focus:border-mk-accent"
            />
            <div className="mt-3 flex justify-end gap-2">
              <button type="button" onClick={onClose} className="rounded-mk-md border border-mk-border px-3 py-1.5 text-[14px] font-semibold text-mk-muted hover:text-mk-ink">
                再想想
              </button>
              <button
                type="button"
                disabled={committing || objectiveDraft.trim() === ""}
                onClick={() => void confirm()}
                className="rounded-mk-md bg-mk-accent px-4 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-50"
              >
                {committing ? "记进中……" : "就用这个"}
              </button>
            </div>
          </div>
        ) : (
          <div className="flex items-center gap-2 border-t border-mk-border px-5 py-3">
            <input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); send(); } }}
              placeholder="用你自己的话说……"
              className="min-w-0 flex-1 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-[14px] text-mk-ink outline-none focus:border-mk-accent"
            />
            <button
              type="button"
              disabled={sending || input.trim() === ""}
              onClick={send}
              className="rounded-mk-md bg-mk-accent px-4 py-2 text-[14px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-50"
            >
              发送
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
