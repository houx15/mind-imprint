import { useEffect, useState } from "react";
import type { ApiClient } from "../api";
import type { EvaluationReport } from "@mind-imprint/contracts";
import { EvaluationReportView } from "../shell/report/EvaluationReport";
import { ArrowLeft, Button, Card, EmptyState, Icon, Loader2 } from "@/ui";

// Task 6 (2026-08-14 retirement pass): the teacher end renders ONLY the new
// EvaluationReport — the same shape the student sees via EvaluationReportView.
// The retired `getStudentReport`/`TeacherReport` fetch, the 证据地图
// (EvidenceMap) projection of that old shape, and the 导出家长版 PDF
// (ParentReport) action are all gone from this view; they leaned on data the
// finish flow no longer writes (it writes `evaluation_report`, not the old
// `evaluation` row). The new pipeline is project-scoped only (no chat/course
// equivalent yet), so the body only renders for surface === "project"; other
// surfaces keep the identity chrome but show a placeholder.
//
// Task 8 (2026-08-14 activity migration): teacher-side polling now mirrors
// `EvaluationReportPage` — a self-rescheduling `setTimeout` (never a bare
// `setInterval`) keeps exactly one GET in flight while the envelope is
// `generating`, and stops once it reaches `ready`/`failed`. Unlike the
// student page this view never calls generate: it is a read-no-call replay
// of whatever the student side already kicked off.

type Client = Pick<ApiClient, "getStudentEvaluationReport">;

const SURFACE_LABEL: Record<string, string> = { project: "项目", course: "课程", chat: "对话" };

const POLL_INTERVAL_MS = 3000;

type LoadState =
  | { status: "loading" }
  | { status: "generating" }
  | { status: "failed" }
  | { status: "empty" }
  | { status: "ready"; report: EvaluationReport };

export function TeacherReportView({
  client,
  classId,
  userId,
  surface,
  scopeId,
  studentName,
  onBack,
}: {
  client: Client;
  classId: string;
  userId: string;
  surface: string;
  scopeId: string;
  studentName?: string;
  onBack: () => void;
}) {
  const [state, setState] = useState<LoadState>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    // Other surfaces render a fixed placeholder below without touching
    // `state` at all — no fetch, no polling, nothing to load.
    if (surface !== "project") return;

    let cancelled = false;
    // Self-rescheduling `setTimeout`, not `setInterval` — mirrors
    // `EvaluationReportPage`. The next poll is only queued AFTER the current
    // GET settles, so there is never more than one in-flight request; a bare
    // `setInterval` could let a stale response land after a later tick
    // already stopped the timer and strand the UI on the spinner.
    let pollTimer: ReturnType<typeof setTimeout> | undefined;
    setState({ status: "loading" });

    function stopPolling() {
      if (pollTimer !== undefined) {
        clearTimeout(pollTimer);
        pollTimer = undefined;
      }
    }

    function applyEnvelope(envelope: Awaited<ReturnType<Client["getStudentEvaluationReport"]>>): boolean {
      // Returns true once the envelope has reached a terminal state
      // (ready/failed/empty) — callers use this to stop polling.
      if (cancelled) return true;
      if (envelope === null) {
        setState({ status: "empty" });
        return true;
      }
      if (envelope.status === "failed") {
        setState({ status: "failed" });
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
            const envelope = await client.getStudentEvaluationReport(classId, userId, scopeId);
            if (cancelled) return;
            const settled = applyEnvelope(envelope);
            if (!settled) schedulePoll();
          } catch {
            // Transient poll error — keep polling rather than flashing a
            // failed state on a single failed tick.
            if (!cancelled) schedulePoll();
          }
        })();
      }, POLL_INTERVAL_MS);
    }

    void (async () => {
      try {
        const envelope = await client.getStudentEvaluationReport(classId, userId, scopeId);
        if (cancelled) return;
        const settled = applyEnvelope(envelope);
        if (!settled) schedulePoll();
      } catch {
        if (!cancelled) setState({ status: "failed" });
      }
    })();

    return () => {
      cancelled = true;
      stopPolling();
    };
  }, [client, classId, userId, surface, scopeId, attempt]);

  const evalReport = state.status === "ready" ? state.report : null;
  const title = evalReport?.basics.title ?? `${SURFACE_LABEL[surface] ?? surface}报告`;
  const chips = [
    SURFACE_LABEL[surface] ?? surface,
    evalReport?.basics.type ?? null,
    evalReport?.generatedAt ? evalReport.generatedAt.slice(0, 10) : null,
  ].filter((c): c is string => !!c);

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", position: "relative" }}>
      <div style={{ maxWidth: 1080, margin: "0 auto", padding: "22px 30px 0" }}>
        {/* header */}
        <div style={{ display: "flex", alignItems: "center", gap: 9, marginBottom: 14, fontSize: 13, fontWeight: 600 }}>
          <Button variant="ghost" size="sm" iconStart={<Icon icon={ArrowLeft} size={16} />} onClick={onBack}>全部学生</Button>
          <span style={{ color: "var(--mk-border)" }}>/</span>
          <span onClick={onBack} style={{ color: "var(--mk-muted)", cursor: "pointer" }}>{studentName ?? title}</span>
        </div>
        <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 20 }}>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontSize: 12, fontWeight: 700, color: "var(--mk-accent-600)", letterSpacing: ".03em" }}>能力报告 · 教师视图</div>
            <div style={{ fontSize: 22, fontWeight: 800, color: "var(--mk-ink)", marginTop: 6, lineHeight: 1.25 }}>{studentName ? `${studentName} · ${title}` : title}</div>
            <div style={{ marginTop: 10, display: "flex", flexWrap: "wrap", gap: 7 }}>
              {chips.map((chip) => (
                <span key={chip} style={{ display: "inline-flex", alignItems: "center", minHeight: 26, padding: "4px 11px", border: "1px solid var(--mk-border)", borderRadius: "var(--mk-radius-full)", background: "var(--mk-paper)", fontSize: 12, color: "var(--mk-secondary)" }}>{chip}</span>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* MAIN REPORT BODY — the SAME EvaluationReport the student sees.
          Project-scoped only; other surfaces get a placeholder (the new
          pipeline has no chat/course report to render yet). */}
      <div style={{ marginTop: 20 }} data-testid="teacher-report-body">
        {surface !== "project" ? (
          <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px" }}>
            <Card className="p-6">
              <EmptyState illustration="completed" title="暂未上线" body="该类型报告的统一格式暂未上线，敬请期待。" />
            </Card>
          </div>
        ) : state.status === "failed" ? (
          <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px" }}>
            <Card className="p-6">
              <EmptyState illustration="completed" title="报告加载失败" body="这份能力报告暂时无法加载，请稍后重试。" action={{ label: "重试", onClick: () => setAttempt((n) => n + 1) }} />
            </Card>
          </div>
        ) : state.status === "loading" ? (
          <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 8, padding: "60px 0", color: "var(--mk-muted)", fontSize: 14.5 }}>
            <Icon icon={Loader2} size={18} className="animate-spin" />
            加载中…
          </div>
        ) : state.status === "generating" ? (
          <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 8, padding: "60px 0", color: "var(--mk-muted)", fontSize: 14.5 }}>
            <Icon icon={Loader2} size={18} className="animate-spin" />
            报告生成中……
          </div>
        ) : state.status === "empty" ? (
          <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px" }}>
            <Card className="p-6">
              <EmptyState illustration="completed" title="还没有报告" body="这个项目还没有生成过程评估报告。" />
            </Card>
          </div>
        ) : (
          <EvaluationReportView report={state.report} />
        )}
      </div>
    </div>
  );
}
