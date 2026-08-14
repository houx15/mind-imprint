import type { EvaluationReport } from "@mind-imprint/contracts";
import { Cover } from "./parts";

export function EvaluationReportPrint({ report }: { report: EvaluationReport }) {
  return (
    <div className="report-print-root-inner">
      <Cover report={report} />
    </div>
  );
}
