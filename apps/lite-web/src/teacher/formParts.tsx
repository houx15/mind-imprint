import type { ReactNode } from "react";

/**
 * formParts — the labelled-field and segmented-tab building blocks shared by
 * every teacher settings form (AssignmentForm, the upload/personalized
 * reading tabs, RubricFields, GradingPage). Moved out of AssignmentForm.tsx
 * (2026-09-16) so a new field component does not need to reach into the
 * page component that happens to render first; behaviour and markup are
 * unchanged from before the move.
 */

export const LABEL_CLS = "text-mk-label font-bold text-mk-muted";

export const INPUT_CLS =
  "w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-small text-mk-ink outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200";

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className={LABEL_CLS}>{label}</span>
      {children}
    </label>
  );
}

export function Segmented<T extends string>({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: { value: T; label: string }[];
  value: T;
  onChange: (v: T) => void;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className={LABEL_CLS}>{label}</span>
      <div className="flex flex-wrap gap-2" role="group" aria-label={label}>
        {options.map((o) => {
          const active = o.value === value;
          return (
            <button
              key={o.value}
              type="button"
              aria-pressed={active}
              onClick={() => onChange(o.value)}
              className={
                "rounded-mk-md border px-4 py-2 text-mk-small transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 " +
                (active ? "border-mk-accent font-bold text-mk-accent-700" : "border-mk-border bg-mk-surface text-mk-ink hover:bg-mk-accent-50")
              }
              style={active ? { background: "color-mix(in srgb, var(--mk-accent-500) 12%, var(--mk-surface))" } : undefined}
            >
              {o.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
