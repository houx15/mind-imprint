import type { BlockKey } from "./mockData";
import { Pebble } from "@/ui/Pebble";

/**
 * NextStepGuide — 印记 offering the next view along the journey (agentic studio,
 * spec §5: "AI drives, you can override"). A single, gentle offer chip beside
 * the stage switcher: tapping it moves you; the switcher right next to it is the
 * always-available override, so this never traps the student (铁律②).
 *
 * This is the DETERMINISTIC first version — the step is derived from real
 * project state (does a plan exist yet? is the draft finished?), not an LLM
 * classifier. A later phase can let the server decision-layer propose richer,
 * moment-aware transitions; the surface (an offer chip you tap) stays the same.
 *
 * GOTCHA (design-system convention): one Tailwind class per competing CSS
 * property; never `bg-mk-<token>/<opacity>` — tints use solid tokens.
 */

type Step = { room: BlockKey; label: string };

/** The single most relevant next step, or null when 印记 has nothing to offer. */
export function deriveNextStep(args: {
  hasPlan: boolean;
  writingFinished: boolean;
  room: BlockKey;
}): Step | null {
  const { hasPlan, writingFinished, room } = args;
  // Draft finished → the journey's next beat is the retrospective.
  if (writingFinished) {
    return room === "reflection" ? null : { room: "reflection", label: "去回顾这段思考" };
  }
  // Plan in hand but still writing → the first plan step is the proposal draft.
  if (hasPlan) {
    return room === "writing" ? null : { room: "writing", label: "一起去写作" };
  }
  // No plan yet → 立项 already guides (finish the four dims → 生成计划); stay quiet
  // rather than double up on that nudge.
  return null;
}

export function NextStepGuide({
  hasPlan,
  writingFinished,
  room,
  onGoRoom,
}: {
  hasPlan: boolean;
  writingFinished: boolean;
  room: BlockKey;
  onGoRoom: (room: BlockKey) => void;
}) {
  const step = deriveNextStep({ hasPlan, writingFinished, room });
  if (!step) return null;
  return (
    <button
      type="button"
      onClick={() => onGoRoom(step.room)}
      className="ml-auto flex shrink-0 items-center gap-1.5 rounded-mk-full border border-mk-accent-200 bg-mk-accent-50 px-3 py-1 text-[11.5px] font-semibold text-mk-accent transition-colors duration-[120ms] ease-mk hover:bg-mk-accent hover:text-white"
    >
      <Pebble size={15} />
      <span>下一步，{step.label} →</span>
    </button>
  );
}
