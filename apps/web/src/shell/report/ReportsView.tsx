import { useEffect, useState } from "react";
import { Lightbulb, ChevronRight, FileText } from "lucide-react";
import type { EvalReportListEntry } from "@/api/evaluationReport";
import { listEvaluationReports } from "@/api/evaluationReport";
import type { ProjectListItem } from "@/api/projects";
import { api } from "@/api";
import { EvaluationReportPage } from "@/shell/report/EvaluationReportPage";
import { Card, EmptyState, Icon, SkeletonRow, coverGradientStyle } from "@/ui";

/**
 * ReportsView — the 评估报告 timeline, living UNDER the 项目 tab (nav
 * restructure). Every finished project's 过程评估报告, laid out as a full-width
 * TIMELINE grouped by time (今天/昨天/本周/本月/更早, newest first) — each row a
 * real reading card carrying the PROJECT'S OWN COVER, its title, a type chip and
 * the report date. Clicking a row opens that project's report via
 * `EvaluationReportPage`. `initialProjectId` lets the finish-flow deep-link
 * straight into a specific report (consumed once so backing out lands on the
 * list, not the same report again).
 *
 * The cover comes from a join with the project list (reports carry only
 * projectId/title/type/date) — exactly how 学习记录 enriches its rows from the
 * course list.
 */

type Row = EvalReportListEntry & { cover?: string; coverUrl?: string };

function formatDate(iso: string): string {
  return iso.slice(0, 10);
}

// Coarse time bucket for the section a row falls under — calendar-day distance
// so "今天/昨天" mean the actual days, not 24h windows. (Mirrors 学习记录's own
// bucketing; the design-system convention keeps a local copy per surface.)
const BUCKET_ORDER = ["今天", "昨天", "本周", "本月", "更早"] as const;
type Bucket = (typeof BUCKET_ORDER)[number];

function bucketOf(iso: string, now: Date): Bucket {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "更早";
  const startOfDay = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diffDays = Math.floor((startOfDay(now) - startOfDay(d)) / 86400000);
  if (diffDays <= 0) return "今天";
  if (diffDays === 1) return "昨天";
  if (diffDays < 7) return "本周";
  if (diffDays < 30) return "本月";
  return "更早";
}

function ReportRow({ entry, onOpen }: { entry: Row; onOpen: () => void }) {
  return (
    <button
      type="button"
      onClick={onOpen}
      className="group flex w-full items-center gap-4 rounded-mk-md border border-mk-border bg-mk-surface p-4 text-left shadow-mk-xs transition-all duration-[140ms] ease-mk hover:-translate-y-0.5 hover:border-mk-accent-200 hover:shadow-mk-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <div
        className="relative flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-mk-md"
        style={entry.cover?.startsWith("img:") && entry.coverUrl ? undefined : coverGradientStyle(entry.cover?.startsWith("grad:") ? entry.cover.slice(5) : entry.title || entry.projectId)}
      >
        {entry.cover?.startsWith("img:") && entry.coverUrl ? (
          <img src={entry.coverUrl} alt="" loading="lazy" className="absolute inset-0 h-full w-full object-cover" />
        ) : (
          <Icon icon={Lightbulb} size={22} className="text-white/85" />
        )}
      </div>
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
  const [rows, setRows] = useState<Row[] | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);
  const [openProjectId, setOpenProjectId] = useState<string | null>(initialProjectId ?? null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        // Reports carry only projectId/title/type/date; the cover comes from the
        // project list (best-effort — a report still shows with a gradient
        // fallback if the project list fetch fails).
        const [list, projects] = await Promise.all([
          listEvaluationReports(),
          api.listProjects().catch(() => [] as ProjectListItem[]),
        ]);
        if (cancelled) return;
        const byId = new Map<string, ProjectListItem>(projects.map((p) => [p.id, p]));
        setRows(
          list.map((e): Row => {
            const p = byId.get(e.projectId);
            return { ...e, cover: p?.cover, coverUrl: p?.coverUrl };
          }),
        );
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

  // Entries arrive newest-first; bucket them in that order so each section stays
  // chronological and the section order is fixed (今天 → 更早).
  const now = new Date();
  const grouped: { bucket: Bucket; rows: Row[] }[] = [];
  for (const r of rows ?? []) {
    const b = bucketOf(r.createdAt, now);
    const last = grouped[grouped.length - 1];
    if (last && last.bucket === b) last.rows.push(r);
    else grouped.push({ bucket: b, rows: [r] });
  }

  return (
    <div className="h-full min-h-0 flex-1 overflow-y-auto bg-mk-paper">
      <div className="px-10 py-9">
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

        {rows === undefined ? (
          <div className="mt-6 flex flex-col gap-3">
            {Array.from({ length: 3 }, (_, i) => (
              <Card key={i} className="p-5">
                <SkeletonRow />
              </Card>
            ))}
          </div>
        ) : rows.length === 0 ? (
          <Card className="mt-6 p-6">
            <EmptyState
              illustration="completed"
              title="还没有报告"
              body="完成一个项目后，过程评估报告会在这里生成。"
            />
          </Card>
        ) : (
          <div className="mt-6 flex flex-col gap-7">
            {grouped.map((g) => (
              <section key={g.bucket}>
                <div className="mb-2.5 text-mk-label font-bold uppercase tracking-wide text-mk-muted">{g.bucket}</div>
                <div className="flex flex-col gap-2.5">
                  {g.rows.map((e) => (
                    <ReportRow key={e.projectId} entry={e} onOpen={() => setOpenProjectId(e.projectId)} />
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
