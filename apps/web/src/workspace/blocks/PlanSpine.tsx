import { useMemo } from "react";
import type { PlanItem } from "@mind-imprint/contracts";

/**
 * PlanSpine — the project plan's sequenced stages as a compact "你在这一步"
 * indicator (agentic studio, spec §3). The plan is the project's spine; this
 * surfaces WHERE the student is along it, always visible beside the stage
 * switcher once a plan exists. Tapping it jumps to 立项 (where the full plan
 * board lives) — the plan is revisitable, per the journey.
 *
 * "Current stage" = the first stage (in plan order) that still has an unfinished
 * item; if every item is done, the last stage. Derived from real plan_items —
 * no separate cursor to drift out of sync.
 *
 * GOTCHA (design-system convention): exactly ONE Tailwind class per competing
 * CSS property (alphabetical emit order); never `bg-mk-<token>/<opacity>` (hex
 * → invalid alpha) — tints use solid tokens.
 */

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

/** Distinct stage labels in plan order + the index of the current stage. */
export function derivePlanStages(items: PlanItem[]): { stages: string[]; currentIndex: number } {
  const stages: string[] = [];
  for (const it of items) {
    if (it.stage && !stages.includes(it.stage)) stages.push(it.stage);
  }
  if (stages.length === 0) return { stages, currentIndex: 0 };
  // First stage with any unfinished item; else the last (all done).
  let currentIndex = stages.length - 1;
  for (let i = 0; i < stages.length; i++) {
    const anyUnfinished = items.some((it) => it.stage === stages[i] && it.column !== "done");
    if (anyUnfinished) {
      currentIndex = i;
      break;
    }
  }
  return { stages, currentIndex };
}

export function PlanSpine({ items, onOpenPlan }: { items: PlanItem[]; onOpenPlan?: () => void }) {
  const { stages, currentIndex } = useMemo(() => derivePlanStages(items), [items]);
  if (stages.length === 0) return null;

  return (
    <button
      type="button"
      onClick={onOpenPlan}
      title="查看项目计划"
      className="flex min-w-0 items-center gap-1.5 rounded-mk-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[11.5px] transition-colors duration-[120ms] ease-mk hover:border-mk-accent"
    >
      <span className="shrink-0 font-bold text-mk-faint">计划</span>
      <span className="flex min-w-0 items-center gap-1">
        {stages.map((stage, i) => {
          const state = i < currentIndex ? "done" : i === currentIndex ? "current" : "todo";
          // Show the short "阶段X" head when the label is "阶段X · 名字", else the
          // whole (short) label — keeps the pill compact.
          const short = stage.split(/\s*·\s*/)[0] || stage;
          return (
            <span key={stage} className="flex items-center gap-1">
              {i > 0 && <span aria-hidden="true" className="text-mk-faint">›</span>}
              <span
                data-state={state}
                className={cx(
                  "truncate rounded-mk-full px-1.5 py-0.5 font-semibold",
                  state === "current" && "bg-mk-accent-50 text-mk-accent",
                  state === "done" && "text-mk-muted",
                  state === "todo" && "text-mk-faint",
                )}
              >
                {short}
              </span>
            </span>
          );
        })}
      </span>
    </button>
  );
}
