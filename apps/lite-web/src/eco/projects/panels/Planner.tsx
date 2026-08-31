import { useState } from "react";
import { ArrowDown, ArrowUp, Check, MessageCircle, Pencil, Plus, RotateCcw, X } from "lucide-react";
import { useEco } from "../../store";
import { activeSteps } from "../../data/plan";
import type { PlanStep, Project } from "../../data/types";
import { Btn, Sys, cx } from "../../ui";

/**
 * 项目计划 — the route, before anything runs.
 *
 * ## What this screen shows, and what it stopped showing
 * Per step: **a number, a name, one line.** That is the whole card.
 *
 * 🚨 The first version printed 你带来 / 印记做 / 决策要点 on every node and gave
 * each one a field for scheduling itself. Two things were wrong. Seven blocks
 * of specification is not something a student reviews — it is something she
 * skims, and skimming a plan you are about to agree to is worse than not
 * showing it. And the scheduling was ceremony: asking a thirteen-year-old to
 * date seven steps before she knows what any of them involve produces seven
 * guesses. It has been removed outright.
 *
 * All that detail moved to **the step itself** (`goal` + the split of work),
 * where it arrives one step at a time and is actually actionable.
 *
 * ## What she can still do
 * Rename a step, rewrite its line, reorder, switch one off, add her own — or
 * say so in the conversation on the left and argue with 印记 about it. Then
 * 开始. Nothing runs until she presses it.
 */
export function Planner({ project }: { project: Project }) {
  const { planEdit, planToggle, planMove, planAdd, approvePlan } = useEco();
  const [editing, setEditing] = useState<string | null>(null);

  const steps = activeSteps(project.plan);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="border-b border-mk-border px-7 py-5">
        <Sys>项目计划 · 印记提议，你确认</Sys>
        <h2 className="mt-1 text-mk-h1 text-mk-ink">
          我建议按这 {steps.length} 步走
        </h2>
        <p className="mt-1.5 max-w-[64ch] text-mk-body leading-[1.85] text-mk-secondary">
          每一步开始的时候，我会说清楚这一步的目标，以及我们各自做什么。
          现在你只需要看一遍顺序——想改就改，或者直接在左边告诉我。
        </p>
      </div>

      {/* ── the route ───────────────────────────────────────────────── */}
      <div className="min-h-0 flex-1 overflow-y-auto px-7 py-6">
        <ol className="relative mx-auto max-w-[680px]">
          {project.plan.map((step, i) => (
            <li key={step.id} className="relative pb-3 pl-14 last:pb-0">
              {/* the connector */}
              {i < project.plan.length - 1 ? (
                <span
                  aria-hidden
                  className="absolute left-[19px] top-10 h-full w-[2px]"
                  style={{ background: "var(--mk-border)" }}
                />
              ) : null}

              <span
                className={cx(
                  "absolute left-0 top-1 flex h-10 w-10 items-center justify-center rounded-mk-full",
                  "font-mono text-[15px] font-bold",
                  step.off ? "text-mk-faint" : "text-white",
                )}
                style={{
                  background: step.off
                    ? "var(--mk-paper)"
                    : "linear-gradient(150deg,var(--mk-accent-400),var(--mk-accent-600))",
                  border: step.off ? "2px dashed var(--mk-border)" : "2px solid var(--mk-surface)",
                }}
              >
                {step.off
                  ? "—"
                  : String(project.plan.filter((x, k) => !x.off && k < i).length + 1).padStart(2, "0")}
              </span>

              <StepRow
                step={step}
                editing={editing === step.id}
                onEdit={() => setEditing(editing === step.id ? null : step.id)}
                onPatch={(p) => planEdit(project.id, step.id, p)}
                onToggle={() => planToggle(project.id, step.id)}
                onMove={(d) => planMove(project.id, step.id, d)}
              />
            </li>
          ))}
        </ol>

        <div className="mx-auto mt-4 max-w-[680px] pl-14">
          <button
            type="button"
            onClick={() => planAdd(project.id)}
            className="flex items-center gap-2 rounded-mk-md border border-dashed border-mk-border px-4 py-2.5
                       text-mk-small text-mk-muted transition-colors duration-[140ms]
                       hover:border-mk-accent hover:text-mk-accent-700 focus-visible:outline-none
                       focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          >
            <Plus size={15} strokeWidth={1.9} />
            加一步我没想到的
          </button>
        </div>
      </div>

      {/* ── start ───────────────────────────────────────────────────── */}
      <div className="border-t border-mk-border bg-mk-surface px-7 py-4">
        <div className="mx-auto flex max-w-[680px] flex-wrap items-center justify-between gap-4">
          <p className="text-mk-body text-mk-secondary">
            准备好了吗？中间任何一步你都可以说「这条不对」。
          </p>
          <div className="flex flex-wrap gap-2">
            <Btn
              variant="quiet"
              iconStart={<MessageCircle size={15} strokeWidth={1.9} />}
              onClick={() => {
                const box = document.querySelector<HTMLTextAreaElement>("form textarea");
                box?.focus();
              }}
            >
              我想改，跟你说说
            </Btn>
            <Btn iconStart={<Check size={16} strokeWidth={2.4} />} onClick={() => approvePlan(project.id)}>
              开始
            </Btn>
          </div>
        </div>
      </div>
    </div>
  );
}

