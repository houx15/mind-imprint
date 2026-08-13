import type { EvaluationReport } from "@mind-imprint/contracts";
import { Card } from "@/ui";

export interface HeaderProps {
  basics: EvaluationReport["basics"];
  title: string;
}

/**
 * STUB (Task 8) — real build is Task 9: title (`mk-h1`), type chip + date
 * range, 5-step milestone stepper, 5 counter tiles. Ports `.rpt-head` /
 * `.steps` / `.counters` from the mockup.
 */
export function Header({ basics, title }: HeaderProps) {
  return (
    <Card className="p-7" data-testid="header-section">
      <div className="text-mk-h1 text-mk-ink">{title}</div>
      <div className="mt-2.5 flex flex-wrap items-center gap-2.5 text-mk-small text-mk-muted">
        <span className="rounded-mk-full bg-mk-mist-bg px-2.5 py-1 text-mk-mist-fg">{basics.type}</span>
        <span>
          {basics.startDate.slice(0, 10)} → {basics.endDate ? basics.endDate.slice(0, 10) : "进行中"}
        </span>
      </div>
      <div className="mt-4 text-mk-body text-mk-secondary">{basics.counters.aiTurns} 次 AI 对话轮次</div>
    </Card>
  );
}
