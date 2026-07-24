import { useState } from "react";
import type { DualAxisReport } from "@mind-imprint/contracts";
import { badgeColor } from "./badgeColor";

// 证据地图 (Task 11) — a PURE client-side projection of the report object
// already fetched by TeacherReportView. No network calls, no effects that
// load data: every node's label/note/detail is derived synchronously from
// `report` + `context` on each render.
//
// Node layout, colors and edge topology mirror the binding design verbatim
// (docs/design/teacher end/project/思维印记 教师端.dc.html:1613-1628). The one
// deliberate deviation (confirmed by the D1 plan's self-review, task-11
// brief): core-only reports (no officialProjection — course/chat surfaces)
// render the map minus the `official` node and its two edges, rather than
// suppressing the whole map per spec §5.5's default.

type NodeId = "center" | "rq" | "official" | "dAxis" | "aAxis" | "prompt" | "ai";

interface MapNodeDef {
  id: NodeId;
  x: number;
  y: number;
  w: number;
  bg: string;
  border: string;
  label: string;
  note: string;
  detail: string;
  levelBadge?: string; // raw text fed to badgeColor() for a small accent chip in the detail panel
}

function depthSummary(depthAxis: DualAxisReport["depthAxis"]): { note: string; detail: string; badge?: string } {
  const levels = depthAxis
    .filter((d) => d.level !== "NA")
    .map((d) => Number(d.level.slice(1)));
  if (levels.length === 0) {
    return { note: "暂无可计入的证据", detail: `${depthAxis.length} 个 D 轴维度目前都没有可计入的证据。` };
  }
  const min = Math.min(...levels);
  const max = Math.max(...levels);
  const range = min === max ? `L${min}` : `L${min}–L${max}`;
  return {
    note: `深度区间 ${range}`,
    detail: `${depthAxis.length} 个 D 轴维度中，认知深度证据落在 ${range} 区间。`,
    badge: `L${max}`,
  };
}

function autonomySummary(autonomyAxis: DualAxisReport["autonomyAxis"]): { note: string; detail: string; badge?: string } {
  const supplied = autonomyAxis.filter((a) => a.opportunity !== "not_supplied");
  if (supplied.length === 0) {
    return { note: "机会未提供，暂无可计入证据", detail: "所有 A 轴维度本轮都未被给到自主机会，暂无可计入的证据。" };
  }
  const mean = supplied.reduce((sum, a) => sum + a.level, 0) / supplied.length;
  const rounded = Math.round(mean * 10) / 10;
  return {
    note: `自主均值 ${rounded.toFixed(1)}`,
    detail: `${supplied.length} 个已提供机会的 A 轴维度中，智识自主均值为 ${rounded.toFixed(1)}（满分 5）。`,
    badge: `L${Math.round(rounded)}`,
  };
}

function lensSummary(promptLens: DualAxisReport["promptLens"]): { note: string; detail: string; badge?: string } {
  const lenses = promptLens.lenses;
  if (lenses.length === 0) {
    return { note: "暂无提示词透镜数据", detail: "本轮没有可读出的提示词透镜维度。" };
  }
  const mean = lenses.reduce((sum, l) => sum + l.level, 0) / lenses.length;
  const rounded = Math.round(mean * 10) / 10;
  return {
    note: `透镜均值 ${rounded.toFixed(1)}`,
    detail: `${lenses.length} 个提示词透镜维度均值为 ${rounded.toFixed(1)}（满分 5）。`,
    badge: `L${Math.round(rounded)}`,
  };
}

function readinessSummary(officialProjection: NonNullable<DualAxisReport["officialProjection"]>): { note: string; detail: string } {
  return {
    note: `就绪度 ${officialProjection.readiness.score}`,
    detail: `${officialProjection.readiness.score} 分 · ${officialProjection.readiness.note}`,
  };
}

// Conditional truncation mirroring the binding design's `short(t,n)` helper
// (docs/design/teacher end/project/思维印记 教师端.dc.html:1518): only append
// the ellipsis when the text actually exceeds the cap.
function short(text: string, n: number): string {
  return text.length > n ? `${text.slice(0, n)}…` : text;
}

function aiSummary(interactionEvidence: DualAxisReport["interactionEvidence"]): { note: string; detail: string } {
  const n = interactionEvidence.length;
  if (n === 0) {
    return { note: "0 轮交互", detail: "本次尚无可读出的交互信号。" };
  }
  return {
    note: `${n} 轮交互`,
    detail: interactionEvidence.map((r) => r.signal).join("　/　"),
  };
}

