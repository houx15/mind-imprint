import { Check, Loader2 } from "lucide-react";
import { Icon } from "@/ui/Icon";

/**
 * SubagentHint — the single-line status for a HIDDEN subagent (reading-list
 * source search, plan generation, compaction, question-relation proposals):
 * a spinner + text while it runs, a check + text once it's done. NEVER a chat
 * bubble — the hidden subagents show only this status line, mirroring
 * RecapHint's icon+text visual language (same dir) but without the bordered
 * card chrome, since this is a lightweight in-flow status, not an opening note.
 *
 * `text-mk-small` (12px) is CORRECT here — this is the one place in the
 * product a 12px hint is allowed (design system §16 reserves ≥14px for
 * everything a student reads as content; this is a genuine transient hint).
 *
 * GOTCHA (design-system convention): one Tailwind class per competing CSS
 * property; never `bg-mk-<token>/<opacity>` on a hex token — colors here are
 * solid `mk-*` tokens only.
 */
export function SubagentHint({ text, done = false }: { text: string; done?: boolean }) {
  return (
    <div className="flex items-center gap-1.5 text-mk-small text-mk-faint">
      <span className={done ? "shrink-0 text-mk-success" : "shrink-0 text-mk-accent"}>
        {done ? <Icon icon={Check} size={12} /> : <Icon icon={Loader2} size={12} className="animate-spin" />}
      </span>
      <span>{text}</span>
    </div>
  );
}
