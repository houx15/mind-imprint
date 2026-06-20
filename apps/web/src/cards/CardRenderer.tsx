import { useState } from "react";
import type { CardSpec, Step } from "@mind-imprint/contracts";
import { fieldRegistry } from "./fieldRegistry";

type Props = {
  card: CardSpec;
  values: Record<string, unknown>;
  onField: (path: string, value: unknown) => void;
  onExpandStep: (stepKey: string) => void;
};

function StepFields({ step, values, onField }: { step: Step; values: Record<string, unknown>; onField: Props["onField"] }) {
  return (
    <div className="space-y-4">
      {step.fields.map((field) => {
        const Cmp = fieldRegistry[field.type];
        return (
          <Cmp
            key={field.key}
            field={field}
            value={values[field.key]}
            onChange={(v: unknown) => onField(field.key, v)}
          />
        );
      })}
    </div>
  );
}

export function CardRenderer({ card, values, onField, onExpandStep }: Props) {
  return (
    <div className="space-y-4">
      {card.steps.map((step) =>
        step.disclose === "on_demand" ? (
          <OnDemandStep key={step.key} step={step} values={values} onField={onField} onExpandStep={onExpandStep} />
        ) : (
          <section key={step.key} className="rounded-mk border border-mk-border-2 bg-white p-5">
            <h3 className="mb-4 text-[15px] font-bold text-mk-ink">{step.title}</h3>
            <StepFields step={step} values={values} onField={onField} />
          </section>
        ),
      )}
    </div>
  );
}

function OnDemandStep({ step, values, onField, onExpandStep }: { step: Step } & Omit<Props, "card">) {
  const [open, setOpen] = useState(false);
  return (
    <section className="overflow-hidden rounded-mk border border-mk-border-2 bg-white">
      <button
        type="button"
        onClick={() => {
          if (!open) onExpandStep(step.key);
          setOpen((v) => !v);
        }}
        className="flex w-full items-center justify-between px-5 py-4 text-left"
      >
        <span className="text-[15px] font-bold text-mk-ink">{step.title}</span>
        <span className="text-[11.5px] font-medium text-mk-muted-2">按需展开</span>
      </button>
      {open && (
        <div className="px-5 pb-5">
          <StepFields step={step} values={values} onField={onField} />
        </div>
      )}
    </section>
  );
}
