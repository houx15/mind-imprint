import type { EvaluationReport } from "@mind-imprint/contracts";
import { Card } from "@/ui";
import { RISK_LABEL } from "./tokens";

export interface RisksProps {
  risks: EvaluationReport["risks"];
}

/**
 * STUB (Task 10/11 fills in) — real build ports `.risks`/`.risk`: type
 * label (`RISK_LABEL`), behaviour + ref link, and the suggestion — framed as
 * "explains evidence scope", not a penalty (see brief §9).
 */
export function Risks({ risks }: RisksProps) {
  return (
    <div className="flex flex-col gap-3" data-testid="risks-section">
      {risks.map((r, i) => (
        <Card key={i} className="border border-mk-warning-bg bg-mk-surface p-4">
          <span className="rounded-mk-full bg-mk-warning-bg px-2.5 py-0.5 text-mk-small font-bold text-mk-warning">
            {RISK_LABEL[r.type]}
          </span>
          <p className="mt-2 text-mk-body text-mk-ink">{r.behaviour}</p>
          <p className="mt-1.5 text-mk-body text-mk-secondary">{r.suggestion}</p>
        </Card>
      ))}
    </div>
  );
}
