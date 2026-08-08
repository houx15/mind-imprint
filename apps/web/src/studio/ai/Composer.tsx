import { useEffect, useRef, type KeyboardEvent } from "react";
import { Send, Square } from "lucide-react";
import { Icon } from "@/ui/Icon";

/**
 * Composer (studio agentic rebuild, spec §13).
 *
 * Shared message-input bar for the four studio rooms. `state` drives the
 * trailing button: empty (disabled, spec-sanctioned `#E7DDD0` literal) →
 * typing (accent send) → replying (■ stop, calls `onStop`). When `state`
 * is omitted it is derived from `value` so callers that don't track a
 * reply-in-flight explicitly still get sensible empty/typing behavior.
 *
 * GOTCHA (same as Card.tsx/forms.tsx): exactly ONE class per competing CSS
 * property — Tailwind's compiled order is not className order.
 */

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export type ComposerState = "empty" | "typing" | "replying";

export interface ComposerProps {
  value: string;
  onChange: (value: string) => void;
  onSend: () => void;
  state?: ComposerState;
  onStop?: () => void;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
}

// ~5 lines at mk-body's 14px/1.6 line-height, plus vertical padding.
const MAX_TEXTAREA_PX = 140;

// Focus treatment note (Round 2): `focus-visible:ring-mk-accent/15` never
// rendered — Tailwind's JIT can't alpha-mix a CSS-variable color, so any
// `mk-*/NN` opacity modifier emits NO CSS. `ring-mk-accent-200` is a solid
// token (no opacity modifier) so it actually paints a visible accent-tinted
// ring; `focus-visible:border-mk-accent` gives the border itself a
// branded state too. This matches the standard focus ring used app-wide
// (see ui/forms.tsx `FIELD_BASE`). Verified with a tailwind build.
const TEXTAREA_BASE =
  "flex-1 resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface " +
  "px-3 py-2 text-mk-body text-mk-ink outline-none transition-colors duration-[120ms] ease-mk " +
  "placeholder:text-[#B8ADA2] focus-visible:border-mk-accent focus-visible:ring-2 focus-visible:ring-mk-accent-200 " +
  "disabled:cursor-not-allowed disabled:text-mk-muted";

const TRAILING_BUTTON_BASE =
  "flex aspect-square shrink-0 items-center justify-center rounded-mk-sm p-[10px] " +
  "transition-colors duration-[120ms] ease-mk disabled:cursor-not-allowed";

export function Composer({
  value,
  onChange,
  onSend,
  state,
  onStop,
  placeholder,
  disabled = false,
  className,
}: ComposerProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const resolvedState: ComposerState = state ?? (value.trim() === "" ? "empty" : "typing");
  const isEmpty = resolvedState === "empty";
  const isReplying = resolvedState === "replying";

  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, MAX_TEXTAREA_PX)}px`;
    el.style.overflowY = el.scrollHeight > MAX_TEXTAREA_PX ? "auto" : "hidden";
  }, [value]);

  function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== "Enter" || event.shiftKey) return;
    // Don't send while an IME composition is active (Chinese/Japanese/Korean
    // input): pressing Enter to accept a candidate must commit text, not send.
    if (event.nativeEvent.isComposing) return;
    event.preventDefault();
    if (resolvedState === "typing") {
      onSend();
    }
  }

  return (
    <div className={cx("flex items-end gap-2", className)}>
      <textarea
        ref={textareaRef}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        disabled={disabled}
        rows={1}
        className={TEXTAREA_BASE}
      />
      {isReplying ? (
        <button
          type="button"
          aria-label="停止"
          onClick={onStop}
          className={cx(TRAILING_BUTTON_BASE, "bg-mk-ink text-white")}
        >
          <Icon icon={Square} size={16} />
        </button>
      ) : (
        <button
          type="button"
          aria-label="发送"
          onClick={onSend}
          disabled={isEmpty || disabled}
          className={cx(
            TRAILING_BUTTON_BASE,
            isEmpty ? "bg-[#E7DDD0] text-mk-muted" : "bg-mk-accent text-white",
          )}
        >
          <Icon icon={Send} size={16} />
        </button>
      )}
    </div>
  );
}
