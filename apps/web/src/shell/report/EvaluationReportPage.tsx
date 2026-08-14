import { useEffect, useState } from "react";
import type { EvaluationReport } from "@mind-imprint/contracts";
import { getEvaluationReport, generateEvaluationReport, type EvalReportEnvelope } from "@/api/evaluationReport";
import { EvaluationReportView } from "@/shell/report/EvaluationReport";
import { ArrowLeft, Button, EmptyState, Icon, PebbleInlineSpinner, useRotatingCaption } from "@/ui";

/**
 * EvaluationReportPage — the report container (Task 12, reworked Task 5 for
 * the three-state envelope). Wraps `EvaluationReportView` in a tabbar-chrome
 * top bar (back + a disabled "导出 PDF" placeholder) and owns the
 * fetch/generate/poll lifecycle against the backend envelope
 * (`null` | `{status:"generating"}` | `{status:"failed"}` |
 * `{status:"ready",report}`):
 *
 *   GET → if `null` (no row yet), POST generate once (claim) → branch:
 *     - `ready`     → render the report.
 *     - `generating` → spinner + rotating caption, poll GET every 3s until
 *       `ready`/`failed` (interval cleared on unmount or on a fresh attempt).
 *     - `failed` (or any fetch throwing) → EmptyState + 重试, which re-runs
 *       the whole lifecycle from GET.
 *
 * This "stops racing the async generator" — it never assumes a null read
 * means "nothing exists forever"; it just isn't ready yet.
 */

const GENERATING_LINES = [
  "印记正在梳理这个项目的过程记录……",
  "印记正在核对认知深度与智识自主……",
  "印记正在生成你的思维印记报告……",
];

const POLL_INTERVAL_MS = 3000;

type LoadState =
  | { status: "loading" }
  | { status: "generating" }
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
  const [attempt, setAttempt] = useState(0);
  const caption = useRotatingCaption(GENERATING_LINES);

  useEffect(() => {
    let cancelled = false;
    // Self-rescheduling `setTimeout`, not `setInterval`: the next poll is
    // only queued AFTER the current GET settles, so there is never more
    // than one in-flight request. A bare `setInterval` can fire a new tick
    // while a prior (slow) one is still in flight; if that stale response
    // lands after a later tick already resolved ready/failed and stopped
    // the timer, applying it would regress the UI back to "generating"
    // with nothing left scheduled — stranding the user on the spinner.
    // This structure makes that ordering impossible by construction.
    let pollTimer: ReturnType<typeof setTimeout> | undefined;
    setState({ status: "loading" });

    function stopPolling() {
      if (pollTimer !== undefined) {
        clearTimeout(pollTimer);
        pollTimer = undefined;
      }
    }

    function applyEnvelope(envelope: EvalReportEnvelope | null): boolean {
      // Returns true once the envelope has reached a terminal state
      // (ready/failed) — callers use this to stop polling.
      if (cancelled) return true;
      if (envelope === null || envelope.status === "failed") {
        setState({ status: "error" });
        return true;
      }
      if (envelope.status === "ready") {
        setState({ status: "ready", report: envelope.report });
        return true;
      }
      setState({ status: "generating" });
      return false;
    }

    function schedulePoll() {
      pollTimer = setTimeout(() => {
        void (async () => {
          try {
            const envelope = await getEvaluationReport(projectId);
            if (cancelled) return;
            const settled = applyEnvelope(envelope);
            if (!settled) schedulePoll();
          } catch {
            // Transient poll error — keep polling rather than flashing an
            // error state on a single failed tick.
            if (!cancelled) schedulePoll();
          }
        })();
      }, POLL_INTERVAL_MS);
    }

    void (async () => {
      try {
        let envelope = await getEvaluationReport(projectId);
        if (envelope === null) envelope = await generateEvaluationReport(projectId);
        if (cancelled) return;
        const settled = applyEnvelope(envelope);
        if (!settled) schedulePoll();
      } catch {
        if (!cancelled) setState({ status: "error" });
      }
    })();

    return () => {
      cancelled = true;
      stopPolling();
    };
  }, [projectId, attempt]);

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
        {(state.status === "loading" || state.status === "generating") && (
          <div className="flex h-full flex-col items-center justify-center gap-3">
            <PebbleInlineSpinner size={28} />
            <p role="status" aria-live="polite" className="text-mk-body text-mk-muted">
              {state.status === "generating" ? caption : "正在加载……"}
            </p>
          </div>
        )}

        {state.status === "error" && (
          <div className="flex h-full items-center justify-center px-6">
            <EmptyState
              illustration="completed"
              title="报告暂时无法生成"
              body="这个项目的过程评估报告还没准备好，请稍后重试。"
              action={{ label: "重试", onClick: () => setAttempt((n) => n + 1) }}
            />
          </div>
        )}

        {state.status === "ready" && <EvaluationReportView report={state.report} />}
      </div>
    </div>
  );
}
