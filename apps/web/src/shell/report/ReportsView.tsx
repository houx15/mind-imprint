import { useEffect, useState } from "react";
import { Lightbulb, ChevronRight, FileText } from "lucide-react";
import type { EvalReportListEntry } from "@/api/evaluationReport";
import { listEvaluationReports } from "@/api/evaluationReport";
import { EvaluationReportPage } from "@/shell/report/EvaluationReportPage";
import { Card, EmptyState, Icon, SkeletonRow } from "@/ui";

/**
 * ReportsView — the 评估报告 timeline, now living UNDER the 项目 tab (nav
 * restructure). A list of every finished project's 过程评估报告; clicking a
 * row opens that project's report via `EvaluationReportPage`. `initialProjectId`
 * lets the finish-flow deep-link straight into a specific report (consumed once
 * so backing out lands on the list, not the same report again).
 *
 * The row is a real reading card — an accent lightbulb tile, the project title,
 * and a type chip + date on the meta line — so a shelf of reports reads as a
 * considered collection, not a flat text list.
 */

function formatDate(iso: string): string {
  return iso.slice(0, 10);
}

function ReportRow({ entry, onOpen }: { entry: EvalReportListEntry; onOpen: () => void }) {
  return (
    <button
      type="button"
      onClick={onOpen}
      className="group flex w-full items-center gap-4 rounded-mk-md border border-mk-border bg-mk-surface p-4 text-left shadow-mk-xs transition-all duration-[140ms] ease-mk hover:-translate-y-0.5 hover:border-mk-accent-200 hover:shadow-mk-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-mk-md bg-mk-accent-50 text-mk-accent-600">
        <Icon icon={Lightbulb} size={22} />
      </span>
      <div className="min-w-0 flex-1">
        <div className="truncate text-mk-h3 text-mk-ink">{entry.title}</div>
        <div className="mt-1.5 flex items-center gap-2">
          <span className="inline-flex items-center gap-1 rounded-mk-full bg-mk-paper px-2 py-0.5 text-mk-small font-semibold text-mk-secondary">
            <Icon icon={FileText} size={12} className="text-mk-muted" />
            {entry.type}
          </span>
          <span className="text-mk-small text-mk-muted">{formatDate(entry.createdAt)}</span>
        </div>
      </div>
      <Icon icon={ChevronRight} size={18} className="shrink-0 text-mk-faint transition-colors group-hover:text-mk-accent-500" />
    </button>
  );
}

export function ReportsView({
  initialProjectId,
  onFocusConsumed,
}: {
  initialProjectId?: string | null;
  onFocusConsumed?: () => void;
}) {
  const [entries, setEntries] = useState<EvalReportListEntry[] | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);
  const [openProjectId, setOpenProjectId] = useState<string | null>(initialProjectId ?? null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await listEvaluationReports();
        if (!cancelled) setEntries(list);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  if (openProjectId) {
    return (
      <EvaluationReportPage
        projectId={openProjectId}
        onBack={() => {
          setOpenProjectId(null);
          onFocusConsumed?.();
        }}
      />
    );
  }

  return (
    <div className="h-full min-h-0 flex-1 overflow-y-auto bg-mk-paper">
      <div className="mx-auto max-w-[760px] px-10 py-9">
        <header className="flex items-center gap-4">
          <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-mk-full bg-mk-accent-50 text-mk-accent-600">
            <Icon icon={Lightbulb} size={26} />
          </span>
          <div className="min-w-0">
            <div className="text-mk-label text-mk-muted">评估报告</div>
            <h1 className="mt-0.5 text-mk-h1 text-mk-ink">你的思维印记</h1>
            <p className="mt-1 text-mk-body text-mk-muted">
              每完成一个项目，都会在这里留下一份过程评估报告——按真实过程给出的诊断，不是分数。
            </p>
          </div>
        </header>

        {error && (
          <div className="mt-4 rounded-mk-sm bg-mk-danger-bg px-3.5 py-2.5 text-mk-body text-mk-danger">{error}</div>
        )}

        {entries === undefined ? (
          <div className="mt-6 flex flex-col gap-3">
            {Array.from({ length: 3 }, (_, i) => (
              <Card key={i} className="p-5">
                <SkeletonRow />
              </Card>
            ))}
          </div>
        ) : entries.length === 0 ? (
          <Card className="mt-6 p-6">
            <EmptyState
              illustration="completed"
              title="还没有报告"
              body="完成一个项目后，过程评估报告会在这里生成。"
            />
          </Card>
        ) : (
          <div className="mt-6 flex flex-col gap-2.5">
            {entries.map((e) => (
              <ReportRow key={e.projectId} entry={e} onOpen={() => setOpenProjectId(e.projectId)} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
