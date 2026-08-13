import type { EvaluationReport } from "@mind-imprint/contracts";
import { Card } from "@/ui";
import { EVENT_KIND } from "./tokens";

export interface TimelineProps {
  events: EvaluationReport["events"];
}

/**
 * STUB (Task 8) — real build is Task 9: vertical timeline, color-coded kind
 * dot (`EVENT_KIND[kind]`), `ts` formatting, `AI ×{aiTurns}` badge.
 */
export function Timeline({ events }: TimelineProps) {
  return (
    <Card className="p-6" data-testid="timeline-section">
      <ul className="flex flex-col gap-3">
        {events.map((ev, i) => {
          const kind = EVENT_KIND[ev.kind];
          return (
            <li key={i} className="flex items-start gap-3">
              <span
                aria-hidden
                className="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-mk-full"
                style={{ background: kind.dot }}
              />
              <div>
                <span
                  className="mr-2 rounded-mk-full px-2 py-0.5 text-mk-small font-bold"
                  style={{ color: kind.fg, background: kind.bg }}
                >
                  {kind.label}
                </span>
                <span className="text-mk-body text-mk-ink">{ev.summary}</span>
                <span className="ml-2 text-mk-small text-mk-muted">AI ×{ev.aiTurns}</span>
              </div>
            </li>
          );
        })}
      </ul>
    </Card>
  );
}
