import { BookOpen, Check, FileText, GitBranch, ListChecks, Search, type LucideIcon } from "lucide-react";
import type { EvaluationReport } from "@mind-imprint/contracts";
import { Card, Icon } from "@/ui";

export interface ToolUsageProps {
  toolUsage: EvaluationReport["toolUsage"];
}

// Tasteful macaron gradient + icon pairs for the cover square. Cycled by
// index (not keyed to `toolId`) so the component stays generic for however
// many tools a real report ends up listing.
const COVER_STYLES: { icon: LucideIcon; gradient: string }[] = [
  { icon: Search, gradient: "linear-gradient(135deg, var(--mk-lake), var(--mk-mist))" },
  { icon: Check, gradient: "linear-gradient(135deg, var(--mk-matcha), var(--mk-lake))" },
  { icon: ListChecks, gradient: "linear-gradient(135deg, var(--mk-peach), var(--mk-butter))" },
  { icon: GitBranch, gradient: "linear-gradient(135deg, var(--mk-taro), var(--mk-berry))" },
  { icon: FileText, gradient: "linear-gradient(135deg, var(--mk-mist), var(--mk-taro))" },
  { icon: BookOpen, gradient: "linear-gradient(135deg, var(--mk-lake), var(--mk-matcha))" },
];

/**
 * ToolUsage — ports `.tools`/`.tool` from
 * `docs/reference/2026-08-13-eval-report-mockup.html`: a 2-column grid, one
 * card per summoned card/subagent — gradient cover tile, `name`, `stage`
 * chip, `purpose` line, and the `summary` of what it changed downstream.
 */
export function ToolUsage({ toolUsage }: ToolUsageProps) {
  return (
    <div className="grid grid-cols-1 gap-3.5 md:grid-cols-2" data-testid="tool-usage-section">
      {toolUsage.map((t, i) => {
        const cover = COVER_STYLES[i % COVER_STYLES.length]!;
        return (
          <Card key={t.toolId} className="flex gap-3.5 p-4">
            <span
              aria-hidden
              className="flex h-11 w-11 shrink-0 items-center justify-center rounded-mk-sm text-white"
              style={{ background: cover.gradient }}
            >
              <Icon icon={cover.icon} size={20} />
            </span>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-mk-h3 text-mk-ink">{t.name}</span>
                <span className="rounded-mk-full bg-mk-peach-bg px-2 py-0.5 text-mk-small font-bold text-mk-peach-fg">
                  {t.stage}
                </span>
              </div>
              <p className="mt-1 text-mk-small text-mk-muted">{t.purpose}</p>
              <p className="mt-1.5 text-mk-body text-mk-secondary">{t.summary}</p>
            </div>
          </Card>
        );
      })}
    </div>
  );
}
