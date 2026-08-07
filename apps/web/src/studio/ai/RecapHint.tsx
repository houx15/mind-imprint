import { Icon } from "@/ui/Icon";
import { Sparkles } from "lucide-react";
import type { ChatMessage } from "./ChatLog";

/**
 * RecapHint — the "welcome back" recap, shown as the opening line INSIDE the
 * continuous 印记 conversation (not a separate banner). Agentic studio: the chat
 * is the primary surface, so the re-entry recap lives in the thread as 印记's
 * own opening note. Rendered as a ChatLog `system`-role node (no bubble chrome).
 *
 * GOTCHA (design-system convention): one Tailwind class per competing CSS
 * property; never `bg-mk-<token>/<opacity>` on a hex token.
 */
/** Prepend the recap as an opening system message, when present. */
export function withRecap(recap: string | null | undefined, messages: ChatMessage[]): ChatMessage[] {
  if (!recap || !recap.trim()) return messages;
  return [{ id: "__recap__", role: "system", node: <RecapHint text={recap} /> }, ...messages];
}

export function RecapHint({ text }: { text: string }) {
  return (
    <div className="flex items-start gap-2 rounded-mk-sm border border-mk-border bg-mk-paper px-3 py-2 text-left">
      <span className="mt-0.5 shrink-0 text-mk-accent">
        <Icon icon={Sparkles} size={14} />
      </span>
      <p className="text-[12.5px] leading-relaxed text-mk-muted">{text}</p>
    </div>
  );
}
