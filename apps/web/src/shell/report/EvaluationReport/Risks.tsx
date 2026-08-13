import type { EvaluationReport, RiskEntry } from "@mind-imprint/contracts";
import { RISK_LABEL } from "./tokens";

export interface RisksProps {
  risks: EvaluationReport["risks"];
}

type RiskTone = "warning" | "danger";

/** `data-scope` reads as a harder/red tint in the mockup; every other type stays warm/amber. */
function riskTone(type: RiskEntry["type"]): RiskTone {
  return type === "data-scope" ? "danger" : "warning";
}

const TYPE_CHIP: Record<RiskTone, string> = {
  warning: "bg-mk-warning-bg text-mk-warning",
  danger: "bg-mk-danger-bg text-mk-danger",
};

/**
 * Risks — ports `.risks`/`.risk` from
 * `docs/reference/2026-08-13-eval-report-mockup.html`: a warm hairline card
 * per `RiskEntry` — a type chip (`RISK_LABEL`), the observed `behaviour`
 * (with an optional `ref` link), and the `suggestion` under "处理方式：".
 * Framed as an observation for the report reader, not a deduction/score
 * against the student (see AGENTS.md 铁律①: AI 不替学生定论). Note: this is
 * a plain warm-bordered `div`, not a `Card`/`Surface level="md"` — object
 * cards are borderless, so a bordered warm card must not be built on one.
 */
export function Risks({ risks }: RisksProps) {
  return (
    <div className="flex flex-col gap-3" data-testid="risks-section">
      {risks.map((r, i) => (
        <div
          key={i}
          className="grid grid-cols-[auto_1fr] gap-4 rounded-mk-sm border border-mk-warning-bg bg-mk-paper p-4 shadow-mk-xs"
          data-testid="risk-card"
        >
          <span
            className={`self-start whitespace-nowrap rounded-mk-full px-2.5 py-1 text-mk-small font-bold ${TYPE_CHIP[riskTone(r.type)]}`}
          >
            {RISK_LABEL[r.type]}
          </span>
          <div className="min-w-0">
            <p className="text-mk-body text-mk-ink">
              {r.behaviour}
              {r.ref && (
                <a href={`#${r.ref.id}`} className="ml-2 text-mk-small text-mk-info">
                  {r.ref.label ?? r.ref.id} →
                </a>
              )}
            </p>
            <p className="mt-1.5 text-mk-body text-mk-secondary">
              <span className="font-semibold text-mk-success">处理方式：</span>
              {r.suggestion}
            </p>
          </div>
        </div>
      ))}
    </div>
  );
}
