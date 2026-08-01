import { useState } from "react";
import type { CardSpec, Step } from "@mind-imprint/contracts";
import { fieldRegistry } from "./fieldRegistry";
import { Markdown } from "./Markdown";

type Props = {
  card: CardSpec;
  values: Record<string, unknown>;
  onField: (path: string, value: unknown) => void;
  onExpandStep: (stepKey: string) => void;
  // Consulting a step's methodology is itself recorded process data (过程即数据):
  // the first expand of each step's panel emits note_open(step_key).
  onNote?: (stepKey: string) => void;
  hideMethodology?: boolean;
};

// A custom (escape-hatch) card renderer is a drop-in replacement for
// CardRenderer: same props, writes only field_values via onField.
export type CardBodyProps = Props;

export function MethodologyPanel({ step, onNote }: { step: Step; onNote?: (stepKey: string) => void }) {
  const m = step.methodology;
  const [open, setOpen] = useState(false);
  if (!m) return null;
  return (
    <div className="mb-4">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => {
          if (!open) onNote?.(step.key);
          setOpen((v) => !v);
        }}
        className="inline-flex items-center gap-1.5 text-[12.5px] font-semibold text-mk-primary"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
          <circle cx="12" cy="12" r="9.5" />
          <path d="M12 16v-4M12 8h.01" />
        </svg>
        方法
      </button>
      {open && (
        <div className="mt-2 space-y-3 rounded-mk border border-mk-primary-tint bg-mk-primary-tint/30 p-4 text-[13px] leading-relaxed text-[#3A4256]">
          <div><div className="mb-1 font-bold text-mk-ink">为什么</div><Markdown text={m.why} /></div>
          <div><div className="mb-1 font-bold text-mk-ink">怎么做</div><Markdown text={m.how} /></div>
          <div><div className="mb-1 font-bold text-mk-ink">什么时候用</div><Markdown text={m.when} /></div>
          {m.example && <div><div className="mb-1 font-bold text-mk-ink">例子</div><Markdown text={m.example} /></div>}
        </div>
      )}
    </div>
  );
}

// #6 · fill-in-the-blank 句式 scaffolds shown above a step's fields. Reference
// skeletons the student adapts into her own words — 印记 never writes the
// sentence for her (铁律①). Always visible (not behind the 方法 toggle) so the
// help is actually seen.
function SentenceFrames({ step }: { step: Step }) {
  const frames = (step as { sentence_frames?: string[] }).sentence_frames;
  if (!frames || frames.length === 0) return null;
  return (
    <div className="mb-4 rounded-mk border border-mk-accent/30 bg-mk-accent-tint/20 p-3">
      <div className="mb-1.5 text-[12px] font-bold text-mk-accent">参考句式 · 换成你自己的话</div>
      <ul className="space-y-1">
        {frames.map((f, i) => (
          <li key={i} className="text-[12.5px] leading-relaxed text-[#3A4256]">{f}</li>
        ))}
      </ul>
    </div>
  );
}

function StepFields({ step, values, onField }: { step: Step; values: Record<string, unknown>; onField: Props["onField"] }) {
  const visible = step.fields.filter((field) => {
    const cond = (field as { show_if?: { key: string; equals: string } }).show_if;
    return !cond || values[cond.key] === cond.equals;
  });
  return (
    <div className="space-y-4">
      {visible.map((field) => {
        const Cmp = fieldRegistry[field.type];
        if (!Cmp) throw new Error(`No component registered for field type "${field.type}"`);
        return <Cmp key={field.key} field={field} value={values[field.key]} onChange={(v: unknown) => onField(field.key, v)} />;
      })}
    </div>
  );
}

export function CardRenderer({ card, values, onField, onExpandStep, onNote, hideMethodology }: Props) {
  return (
    <div className="space-y-4">
      {card.steps.map((step) =>
        step.disclose === "on_demand" ? (
          <OnDemandStep key={step.key} step={step} values={values} onField={onField} onExpandStep={onExpandStep} onNote={onNote} hideMethodology={hideMethodology} />
        ) : (
          <section key={step.key} className="rounded-mk border border-mk-border-2 bg-white p-5">
            <h3 className="mb-4 text-[15px] font-bold text-mk-ink">{step.title}</h3>
            {!hideMethodology && <MethodologyPanel step={step} onNote={onNote} />}
            <SentenceFrames step={step} />
            <StepFields step={step} values={values} onField={onField} />
          </section>
        ),
      )}
    </div>
  );
}

function OnDemandStep({ step, values, onField, onExpandStep, onNote, hideMethodology }: { step: Step } & Omit<Props, "card">) {
  const [open, setOpen] = useState(false);
  return (
    <section className="overflow-hidden rounded-mk border border-mk-border-2 bg-white">
      <button
        type="button"
        onClick={() => {
          if (!open) onExpandStep(step.key);
          setOpen((v) => !v);
        }}
        aria-expanded={open}
        className="flex w-full items-center justify-between px-5 py-4 text-left"
      >
        <span className="text-[15px] font-bold text-mk-ink">{step.title}</span>
        <span className="text-[11.5px] font-medium text-mk-muted-2">按需展开</span>
      </button>
      {open && (
        <div className="px-5 pb-5">
          {!hideMethodology && <MethodologyPanel step={step} onNote={onNote} />}
          <StepFields step={step} values={values} onField={onField} />
        </div>
      )}
    </section>
  );
}
