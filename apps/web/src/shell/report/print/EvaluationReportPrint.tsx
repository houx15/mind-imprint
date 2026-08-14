import type { EvaluationReport } from "@mind-imprint/contracts";
import { AboutReport, Cover } from "./parts";
import { MaterialsSection, SummarySection, TimelineSection } from "./sections";
import { AutonomySection, DepthSection } from "./axis";

export function EvaluationReportPrint({ report }: { report: EvaluationReport }) {
  return (
    <div className="report-print-root-inner">
      <Cover report={report} />
      <AboutReport report={report} />
      <SummarySection report={report} />
      <TimelineSection events={report.events} />
      <MaterialsSection materials={report.materials} />
      <DepthSection dims={report.depth} />
      <AutonomySection dims={report.autonomy} />
    </div>
  );
}
