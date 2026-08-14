import type { DepthDimResult } from "@mind-imprint/contracts";
import { DepthDims } from "@mind-imprint/contracts";
import { DEPTH_RAMP } from "../EvaluationReport/tokens";
import { EvidenceList, PrintSection } from "./parts";

const RUNGS = ["L1", "L2", "L3", "L4"] as const;

function DepthDimBlock({ d }: { d: DepthDimResult }) {
  const model = DepthDims().find((x) => x.id === d.id);
  const anchors = model?.anchors;
  // Position by COLOR ONLY — never print d.level, never mark a "current" rung.
  const chip = DEPTH_RAMP[d.level - 1] ?? DEPTH_RAMP[DEPTH_RAMP.length - 1];
  return (
    <div className="print-avoid-break" style={{ marginBottom: 20, borderTop: "1px solid var(--mk-border)", paddingTop: 14 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 9 }}>
        <span
          data-color-chip={d.id}
          aria-hidden
          style={{ width: 16, height: 16, borderRadius: 4, background: chip, flexShrink: 0 }}
        />
        <span style={{ fontSize: 12, fontWeight: 800, color: "var(--mk-secondary)" }}>{d.id}</span>
        <span style={{ fontSize: 15, fontWeight: 700, color: "var(--mk-ink)" }}>{model?.name ?? d.id}</span>
      </div>
      {model?.means ? (
        <div style={{ marginTop: 3, fontSize: 11.5, color: "var(--mk-muted)" }}>{model.means}</div>
      ) : null}

      {anchors ? (
        <div style={{ marginTop: 10, display: "flex", flexDirection: "column", gap: 6 }}>
          {RUNGS.map((r, i) => (
            <div key={r} style={{ display: "flex", gap: 8, alignItems: "flex-start" }}>
              <span
                aria-hidden
                style={{ width: 4, alignSelf: "stretch", borderRadius: 2, background: DEPTH_RAMP[i], flexShrink: 0 }}
              />
              <span style={{ fontSize: 11, fontWeight: 700, color: "var(--mk-secondary)", width: 22, flexShrink: 0 }}>{r}</span>
              <span style={{ fontSize: 11.5, color: "var(--mk-secondary)", lineHeight: 1.55 }}>{anchors[r]}</span>
            </div>
          ))}
        </div>
      ) : null}
      {d.id === "D6" && model?.reflectionRule ? (
        <div style={{ marginTop: 8, fontSize: 11, color: "var(--mk-muted)", fontStyle: "italic" }}>{model.reflectionRule}</div>
      ) : null}

      <p style={{ marginTop: 12, fontSize: 12.5, color: "var(--mk-ink)", lineHeight: 1.7 }}>{d.summary}</p>
      <EvidenceList evidence={d.evidence} />
      {d.suggestion ? (
        <div style={{ marginTop: 12, background: "var(--mk-butter-bg)", borderRadius: 6, padding: "9px 11px", fontSize: 12, color: "var(--mk-ink)" }}>
          <b style={{ color: "var(--mk-butter-fg)" }}>建议：</b>{d.suggestion}
        </div>
      ) : null}
    </div>
  );
}

export function DepthSection({ dims }: { dims: DepthDimResult[] }) {
  return (
    <PrintSection
      n="05"
      title="认知深度 D · 想得有多深"
      breakBefore
      intro="认知深度看学生把思考推进到多深。每个维度给出 L1→L4 四级参照锚点——由浅到深的定性描述，不是 1–4 打分，也不与其它轴合成总分。学生所处的位置只用颜色深浅示意，本版本不标注具体档位。"
    >
      {dims.map((d) => <DepthDimBlock key={d.id} d={d} />)}
    </PrintSection>
  );
}
