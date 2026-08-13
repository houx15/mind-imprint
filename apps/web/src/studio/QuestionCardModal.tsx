import { useEffect, useRef, useState } from "react";
import { questionCardTurn, commitQuestionCard, getQuestionCardState, type QuestionCardMsg } from "../api/questionCard";

// QuestionCardModal — slice 3a · the 提问卡 adaptive sub-agent as a chat modal
// (all-statuses.md §2), the first sub-agent card renderer. It guides the student
// from a vague/empty 目标 to a focused, personal research question, one question
// at a time (铁律③), never writing it for them (铁律①). On confirm it fills
// proposal.objective and closes.

export type QuestionCardDeps = {
  turn: typeof questionCardTurn;
  commit: typeof commitQuestionCard;
  // Optional: load the saved in-progress transcript so a reopen CONTINUES the
  // chat (§2). Absent ⇒ the modal always opens fresh (used by unit tests).
  load?: typeof getQuestionCardState;
};

const realDeps: QuestionCardDeps = { turn: questionCardTurn, commit: commitQuestionCard, load: getQuestionCardState };

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

  // Open once: continue a saved conversation if one exists (§2), else start
  // with the sub-agent's first question.
  useEffect(() => {
    if (kickedOff.current) return;
    kickedOff.current = true;
    void openConversation();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function openConversation() {
    if (deps.load) {
      try {
        const saved = await deps.load(projectId);
        if (saved.messages.length > 0) {
          setMessages(saved.messages);
          if (saved.done && saved.objective.trim() !== "") {
            setDone(true);
            setObjectiveDraft(saved.objective);
          }
          return; // continue where the student left off — no fresh opening turn
        }
      } catch {
        // fall through to a fresh opening question
      }
    }
    await runTurn([]);
  }

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
      // Send the whole conversation — it is the 提问卡's detailed content (§2).
      await deps.commit(projectId, obj, messages);
      onCommitted(obj);
      onClose();
    } catch {
      setCommitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" onClick={onClose}>
      <div
        className="flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-mk-lg bg-mk-surface shadow-mk-lg"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 border-b border-mk-border px-5 py-3">
          <span className="rounded-full bg-mk-accent px-2 py-0.5 text-[12px] font-bold text-white">提问卡</span>
          <h2 className="font-sans text-[15px] font-bold text-mk-ink">从大题目，问出一个值得研究的问题</h2>
          <button type="button" onClick={onClose} className="ml-auto text-[14px] font-bold text-mk-faint hover:text-mk-ink">
            ✕
          </button>
        </div>

        {/* Fixed methodology — the thinking-map from a broad prompt to a
            specific, researchable question. Always in view above the
            conversation so the student can follow the steps as they talk. */}
        <div className="border-b border-mk-border bg-mk-paper px-5 py-3.5">
          <p className="text-[12px] leading-relaxed text-mk-muted">
            <span className="font-bold text-mk-ink">什么算「具体」的研究问题？</span>
            {" "}不是「AI 好不好」，而是「在某个条件下，A 对 B 到底有没有影响」——小、能查、落得到证据。
          </p>
          <div className="mt-3 flex flex-wrap items-stretch gap-1.5">
            <FlowStep n="1" title="拆解" desc="圈出题目里的关键词" />
            <FlowArrow />
            <FlowStep n="2" title="追问" desc="每个词到底指什么？" />
            <FlowArrow />
            <FlowStep n="3" title="连接" desc="连到你的经历 / 已知材料" />
            <FlowArrow />
            <FlowStep n="4" title="连不上就去探索" desc="到阅读室或搜索，从一个关键词看大家在争什么" tail />
          </div>
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
            <p className="text-[13px] font-bold text-mk-muted">这就是你要研究的问题吗？确认后它会成为你的研究目标，提问卡收起。（可以再改）</p>
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
                {committing ? "记进中……" : "确认为研究问题"}
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

// One step in the fixed methodology flow. `tail` marks the "go explore" fallback
// step in the accent color (it points OUT of the modal, into the reading room).
function FlowStep({ n, title, desc, tail }: { n: string; title: string; desc: string; tail?: boolean }) {
  return (
    <div
      className={`flex min-w-[112px] flex-1 flex-col gap-1 rounded-mk-md border px-2.5 py-2 ${
        tail ? "border-mk-accent bg-mk-accent-50" : "border-mk-border bg-mk-surface"
      }`}
    >
      <span className="flex items-center gap-1.5">
        <span
          className={`flex h-4 w-4 flex-none items-center justify-center rounded-full text-[10px] font-bold text-white ${
            tail ? "bg-mk-accent" : "bg-mk-ink"
          }`}
        >
          {n}
        </span>
        <span className="text-[12px] font-bold text-mk-ink">{title}</span>
      </span>
      <span className="text-[11px] leading-snug text-mk-muted">{desc}</span>
    </div>
  );
}

function FlowArrow() {
  return <span className="flex flex-none items-center self-center text-[13px] font-bold text-mk-faint">→</span>;
}
