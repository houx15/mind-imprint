import { useEffect, useState } from "react";
import type { ApiClient } from "../api";
import type { EvaluationReport } from "@mind-imprint/contracts";
import { EvaluationReportView } from "../shell/report/EvaluationReport";

// Task 6 (2026-08-14 retirement pass): the teacher end renders ONLY the new
// EvaluationReport — the same shape the student sees via EvaluationReportView.
// The retired `getStudentReport`/`TeacherReport` fetch, the 证据地图
// (EvidenceMap) projection of that old shape, and the 导出家长版 PDF
// (ParentReport) action are all gone from this view; they leaned on data the
// finish flow no longer writes (it writes `evaluation_report`, not the old
// `evaluation` row). The new pipeline is project-scoped only (no chat/course
// equivalent yet), so the body only renders for surface === "project"; other
// surfaces keep the identity chrome but show a placeholder.

type Client = Pick<ApiClient, "getStudentEvaluationReport">;

const SURFACE_LABEL: Record<string, string> = { project: "项目", course: "课程", chat: "对话" };

const CARD_STYLE: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: 16 };

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

  function load() {
    if (surface !== "project") return;
    setState({ status: "loading" });
    let cancelled = false;
    client
      .getStudentEvaluationReport(classId, userId, scopeId)
      .then((envelope) => {
        if (cancelled) return;
        if (envelope === null) setState({ status: "empty" });
        else if (envelope.status === "ready") setState({ status: "ready", report: envelope.report });
        else if (envelope.status === "generating") setState({ status: "generating" });
        else setState({ status: "failed" });
      })
      .catch(() => {
        if (!cancelled) setState({ status: "failed" });
      });
    return () => {
      cancelled = true;
    };
  }
  useEffect(load, [client, classId, userId, surface, scopeId]);

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
          <span onClick={onBack} style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "#6C7488", cursor: "pointer" }}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round"><path d="M19 12H5M11 18l-6-6 6-6" /></svg>
            全部学生
          </span>
          <span style={{ color: "#C6CBD8" }}>/</span>
          <span onClick={onBack} style={{ color: "#6C7488", cursor: "pointer" }}>{studentName ?? title}</span>
        </div>
        <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 20 }}>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontSize: 12, fontWeight: 700, color: "#2A3B7A", letterSpacing: ".03em" }}>能力报告 · 教师视图</div>
            <div style={{ fontSize: 22, fontWeight: 800, color: "#1C2333", marginTop: 6, lineHeight: 1.25 }}>{studentName ? `${studentName} · ${title}` : title}</div>
            <div style={{ marginTop: 10, display: "flex", flexWrap: "wrap", gap: 7 }}>
              {chips.map((chip) => (
                <span key={chip} style={{ display: "inline-flex", alignItems: "center", minHeight: 26, padding: "4px 11px", border: "1px solid #E4E7EF", borderRadius: 20, background: "#FAFBFD", fontSize: 12, color: "#4A5060" }}>{chip}</span>
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
            <div style={CARD_STYLE}>该类型报告的统一格式暂未上线，敬请期待。</div>
          </div>
        ) : state.status === "failed" ? (
          <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px" }}>
            <div style={{ ...CARD_STYLE, color: "#C76B6B" }}>
              报告加载失败 · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span>
            </div>
          </div>
        ) : state.status === "loading" ? (
          <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px" }}>
            <div style={CARD_STYLE}>加载中……</div>
          </div>
        ) : state.status === "generating" ? (
          <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px" }}>
            <div style={CARD_STYLE}>报告生成中……</div>
          </div>
        ) : state.status === "empty" ? (
          <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px" }}>
            <div style={CARD_STYLE}>这个项目还没有生成过程评估报告。</div>
          </div>
        ) : (
          <EvaluationReportView report={state.report} />
        )}
      </div>
    </div>
  );
}
