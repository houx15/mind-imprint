import { Minus, Plus } from "lucide-react";
import { stepNumber } from "./numberLogic";
import "./controls.css";

// teacher/controls/NumberField.tsx — a number box with − / + buttons, in place
// of the browser's spinner arrows. The value stays the string the caller
// already keeps (a half-typed or empty box is allowed; the caller validates).

export function NumberField({
  value,
  onChange,
  min,
  max,
  step = 1,
  ariaLabel,
  disabled = false,
  className = "",
}: {
  value: string;
  onChange: (value: string) => void;
  min?: number;
  max?: number;
  step?: number;
  ariaLabel?: string;
  disabled?: boolean;
  className?: string;
}) {
  const down = stepNumber(value, -step, min, max);
  const up = stepNumber(value, step, min, max);
  return (
    <div className={`tc-field tc-number ${className}`} data-disabled={disabled || undefined}>
      <input
        type="text"
        inputMode="numeric"
        aria-label={ariaLabel}
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value.replace(/[^\d]/g, ""))}
        onKeyDown={(e) => {
          const next = e.key === "ArrowUp" ? up : e.key === "ArrowDown" ? down : undefined;
          if (next === undefined) return;
          e.preventDefault();
          if (next !== null) onChange(next);
        }}
      />
      {/* The input comes first in the DOM: a <label> around this field
          activates its first labelable descendant, which must not be −.
          CSS `order` draws − on the left. The buttons are out of the tab
          order; ↑/↓ in the box do the same. */}
      <button type="button" tabIndex={-1} className="tc-number-btn tc-number-minus" aria-label="减少" disabled={disabled || down === null} onClick={() => down !== null && onChange(down)}>
        <Minus size={14} aria-hidden="true" />
      </button>
      <button type="button" tabIndex={-1} className="tc-number-btn tc-number-plus" aria-label="增加" disabled={disabled || up === null} onClick={() => up !== null && onChange(up)}>
        <Plus size={14} aria-hidden="true" />
      </button>
    </div>
  );
}
