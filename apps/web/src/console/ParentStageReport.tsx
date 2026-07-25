import { useEffect, useState } from "react";
import type { ParentStageReport as ParentStageReportDTO } from "@mind-imprint/contracts";
import { api, ApiError } from "../api";
import { ParentReportChrome } from "./ParentReportChrome";

// Full-screen printable overlay for the stage-mode (per-student-per-week)
// parent projection: reuses ParentReportChrome for the mode-independent
// front-matter/生成 button/下一步/名词解释/footer, supplying only the
// stage-specific middle — 4 usage stat cards + 这段时间的变化 + (optional)
// 本阶段亮点 + 往前看. No D/A rows here (no per-project canonical readings at
// stage scope) and — like the project view — no A-axis number anywhere.

const CARD: React.CSSProperties = { border: "1px solid #E4E7EF", borderRadius: 14, padding: "16px 18px" };
const H2: React.CSSProperties = { fontSize: 19, fontWeight: 800, color: "#1C2333", margin: "30px 0 10px" };

export function ParentStageReport({
  classId,
  studentId,
  studentName,
  onClose,
}: {
  classId: string;
  studentId: string;
  studentName?: string;
  onClose: () => void;
}) {
  const [data, setData] = useState<ParentStageReportDTO | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);

  useEffect(() => {
    setError(null);
    api.getParentStageReport(classId, studentId).then(setData).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }, [classId, studentId]);

  async function handleGenerate() {
    setGenerating(true);
    try {
      const next = await api.generateParentStageProse(classId, studentId);
      setData(next);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "生成失败");
    } finally {
      setGenerating(false);
    }
  }

  return (
    <div style={{ position: "fixed", inset: 0, zIndex: 100, background: "#F3F4F8", overflowY: "auto" }}>
      <style>{"@media print { .no-print { display:none !important; } }"}</style>
      {error && (
        <div style={{ margin: "20px 24px", color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>{error}</div>
      )}
      {data && (
        <ParentReportChrome
          studentName={studentName}
          cover={data.cover}
          advice={data.advice}
          proseAbsent={data.prose === null}
          generating={generating}
          generateLabel="生成家长版正文"
          onGenerate={handleGenerate}
          onClose={onClose}
        >
          {/* 这一阶段的使用与成长 */}
          <h2 style={H2}>这一阶段的使用与成长</h2>
          <p style={{ fontSize: 14, lineHeight: 1.8, color: "#5A6377", margin: "0 0 14px" }}>这段时间，孩子在思维印记上的使用情况，以及思考维度的变化。</p>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 10 }}>
            {data.stats.map((s) => (
              <div key={s.label} style={{ border: "1px solid #E4E7EF", borderRadius: 12, padding: "14px 12px", textAlign: "center" }}>
                <div style={{ fontSize: 24, fontWeight: 900, color: "#2A3B7A" }}>{s.value}</div>
                <div style={{ fontSize: 12, color: "#6C7488", marginTop: 4 }}>{s.label}</div>
              </div>
            ))}
          </div>
          {data.stageGrowth ? (
            <div style={{ ...CARD, marginTop: 16 }}>
              <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333" }}>这段时间的变化</div>
              <div style={{ marginTop: 9, fontSize: 14, lineHeight: 1.8, color: "#3A4256" }}>{data.stageGrowth}</div>
            </div>
          ) : null}
          {data.stageHighlight ? (
            <div style={{ marginTop: 12, border: "1px solid #DFEDE7", background: "linear-gradient(180deg,#F3F9F6,#fff)", borderRadius: 14, padding: "16px 18px" }}>
              <div style={{ fontSize: 14, fontWeight: 800, color: "#3E8A6E" }}>本阶段亮点</div>
              <div style={{ marginTop: 8, fontSize: 14, lineHeight: 1.8, color: "#2A3040" }}>{data.stageHighlight}</div>
            </div>
          ) : null}
          {data.stageForward ? (
            <div style={{ marginTop: 12, fontSize: 14, lineHeight: 1.85, color: "#3A4256" }}>
              <b style={{ color: "#20263A" }}>往前看：</b>{data.stageForward}
            </div>
          ) : null}
        </ParentReportChrome>
      )}
    </div>
  );
}
