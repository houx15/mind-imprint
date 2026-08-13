import type { EvaluationReport } from "@mind-imprint/contracts";
import { Card } from "@/ui";
import { EVENT_KIND } from "./tokens";

export interface TimelineProps {
  events: EvaluationReport["events"];
}

/** ISO timestamp → `MM-DD HH:mm`. */
function formatEventTs(ts: string): string {
  return `${ts.slice(5, 10)} ${ts.slice(11, 16)}`;
}

/**
 * Process timeline — ports `.tl`/`.ev` from
 * `docs/reference/2026-08-13-eval-report-mockup.html`: a vertical rail with
 * one entry per event — color-coded kind dot, kind chip, formatted `ts`,
 * `AI ×{aiTurns}` badge, and the summary.
 */
export function Timeline({ events }: TimelineProps) {
  return (
    <Card className="p-6" data-testid="timeline-section">
      <ul className="relative flex flex-col gap-6 pl-7">
        <li aria-hidden className="pointer-events-none absolute bottom-1 left-[7px] top-1 w-0.5 rounded-full bg-mk-border" />
        {events.map((ev, i) => {
          const kind = EVENT_KIND[ev.kind];
          return (
            <li key={i} className="relative">
              <span
                aria-hidden
                className="absolute -left-7 top-0.5 h-4 w-4 rounded-mk-full border-2 border-mk-surface"
                style={{ background: kind.dot }}
              />
              <div className="flex flex-wrap items-center gap-2.5">
                <span
                  className="rounded-mk-full px-2.5 py-0.5 text-mk-small font-bold tracking-wide"
                  style={{ color: kind.fg, background: kind.bg }}
                >
                  {kind.label}
                </span>
                <span className="text-mk-small tabular-nums text-mk-faint">{formatEventTs(ev.ts)}</span>
                <span className="rounded-mk-full bg-mk-accent-50 px-2.5 py-0.5 text-mk-small font-bold text-mk-accent-600">
                  AI ×{ev.aiTurns}
                </span>
              </div>
              <p className="mt-1.5 text-mk-body text-mk-ink">{ev.summary}</p>
            </li>
          );
        })}
      </ul>
    </Card>
  );
}
