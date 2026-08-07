import { Pebble } from "@/ui";

/**
 * The calm empty state for the interactive area when 印记 hasn't opened a room
 * yet (studio_state.openTool === "chat", agentic studio spec §2). The chat
 * panel stays primary; this is just what fills <main> until 印记 opens the
 * right work surface. Design-system: solid `mk-*` tokens, one class per
 * competing property.
 */
export function ChatFirstLanding() {
  return (
    <div data-testid="chat-first" className="flex h-full flex-col items-center justify-center gap-3 text-center">
      <Pebble state="idle" size={40} />
      <p className="text-[13px] leading-relaxed text-mk-muted">
        印记正在陪你把项目理清楚
        <br />
        准备好了，它会为你打开对的工作台
      </p>
    </div>
  );
}
