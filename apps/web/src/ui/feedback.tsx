import type { ReactNode } from "react";
import { Icon, Check } from "@/ui/Icon";

/**
 * Feedback primitives (design-system foundation, Part 1 Task 7): Badge,
 * CountBadge, Tabs, Segmented, Progress, Stepper, Tooltip (spec §9).
 *
 * GOTCHA (carried from Task 5/6): Tailwind emits same-CSS-property utility
 * classes in ALPHABETICAL order in the compiled stylesheet, not className
 * order. Every element below applies exactly ONE class per competing CSS
 * property (bg-*, border-color, text-<color>, rounded-*), selected via a
 * ternary/lookup — never two competing utilities stacked "for order".
 */

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

// ---------------------------------------------------------------------------
// Badge / CountBadge
// ---------------------------------------------------------------------------

export type BadgeTone = "progress" | "done" | "draft" | "pending";

// Each tone is one self-contained class string (bg + text + optional border)
// so no two tones' classes are ever combined on the same element.
const BADGE_TONES: Record<BadgeTone, string> = {
  progress: "bg-mk-accent-50 text-mk-accent-700",
  done: "bg-mk-success-bg text-mk-success",
  draft: "border border-mk-border bg-mk-paper text-mk-muted",
  pending: "bg-mk-warning-bg text-mk-warning",
};

export interface BadgeProps {
  tone: BadgeTone;
  children?: ReactNode;
  className?: string;
}

export function Badge({ tone, children, className }: BadgeProps) {
  return (
    <span
      data-tone={tone}
      className={cx(
        "inline-flex items-center rounded-mk-full px-2 py-0.5 text-mk-small font-medium",
        BADGE_TONES[tone],
        className,
      )}
    >
      {children}
    </span>
  );
}

export interface CountBadgeProps {
  n: number;
  className?: string;
}

/** Accent solid filled circle with a white number (e.g. unread count). */
export function CountBadge({ n, className }: CountBadgeProps) {
  return (
    <span
      className={cx(
        "inline-flex h-5 min-w-[20px] items-center justify-center rounded-mk-full bg-mk-accent px-1 text-mk-small font-medium text-white",
        className,
      )}
    >
      {n}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Tabs (underline style)
// ---------------------------------------------------------------------------

export interface TabItem {
  key: string;
  label: ReactNode;
}

export interface TabsProps {
  tabs: TabItem[];
  value: string;
  onChange: (key: string) => void;
  className?: string;
}

export function Tabs({ tabs, value, onChange, className }: TabsProps) {
  return (
    <div role="tablist" className={cx("flex gap-4 border-b border-mk-border", className)}>
      {tabs.map((tab) => {
        const active = tab.key === value;
        return (
          <button
            key={tab.key}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => onChange(tab.key)}
            className={cx(
              "relative -mb-px border-b-2 px-1 pb-2 text-mk-body transition-colors duration-[120ms] ease-mk",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
              active ? "border-mk-accent text-mk-ink" : "border-transparent text-mk-muted",
            )}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Segmented (pill container + white "thumb" on the active option)
// ---------------------------------------------------------------------------

export interface SegmentedOption {
  value: string;
  label: ReactNode;
}

export interface SegmentedProps {
  options: SegmentedOption[];
  value: string;
  onChange: (value: string) => void;
  className?: string;
  /**
   * `plain` (default) — a paper groove with a white thumb, for switchers that
   * sit inside a card or surface. `island` — an elevated surface pill (shadow +
   * hairline ring) with an accent-tinted active lozenge, for a switcher that
   * floats on its own over the page background (项目/课程 top switchers).
   */
  variant?: "plain" | "island";
}

export function Segmented({ options, value, onChange, className, variant = "plain" }: SegmentedProps) {
  const island = variant === "island";
  return (
    <div
      className={cx(
        "inline-flex gap-0.5 rounded-mk-full p-1",
        island ? "bg-mk-surface shadow-mk-md ring-1 ring-mk-border" : "bg-mk-paper",
        className,
      )}
    >
      {options.map((opt) => {
        const active = opt.value === value;
        return (
          <button
            key={opt.value}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(opt.value)}
            className={cx(
              "rounded-mk-full text-mk-small transition-colors duration-[120ms] ease-mk",
              island ? "px-4 py-1.5" : "px-3 py-1",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
              // The "thumb" is simply the active option's own fill — functionally
              // identical to a sliding thumb without absolute positioning. On an
              // island (white) pill a white thumb would vanish, so the active
              // lozenge is an accent tint instead.
              active
                ? island
                  ? "bg-mk-accent-50 font-semibold text-mk-accent-700 shadow-mk-xs"
                  : "bg-mk-surface text-mk-ink shadow-mk-xs"
                : "bg-transparent text-mk-muted",
            )}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Progress (linear, clamped 0–100)
// ---------------------------------------------------------------------------

export interface ProgressProps {
  value: number;
  className?: string;
}

export function Progress({ value, className }: ProgressProps) {
  const clamped = Math.min(100, Math.max(0, value));
  return (
    <div
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={clamped}
      className={cx("h-2 w-full overflow-hidden rounded-mk-full bg-mk-accent-50", className)}
    >
      <div
        className="h-full rounded-mk-full bg-mk-accent transition-[width] duration-[200ms] ease-mk"
        style={{ width: `${clamped}%` }}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Stepper (done = accent✓, current = accent ring, todo = grey)
// ---------------------------------------------------------------------------

export interface StepperProps {
  steps: string[];
  current: number;
  className?: string;
}

export function Stepper({ steps, current, className }: StepperProps) {
  return (
    <ol className={cx("flex items-center", className)}>
      {steps.map((label, i) => {
        const state = i < current ? "done" : i === current ? "current" : "todo";
        return (
          <li key={`${label}-${i}`} className="flex items-center">
            <div className="flex flex-col items-center gap-1">
              <span
                data-state={state}
                className={cx(
                  "flex h-6 w-6 shrink-0 items-center justify-center rounded-mk-full text-mk-small font-medium",
                  state === "done" && "bg-mk-accent text-white",
                  state === "current" && "bg-mk-surface text-mk-accent ring-2 ring-mk-accent",
                  state === "todo" && "bg-mk-border text-mk-muted",
                )}
              >
                {state === "done" ? <Icon icon={Check} size={14} /> : i + 1}
              </span>
              <span className={cx("text-mk-small", state === "todo" ? "text-mk-muted" : "text-mk-ink")}>
                {label}
              </span>
            </div>
            {i < steps.length - 1 && <span aria-hidden="true" className="mx-2 h-px w-6 shrink-0 bg-mk-border" />}
          </li>
        );
      })}
    </ol>
  );
}

// ---------------------------------------------------------------------------
// Tooltip (dark bubble on hover/focus, no JS timers)
// ---------------------------------------------------------------------------

export interface TooltipProps {
  label: ReactNode;
  children: ReactNode;
  className?: string;
}

export function Tooltip({ label, children, className }: TooltipProps) {
  return (
    <span tabIndex={0} className={cx("group relative inline-flex focus:outline-none", className)}>
      {children}
      <span
        role="tooltip"
        className={cx(
          "pointer-events-none absolute bottom-full left-1/2 z-10 mb-2 -translate-x-1/2 whitespace-nowrap",
          "rounded-mk-xs bg-mk-ink px-2 py-1 text-mk-small text-white opacity-0",
          "transition-opacity duration-[120ms] ease-mk",
          "group-hover:opacity-100 group-focus-within:opacity-100",
        )}
      >
        {label}
      </span>
    </span>
  );
}
