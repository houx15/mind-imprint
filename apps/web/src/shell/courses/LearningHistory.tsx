import { useEffect, useState } from "react";
import { GraduationCap, ChevronRight, CheckCircle2 } from "lucide-react";
import type { CourseSummary } from "@mind-imprint/contracts";
import type { CourseHistoryItem } from "@/api/courses";
import { api } from "@/api";
import { Card, EmptyState, Icon, SkeletonRow, coverGradientStyle } from "@/ui";

/**
 * LearningHistory — the 学习记录 section of the 课程 tab. Lists the courses the
 * student has actually engaged with (a runtime session or legacy progress row),
 * newest activity first, merging the server's per-course status/updatedAt
 * (`api.getCourseHistory`) with the course list's title/cover/branch. A
 * completed course reads 已学完; anything mid-flight reads 学习中. Clicking a row
 * re-opens that course (resume, or review if finished).
 */

type Row = CourseHistoryItem & {
  title: string;
  branch: string;
  coverUrl?: string;
  stepCount: number;
};

function isCompleted(status: string): boolean {
  return status === "completed";
}

function formatDate(iso: string): string {
  return iso.slice(0, 10);
}

function HistoryRow({ row, onOpen }: { row: Row; onOpen: () => void }) {
  const done = isCompleted(row.status);
  const pct =
    done ? 100 : row.stepCount > 0 && row.completedCount > 0 ? Math.round((row.completedCount / row.stepCount) * 100) : null;
  return (
    <button
      type="button"
      onClick={onOpen}
      className="group flex w-full items-center gap-4 rounded-mk-md border border-mk-border bg-mk-surface p-3.5 text-left shadow-mk-xs transition-all duration-[140ms] ease-mk hover:-translate-y-0.5 hover:border-mk-accent-200 hover:shadow-mk-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <div className="relative flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-mk-md" style={row.coverUrl ? undefined : coverGradientStyle(row.slug)}>
        {row.coverUrl ? (
          <img src={row.coverUrl} alt="" loading="lazy" className="absolute inset-0 h-full w-full object-cover" />
        ) : (
          <Icon icon={GraduationCap} size={22} className="text-white/85" />
        )}
      </div>
      <div className="min-w-0 flex-1">
        <div className="truncate text-mk-h3 text-mk-ink">{row.title}</div>
        <div className="mt-1 flex items-center gap-2 text-mk-small text-mk-muted">
          <span className="truncate">{row.branch}</span>
          <span aria-hidden>·</span>
          <span>{formatDate(row.updatedAt)}</span>
        </div>
        {pct != null && !done && (
          <div className="mt-2 flex items-center gap-2">
            <div className="h-1.5 flex-1 overflow-hidden rounded-mk-full bg-mk-border">
              <div className="h-full rounded-mk-full bg-mk-accent-500" style={{ width: `${pct}%` }} />
            </div>
            <span className="text-mk-small font-semibold text-mk-muted">{pct}%</span>
          </div>
        )}
      </div>
      {done ? (
        <span className="inline-flex shrink-0 items-center gap-1 rounded-mk-full bg-mk-success-bg px-2.5 py-1 text-mk-small font-bold text-mk-success">
          <Icon icon={CheckCircle2} size={13} />
          已学完
        </span>
      ) : (
        <span className="inline-flex shrink-0 items-center rounded-mk-full bg-mk-accent-50 px-2.5 py-1 text-mk-small font-bold text-mk-accent-600">
          学习中
        </span>
      )}
      <Icon icon={ChevronRight} size={18} className="shrink-0 text-mk-faint transition-colors group-hover:text-mk-accent-500" />
    </button>
  );
}

export function LearningHistory({ onOpenCourse }: { onOpenCourse: (slug: string) => void }) {
  const [rows, setRows] = useState<Row[] | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const [history, courses] = await Promise.all([api.getCourseHistory(), api.listCourses()]);
        if (cancelled) return;
        const bySlug = new Map<string, CourseSummary>(courses.map((c) => [c.slug, c]));
        const merged: Row[] = history
          .map((h): Row | null => {
            const c = bySlug.get(h.slug);
            if (!c) return null; // course unpublished/hidden — skip
            return { ...h, title: c.title, branch: c.branch, coverUrl: c.coverUrl, stepCount: c.step_count };
          })
          .filter((r): r is Row => r !== null);
        setRows(merged);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="h-full min-h-0 flex-1 overflow-y-auto bg-mk-paper">
      <div className="mx-auto max-w-[760px] px-10 py-9">
        <header className="flex items-center gap-4">
          <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-mk-full bg-mk-accent-50 text-mk-accent-600">
            <Icon icon={GraduationCap} size={26} />
          </span>
          <div className="min-w-0">
            <div className="text-mk-label text-mk-muted">学习记录</div>
            <h1 className="mt-0.5 text-mk-h1 text-mk-ink">你走过的课程</h1>
            <p className="mt-1 text-mk-body text-mk-muted">这里记着你学过、正在学的每一门课——随时回来接着学，或回顾一遍。</p>
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
              illustration="emptyProjects"
              title="还没有学习记录"
              body="去课程里挑一门开始学，学习进度会记录在这里。"
            />
          </Card>
        ) : (
          <div className="mt-6 flex flex-col gap-2.5">
            {rows.map((r) => (
              <HistoryRow key={r.slug} row={r} onOpen={() => onOpenCourse(r.slug)} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
