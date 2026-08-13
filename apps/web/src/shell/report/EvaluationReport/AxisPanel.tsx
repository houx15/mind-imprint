import type { AutonomyDimResult, DepthDimResult } from "@mind-imprint/contracts";
import { Card } from "@/ui";
import { AUTONOMY_RAMP, DEPTH_RAMP } from "./tokens";

export type AxisPanelProps =
  | { axis: "depth"; dims: DepthDimResult[] }
  | { axis: "autonomy"; dims: AutonomyDimResult[] };

/**
 * STUB (Task 10/11 fills in) — real build ports `.dims`/`.dim`: D1–D6 /
 * A1–A6 cards with 中文 dimension names, full evidence lists + boundary +
 * suggestion. Level/band shown by COLOR ONLY (ramp lookup), never a digit.
 */
export function AxisPanel(props: AxisPanelProps) {
  if (props.axis === "depth") return <AxisPanelDepth dims={props.dims} />;
  return <AxisPanelAutonomy dims={props.dims} />;
}

function AxisPanelDepth({ dims }: { dims: DepthDimResult[] }) {
  return (
    <div data-testid="axis-panel-depth">
      <div className="mb-4 text-mk-h2 text-mk-ink">认知深度 D · 想得有多深</div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {dims.map((d) => (
          <AxisDimCard
            key={d.id}
            id={d.id}
            color={DEPTH_RAMP[d.level - 1] ?? DEPTH_RAMP[3]}
            summary={d.summary}
            quote={d.evidence[0]?.quote}
          />
        ))}
      </div>
    </div>
  );
}

function AxisPanelAutonomy({ dims }: { dims: AutonomyDimResult[] }) {
  return (
    <div data-testid="axis-panel-autonomy">
      <div className="mb-4 text-mk-h2 text-mk-ink">智识自主 A · 是否自己驱动认知</div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {dims.map((d) => (
          <AxisDimCard
            key={d.id}
            id={d.id}
            color={AUTONOMY_RAMP[d.band] ?? AUTONOMY_RAMP[5]}
            summary={d.summary}
            quote={d.evidence[0]?.quote}
          />
        ))}
      </div>
    </div>
  );
}

function AxisDimCard({ id, color, summary, quote }: { id: string; color: string; summary: string; quote?: string }) {
  return (
    <Card className="overflow-hidden p-0">
      <div className="flex items-stretch">
        <span aria-hidden className="w-2 shrink-0" style={{ background: color }} />
        <div className="flex-1 p-4">
          <div className="flex items-center gap-2">
            <span className="rounded-mk-xs px-2 py-0.5 text-mk-small font-extrabold text-white" style={{ background: color }}>
              {id}
            </span>
            <span className="text-mk-h3 text-mk-ink">{summary}</span>
          </div>
          {quote ? (
            <blockquote className="mt-2 border-l-2 border-mk-lake pl-2.5 text-mk-body text-mk-ink">{quote}</blockquote>
          ) : null}
        </div>
      </div>
    </Card>
  );
}