function StepRow({
  step,
  editing,
  onEdit,
  onPatch,
  onToggle,
  onMove,
}: {
  step: PlanStep;
  editing: boolean;
  onEdit: () => void;
  onPatch: (p: Partial<PlanStep>) => void;
  onToggle: () => void;
  onMove: (d: -1 | 1) => void;
}) {
  return (
    <div
      className={cx(
        "group rounded-mk-lg border p-4 transition-colors duration-[140ms]",
        step.off ? "border-dashed border-mk-border" : "border-mk-border bg-mk-surface",
      )}
      style={{ opacity: step.off ? 0.5 : 1 }}
    >
      {editing ? (
        <>
          <input
            value={step.title}
            onChange={(e) => onPatch({ title: e.target.value })}
            className="w-full rounded-mk-md border border-mk-input-border bg-mk-paper px-3 py-2
                       text-mk-h3 text-mk-ink focus:border-mk-accent focus:outline-none"
          />
          <textarea
            value={step.blurb}
            onChange={(e) => onPatch({ blurb: e.target.value })}
            rows={2}
            className="mt-2 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-paper
                       px-3 py-2 text-mk-body leading-[1.75] text-mk-ink
                       focus:border-mk-accent focus:outline-none"
          />
          <Btn size="sm" variant="outline" className="mt-2.5" onClick={onEdit}>
            改好了
          </Btn>
        </>
      ) : (
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h3 className="text-mk-h3 text-mk-ink">{step.title}</h3>
            <p className="mt-1 text-mk-body leading-[1.8] text-mk-secondary">{step.blurb}</p>
          </div>
          {/* Controls stay quiet until she looks at the row. The plan is for
              reading first; editing is the second thing she does, not the
              first thing she sees. */}
          <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity duration-[140ms]
                          group-hover:opacity-100 group-focus-within:opacity-100">
            <IconBtn label="改这一步" onClick={onEdit}>
              <Pencil size={13} strokeWidth={1.9} />
            </IconBtn>
            <IconBtn label="上移" onClick={() => onMove(-1)}>
              <ArrowUp size={13} strokeWidth={2} />
            </IconBtn>
            <IconBtn label="下移" onClick={() => onMove(1)}>
              <ArrowDown size={13} strokeWidth={2} />
            </IconBtn>
            <IconBtn label={step.off ? "加回来" : "这一步不要"} onClick={onToggle}>
              {step.off ? <RotateCcw size={13} strokeWidth={2} /> : <X size={13} strokeWidth={2} />}
            </IconBtn>
          </div>
        </div>
      )}
    </div>
  );
}

function IconBtn({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className="flex h-7 w-7 items-center justify-center rounded-mk-sm text-mk-faint
                 transition-colors duration-[120ms] hover:bg-mk-paper hover:text-mk-ink
                 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      {children}
    </button>
  );
}
