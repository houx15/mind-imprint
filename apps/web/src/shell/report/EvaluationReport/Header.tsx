import type { EvaluationReport } from "@mind-imprint/contracts";
import { BookOpen, MessageCircle, MessageSquare, PenLine, RotateCcw, type LucideIcon } from "lucide-react";
import { Card, Icon, MACARONS, type MacaronName } from "@/ui";

export interface HeaderProps {
  basics: EvaluationReport["basics"];
  title: string;
}

type Milestones = EvaluationReport["basics"]["milestones"];
type Counters = EvaluationReport["basics"]["counters"];

/** 5-step milestone stepper order — ports `.steps`/`.step` from the mockup. */
const MILESTONE_STEPS: { key: keyof Milestones; label: string }[] = [
  { key: "started", label: "立项" },
  { key: "frameworkFinished", label: "立题" },
  { key: "proposalFinished", label: "提案" },
  { key: "writingFinished", label: "写作" },
  { key: "projectFinished", label: "完成" },
];

/** 5 counter tiles — ports `.counters`/`.stat` from the mockup. */
const COUNTER_TILES: { key: keyof Counters; label: string; icon: LucideIcon; macaron: MacaronName }[] = [
  { key: "aiTurns", label: "AI 对话轮次", icon: MessageCircle, macaron: "mist" },
  { key: "materialsRead", label: "阅读材料", icon: BookOpen, macaron: "lake" },
  { key: "wordsWritten", label: "写作字数", icon: PenLine, macaron: "matcha" },
  { key: "aiCommentCount", label: "AI 批注", icon: MessageSquare, macaron: "taro" },
  { key: "editCount", label: "修改次数", icon: RotateCcw, macaron: "peach" },
];

/** ISO timestamp → `MM-DD`. */
function formatMonthDay(ts: string): string {
  return ts.slice(5, 10);
}

/** Whole-day span between two ISO timestamps (end defaults to now for an
 * in-progress project), floored, min 1 — the "历时 N 天" data-summary fact. */
function durationDays(startIso: string, endIso: string | null): number {
  const start = new Date(startIso).getTime();
  const end = endIso ? new Date(endIso).getTime() : Date.now();
  if (Number.isNaN(start) || Number.isNaN(end)) return 1;
  return Math.max(1, Math.floor((end - start) / 86400000));
}

/** One fact in the header's data summary: a muted label over an ink value. */
function SummaryFact({ label, value }: { label: string; value: string }) {
  return (
    <span className="inline-flex items-baseline gap-1.5 rounded-mk-full bg-mk-paper px-3 py-1">
      <span className="text-mk-small text-mk-muted">{label}</span>
      <span className="text-mk-small font-semibold tabular-nums text-mk-ink">{value}</span>
    </span>
  );
}

/**
 * Report header — ports `.rpt-head`/`.steps`/`.counters` from
 * `docs/reference/2026-08-13-eval-report-mockup.html`: title + date range
 * (project type omitted until multi-type support lands), 5-step milestone
 * stepper (done = non-null timestamp), and 5
 * macaron-tinted counter tiles.
 */
export function Header({ basics, title }: HeaderProps) {
  return (
    <Card className="p-7" data-testid="header-section">
      <div className="text-mk-h1 text-mk-ink">{title}</div>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <SummaryFact label="开始" value={basics.startDate.slice(0, 10)} />
        <SummaryFact label="完成" value={basics.endDate ? basics.endDate.slice(0, 10) : "进行中"} />
        <SummaryFact label="历时" value={`${durationDays(basics.startDate, basics.endDate)} 天`} />
      </div>

      <div className="mt-6 flex border-t border-dashed border-mk-input-border pt-6">
        {MILESTONE_STEPS.map((step) => {
          const ts = basics.milestones[step.key];
          const done = ts !== null;
          return (
            <div key={step.key} className="relative flex-1 text-center">
              <span
                aria-hidden
                className="mx-auto block h-3.5 w-3.5 rounded-mk-full border-2 border-mk-surface"
                style={{
                  background: done ? "var(--mk-success)" : "var(--mk-input-border)",
                  boxShadow: done ? "0 0 0 2px var(--mk-success)" : "none",
                }}
              />
              <div className="mt-2 text-mk-body font-semibold text-mk-ink">{step.label}</div>
              <div className="mt-0.5 text-mk-small tabular-nums text-mk-faint">{done ? formatMonthDay(ts) : "—"}</div>
            </div>
          );
        })}
      </div>

      <div className="mt-6 grid grid-cols-2 gap-3.5 md:grid-cols-5">
        {COUNTER_TILES.map((tile) => {
          const macaron = MACARONS[tile.macaron];
          const value = basics.counters[tile.key];
          return (
            <div key={tile.key} className="flex flex-col gap-2.5 rounded-mk-sm border border-mk-border p-4 shadow-mk-xs">
              <span
                className="flex h-8 w-8 items-center justify-center rounded-mk-sm"
                style={{ background: macaron.bg, color: macaron.fg }}
              >
                <Icon icon={tile.icon} size={18} />
              </span>
              <span className="text-mk-h1 tabular-nums text-mk-ink">{value}</span>
              <span className="text-mk-small text-mk-muted">{tile.label}</span>
            </div>
          );
        })}
      </div>
    </Card>
  );
}