function buildNodes(
  report: DualAxisReport,
  context: { projectTitle?: string; researchQuestion?: string },
  studentName?: string,
): MapNodeDef[] {
  const d = depthSummary(report.depthAxis);
  const a = autonomySummary(report.autonomyAxis);
  const p = lensSummary(report.promptLens);
  const ai = aiSummary(report.interactionEvidence);

  const nodes: MapNodeDef[] = [
    {
      id: "center",
      x: 50,
      y: 44,
      w: 150,
      bg: "#EDEFF9",
      border: "#2A3B7A",
      label: `${studentName ?? "学生"} · 能力报告`,
      note: "点击节点看证据",
      detail: report.narrative || "本次会话暂无叙述。",
    },
    {
      id: "rq",
      x: 50,
      y: 13,
      w: 170,
      bg: "#FBEFD8",
      border: "#C68A3A",
      label: "Research Question",
      note: context.researchQuestion ? short(context.researchQuestion, 24) : "暂无研究问题",
      detail: context.researchQuestion || "本报告未提供 Research Question。",
    },
  ];

  if (report.officialProjection) {
    const r = readinessSummary(report.officialProjection);
    nodes.push({
      id: "official",
      x: 83,
      y: 26,
      w: 140,
      bg: "#E1EDF5",
      border: "#3E7CA8",
      label: "官方作品投影",
      note: r.note,
      detail: r.detail,
    });
  }

  nodes.push(
    {
      id: "dAxis",
      x: 85,
      y: 60,
      w: 130,
      bg: "#E7E1F0",
      border: "#6C5A94",
      label: "D 轴 · 认知深度",
      note: d.note,
      detail: d.detail,
      levelBadge: d.badge,
    },
    {
      id: "aAxis",
      x: 50,
      y: 82,
      w: 130,
      bg: "#E4F0EA",
      border: "#3E8A6E",
      label: "A 轴 · 智识自主",
      note: a.note,
      detail: a.detail,
      levelBadge: a.badge,
    },
    {
      id: "prompt",
      x: 16,
      y: 60,
      w: 140,
      bg: "#EEF0F4",
      border: "#7A8296",
      label: "提示词透镜",
      note: p.note,
      detail: p.detail,
      levelBadge: p.badge,
    },
    {
      id: "ai",
      x: 16,
      y: 26,
      w: 140,
      bg: "#F7E6E4",
      border: "#C4574D",
      label: "AI 互动证据",
      note: ai.note,
      detail: ai.detail,
    },
  );

  return nodes;
}

const ALL_EDGES: [NodeId, NodeId][] = [
  ["rq", "center"],
  ["center", "official"],
  ["official", "dAxis"],
  ["dAxis", "aAxis"],
  ["aAxis", "prompt"],
  ["prompt", "ai"],
  ["ai", "center"],
  ["rq", "official"],
];

export function EvidenceMap({
  report,
  context,
  studentName,
}: {
  report: DualAxisReport;
  context: { projectTitle?: string; researchQuestion?: string };
  studentName?: string;
}) {
  const [selected, setSelected] = useState<NodeId>("center");

  const nodes = buildNodes(report, context, studentName);
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const edges = ALL_EDGES.filter(([a, b]) => byId.has(a) && byId.has(b));

  // `nodes` always includes at least `center` (built unconditionally in
  // buildNodes), so this fallback is unreachable in practice; the `!`
  // satisfies noUncheckedIndexedAccess.
  const active = byId.get(selected) ?? nodes[0]!;

  return (
    <div>
      <div
        style={{
          position: "relative",
          height: 460,
          background:
            "linear-gradient(#F4F5F9 1px,transparent 1px),linear-gradient(90deg,#F4F5F9 1px,transparent 1px),#FCFCFE",
          backgroundSize: "26px 26px",
          border: "1px solid #EAECF2",
          borderRadius: 16,
          overflow: "hidden",
        }}
      >
        <svg
          viewBox="0 0 100 100"
          preserveAspectRatio="none"
          style={{ position: "absolute", inset: 0, width: "100%", height: "100%", pointerEvents: "none" }}
        >
          {edges.map(([a, b]) => {
            const A = byId.get(a)!;
            const B = byId.get(b)!;
            return <line key={`${a}-${b}`} x1={A.x} y1={A.y} x2={B.x} y2={B.y} stroke="#CBD2E0" strokeWidth={0.35} />;
          })}
        </svg>
        {nodes.map((n) => {
          const isActive = n.id === selected;
          return (
            <div
              key={n.id}
              role="button"
              data-testid={`evidence-map-node-${n.id}`}
              onClick={() => setSelected(n.id)}
              style={{
                position: "absolute",
                left: `${n.x}%`,
                top: `${n.y}%`,
                transform: "translate(-50%,-50%)",
                width: n.w,
                padding: "10px 12px",
                border: `2px solid ${isActive ? "#2A3B7A" : n.border}`,
                borderRadius: 12,
                background: n.bg,
                boxShadow: isActive ? "0 12px 26px rgba(42,59,122,.22)" : "0 5px 14px rgba(20,30,60,.08)",
                cursor: "pointer",
                textAlign: "center",
              }}
            >
              <div style={{ fontSize: 12.5, fontWeight: 800, color: "#1C2333" }}>{n.label}</div>
              <div style={{ fontSize: 10.5, color: "#6C7488", marginTop: 3, lineHeight: 1.3 }}>{n.note}</div>
            </div>
          );
        })}
      </div>
      <div
        data-testid="evidence-map-detail"
        style={{ marginTop: 14, background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "16px 18px" }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <div style={{ fontSize: 13, fontWeight: 800, color: "#2A3B7A" }}>{active.label}</div>
          {active.levelBadge
            ? (() => {
                const c = badgeColor(active.levelBadge!);
                return (
                  <span style={{ fontSize: 11, fontWeight: 800, color: c.fg, background: c.bg, padding: "1px 8px", borderRadius: 8 }}>
                    {active.levelBadge}
                  </span>
                );
              })()
            : null}
        </div>
        <div style={{ marginTop: 8, fontSize: 13, color: "#4A5060", lineHeight: 1.75 }}>{active.detail}</div>
      </div>
    </div>
  );
}
