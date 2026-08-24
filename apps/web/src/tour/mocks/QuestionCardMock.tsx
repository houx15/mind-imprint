// QuestionCardMock — a STATIC, non-interactive replica of
// `apps/web/src/studio/QuestionCardModal.tsx`, used only inside the guided
// tour's `demoModal` step (§Task 2). No handlers, no state, no LLM calls —
// it exists purely so a new student can see what 提问卡 looks like before
// ever opening the real one. Mirrors the real card's structure (header pill
// + title, fixed 4-step methodology strip, chat bubbles, disabled input row)
// but every interactive control is inert.

const SAMPLE_MESSAGES: { role: "student" | "ai"; text: string }[] = [
  { role: "ai", text: "「中国是否让地球更可持续」这个题目挺大的。我们先拆一下——你觉得这里面哪个词最需要先说清楚？" },
  { role: "student", text: "可能是「可持续」吧，它可以指很多方面。" },
  { role: "ai", text: "很好。那在你最关心的那个方面——比如能源、碳排放、还是生物多样性——你更想聚焦哪一个？先选一个，我们把题目缩小。" },
];

export function QuestionCardMock() {
  return (
    <div className="flex max-h-[70vh] w-full flex-col overflow-hidden rounded-mk-md ring-1 ring-mk-border">
      <div className="flex items-center gap-2 border-b border-mk-border px-5 py-3">
        <span className="rounded-full bg-mk-accent px-2 py-0.5 text-[12px] font-bold text-white">提问卡</span>
        <h2 className="font-sans text-[15px] font-bold text-mk-ink">从大题目，问出一个值得研究的问题</h2>
      </div>

      {/* Fixed methodology strip — same 4 steps as the real card. */}
      <div className="border-b border-mk-border bg-mk-paper px-5 py-3.5">
        <div className="flex flex-wrap items-stretch gap-1.5">
          <FlowStep n="1" title="拆解" desc="圈出题目里的关键词" />
          <FlowArrow />
          <FlowStep n="2" title="追问" desc="每个词到底指什么？" />
          <FlowArrow />
          <FlowStep n="3" title="连接" desc="连到你的经历 / 已知材料" />
          <FlowArrow />
          <FlowStep n="4" title="连不上就去探索" desc="到阅读室或搜索" tail />
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-5 py-4">
        {SAMPLE_MESSAGES.map((m, i) => (
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
      </div>

      {/* Disabled input row — non-interactive, no handlers/state. */}
      <div className="flex items-center gap-2 border-t border-mk-border px-5 py-3">
        <input
          value=""
          readOnly
          disabled
          placeholder="用你自己的话说……"
          className="min-w-0 flex-1 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-[14px] text-mk-ink outline-none disabled:opacity-60"
        />
        <button
          type="button"
          disabled
          className="rounded-mk-md bg-mk-accent px-4 py-2 text-[14px] font-bold text-white disabled:opacity-50"
        >
          发送
        </button>
      </div>
    </div>
  );
}

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
