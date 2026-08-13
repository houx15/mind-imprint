import { useEffect, useState } from "react";
import { Lightbulb } from "lucide-react";
import type { EvalReportListEntry } from "@/api/evaluationReport";
import { listEvaluationReports } from "@/api/evaluationReport";
import { EvaluationReportPage } from "@/shell/report/EvaluationReportPage";
import { ToolkitCards } from "@/shell/growth/ToolkitCards";
import { Card, CompactRow, EmptyState, Icon, Segmented, SkeletonRow } from "@/ui";

/**
 * AssessmentView — the 评估 top-level tab (Task 12). Replaces the old
 * 图鉴-only nav destination: a `Segmented` switches between `成长报告` (a
 * timeline of every finished project's 过程评估报告, `listEvaluationReports`)
 * and `图鉴` (the existing tool-card catalog, unchanged). Clicking a timeline
 * row opens that project's report via `EvaluationReportPage`; `initialProjectId`
 * lets the finish-flow deep-link straight into a specific report.
 */

type Section = "growth" | "gallery";

function formatDate(iso: string): string {
  return iso.slice(0, 10);
}

function ReportTimeline({ initialProjectId }: { initialProjectId?: string | null }) {
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
    return <EvaluationReportPage projectId={openProjectId} onBack={() => setOpenProjectId(null)} />;
  }

  return (
    <div className="h-full min-h-0 flex-1 overflow-y-auto bg-mk-paper">
      <div className="mx-auto max-w-[760px] px-10 py-9">
        <header className="flex items-center gap-4">
          <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-mk-full bg-mk-accent-50 text-mk-accent-600">
            <Icon icon={Lightbulb} size={26} />
          </span>
          <div className="min-w-0">
            <div className="text-mk-label text-mk-muted">评估</div>
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
          <div className="mt-4 flex flex-col gap-3">
            {Array.from({ length: 3 }, (_, i) => (
              <Card key={i} className="p-5">
                <SkeletonRow />
              </Card>
            ))}
          </div>
        ) : entries.length === 0 ? (
          <Card className="mt-4 p-6">
            <EmptyState
              illustration="completed"
              title="还没有报告"
              body="完成一个项目后，过程评估报告会在这里生成。"
            />
          </Card>
        ) : (
          <div className="mt-4 flex flex-col gap-2">
            {entries.map((e) => (
              <CompactRow
                key={e.projectId}
                title={e.title}
                meta={`${e.type} · ${formatDate(e.createdAt)}`}
                onClick={() => setOpenProjectId(e.projectId)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

export function AssessmentView({
  initialProjectId,
  onOpenCourse,
}: {
  initialProjectId?: string | null;
  onOpenCourse?: (courseId: string) => void;
}) {
  const [section, setSection] = useState<Section>("growth");

  return (
    <div className="flex h-full w-full flex-col overflow-hidden bg-mk-paper">
      <div className="flex shrink-0 items-center justify-center border-b border-mk-border bg-mk-surface p-3">
        <Segmented
          value={section}
          onChange={(v) => setSection(v === "gallery" ? "gallery" : "growth")}
          options={[
            { value: "growth", label: "成长报告" },
            { value: "gallery", label: "图鉴" },
          ]}
        />
      </div>
      <div className="min-h-0 flex-1 overflow-hidden">
        {section === "growth" ? (
          <ReportTimeline initialProjectId={initialProjectId} />
        ) : (
          <ToolkitCards onOpenCourse={onOpenCourse} />
        )}
      </div>
    </div>
  );
}
