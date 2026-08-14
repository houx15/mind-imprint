import { useEffect, useState } from "react";
import { createPortal, flushSync } from "react-dom";
import type { EvaluationReport } from "@mind-imprint/contracts";
import { Button } from "@/ui";
import { EvaluationReportPrint } from "./EvaluationReportPrint";

export function ReportPrintButton({ report }: { report: EvaluationReport }) {
  const [printing, setPrinting] = useState(false);

  useEffect(() => {
    if (!printing) return;
    // `afterprint` fires from the native print dialog, entirely outside
    // React's event system — React 18's automatic batching would otherwise
    // defer this update to a microtask. flushSync forces the removal to
    // apply in the same tick the event fires, so the print-only tree never
    // lingers after the dialog closes.
    const done = () => flushSync(() => setPrinting(false));
    window.addEventListener("afterprint", done);
    // Print after the portal has painted this frame.
    const id = window.setTimeout(() => window.print(), 0);
    return () => {
      window.removeEventListener("afterprint", done);
      window.clearTimeout(id);
    };
  }, [printing]);

  return (
    <>
      <Button variant="secondary" size="sm" onClick={() => setPrinting(true)}>
        导出 PDF
      </Button>
      {printing &&
        createPortal(
          <div className="report-print-root">
            <EvaluationReportPrint report={report} />
          </div>,
          document.body,
        )}
    </>
  );
}
