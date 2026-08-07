import { useState } from "react";
import type { CardBodyProps } from "../CardRenderer";
import { MethodologyPanel } from "../CardRenderer";
import { TextAreaField } from "../fields/TextAreaField";
import { Battery } from "../teaching/assets/Battery";
import { INNER_PARTS } from "../teaching/assets/InnerPartsCast";

// Custom escape-hatch renderer for the emotional-alignment (内在小人) card.
// Writes ONLY the field keys defined in emotional-alignment.json via onField:
//   energy_level, inner_part_choice, not_want_reason, micro_action_choice
// All other display logic is cosmetic only — no extra keys, standard envelope intact.

// ──────────────────────────────────────────────
// Battery section — interactive energy meter
// ──────────────────────────────────────────────
function EnergyBatteryControl({
  value,
  onField,
}: {
  value: number;
  onField: (k: string, v: unknown) => void;
}) {
  return (
    <div className="flex items-center gap-2">
      {[1, 2, 3, 4, 5].map((n) => (
        <button
          key={n}
          type="button"
          // First cell carries aria-label="电量 格1" so getByLabelText(/电量/) finds one element.
          // Remaining cells use generic sr-only text.
          aria-label={n === 1 ? "电量 格1" : undefined}
          aria-pressed={value === n}
          onClick={() => onField("energy_level", n)}
          className={`relative h-9 w-6 rounded-[4px] border transition-colors ${
            n <= value
              ? n <= 2
                ? "border-[#C2557A] bg-[#C2557A]"
                : "border-[#C9743C] bg-[#C9743C]"
              : "border-[#EAECF2] bg-[#F3F4F8]"
          }`}
        >
          <span className="sr-only">{n} 格</span>
        </button>
      ))}
      {/* decorative battery preview at current level */}
      {value > 0 && (
        <span className="ml-2 text-[14px] font-semibold text-mk-muted-2">
          {value} / 5
        </span>
      )}
    </div>
  );
}

// ──────────────────────────────────────────────
// On-demand step wrapper (mirrors CardRenderer's OnDemandStep)
// ──────────────────────────────────────────────
function OnDemandSection({
  title,
  onExpandStep,
  stepKey,
  children,
}: {
  title: string;
  stepKey: string;
  onExpandStep: (k: string) => void;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <section className="overflow-hidden rounded-mk border border-mk-border-2 bg-white">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => {
          if (!open) onExpandStep(stepKey);
          setOpen((v) => !v);
        }}
        className="flex w-full items-center justify-between px-5 py-4 text-left"
      >
        <span className="text-[15px] font-bold text-mk-ink">{title}</span>
        <span className="text-[12px] font-medium text-mk-muted-2">
          按需展开
        </span>
      </button>
      {open && <div className="px-5 pb-5">{children}</div>}
    </section>
  );
}

