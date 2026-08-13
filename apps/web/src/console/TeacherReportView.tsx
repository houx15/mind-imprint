import { useEffect, useState } from "react";
import type { ApiClient, TeacherReport } from "../api";
import { ApiError } from "../api";
import type { EvaluationReport } from "@mind-imprint/contracts";
import { EvaluationReportView } from "../shell/report/EvaluationReport";
import { EvidenceMap } from "./EvidenceMap";
import { ParentReport } from "./ParentReport";

// Task 13: the teacher end now renders the SAME `EvaluationReport` the
// student sees (`EvaluationReportView`, apps/web/src/shell/report/EvaluationReport)
// as the main report body, instead of a hand-rolled dual-axis rendering. The
// new pipeline is project-scoped only (evaluation_report rows key off
// project_id — there is no chat/course equivalent yet), so the main body
// only renders for surface === "project"; other surfaces keep the identity
// chrome + 证据地图 but show a placeholder where the dual-axis body used to
// be, rather than resurrecting the retired rendering.
//
// Kept auxiliary surfaces (still fed by the ORIGINAL `getStudentReport` fetch,
// which stays alive alongside the new one — two fetches is fine):
//   - 证据地图 (`EvidenceMap`) — a pure projection of the old DualAxisReport
//     shape, unrelated to the removed hand-rolled JSX; still compiles as-is.
//   - `ParentReport` — its own fetch (via `../api`'s `api` export), untouched.
//   - the identity/context header (title, chips, D/A badges) — still sourced
//     from `context`/`report` top-level fields, not the removed dimension body.
// Gated (commented out, not deleted): none needed — nothing here imports the
// retired dual-axis type in a way that broke once the D/A section left.

type Client = Pick<ApiClient, "getStudentReport" | "getStudentEvaluationReport">;

const SURFACE_LABEL: Record<string, string> = { project: "项目", course: "课程", chat: "对话" };

const CARD_STYLE: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: 16 };
const H2_STYLE: React.CSSProperties = { fontSize: 20, fontWeight: 800, color: "#1C2333", margin: "38px 0 16px" };
const CAPTION_STYLE: React.CSSProperties = { fontSize: 12.5, color: "#8A92A3", lineHeight: 1.7, marginBottom: 16 };

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
  const [data, setData] = useState<TeacherReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [parentReportOpen, setParentReportOpen] = useState(false);

  // Task 13: the new SAME-shape evaluation report, project-scoped only.
  const [evalReport, setEvalReport] = useState<EvaluationReport | null>(null);
  const [evalError, setEvalError] = useState(false);
  const [evalLoaded, setEvalLoaded] = useState(false);

  function load() {
    setError(null);
    client.getStudentReport(classId, userId, surface, scopeId).then(setData).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }
  useEffect(load, [client, classId, userId, surface, scopeId]);

  function loadEval() {
    setEvalReport(null);
    setEvalError(false);
    setEvalLoaded(false);
    if (surface !== "project") return;
    let cancelled = false;
    client
      .getStudentEvaluationReport(classId, userId, scopeId)
      .then((r) => {
        if (cancelled) return;
        setEvalReport(r);
        setEvalLoaded(true);
      })
      .catch(() => {
        if (!cancelled) {
          setEvalError(true);
          setEvalLoaded(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }
  useEffect(loadEval, [client, classId, userId, surface, scopeId]);

  if (error) {
    return (
      <div style={{ flex: 1, padding: 40 }}>
        <button onClick={onBack} style={{ background: "transparent", border: "none", color: "#8A92A3", fontSize: 14, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: 0 }}>← 全部学生</button>
        <div style={{ marginTop: 20, color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
      </div>
    );
  }
  if (!data) {
    return <div style={{ flex: 1 }} />;
  }

  const { report, context } = data;

  const title = context.title ?? context.projectTitle ?? `${SURFACE_LABEL[surface] ?? surface}报告`;
  const chips = [
    SURFACE_LABEL[surface] ?? surface,
    report.officialProjection ? report.officialProjection.standard.name : null,
    report.generatedAt ? report.generatedAt.slice(0, 10) : null,
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
          {surface === "project" ? (
            <button
              onClick={() => setParentReportOpen(true)}
              style={{ flex: "none", display: "inline-flex", alignItems: "center", gap: 7, background: "#fff", border: "1px solid #DCE0EA", color: "#2A3B7A", fontSize: 13, fontWeight: 700, padding: "10px 16px", borderRadius: 11, cursor: "pointer", fontFamily: "inherit" }}
            >
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" /></svg>
              导出家长版 PDF
            </button>
          ) : (
            <span
              title="家长版报告即将上线"
              aria-disabled="true"
              style={{ flex: "none", display: "inline-flex", alignItems: "center", gap: 7, background: "#F5F6FA", border: "1px solid #E1E4ED", color: "#B4BAC8", fontSize: 13, fontWeight: 700, padding: "10px 16px", borderRadius: 11, cursor: "not-allowed" }}
            >
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" /></svg>
              导出家长版 PDF
            </span>
          )}
        </div>
      </div>

      {/* MAIN REPORT BODY — Task 13: the SAME EvaluationReport the student
          sees. Project-scoped only; other surfaces get a placeholder (the
          new pipeline has no chat/course report to render yet). */}
      <div style={{ marginTop: 20 }} data-testid="teacher-report-body">
        {surface !== "project" ? (
          <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px" }}>
            <div style={CARD_STYLE}>该类型报告的统一格式暂未上线，敬请期待。</div>
          </div>
        ) : evalError ? (
          <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px" }}>
            <div style={{ ...CARD_STYLE, color: "#C76B6B" }}>
              报告加载失败 · <span onClick={loadEval} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span>
            </div>
          </div>
        ) : !evalLoaded ? (
          <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px" }}>
            <div style={CARD_STYLE}>加载中……</div>
          </div>
        ) : !evalReport ? (
          <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px" }}>
            <div style={CARD_STYLE}>这个项目还没有生成过程评估报告。</div>
          </div>
        ) : (
          <EvaluationReportView report={evalReport} />
        )}
      </div>

      {/* 证据地图 — auxiliary surface, still fed by the original getStudentReport
          fetch (a pure client-side projection of `report`/`context`,
          unaffected by the removal of the hand-rolled dual-axis body). */}
      <div style={{ maxWidth: 1080, margin: "0 auto", padding: "0 30px 72px" }}>
        <h2 style={H2_STYLE}>证据地图</h2>
        <div style={CAPTION_STYLE}>点击节点，看 RQ、官方投影、双轴、AI 互动如何串起这个项目的证据。</div>
        <div data-testid="evidence-map-slot">
          <EvidenceMap report={report} context={context} studentName={studentName} />
        </div>
      </div>

      {parentReportOpen && surface === "project" && (
        <ParentReport
          classId={classId}
          studentId={userId}
          surface="project"
          scopeId={scopeId}
          studentName={studentName}
          onClose={() => setParentReportOpen(false)}
        />
      )}
    </div>
  );
}
