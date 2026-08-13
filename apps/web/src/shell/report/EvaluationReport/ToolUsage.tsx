import type { EvaluationReport } from "@mind-imprint/contracts";
import { Card } from "@/ui";

export interface ToolUsageProps {
  toolUsage: EvaluationReport["toolUsage"];
}

/**
 * STUB (Task 10/11 fills in) — real build ports `.tools`/`.tool`: macaron
 * gradient icon tile, stage badge, purpose line, summary of whether the
 * card/subagent was taken up and what it changed downstream.
 */
export function ToolUsage({ toolUsage }: ToolUsageProps) {
  return (
    <div className="grid grid-cols-1 gap-3.5 md:grid-cols-2" data-testid="tool-usage-section">
      {toolUsage.map((t) => (
        <Card key={t.toolId} className="p-4">
          <div className="flex items-center gap-2">
            <span className="text-mk-h3 text-mk-ink">{t.name}</span>
            <span className="rounded-mk-full bg-mk-peach-bg px-2 py-0.5 text-mk-small font-bold text-mk-peach-fg">{t.stage}</span>
          </div>
          <p className="mt-1 text-mk-small text-mk-muted">{t.purpose}</p>
          <p className="mt-1.5 text-mk-body text-mk-secondary">{t.summary}</p>
        </Card>
      ))}
    </div>
  );
}
