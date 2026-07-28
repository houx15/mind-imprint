import { useState } from "react";
import { Icon } from "../Icon";
import { project, reflectionPrompts, mirrorSections, carryForwards } from "../protoData";

// The Review block: a mirror, not a report card. The student's own reflection
// leads (left); the AI-assembled "你的思维印记" narrative sits alongside as
// support (right). The rubric/assessment snapshot runs quietly underneath and
// feeds the teacher/parent reports — it is NOT shown here as a grade.
export function ReviewBlock() {
  const [answers, setAnswers] = useState<string[]>(reflectionPrompts.map(() => ""));
  const [done, setDone] = useState(false);

  return (
    <div className="grid h-full grid-cols-[1fr,380px]">
      {/* main · the student's reflection */}
      <div className="min-h-0 overflow-y-auto px-10 py-9">
        <div className="mx-auto max-w-2xl">
          <header className="mb-6">
            <p className="text-[12px] font-semibold uppercase tracking-[0.18em] text-mk-muted-2">项目收尾</p>
            <h1 className="mt-1 font-sans text-[26px] font-bold leading-tight text-mk-ink">回过头看看这一程</h1>
            <p className="mt-1.5 text-[14px] text-mk-muted">用你自己的话回答几个问题。右边是印记帮你整理的过程，卡壳时可以看看——但话得你自己说。</p>
          </header>

          <div className="flex flex-col gap-6">
            {reflectionPrompts.map((p, i) => (
              <div key={i}>
                <div className="flex items-center gap-2">
                  <span className="flex h-6 w-6 flex-none items-center justify-center rounded-full bg-mk-primary-tint text-[12px] font-bold text-mk-primary">{i + 1}</span>
                  <span className="rounded-full bg-mk-bg px-2 py-0.5 text-[11px] font-bold text-mk-muted">{p.label}</span>
                </div>
                <p className="mt-1.5 text-[14.5px] font-bold leading-snug text-mk-ink">{p.q}</p>
                {p.anchor === "goal" && (
                  <p className="mt-1.5 rounded-mk border-l-2 border-mk-accent bg-mk-accent-tint/40 px-3 py-2 text-[12.5px] leading-relaxed text-mk-muted">
                    <span className="font-bold text-mk-accent">开题时你写的目标 · </span>{project.proposal.objective}
                  </p>
                )}
                <textarea
                  value={answers[i]}
                  onChange={(e) => setAnswers((a) => a.map((x, j) => (j === i ? e.target.value : x)))}
                  rows={4}
                  placeholder="写下你的想法……"
                  className="mt-2 w-full resize-none rounded-mk-lg border border-mk-border bg-mk-surface px-4 py-3 text-[14px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
                />
              </div>
            ))}
          </div>

          <div className="mt-7 flex items-center gap-4">
            <button
              type="button"
              onClick={() => setDone(true)}
              className="rounded-mk bg-mk-accent px-5 py-2.5 text-[14px] font-bold text-white transition hover:bg-mk-accent-hover"
            >
              完成回顾
            </button>
            {done ? (
              <span className="text-[12.5px] font-semibold text-mk-green">已归档 · 这次的过程评估已记入你的成长报告</span>
            ) : (
              <span className="text-[12.5px] text-mk-muted-2">完成后会生成过程评估，记入成长报告（老师 / 家长可见），这里不打分。</span>
            )}
          </div>
        </div>
      </div>

      {/* aside · the mirror */}
      <aside className="flex min-h-0 flex-col border-l border-mk-border bg-mk-surface">
        <header className="border-b border-mk-border px-5 py-4">
          <div className="flex items-center gap-2 text-mk-primary">
            <Icon name="spark" size={16} />
            <h2 className="font-sans text-[15px] font-bold">你的思维印记</h2>
          </div>
          <p className="mt-1 text-[11.5px] text-mk-muted-2">印记根据你的全过程整理，供你参考——不是评分。</p>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          <div className="flex flex-col gap-4">
            {mirrorSections.map((s, i) => (
              <div key={i} className="border-l-2 border-mk-primary/30 pl-3">
                <p className="text-[12px] font-bold text-mk-primary">{s.title}</p>
                <p className="mt-1 text-[13px] leading-relaxed text-mk-ink">{s.body}</p>
              </div>
            ))}
          </div>

          <div className="mt-6 rounded-mk-lg border border-mk-accent/30 bg-mk-accent-tint/50 p-4">
            <p className="mb-2 flex items-center gap-1.5 text-[12px] font-bold text-mk-accent">
              <Icon name="arrow" size={14} /> 带走这两点
            </p>
            <ul className="flex flex-col gap-2">
              {carryForwards.map((c, i) => (
                <li key={i} className="text-[13px] leading-relaxed text-mk-ink">· {c}</li>
              ))}
            </ul>
          </div>
        </div>
      </aside>
    </div>
  );
}
