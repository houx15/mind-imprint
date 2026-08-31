import { activeSteps } from "../../data/plan";
import type { PlanStep, Project } from "../../data/types";
import { Sys } from "../../ui";

/**
 * 一步开始的时候，印记说清楚三件事.
 *
 * ## Why the detail lives here and not on the plan
 * The plan screen shows a number, a name and one line per step — because a
 * student reviewing seven blocks of specification is skimming, and a skimmed
 * plan is worse than no plan. Everything else arrives **one step at a time**,
 * at the moment it becomes actionable:
 *
 *   ① **目标** — what this step is for.
 *   ② **分工方案** — 印记 does this, you do that, then you hand back X.
 *   ③ **决策要点** — the judgement that stays hers here.
 *
 * ## Why 「然后」 is not decoration
 * 「你来做 X」 on its own is an assignment. 「你来做 X，然后把 Y 交回来，我按它
 * 继续」 is a division of labour with a meeting point — and the meeting point
 * is the whole reason the student's half matters to the machine's half. A
 * split that never converges is just two people working alone.
 */
export function StepIntro({ project, stepId }: { project: Project; stepId: string }) {
  const steps = activeSteps(project.plan);
  const index = steps.findIndex((s) => s.id === stepId);
  const step = index >= 0 ? steps[index] : project.plan.find((s) => s.id === stepId);
  if (!step) return null;

  return (
    <div className="flex gap-3">
      <span
        className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-bold text-white"
        style={{ background: "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))" }}
      >
        印
      </span>
      <div
        className="eco-in min-w-0 flex-1 rounded-mk-lg border p-4"
        style={{ borderColor: "var(--mk-accent-200)", background: "var(--mk-accent-50)" }}
      >
        <Sys className="!text-mk-accent-700">
          {index >= 0 ? `第 ${index + 1} / ${steps.length} 步` : "补做的一步"}
        </Sys>
        <h3 className="mt-1 text-mk-h2 text-mk-ink">{step.title}</h3>

        {step.goal ? (
          <p className="mt-2 text-mk-body leading-[1.85] text-mk-ink">
            <span className="text-mk-muted">目标 · </span>
            {step.goal}
          </p>
        ) : null}

        <Split step={step} />

        {step.decide ? (
          <p className="mt-3 border-t border-mk-accent-200 pt-2.5 text-mk-body leading-[1.8] text-mk-ink">
            <span className="text-mk-accent-700">决策要点 · </span>
            {step.decide}
          </p>
        ) : null}
      </div>
    </div>
  );
}

/** 分工方案. Three lines, always in this order: what the machine does, what
 *  she does, where the two meet. */
function Split({ step }: { step: PlanStep }) {
  const rows: [string, string, string][] = [
    ["印记做", step.iBring, "#4E7EA6"],
    ["你来做", step.youBring, "var(--mk-accent-700)"],
    ["然后", step.then, "var(--mk-secondary)"],
  ];
  if (rows.every(([, v]) => !v?.trim())) return null;

  return (
    <div
      className="mt-3 rounded-mk-md border p-3.5"
      style={{ borderColor: "var(--mk-accent-200)", background: "var(--mk-surface)" }}
    >
      <Sys>分工方案</Sys>
      <dl className="mt-2 space-y-2">
        {rows
          .filter(([, v]) => v?.trim())
          .map(([k, v, hue]) => (
            <div key={k} className="flex gap-2.5">
              <dt className="eco-mono w-[44px] shrink-0 pt-[3px]" style={{ color: hue }}>
                {k}
              </dt>
              <dd className="min-w-0 flex-1 text-mk-body leading-[1.75] text-mk-secondary">{v}</dd>
            </div>
          ))}
      </dl>
    </div>
  );
}
