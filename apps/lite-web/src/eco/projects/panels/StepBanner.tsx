import { Target } from "lucide-react";
import { activeSteps } from "../../data/plan";
import type { Project } from "../../data/types";
import { Sys } from "../../ui";

/**
 * 你在哪一步，以及为什么这件事重要.
 *
 * ## The problem this solves
 * A panel full of fields is, on its own, indistinguishable from homework. A
 * student looking at four textareas has no way to tell whether she is doing
 * something that matters or filling in a form because a screen asked her to —
 * and the honest answer to *why am I typing this* is different for every card,
 * so it cannot live in one general reassurance somewhere in the UI.
 *
 * So every working surface opens with the same three facts:
 *
 *   ① **你在这里** — step N of M in the plan SHE approved, by name. Not a
 *      progress bar: the name of the step, so the panel is visibly part of a
 *      route she agreed to rather than a thing that appeared.
 *   ② **这一步你判断** — the judgement that is hers here (`PlanStep.decide`).
 *   ③ **填完它会发生什么** — `payoff`. 🚨 This one is the point. It must name a
 *      REAL downstream consequence in this project — *box three becomes the
 *      thing 印记 checks its own work against* — and never 「这会帮助你思考」.
 *      A student who knows what her answer is FOR writes a different answer.
 *
 * Placed above the fields on cards, artifacts and forms alike, because the
 * question it answers is asked at the same moment on all three.
 */
export function StepBanner({
  project,
  payoff,
  /** The plan step this surface belongs to, if it was reached through the
   *  plan. Opening a card early from 材料 has no step, and that is fine — the
   *  payoff still applies, the position does not. */
  stepId,
}: {
  project: Project;
  payoff: string;
  stepId?: string;
}) {
  const steps = activeSteps(project.plan);
  const index = stepId ? steps.findIndex((s) => s.id === stepId) : -1;
  const step = index >= 0 ? steps[index] : undefined;

  return (
    <div
      className="mb-4 rounded-mk-lg border p-4"
      style={{ borderColor: "var(--mk-accent-200)", background: "var(--mk-accent-50)" }}
    >
      {step ? (
        <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
          <Sys className="!text-mk-accent-700">
            第 {index + 1} / {steps.length} 步
          </Sys>
          <span className="text-mk-body font-semibold text-mk-ink">{step.title}</span>
          {step.when ? (
            <span className="eco-mono text-mk-muted" style={{ letterSpacing: 0 }}>
              你排的时间：{step.when}
            </span>
          ) : null}
        </div>
      ) : (
        <Sys className="!text-mk-accent-700">你自己提前打开的</Sys>
      )}

      {step?.decide ? (
        <p className="mt-2 flex items-start gap-2 text-mk-body leading-[1.8] text-mk-ink">
          <Target size={14} strokeWidth={2} className="mt-1 shrink-0 text-mk-accent-700" />
          <span>
            <span className="text-mk-muted">这一步你判断：</span>
            {step.decide}
          </span>
        </p>
      ) : null}

      <p className="mt-2.5 border-t border-mk-accent-200 pt-2.5 text-mk-body leading-[1.85] text-mk-ink">
        <span className="text-mk-accent-700">填完它会发生什么 · </span>
        {payoff}
      </p>
    </div>
  );
}

/** Which plan step opened this card / artifact, if any. Read off the plan
 *  rather than tracked separately, so it cannot drift. */
export function stepIdFor(project: Project, ref: { kind: "card" | "make"; id: string }): string | undefined {
  return project.plan.find((s) =>
    s.opens?.kind === "card"
      ? ref.kind === "card" && s.opens.cardId === ref.id
      : s.opens?.kind === "make"
        ? ref.kind === "make" && s.opens.artifactId === ref.id
        : false,
  )?.id;
}
