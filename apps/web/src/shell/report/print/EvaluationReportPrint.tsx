import type { EvaluationReport } from "@mind-imprint/contracts";
import { AboutReport, Cover } from "./parts";
import { SummarySection } from "./sections";

export function EvaluationReportPrint({ report }: { report: EvaluationReport }) {
  return (
    <div className="report-print-root-inner">
      <Cover report={report} />
      <AboutReport report={report} />
      <SummarySection report={report} />
    </div>
  );
}
