import type {
  InputHTMLAttributes,
  TextareaHTMLAttributes,
  SelectHTMLAttributes,
  ReactNode,
} from "react";
import { Star } from "lucide-react";
import { Icon } from "@/ui/Icon";

/**
 * Form controls / card-runtime primitives (design-system foundation, Part 1
 * Task 6).
 *
 * These are the input primitives tool-card schemas compose (spec §8):
 * `text · textarea · select · single_choice · multi_choice · rating ·
 * toggle`. All are controlled (`value`/`checked` + `onChange`) — no
 * uncontrolled/defaultValue variants — so the card runtime is the single
 * source of truth for field state.
 *
 * GOTCHA (learned building Task 5's Surface/Card): Tailwind emits same-CSS-
 * property utility classes in ALPHABETICAL order in the compiled stylesheet,
 * not className/config order. So every element below applies exactly ONE
 * class per competing CSS property (e.g. exactly one border-color class via
 * a ternary, never two `border-mk-*`/`bg-*` stacked "for cascade order").
 */

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

// Shared field chrome: border mk-input-border, radius 8 (mk-xs), focus =
// accent border + focus ring, error = danger border + danger bg. Exactly one
// border-color utility and one background utility per state.
const FIELD_BASE =
  "w-full rounded-mk-xs text-mk-body text-mk-ink transition-colors duration-[120ms] ease-mk " +
  "placeholder:text-[#B8ADA2] outline-none " +
  "focus-visible:ring-[3px] focus-visible:ring-mk-accent/15 " +
  "disabled:cursor-not-allowed disabled:text-mk-muted";

function fieldStateClasses(hasError: boolean): string {
  return hasError
    ? "border border-mk-danger bg-mk-danger-bg focus:border-mk-danger"
    : "border border-mk-input-border bg-mk-surface focus:border-mk-accent";
}

function ErrorLine({ error }: { error?: string }) {
  if (!error) return null;
  return <p className="mt-1 text-mk-small text-mk-danger">{error}</p>;
}

// ---------------------------------------------------------------------------
// Input / Textarea
// ---------------------------------------------------------------------------

export interface InputProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, "value" | "onChange"> {
  value: string;
  onChange: (value: string) => void;
  error?: string;
}

export function Input({ value, onChange, error, className, ...rest }: InputProps) {
  return (
    <div className="w-full">
      <input
        {...rest}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={Boolean(error) || undefined}
        className={cx(FIELD_BASE, fieldStateClasses(Boolean(error)), "px-3 py-2", className)}
      />
      <ErrorLine error={error} />
    </div>
  );
}

export interface TextareaProps
  extends Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, "value" | "onChange"> {
  value: string;
  onChange: (value: string) => void;
  error?: string;
}

