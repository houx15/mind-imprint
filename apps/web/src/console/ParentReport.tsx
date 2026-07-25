import { useEffect, useState } from "react";
import type { ParentReport as ParentReportDTO } from "@mind-imprint/contracts";
import { api, ApiError } from "../api";
import { D_BADGE_COLOR, A_STATE_COLOR } from "./parentReportContent";
import { ParentReportChrome } from "./ParentReportChrome";

// Full-screen printable overlay reproducing the binding design's 项目模式
// layout (docs/design/teacher end/project/家长报告.dc.html). Static
// front-matter/chrome lives in ParentReportChrome.tsx; this component
// supplies only the project-specific middle (glance + D rows + A rows +
// opportunity) — badge/state colors are looked up ONLY from the label maps,
// never recomputed client-side, and no A-axis number is ever rendered (RL-5).

const H2: React.CSSProperties = { fontSize: 19, fontWeight: 800, color: "#1C2333", margin: "30px 0 10px" };

const D_BADGE_FALLBACK = { color: "#8A92A3", bg: "#F1F2F6" };
const A_STATE_FALLBACK = { color: "#C4574D", bg: "#F7E6E4" };

function DBadge({ badge }: { badge: string }) {
  const c = D_BADGE_COLOR[badge] ?? D_BADGE_FALLBACK;
  return (
    <span style={{ flex: "none", padding: "4px 12px", borderRadius: 20, background: c.bg, color: c.color, fontSize: 12.5, fontWeight: 800 }}>
      {badge}
    </span>
  );
}

function AState({ state }: { state: string }) {
  const c = A_STATE_COLOR[state] ?? A_STATE_FALLBACK;
  return (
    <span style={{ flex: "none", padding: "4px 12px", borderRadius: 20, background: c.bg, color: c.color, fontSize: 12, fontWeight: 800 }}>
      {state}
    </span>
  );
}

export function ParentReport({
  classId,
  studentId,
  surface,
  scopeId,
  studentName,
  onClose,
}: {
  classId: string;
  studentId: string;
  surface: string;
  scopeId: string;
  studentName?: string;
  onClose: () => void;
}) {
  const [data, setData] = useState<ParentReportDTO | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);

  useEffect(() => {
    setError(null);
    api.getParentReport(classId, studentId, surface, scopeId).then(setData).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }, [classId, studentId, surface, scopeId]);

  async function handleGenerate() {
    setGenerating(true);
    try {
      const next = await api.generateParentReportProse(classId, studentId, surface, scopeId);
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
          {/* {name} 这次的表现 */}
          <h2 style={H2}>{data.cover.name} 这次的表现</h2>
          {data.glance ? (
            <div style={{ border: "1px solid #E4E7EF", background: "#F7F8FB", borderRadius: 14, padding: "16px 18px" }}>
              <div style={{ fontSize: 13, fontWeight: 800, color: "#2A3B7A" }}>一眼判断</div>
              <div style={{ marginTop: 7, fontSize: 14.5, lineHeight: 1.75, color: "#20263A", fontWeight: 600 }}>{data.glance}</div>
            </div>
          ) : null}

          <div style={{ fontSize: 16, fontWeight: 800, color: "#2A3B7A", margin: "22px 0 4px" }}>D 轴 · 认知深度（逐维）</div>
          {data.dOverview ? <div style={{ fontSize: 13, color: "#7A8296", marginBottom: 12 }}>{data.dOverview}</div> : null}
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {data.dRows.map((r) => (
              <div key={r.code} style={{ border: "1px solid #E4E7EF", borderRadius: 12, padding: "14px 16px" }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                  <div style={{ fontSize: 14.5, fontWeight: 800, color: "#1C2333" }}>{r.name}</div>
                  <DBadge badge={r.badge} />
                </div>
                {r.reading ? <div style={{ marginTop: 9, fontSize: 13.5, lineHeight: 1.75, color: "#3A4256" }}>{r.reading}</div> : null}
              </div>
            ))}
          </div>

          <div style={{ fontSize: 16, fontWeight: 800, color: "#3E8A6E", margin: "24px 0 4px" }}>A 轴 · 智识自主（信号，不打分）</div>
          {data.aOverview ? <div style={{ fontSize: 13, color: "#7A8296", marginBottom: 12 }}>{data.aOverview}</div> : null}
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {data.aRows.map((r) => (
              <div key={r.code} style={{ border: "1px solid #E4E7EF", borderRadius: 12, padding: "14px 16px" }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                  <div style={{ fontSize: 14.5, fontWeight: 800, color: "#1C2333" }}>{r.name}</div>
                  <AState state={r.state} />
                </div>
                {r.reading ? <div style={{ marginTop: 9, fontSize: 13.5, lineHeight: 1.75, color: "#3A4256" }}>{r.reading}</div> : null}
              </div>
            ))}
          </div>

          {data.opportunity ? (
            <div style={{ border: "1px solid #E4E7EF", borderRadius: 12, padding: "14px 16px", marginTop: 16 }}>
              <div style={{ fontSize: 13, fontWeight: 800, color: "#B0863A" }}>关于"机会"与真实性</div>
              <div style={{ marginTop: 7, fontSize: 13.5, lineHeight: 1.75, color: "#3A4256" }}>{data.opportunity}</div>
            </div>
          ) : null}
        </ParentReportChrome>
      )}
    </div>
  );
}