// ──────────────────────────────────────────────
// Main renderer
// ──────────────────────────────────────────────
export function InnerPartsRenderer({
  card,
  values,
  onField,
  onExpandStep,
  onNote,
  hideMethodology,
}: CardBodyProps) {
  // Pull step definitions from card spec (single source of truth)
  const energyStep = card.steps.find((s) => s.key === "energy")!;
  const innerPartStep = card.steps.find((s) => s.key === "inner_part")!;
  const safeIslandStep = card.steps.find((s) => s.key === "safe_island")!;
  const microActionStep = card.steps.find((s) => s.key === "micro_action")!;

  // Field defs — read from card spec
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const getField = (step: typeof energyStep, key: string): any =>
    step.fields.find((f) => f.key === key);

  const energyField = getField(energyStep, "energy_level");
  const innerPartField = getField(innerPartStep, "inner_part_choice");
  const notWantField = getField(safeIslandStep, "not_want_reason");
  const microActionField = getField(microActionStep, "micro_action_choice");

  // The JSON options for inner_part_choice — verbatim strings that must be written
  const innerPartOptions: string[] = innerPartField?.options ?? [];
  // The JSON options for micro_action_choice
  const microActionOptions: string[] = microActionField?.options ?? [];

  const energyValue =
    typeof values.energy_level === "number" ? (values.energy_level as number) : 0;
  const selectedInnerPart =
    typeof values.inner_part_choice === "string"
      ? (values.inner_part_choice as string)
      : null;

  return (
    <div className="space-y-4">
      {/* ── Step 1: Energy level ── */}
      <section className="rounded-mk border border-mk-border-2 bg-white p-5">
        <h3 className="mb-4 text-[15px] font-bold text-mk-ink">
          {energyStep.title}
        </h3>
        {!hideMethodology && (
          <MethodologyPanel step={energyStep} onNote={onNote} />
        )}
        <div className="space-y-3">
          <p className="text-[14px] text-mk-muted-2">{energyField?.label}</p>
          <EnergyBatteryControl
            value={energyValue}
            onField={onField}
          />
          {/* Decorative Battery display mirrors chosen level */}
          {energyValue > 0 && (
            <Battery level={energyValue} max={5} />
          )}
        </div>
      </section>

      {/* ── Step 2: Inner part choice ── */}
      <section className="rounded-mk border border-mk-border-2 bg-white p-5">
        <h3 className="mb-4 text-[15px] font-bold text-mk-ink">
          {innerPartStep.title}
        </h3>
        {!hideMethodology && (
          <MethodologyPanel step={innerPartStep} onNote={onNote} />
        )}
        <p className="mb-4 text-[14px] text-mk-muted-2">
          {innerPartField?.label}
        </p>
        {/* Character cards — one per inner part */}
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          {INNER_PARTS.map((part, i) => {
            // Map INNER_PARTS entry to its JSON option string by index order
            // (the JSON options and INNER_PARTS are defined in the same order)
            const optionString = innerPartOptions[i] ?? "";
            const isSelected = selectedInnerPart === optionString;
            return (
              <button
                key={part.key}
                type="button"
                aria-pressed={isSelected}
                onClick={() => onField("inner_part_choice", optionString)}
                className={`flex flex-col items-center gap-2 rounded-[14px] border-2 p-4 text-center transition-colors ${
                  isSelected
                    ? "border-mk-primary bg-mk-accent-50"
                    : "border-mk-border-2 bg-white hover:border-mk-primary/40"
                }`}
              >
                <part.Avatar size={48} />
                <span className="text-[14px] font-bold text-mk-ink">
                  {part.name}
                </span>
                <span className="text-[12px] leading-snug text-mk-muted-2">
                  「{part.quote}」
                </span>
              </button>
            );
          })}
        </div>
      </section>

      {/* ── Step 3: Safe island (textarea, optional) ── */}
      <section className="rounded-mk border border-mk-border-2 bg-white p-5">
        <h3 className="mb-4 text-[15px] font-bold text-mk-ink">
          {safeIslandStep.title}
        </h3>
        {!hideMethodology && (
          <MethodologyPanel step={safeIslandStep} onNote={onNote} />
        )}
        <TextAreaField
          field={notWantField}
          value={values.not_want_reason}
          onChange={(v) => onField("not_want_reason", v)}
        />
      </section>

      {/* ── Step 4: Micro-action (on_demand, optional) ── */}
      <OnDemandSection
        title={microActionStep.title}
        stepKey={microActionStep.key}
        onExpandStep={onExpandStep}
      >
        {!hideMethodology && (
          <MethodologyPanel step={microActionStep} onNote={onNote} />
        )}
        <p className="mb-3 text-[14px] text-mk-muted-2">
          {microActionField?.label}
        </p>
        <div className="flex flex-col gap-2 sm:flex-row">
          {microActionOptions.map((opt) => (
            <button
              key={opt}
              type="button"
              aria-pressed={values.micro_action_choice === opt}
              onClick={() => onField("micro_action_choice", opt)}
              className={`flex-1 rounded-[12px] border-2 px-4 py-3 text-[14px] font-semibold transition-colors ${
                values.micro_action_choice === opt
                  ? "border-mk-primary bg-mk-primary text-white"
                  : "border-mk-border-2 bg-white text-mk-ink hover:border-mk-primary/40"
              }`}
            >
              {opt}
            </button>
          ))}
        </div>
      </OnDemandSection>
    </div>
  );
}
