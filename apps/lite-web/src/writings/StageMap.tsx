import { Check } from "lucide-react";
import { Icon } from "@/ui";

/**
 * StageMap — the 结构 / 行文 / 段落 / 成稿 indicator (four steps since 2026-09-20).
 *
 * It was four steps until 2026-08-27. 构思 came first and owned a page whose
 * entire content was a 目标字数 input that gave no feedback — a screen that
 * looked like a step and did nothing. What it was *supposed* to host (working
 * out what you think) now lives where it belongs: the entry 设定 dialog takes
 * the settings, and the thinking happens in the coach's opening line and in
 * the per-block guiding questions, at the moment each block is actually being
 * written rather than all up front.
 *
 * 铁律②: stages are a MAP, not a gate (writing_stage.go's own file comment —
 * jumping forward OR backward is a plain 200 server-side, and the server
 * records the transition rather than refusing it). Every step is ALWAYS a
 * clickable button, never disabled, whatever stage she is on. There is no
 * "you haven't unlocked 成稿 yet" state anywhere in this component.
 */

export type WritingStageKey = "outline" | "flow" | "snippets" | "draft";

/**
 * 四步。「行文」是 2026-09-20 加的（同事的意见 4）：
 *
 *	「要在开始写之前先想好整个文章组织框架（并非填充内容）如何搭建，
 *	  现在只有文本内容的引导。」
 *
 * 结构长出「有哪些点」，段落直接开始写字，中间少了「这些点怎么组织」那一问。
 */
const STEPS: { key: WritingStageKey; label: string }[] = [
  { key: "outline", label: "结构" },
  { key: "flow", label: "行文" },
  { key: "snippets", label: "段落" },
  { key: "draft", label: "成稿" },
];

/**
 * `writing.stage` carries two values with no step of their own: 'finished'
 * (a terminal status that still means "she was working in 成稿") and the
 * retired 'ideate' (migration 0100 moved the rows, but a stale payload could
 * still carry one). Both map onto a real step rather than growing a dot
 * nothing points at, or — worse — highlighting nothing at all.
 */
function normalizeStage(stage: string): WritingStageKey {
  if (stage === "finished") return "draft";
  if (stage === "ideate") return "outline";
  return STEPS.some((s) => s.key === stage) ? (stage as WritingStageKey) : "outline";
}

export function StageMap({
  stage,
  onJump,
  disabled = false,
}: {
  stage: string;
  onJump: (stage: WritingStageKey) => void;
  disabled?: boolean;
}) {
  const current = normalizeStage(stage);
  const currentIndex = STEPS.findIndex((s) => s.key === current);

  return (
    <nav aria-label="写作四步" className="flex items-center gap-1">
      {STEPS.map((step, i) => {
        const active = step.key === current;
        const done = i < currentIndex;
        return (
          <button
            key={step.key}
            type="button"
            aria-current={active ? "step" : undefined}
            disabled={disabled}
            onClick={() => onJump(step.key)}
            className="flex items-center gap-1.5 rounded-mk-full px-2.5 py-1.5 text-mk-small font-medium transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:cursor-not-allowed"
            style={
              active
                ? { background: "var(--mk-accent-500)", color: "white" }
                : done
                  ? { background: "color-mix(in srgb, var(--mk-accent-500) 12%, transparent)", color: "var(--mk-accent-700)" }
                  : { color: "var(--mk-muted)" }
            }
          >
            <span
              className="flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-mk-full text-mk-label"
              style={
                active
                  ? { background: "rgba(255,255,255,0.3)" }
                  : done
                    ? { background: "color-mix(in srgb, var(--mk-accent-500) 22%, transparent)" }
                    : { border: "1px solid currentColor" }
              }
            >
              {done ? <Icon icon={Check} size={10} /> : i + 1}
            </span>
            {step.label}
          </button>
        );
      })}
    </nav>
  );
}
