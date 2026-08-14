import type { AutonomyDimResult, DepthDimResult, EvidenceItem } from "@mind-imprint/contracts";
import { AutonomySignals, DepthDims } from "@mind-imprint/contracts";
import { Surface } from "@/ui";
import { autonomyTier, depthTier, type AxisTier } from "./tokens";

export type AxisPanelProps =
  | { axis: "depth"; dims: DepthDimResult[] }
  | { axis: "autonomy"; dims: AutonomyDimResult[] };

const AXIS_META = {
  depth: { title: "认知深度 D · 想得有多深" },
  autonomy: { title: "智识自主 A · 是否自己驱动认知" },
} as const;

/**
 * AxisPanel — the D (depth) / A (autonomy) axis section. Each dimension is one
 * card with four clearly-labeled parts: 这一维看的是 (definition, static from the
 * rubric) · a colored verdict pill (需加深…扎实) · 你的表现 (behaviour + evidence)
 * · 建议 (suggestion). Position is shown by SEMANTIC COLOR + a plain word only —
 * no level digit, no L1–L4 ladder, no ramp legend.
 */
export function AxisPanel(props: AxisPanelProps) {
  const meta = AXIS_META[props.axis];
  return (
    <div data-testid={`axis-panel-${props.axis}`}>
      <div className="mb-1 text-mk-h2 text-mk-ink">{meta.title}</div>
      <p className="mb-4 text-mk-small text-mk-muted">
        颜色表示所处位置：<b style={{ color: "#AE531B" }}>橙</b>/<b style={{ color: "#856110" }}>黄</b>提示需加强，
        <b style={{ color: "#477433" }}>浅绿</b>/<b style={{ color: "#1E6A40" }}>深绿</b>表示做得好——不打分、不排名。
      </p>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {props.axis === "depth"
          ? props.dims.map((d) => (
              <AxisDimCard
                key={d.id}
                code={d.id}
                name={DepthDims().find((x) => x.id === d.id)?.name ?? d.id}
                means={DepthDims().find((x) => x.id === d.id)?.means ?? ""}
                tier={depthTier(d.level)}
                summary={d.summary}
                evidence={d.evidence}
                suggestion={d.suggestion}
              />
            ))
          : props.dims.map((d) => (
              <AxisDimCard
                key={d.id}
                code={d.id}
                name={AutonomySignals().find((x) => x.id === d.id)?.name ?? d.id}
                means={AutonomySignals().find((x) => x.id === d.id)?.means ?? ""}
                tier={autonomyTier(d.band)}
                summary={d.summary}
                evidence={d.evidence}
                suggestion={d.suggestion}
              />
            ))}
      </div>
    </div>
  );
}

interface AxisDimCardProps {
  code: string;
  name: string;
  means: string;
  tier: AxisTier;
  summary: string;
  evidence: EvidenceItem[];
  suggestion: string;
}

function AxisDimCard({ code, name, means, tier, summary, evidence, suggestion }: AxisDimCardProps) {
  return (
    <Surface level="md" radius="md" className="overflow-hidden p-0" data-testid={`axis-dim-${code}`}>
      <div className="flex items-stretch">
        <span aria-hidden className="w-2 shrink-0" style={{ background: tier.bar }} data-dim-bar={code} />
        <div className="min-w-0 flex-1 p-4">
          {/* Header: dimension name + colored verdict pill (word + color = the position). */}
          <div className="flex flex-wrap items-center gap-2.5">
            <span className="text-mk-small font-extrabold text-mk-secondary">{code}</span>
            <span className="text-mk-h3 text-mk-ink">{name}</span>
            <span
              className="ml-auto rounded-mk-full px-2.5 py-0.5 text-mk-small font-bold"
              style={{ background: tier.bg, color: tier.fg }}
              data-dim-verdict={code}
            >
              {tier.word}
            </span>
          </div>

          {/* 这一维看的是 — the definition (static rubric text). */}
          {means ? (
            <p className="mt-2 text-mk-small text-mk-muted">
              <b className="text-mk-secondary">这一维看的是 · </b>
              {means}
            </p>
          ) : null}

          {/* 你的表现 — what the student did, plus the verbatim evidence. */}
          <div className="mt-3 border-t border-dashed border-mk-input-border pt-3">
            <div className="text-mk-small font-bold text-mk-secondary">你的表现</div>
            <p className="mt-1 text-mk-body text-mk-secondary">{summary}</p>
            <ul className="mt-2.5 flex flex-col gap-3">
              {evidence.map((ev, i) => (
                <li key={i}>
                  <blockquote className="border-l-2 border-mk-lake pl-2.5 text-mk-body text-mk-ink">{ev.quote}</blockquote>
                  <div className="mt-1.5 text-mk-small text-mk-muted">
                    {ev.stage ? `${ev.stage} · ` : null}
                    <a href={`#${ev.id}`} className="border-b border-dashed border-mk-info text-mk-info no-underline">
                      {ev.id}
                    </a>
                    {ev.observation ? ` — ${ev.observation}` : null}
                  </div>
                  {ev.boundary ? <div className="mt-1 text-mk-small text-mk-danger">边界：{ev.boundary}</div> : null}
                </li>
              ))}
            </ul>
          </div>

          {/* 建议 — the next step. */}
          <div className="mt-3.5 rounded-mk-sm bg-mk-butter-bg p-3 text-mk-body text-mk-ink">
            <b className="text-mk-butter-fg">建议 · </b>
            {suggestion}
          </div>
        </div>
      </div>
    </Surface>
  );
}
