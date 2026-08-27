import { Check } from "lucide-react";
import { Icon } from "@/ui";

/**
 * StageMap — the four-step 构思/大纲/段落/成稿 indicator.
 *
 * 铁律②: "Stages are a MAP, not a gate" (writing_stage.go's own file
 * comment — jumping forward OR backward is a plain 200 server-side, and the
 * server records the transition rather than refusing it). Every step is
 * ALWAYS a clickable button, never disabled, whatever stage she is
 * currently on — there is no "you haven't unlocked 成稿 yet" state anywhere
 * in this component.
 */

export type WritingStageKey = "ideate" | "outline" | "snippets" | "draft";

const STEPS: { key: WritingStageKey; label: string }[] = [
  { key: "ideate", label: "构思" },
  { key: "outline", label: "大纲" },
  { key: "snippets", label: "段落" },
  { key: "draft", label: "成稿" },
];

/** `writing.stage` also carries a fifth value, `'finished'` — a terminal
 *  status that still means "she was working in 成稿". Maps it onto the same
 *  four-step highlight rather than growing a fifth dot nothing points at. */
function normalizeStage(stage: string): WritingStageKey {
  return stage === "finished" ? "draft" : (stage as WritingStageKey);
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
              className="flex h-4 w-4 shrink-0 items-center justify-center rounded-mk-full text-[10px]"
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
