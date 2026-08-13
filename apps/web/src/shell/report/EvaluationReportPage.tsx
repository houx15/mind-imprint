import { useEffect, useState } from "react";
import type { EvaluationReport } from "@mind-imprint/contracts";
import { getEvaluationReport, generateEvaluationReport } from "@/api/evaluationReport";
import { EvaluationReportView } from "@/shell/report/EvaluationReport";
import { ArrowLeft, Button, EmptyState, Icon, PebbleInlineSpinner, useRotatingCaption } from "@/ui";

/**
 * EvaluationReportPage — the report container (Task 12). Wraps
 * `EvaluationReportView` in a tabbar-chrome top bar (back + a disabled
 * "导出 PDF" placeholder) and owns the fetch-or-generate lifecycle:
 * `getEvaluationReport` first, and only if that comes back `null`
 * (no report exists yet) does it call `generateEvaluationReport` —
 * first-open-wins, matching the pattern other AI-generated surfaces in the
 * shell use (e.g. `GrowthReport`'s history fetch).
 */

const GENERATING_LINES = [
  "印记正在梳理这个项目的过程记录……",
  "印记正在核对认知深度与智识自主……",
  "印记正在生成你的思维印记报告……",
];

type LoadState =
  | { status: "loading" }
  | { status: "error" }
  | { status: "ready"; report: EvaluationReport };

export function EvaluationReportPage({
  projectId,
  onBack,
}: {
  projectId: string;
  onBack: () => void;
}) {
  const [state, setState] = useState<LoadState>({ status: "loading" });
  const caption = useRotatingCaption(GENERATING_LINES);

  useEffect(() => {
    let cancelled = false;
    setState({ status: "loading" });
    void (async () => {
      try {
        let report = await getEvaluationReport(projectId);
        if (!report) report = await generateEvaluationReport(projectId);
        if (cancelled) return;
        if (!report) setState({ status: "error" });
        else setState({ status: "ready", report });
      } catch {
        if (!cancelled) setState({ status: "error" });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  return (
    <div className="flex h-full w-full min-h-0 flex-col overflow-hidden bg-mk-paper">
      <div className="flex shrink-0 items-center justify-between border-b border-mk-border bg-mk-surface px-5 py-3">
        <button
          type="button"
          onClick={onBack}
          className="inline-flex items-center gap-1.5 rounded-mk-sm px-2 py-1.5 text-mk-body text-mk-muted transition-colors duration-[120ms] ease-mk hover:bg-mk-paper hover:text-mk-ink"
        >
          <Icon icon={ArrowLeft} size={16} />
          返回
        </button>
        <Button variant="secondary" size="sm" disabled title="即将上线">
          导出 PDF
        </Button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {state.status === "loading" && (
          <div className="flex h-full flex-col items-center justify-center gap-3">
            <PebbleInlineSpinner size={28} />
            <p role="status" aria-live="polite" className="text-mk-body text-mk-muted">
              {caption}
            </p>
          </div>
        )}

        {state.status === "error" && (
          <div className="flex h-full items-center justify-center px-6">
            <EmptyState
              illustration="completed"
              title="报告暂时无法生成"
              body="这个项目的过程评估报告还没准备好，请稍后重试。"
            />
          </div>
        )}

        {state.status === "ready" && <EvaluationReportView report={state.report} />}
      </div>
    </div>
  );
}
