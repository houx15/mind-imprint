import type { EvaluationReport } from "@mind-imprint/contracts";
import { Card } from "@/ui";

export interface MaterialsProps {
  materials: EvaluationReport["materials"];
}

/**
 * STUB (Task 10/11 fills in) — real build ports `.mats`/`.mat`: per-source
 * finalStatus credibility chip, comment, and `cannotSupport` boundary line.
 */
export function Materials({ materials }: MaterialsProps) {
  return (
    <div className="flex flex-col gap-3.5" data-testid="materials-section">
      {materials.map((m) => (
        <Card key={m.materialId} className="p-4">
          <div className="flex flex-wrap items-center gap-2.5">
            <span className="text-mk-h3 text-mk-ink">{m.source}</span>
            <span className="rounded-mk-full bg-mk-paper px-2 py-0.5 text-mk-small text-mk-muted">{m.finalStatus}</span>
          </div>
          <p className="mt-2 text-mk-body text-mk-secondary">{m.comment}</p>
          <p className="mt-1.5 text-mk-small text-mk-danger">不能支撑：{m.cannotSupport}</p>
        </Card>
      ))}
    </div>
  );
}
