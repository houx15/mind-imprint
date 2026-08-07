import { useEffect, useRef, type ReactNode } from "react";

/**
 * ChatLog (studio agentic rebuild, spec §13).
 *
 * Shared chat-log renderer for the four studio rooms (Plan / Reading /
 * Writing / Review) — replaces four near-duplicate chat implementations.
 * A message with `node` lets a room inject a rich inline element
 * (CardTurnChip / CoachProposal) alongside or instead of plain text.
 *
 * GOTCHA (learned building Card/forms): Tailwind emits same-CSS-property
 * utility classes in alphabetical order in the compiled stylesheet, not
 * className order — so every bubble applies exactly ONE class per
 * competing CSS property (background, radius, etc via a ternary).
 */

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export type ChatRole = "assistant" | "student" | "system";

export type ChatMessage = {
  id: string;
  role: ChatRole;
  text?: string;
  node?: ReactNode;
};

export interface ChatLogProps {
  messages: ChatMessage[];
  thinking?: boolean;
  className?: string;
}

// Bubble corner radii are non-token literals from spec §13 (assistant tail
// bottom-left-ish "4/13/13/13", student tail "13/4/13/13") — arbitrary
// Tailwind values, one radius class per bubble, never stacked.
const ASSISTANT_RADIUS = "rounded-[4px_13px_13px_13px]";
const STUDENT_RADIUS = "rounded-[13px_4px_13px_13px]";
const BUBBLE_BASE = "inline-block max-w-[85%] px-4 py-3 text-mk-body text-mk-ink";

export function ChatLog({ messages, thinking = false, className }: ChatLogProps) {
  const bottomRef = useRef<HTMLDivElement>(null);
  const count = messages.length;

  useEffect(() => {
    // jsdom does not implement scrollIntoView — guard existence so tests
    // don't throw, while real browsers still get the smooth auto-scroll.
    bottomRef.current?.scrollIntoView?.({ block: "end" });
  }, [count, thinking]);

  return (
    <div className={cx("mk-scroll flex flex-col gap-3 overflow-y-auto", className)}>
      {messages.map((message) => (
        <ChatBubble key={message.id} message={message} />
      ))}
      {thinking && <ThinkingRow />}
      <div ref={bottomRef} />
    </div>
  );
}

function ChatBubble({ message }: { message: ChatMessage }) {
  if (message.role === "system") {
    return (
      <div className="text-center text-mk-caption text-mk-muted" data-role="system">
        {message.text}
        {message.node}
      </div>
    );
  }

  const isStudent = message.role === "student";
  return (
    <div className={cx("flex", isStudent ? "justify-end" : "justify-start")}>
      <div
        data-role={message.role}
        className={cx(
          BUBBLE_BASE,
          // 印记 bubble is white (spec §13); a hairline keeps it a visible
          // 对话框 even against the panel's own light surface. Student bubble
          // is accent-tinted and needs no border.
          isStudent ? "bg-mk-accent-50" : "bg-mk-surface shadow-mk-xs",
          isStudent ? STUDENT_RADIUS : ASSISTANT_RADIUS,
        )}
      >
        {message.text}
        {message.node}
      </div>
    </div>
  );
}

function ThinkingRow() {
  return (
    <div className="flex justify-start" data-role="assistant" aria-label="印记正在打字">
      <div className={cx(BUBBLE_BASE, "bg-mk-surface", ASSISTANT_RADIUS, "flex items-center gap-1")}>
        <span className="mk-think-dot" />
        <span className="mk-think-dot [animation-delay:0.15s]" />
        <span className="mk-think-dot [animation-delay:0.3s]" />
      </div>
    </div>
  );
}
