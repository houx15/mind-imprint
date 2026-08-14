import type { AutonomyDimResult, DepthDimResult } from "@mind-imprint/contracts";
import { AutonomySignals, DepthDims } from "@mind-imprint/contracts";
import { autonomyTier, depthTier, type AxisTier } from "../EvaluationReport/tokens";
import { EvidenceList, PrintSection } from "./parts";

// One dimension block for the print doc: name + colored verdict pill, then three
// labeled parts — 这一维看的是 (definition) · 你的表现 (behaviour + evidence) · 建议.
// Position is shown by SEMANTIC COLOR + a plain word only — never a level digit,
// never an L1–L4 ladder or 0–5 band strip.
function DimBlock({
  code,
  name,
  means,
  tier,
  summary,
  evidence,
  suggestion,
  reflectionRule,
}: {
  code: string;
  name: string;
  means: string;
  tier: AxisTier;
  summary: string;
  evidence: DepthDimResult["evidence"];
  suggestion: string;
  reflectionRule?: string;
}) {
  return (
    <div className="print-avoid-break" style={{ marginBottom: 20, borderTop: "1px solid var(--mk-border)", paddingTop: 14 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 9 }}>
        <span data-color-chip={code} aria-hidden style={{ width: 6, alignSelf: "stretch", minHeight: 18, borderRadius: 3, background: tier.bar, flexShrink: 0 }} />
        <span style={{ fontSize: 12, fontWeight: 800, color: "var(--mk-secondary)" }}>{code}</span>
        <span style={{ fontSize: 15, fontWeight: 700, color: "var(--mk-ink)" }}>{name}</span>
        <span
          style={{ marginLeft: "auto", fontSize: 11.5, fontWeight: 700, background: tier.bg, color: tier.fg, borderRadius: 999, padding: "2px 10px" }}
        >
          {tier.word}
        </span>
      </div>

      {means ? (
        <p style={{ marginTop: 6, fontSize: 11.5, color: "var(--mk-muted)", lineHeight: 1.6 }}>
          <b style={{ color: "var(--mk-secondary)" }}>这一维看的是 · </b>{means}
        </p>
      ) : null}
      {reflectionRule ? (
        <div style={{ marginTop: 4, fontSize: 11, color: "var(--mk-muted)", fontStyle: "italic" }}>{reflectionRule}</div>
      ) : null}

      <div style={{ marginTop: 10, fontSize: 11.5, fontWeight: 700, color: "var(--mk-secondary)" }}>你的表现</div>
      <p style={{ marginTop: 3, fontSize: 12.5, color: "var(--mk-ink)", lineHeight: 1.7 }}>{summary}</p>
      <EvidenceList evidence={evidence} />

      {suggestion ? (
        <div style={{ marginTop: 12, background: "var(--mk-butter-bg)", borderRadius: 6, padding: "9px 11px", fontSize: 12, color: "var(--mk-ink)" }}>
          <b style={{ color: "var(--mk-butter-fg)" }}>建议 · </b>{suggestion}
        </div>
      ) : null}
    </div>
  );
}

function DepthDimBlock({ d }: { d: DepthDimResult }) {
  const model = DepthDims().find((x) => x.id === d.id);
  return (
    <DimBlock
      code={d.id}
      name={model?.name ?? d.id}
      means={model?.means ?? ""}
      tier={depthTier(d.level)}
      summary={d.summary}
      evidence={d.evidence}
      suggestion={d.suggestion}
      reflectionRule={d.id === "D6" ? model?.reflectionRule : undefined}
    />
  );
}

export function DepthSection({ dims }: { dims: DepthDimResult[] }) {
  return (
    <PrintSection
      n="05"
      title="认知深度 D · 想得有多深"
      breakBefore
      intro="认知深度看学生把思考推进到多深。每个维度分三块读：这一维看的是（定义）、你的表现（做了什么，配原话证据）、建议（下一步）。所处位置只用颜色示意——橙/黄提示需加强，浅绿/深绿表示做得好——不打分、不排名、不与其它轴合成总分。"
    >
      {dims.map((d) => <DepthDimBlock key={d.id} d={d} />)}
    </PrintSection>
  );
}

function AutonomyDimBlock({ d }: { d: AutonomyDimResult }) {
  const model = AutonomySignals().find((x) => x.id === d.id);
  return (
    <DimBlock
      code={d.id}
      name={model?.name ?? d.id}
      means={model?.means ?? ""}
      tier={autonomyTier(d.band)}
      summary={d.summary}
      evidence={d.evidence}
      suggestion={d.suggestion}
    />
  );
}

export function AutonomySection({ dims }: { dims: AutonomyDimResult[] }) {
  return (
    <PrintSection
      n="06"
      title="智识自主 A · 是否自己驱动认知"
      breakBefore
      intro="智识自主看学生是否自己驱动认知——方向、发起、边界、检验、署名、求真这几件事上，是她带着走还是被 AI 带着走。读法与上一节相同：定义 / 你的表现 / 建议。位置只用颜色示意（橙/黄=偏依赖，绿=较自主），不打分。"
    >
      {dims.map((d) => <AutonomyDimBlock key={d.id} d={d} />)}
    </PrintSection>
  );
}
