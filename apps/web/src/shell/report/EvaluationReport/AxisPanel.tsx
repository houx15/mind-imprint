import type { AutonomyDimResult, DepthDimResult, EvidenceItem } from "@mind-imprint/contracts";
import { Surface } from "@/ui";
import { AUTONOMY_RAMP, DEPTH_RAMP } from "./tokens";

export type AxisPanelProps =
  | { axis: "depth"; dims: DepthDimResult[] }
  | { axis: "autonomy"; dims: AutonomyDimResult[] };

/** D1–D6 中文 dimension names, lifted verbatim from the mockup's `.dnm` spans. */
const D_NAMES: Record<DepthDimResult["id"], string> = {
  D1: "任务理解与问题表述",
  D2: "证据与信源",
  D3: "论证结构",
  D4: "视角、反方与限制",
  D5: "反馈处理与修订",
  D6: "反思与元认知",
};

/** A1–A6 中文 dimension names, lifted verbatim from the mockup's `.dnm` spans. */
const A_NAMES: Record<AutonomyDimResult["id"], string> = {
  A1: "方向自主",
  A2: "发起自主",
  A3: "边界主权",
  A4: "对抗与检验",
  A5: "判断署名",
  A6: "求真优先",
};

const AXIS_META = {
  depth: { title: "认知深度 D · 想得有多深", ramp: DEPTH_RAMP, lowLabel: "浅", highLabel: "深" },
  autonomy: { title: "智识自主 A · 是否自己驱动认知", ramp: AUTONOMY_RAMP, lowLabel: "低", highLabel: "高" },
} as const;

/**
 * AxisPanel — ports `.axis-legend`/`.dims`/`.dim` from
 * `docs/reference/2026-08-13-eval-report-mockup.html` for both the D
 * (depth) and A (autonomy) axes: a ramp legend, then a 2-column grid of
 * dimension cards, all expanded by default. Level/band is shown by COLOR
 * ONLY (ramp lookup) — never rendered as a digit.
 */
export function AxisPanel(props: AxisPanelProps) {
  const meta = AXIS_META[props.axis];
  return (
    <div data-testid={`axis-panel-${props.axis}`}>
      <div className="mb-1 text-mk-h2 text-mk-ink">{meta.title}</div>
      <RampLegend ramp={meta.ramp} lowLabel={meta.lowLabel} highLabel={meta.highLabel} />
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {props.axis === "depth"
          ? props.dims.map((d) => (
              <AxisDimCard
                key={d.id}
                code={d.id}
                name={D_NAMES[d.id]}
                color={DEPTH_RAMP[d.level - 1] ?? DEPTH_RAMP[DEPTH_RAMP.length - 1]!}
                summary={d.summary}
                evidence={d.evidence}
                suggestion={d.suggestion}
              />
            ))
          : props.dims.map((d) => (
              <AxisDimCard
                key={d.id}
                code={d.id}
                name={A_NAMES[d.id]}
                color={AUTONOMY_RAMP[d.band] ?? AUTONOMY_RAMP[AUTONOMY_RAMP.length - 1]!}
                summary={d.summary}
                evidence={d.evidence}
                suggestion={d.suggestion}
              />
            ))}
      </div>
    </div>
  );
}

function RampLegend({ ramp, lowLabel, highLabel }: { ramp: readonly string[]; lowLabel: string; highLabel: string }) {
  return (
    <div className="mb-4 flex items-center gap-2 text-mk-small text-mk-muted">
      <span>{lowLabel}</span>
      <span aria-hidden className="flex overflow-hidden rounded-mk-xs">
        {ramp.map((color, i) => (
          <span key={i} className="h-2.5 w-4" style={{ background: color }} />
        ))}
      </span>
      <span>{highLabel}</span>
    </div>
  );
}

interface AxisDimCardProps {
  code: string;
  name: string;
  color: string;
  summary: string;
  evidence: EvidenceItem[];
  suggestion: string;
}

function AxisDimCard({ code, name, color, summary, evidence, suggestion }: AxisDimCardProps) {
  return (
    <Surface level="md" radius="md" className="overflow-hidden p-0" data-testid={`axis-dim-${code}`}>
      <div className="flex items-stretch">
        <span aria-hidden className="w-2 shrink-0" style={{ background: color }} data-dim-bar={code} />
        <div className="min-w-0 flex-1 p-4">
          <div className="flex items-center gap-2.5">
            <span className="rounded-mk-xs px-2 py-0.5 text-mk-small font-extrabold text-white" style={{ background: color }}>
              {code}
            </span>
            <span className="text-mk-h3 text-mk-ink">{name}</span>
          </div>
          <p className="mt-2 text-mk-body text-mk-secondary">{summary}</p>

          <ul className="mt-3.5 flex flex-col gap-3.5 border-t border-dashed border-mk-input-border pt-3.5">
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

          <div className="mt-3.5 rounded-mk-sm bg-mk-butter-bg p-3 text-mk-body text-mk-ink">
            <b className="text-mk-butter-fg">建议：</b>
            {suggestion}
          </div>
        </div>
      </div>
    </Surface>
  );
}
