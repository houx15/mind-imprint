import { useRef } from "react";
import { MethodologyPanel, type CardBodyProps } from "../CardRenderer";
import { TextField } from "../fields/TextField";
import { TextAreaField } from "../fields/TextAreaField";

// Escape-hatch custom renderer for belief-spectrum. The novel interaction is a
// SHARED axis: every stance and 我 sit on the same 5-stop line, so you see them
// relative to each other (a table can't). Snap-to-stops keeps the stored value
// the integer stop index (0..4) — identical to the schema `spectrum` field, so
// the standard envelope / eval are unchanged. Writes only via onField.

const idxOf = (v: unknown): number => (typeof v === "number" ? v : -1);

function StopHeader({ stops }: { stops: string[] }) {
  return (
    <div className="flex items-center gap-3">
      <span className="w-16 shrink-0" aria-hidden />
      <div className="flex flex-1">
        {stops.map((stop, i) => (
          <span key={i} className="flex-1 text-center text-[12px] text-mk-muted-2">{stop}</span>
        ))}
      </div>
    </div>
  );
}

function StopRow({ label, stops, index, onChange }: {
  label: string; stops: string[]; index: number; onChange: (i: number) => void;
}) {
  const max = stops.length - 1;
  const trackRef = useRef<HTMLDivElement>(null);
  const clamp = (n: number) => Math.max(0, Math.min(max, n));

  function fromClientX(clientX: number): number {
    const el = trackRef.current;
    if (!el) return index < 0 ? 0 : index;
    const r = el.getBoundingClientRect();
    if (!r.width) return index < 0 ? 0 : index;
    return clamp(Math.round(((clientX - r.left) / r.width) * max));
  }

  function onKeyDown(e: React.KeyboardEvent) {
    const from = index < 0 ? 0 : index;
    if (e.key === "ArrowRight" || e.key === "ArrowUp") { e.preventDefault(); onChange(clamp(from + 1)); }
    else if (e.key === "ArrowLeft" || e.key === "ArrowDown") { e.preventDefault(); onChange(clamp(from - 1)); }
    else if (e.key === "Home") { e.preventDefault(); onChange(0); }
    else if (e.key === "End") { e.preventDefault(); onChange(max); }
  }

  return (
    <div className="flex items-center gap-3">
      <span className="w-16 shrink-0 truncate text-[12px] font-semibold text-mk-ink" title={label}>{label}</span>
      <div
        ref={trackRef}
        role="radiogroup"
        aria-label={label}
        tabIndex={0}
        onKeyDown={onKeyDown}
        onPointerDown={(e) => { (e.currentTarget as Element).setPointerCapture?.(e.pointerId); onChange(fromClientX(e.clientX)); }}
        onPointerMove={(e) => { if (e.buttons) onChange(fromClientX(e.clientX)); }}
        className="relative flex flex-1 items-center outline-none"
      >
        <div className="pointer-events-none absolute left-[10%] right-[10%] top-1/2 h-[2px] -translate-y-1/2 bg-[#EEF0F4]" aria-hidden />
        {stops.map((stop, i) => (
          <button
            key={i}
            type="button"
            role="radio"
            aria-checked={index === i}
            aria-label={stop}
            tabIndex={-1}
            onClick={() => onChange(i)}
            className="relative z-[1] flex flex-1 items-center justify-center bg-transparent py-2"
          >
            <span className={`h-3.5 w-3.5 rounded-full ${index === i ? "bg-mk-primary" : "bg-[#D9DDE7]"}`} />
          </button>
        ))}
      </div>
    </div>
  );
}

type Stance = { who?: string; position?: number; believes?: string; evidence?: string; interest?: string };

export function BeliefSpectrumRenderer({ card, values, onField, onNote }: CardBodyProps) {
  const step = card.steps[0]!;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const field = (key: string): any => step.fields.find((f) => f.key === key);
  const issueF = field("issue");
  const stancesF = field("stances");
  const selfF = field("self");
  const reasonF = field("self_reason");
  const stops: string[] = selfF.stops;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const itemField = (key: string): any => stancesF.item_fields.find((f: { key: string }) => f.key === key);

  const stances: Stance[] = Array.isArray(values.stances) ? (values.stances as Stance[]) : [];
  const writeStances = (next: Stance[]) => onField("stances", next);
  const setStance = (i: number, patch: Partial<Stance>) => writeStances(stances.map((s, idx) => (idx === i ? { ...s, ...patch } : s)));
  const addStance = () => writeStances([...stances, { position: 2 }]);
  const removeStance = (i: number) => writeStances(stances.filter((_, idx) => idx !== i));

  return (
    <div className="space-y-5">
      <MethodologyPanel step={step} onNote={onNote} />
      <section className="space-y-4 rounded-mk border border-mk-border-2 bg-white p-5">
        <h3 className="text-[15px] font-bold text-mk-ink">{step.title}</h3>
        <TextField field={issueF} value={values.issue} onChange={(v) => onField("issue", v)} />

        {/* Shared spectrum axis — all parties + 我 on one line */}
        <div className="space-y-2 rounded-mk border border-mk-border-2 bg-[#FAFBFC] p-4">
          <StopHeader stops={stops} />
          <StopRow label="我" stops={stops} index={idxOf(values.self)} onChange={(i) => onField("self", i)} />
          {stances.map((s, i) => (
            <StopRow
              key={i}
              label={s.who || `第${i + 1}方`}
              stops={stops}
              index={idxOf(s.position)}
              onChange={(idx) => setStance(i, { position: idx })}
            />
          ))}
        </div>

        {/* Per-stance detail */}
        {stances.map((s, i) => (
          <div key={i} className="space-y-3 rounded-mk border border-mk-border-2 bg-white p-4">
            <div className="flex items-start justify-between gap-3">
              <div className="flex-1"><TextField field={itemField("who")} value={s.who} onChange={(v) => setStance(i, { who: v as string })} /></div>
              <button type="button" onClick={() => removeStance(i)} className="mt-7 shrink-0 text-[12px] font-semibold text-mk-muted-2">移除</button>
            </div>
            <TextAreaField field={itemField("believes")} value={s.believes} onChange={(v) => setStance(i, { believes: v as string })} />
            <TextAreaField field={itemField("evidence")} value={s.evidence} onChange={(v) => setStance(i, { evidence: v as string })} />
            <TextAreaField field={itemField("interest")} value={s.interest} onChange={(v) => setStance(i, { interest: v as string })} />
          </div>
        ))}

        <button type="button" onClick={addStance} className="text-[14px] font-semibold text-mk-primary">+ 添加一方</button>

        <TextAreaField field={reasonF} value={values.self_reason} onChange={(v) => onField("self_reason", v)} />
      </section>
    </div>
  );
}