export function Textarea({ value, onChange, error, className, ...rest }: TextareaProps) {
  return (
    <div className="w-full">
      <textarea
        {...rest}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={Boolean(error) || undefined}
        className={cx(
          FIELD_BASE,
          fieldStateClasses(Boolean(error)),
          "min-h-[88px] resize-y px-3 py-2",
          className,
        )}
      />
      <ErrorLine error={error} />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Select
// ---------------------------------------------------------------------------

export interface SelectOption {
  value: string;
  label: string;
}

export interface SelectProps
  extends Omit<SelectHTMLAttributes<HTMLSelectElement>, "value" | "onChange"> {
  value: string;
  onChange: (value: string) => void;
  options: SelectOption[];
  error?: string;
  placeholder?: string;
}

export function Select({
  value,
  onChange,
  options,
  error,
  placeholder,
  className,
  ...rest
}: SelectProps) {
  return (
    <div className="w-full">
      <select
        {...rest}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={Boolean(error) || undefined}
        className={cx(FIELD_BASE, fieldStateClasses(Boolean(error)), "px-3 py-2", className)}
      >
        {placeholder && (
          <option value="" disabled>
            {placeholder}
          </option>
        )}
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
      <ErrorLine error={error} />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Toggle
// ---------------------------------------------------------------------------

export interface ToggleProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label?: string;
  disabled?: boolean;
  className?: string;
}

export function Toggle({ checked, onChange, label, disabled, className }: ToggleProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cx(
        "relative inline-flex h-[26px] w-[44px] shrink-0 items-center rounded-mk-full transition-colors duration-[120ms] ease-mk",
        "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-mk-accent/15",
        disabled && "cursor-not-allowed opacity-50",
        checked ? "bg-mk-accent" : "bg-mk-border",
        className,
      )}
    >
      <span
        className={cx(
          "inline-block h-[20px] w-[20px] transform rounded-mk-full bg-mk-surface shadow-mk-sm transition-transform duration-[120ms] ease-mk",
          checked ? "translate-x-[21px]" : "translate-x-[3px]",
        )}
      />
    </button>
  );
}

// ---------------------------------------------------------------------------
// Radio (single_choice)
// ---------------------------------------------------------------------------

export interface RadioOption {
  value: string;
  label: ReactNode;
}

export interface RadioProps {
  name: string;
  value: string;
  onChange: (value: string) => void;
  options: RadioOption[];
  className?: string;
}

export function Radio({ name, value, onChange, options, className }: RadioProps) {
  return (
    <div className={cx("flex flex-col gap-2", className)} role="radiogroup" aria-label={name}>
      {options.map((opt) => {
        const selected = opt.value === value;
        return (
          <label key={opt.value} className="flex cursor-pointer items-center gap-2">
            <input
              type="radio"
              name={name}
              value={opt.value}
              checked={selected}
              onChange={() => onChange(opt.value)}
              className="peer sr-only"
            />
            <span
              aria-hidden="true"
              className={cx(
                "flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full border",
                "peer-focus-visible:ring-[3px] peer-focus-visible:ring-mk-accent/15",
                selected ? "border-mk-accent bg-mk-accent" : "border-mk-input-border bg-mk-surface",
              )}
            >
              {selected && <span className="h-2 w-2 rounded-mk-full bg-mk-surface" />}
            </span>
            <span className="text-mk-body text-mk-ink">{opt.label}</span>
          </label>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Checkbox (single control; multi_choice composes N of these)
// ---------------------------------------------------------------------------

export interface CheckboxProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label?: ReactNode;
  disabled?: boolean;
  className?: string;
}

export function Checkbox({ checked, onChange, label, disabled, className }: CheckboxProps) {
  return (
    <label className={cx("flex cursor-pointer items-center gap-2", disabled && "cursor-not-allowed opacity-50", className)}>
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        className="peer sr-only"
      />
      <span
        aria-hidden="true"
        className={cx(
          "flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-xs border",
          "peer-focus-visible:ring-[3px] peer-focus-visible:ring-mk-accent/15",
          checked ? "border-mk-accent bg-mk-accent" : "border-mk-input-border bg-mk-surface",
        )}
      >
        {checked && (
          <svg viewBox="0 0 20 20" width={13} height={13} fill="none" stroke="white" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round">
            <path d="M4.5 10.5l3.5 3.5 7.5-8" />
          </svg>
        )}
      </span>
      {label && <span className="text-mk-body text-mk-ink">{label}</span>}
    </label>
  );
}

// ---------------------------------------------------------------------------
// Chip (multi-select pill, e.g. multi_choice rendered as pills)
// ---------------------------------------------------------------------------

export interface ChipProps {
  label: ReactNode;
  selected: boolean;
  onChange: (selected: boolean) => void;
  disabled?: boolean;
  className?: string;
}

export function Chip({ label, selected, onChange, disabled, className }: ChipProps) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      disabled={disabled}
      onClick={() => onChange(!selected)}
      className={cx(
        "inline-flex items-center rounded-mk-full border px-3 py-1 text-mk-small transition-colors duration-[120ms] ease-mk",
        "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-mk-accent/15",
        disabled && "cursor-not-allowed opacity-50",
        selected
          ? "border-mk-accent-50 bg-mk-accent-50 text-mk-accent-700"
          : "border-mk-border bg-mk-surface text-mk-ink",
        className,
      )}
    >
      {label}
    </button>
  );
}

// ---------------------------------------------------------------------------
// Rating (5-star; fill is warning gold, NOT accent — spec §8)
// ---------------------------------------------------------------------------

export interface RatingProps {
  value: number;
  onChange: (value: number) => void;
  max?: number;
  className?: string;
}

export function Rating({ value, onChange, max = 5, className }: RatingProps) {
  return (
    <div className={cx("inline-flex items-center gap-1", className)} role="radiogroup" aria-label="rating">
      {Array.from({ length: max }, (_, i) => i + 1).map((n) => {
        const filled = n <= value;
        return (
          <button
            key={n}
            type="button"
            role="radio"
            aria-checked={n === value}
            aria-label={`${n} star${n === 1 ? "" : "s"}`}
            onClick={() => onChange(n)}
            className="p-0.5 text-mk-warning focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-mk-accent/15"
          >
            <Icon icon={Star} size={20} fill={filled ? "currentColor" : "none"} />
          </button>
        );
      })}
    </div>
  );
}
