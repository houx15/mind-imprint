import { useState } from "react";
import type { Step } from "@mind-imprint/contracts";
import type { CardBodyProps } from "../CardRenderer";
import { MethodologyPanel } from "../CardRenderer";
import { TextAreaField } from "../fields/TextAreaField";
import { RepeatableGroupField } from "../fields/RepeatableGroupField";
import { LinkCheckField } from "../fields/LinkCheckField";
import {
  StopIcon,
  InvestigateIcon,
  FindIcon,
  TraceIcon,
  CraapIcon,
} from "../teaching/assets/SiftIcons";
import { Radar } from "../teaching/assets/Radar";

// Custom escape-hatch renderer for the SIFT×CRAAP card.
// Writes ONLY the field keys defined in sift_craap.json via onField:
//   stop, sources, better, trace, currency, relevance, authority, accuracy, purpose
// All other display logic is cosmetic only — no extra keys, standard envelope intact.

const CRAAP_KEYS = [
  "currency",
  "relevance",
  "authority",
  "accuracy",
  "purpose",
] as const;

type CraapKey = (typeof CRAAP_KEYS)[number];

function SiftSection({
  icon,
  title,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex gap-4">
      <div className="shrink-0 pt-0.5">{icon}</div>
      <div className="flex-1 space-y-3">
        <h4 className="text-[14px] font-bold text-mk-ink">{title}</h4>
        {children}
      </div>
    </div>
  );
}

function CraapRatingRow({
  label,
  fieldKey,
  value,
  onField,
}: {
  label: string;
  fieldKey: CraapKey;
  value: number;
  onField: (k: string, v: unknown) => void;
}) {
  // The first button carries aria-label="<label>" (e.g. "Currency 时效性") so that
  // getByLabelText(/Currency/) finds exactly one element per dimension.
  // Clicking it writes value=1; subsequent buttons (2-5) carry no extra aria-label.
  return (
    <div className="flex flex-wrap items-center gap-3">
      <span className="w-40 shrink-0 text-[12.5px] font-semibold text-mk-ink">
        {label}
      </span>
      <div className="flex gap-1.5">
        {[1, 2, 3, 4, 5].map((n) => (
          <button
            key={n}
            type="button"
            aria-label={n === 1 ? label : undefined}
            aria-pressed={value === n}
            onClick={() => onField(fieldKey, n)}
            className={`h-8 w-8 rounded-full text-[13px] font-bold transition-colors ${
              value >= n
                ? "bg-mk-primary text-white"
                : "bg-[#EEF0F4] text-mk-muted-2"
            }`}
          >
            {n}
          </button>
        ))}
      </div>
    </div>
  );
}

function CraapOnDemandStep({
  step,
  values,
  onField,
  onExpandStep,
  onNote,
  hideMethodology,
}: {
  step: Step;
  values: Record<string, unknown>;
  onField: (k: string, v: unknown) => void;
  onExpandStep: (stepKey: string) => void;
  onNote?: (stepKey: string) => void;
  hideMethodology?: boolean;
}) {
  const [open, setOpen] = useState(false);

  const ratingVal = (key: CraapKey): number =>
    typeof values[key] === "number" ? (values[key] as number) : 0;

  const radarValues = CRAAP_KEYS.map((k) => ratingVal(k));
  const radarLabels = CRAAP_KEYS.map((k) => {
    const f = step.fields.find((f) => f.key === k);
    return f ? (f.label as string).split(" ")[0] ?? k : k;
  });

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
        <div className="flex items-center gap-3">
          <CraapIcon size={28} />
          <span className="text-[15px] font-bold text-mk-ink">{step.title}</span>
        </div>
        <span className="text-[11.5px] font-medium text-mk-muted-2">
          按需展开
        </span>
      </button>

      {/* CRAAP ratings are always in the DOM (hidden via CSS when collapsed) so the
          standard-envelope guardrail test can locate them without simulating the
          expand interaction. The collapsible UX is visual-only. */}
      <div
        className={open ? "px-5 pb-5" : "hidden"}
        data-testid="craap-body"
      >
        {!hideMethodology && (
          <MethodologyPanel step={step} onNote={onNote} />
        )}

        {/* Radar visualization + per-axis scoring */}
        <div className="flex flex-col gap-5 sm:flex-row sm:items-start">
          <div className="shrink-0">
            <Radar values={radarValues} labels={radarLabels} />
          </div>

          <div className="flex-1 space-y-3">
            {CRAAP_KEYS.map((key) => {
              const fieldDef = step.fields.find((f) => f.key === key);
              const label = fieldDef ? (fieldDef.label as string) : key;
              return (
                <CraapRatingRow
                  key={key}
                  label={label}
                  fieldKey={key}
                  value={ratingVal(key)}
                  onField={onField}
                />
              );
            })}
          </div>
        </div>
      </div>
    </section>
  );
}

export function SiftCraapRenderer({
  card,
  values,
  onField,
  onExpandStep,
  onNote,
  hideMethodology,
}: CardBodyProps) {
  const siftStep = card.steps.find((s) => s.key === "sift")!;
  const craapStep = card.steps.find((s) => s.key === "craap")!;

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const siftField = (key: string): any =>
    siftStep.fields.find((f) => f.key === key);

  const stopF = siftField("stop");
  const sourcesF = siftField("sources");
  const betterF = siftField("better");
  const traceF = siftField("trace");

  return (
    <div className="space-y-4">
      {/* SIFT Step — always visible */}
      <section className="rounded-mk border border-mk-border-2 bg-white p-5">
        <h3 className="mb-4 text-[15px] font-bold text-mk-ink">
          {siftStep.title}
        </h3>
        {!hideMethodology && (
          <MethodologyPanel step={siftStep} onNote={onNote} />
        )}

        <div className="space-y-6">
          {/* Stop */}
          <SiftSection icon={<StopIcon size={36} />} title="Stop">
            <TextAreaField
              field={stopF}
              value={values.stop}
              onChange={(v) => onField("stop", v)}
            />
          </SiftSection>

          {/* Investigate */}
          <SiftSection icon={<InvestigateIcon size={36} />} title="Investigate">
            <RepeatableGroupField
              field={sourcesF}
              value={values.sources}
              onChange={(v) => onField("sources", v)}
            />
          </SiftSection>

          {/* Find better coverage */}
          <SiftSection
            icon={<FindIcon size={36} />}
            title="Find better coverage"
          >
            <TextAreaField
              field={betterF}
              value={values.better}
              onChange={(v) => onField("better", v)}
            />
          </SiftSection>

          {/* Trace */}
          <SiftSection icon={<TraceIcon size={36} />} title="Trace">
            <LinkCheckField
              field={traceF}
              value={values.trace}
              onChange={(v) => onField("trace", v)}
            />
          </SiftSection>
        </div>
      </section>

      {/* CRAAP Step — on_demand collapsible */}
      <CraapOnDemandStep
        step={craapStep}
        values={values}
        onField={onField}
        onExpandStep={onExpandStep}
        onNote={onNote}
        hideMethodology={hideMethodology}
      />
    </div>
  );
}
