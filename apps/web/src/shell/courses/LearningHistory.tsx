import { useEffect, useState } from "react";
import { GraduationCap, CheckCircle2, FileText, PlayCircle } from "lucide-react";
import type { CourseSummary } from "@mind-imprint/contracts";
import type { CourseHistoryItem } from "@/api/courses";
import type { CourseOpenTarget } from "@/shell/courses/CoursesContainer";
import { api } from "@/api";
import { Card, EmptyState, Icon, SkeletonRow, coverGradientStyle } from "@/ui";

/**
 * LearningHistory — the 学习记录 section of the 课程 tab. An attempt-log (0077): one
 * row per attempt, newest first, grouped into time buckets. A FINISHED attempt is
 * frozen (its date is the completion date) and clicking it opens THAT run's
 * report; an IN-PROGRESS attempt shows its date as last-activity and clicking it
 * continues the course. Relearning a finished course adds a fresh in-progress row
 * beside the kept finished one, so a course can appear several times.
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

// The date a row is shown and sorted by: a finished attempt's FROZEN completion
// date, otherwise its last-activity time.
function rowDate(row: Row): string {
  return isCompleted(row.status) && row.completedAt ? row.completedAt : row.updatedAt;
}

function formatDate(iso: string): string {
  return iso.slice(0, 10);
}

// Coarse time bucket for the section a row falls under. Uses calendar-day
// distance so "今天/昨天" mean the actual days, not 24h windows.
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

function HistoryRow({ row, onOpen }: { row: Row; onOpen: () => void }) {
  const done = isCompleted(row.status);
  const pct =
    done ? 100 : row.stepCount > 0 && row.completedCount > 0 ? Math.round((row.completedCount / row.stepCount) * 100) : null;
  return (
    <button
      type="button"
      onClick={onOpen}
      className="group flex w-full items-center gap-4 rounded-mk-md border border-mk-border bg-mk-surface p-4 text-left shadow-mk-xs transition-all duration-[140ms] ease-mk hover:-translate-y-0.5 hover:border-mk-accent-200 hover:shadow-mk-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <div className="relative flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-mk-md" style={row.coverUrl ? undefined : coverGradientStyle(row.slug)}>
        {row.coverUrl ? (
          <img src={row.coverUrl} alt="" loading="lazy" className="absolute inset-0 h-full w-full object-cover" />
        ) : (
          <Icon icon={GraduationCap} size={24} className="text-white/85" />
        )}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-mk-h3 text-mk-ink">{row.title}</span>
          {done ? (
            <span className="inline-flex shrink-0 items-center gap-1 rounded-mk-full bg-mk-success-bg px-2 py-0.5 text-mk-small font-bold text-mk-success">
              <Icon icon={CheckCircle2} size={12} />
              已学完
            </span>
          ) : (
            <span className="inline-flex shrink-0 items-center rounded-mk-full bg-mk-accent-50 px-2 py-0.5 text-mk-small font-bold text-mk-accent-600">
              学习中
            </span>
          )}
        </div>
        <div className="mt-1 flex items-center gap-2 text-mk-small text-mk-muted">
          <span className="truncate">{row.branch}</span>
          <span aria-hidden>·</span>
          <span>{done ? "学完于 " : "最近学习 "}{formatDate(rowDate(row))}</span>
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
      <span className="inline-flex shrink-0 items-center gap-1.5 rounded-mk-full border border-mk-border px-3 py-1.5 text-mk-small font-bold text-mk-secondary transition-colors group-hover:border-mk-accent-200 group-hover:text-mk-accent-600">
        <Icon icon={done ? FileText : PlayCircle} size={14} />
        {done ? "查看报告" : "继续"}
      </span>
    </button>
  );
}

export function LearningHistory({ onOpen }: { onOpen: (target: CourseOpenTarget) => void }) {
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

  // Clicking a row goes straight to the intent: a finished attempt → its own
  // frozen report (addressed by attemptId); an in-progress one → resume the player.
  const openRow = (row: Row) => {
    if (isCompleted(row.status)) {
      onOpen({ slug: row.slug, mode: "report", attemptId: row.attemptId || undefined });
    } else {
      onOpen({ slug: row.slug, mode: "player" });
    }
  };

  // Rows arrive newest-first from the server; bucket them in that order so each
  // section stays chronological and the section order is fixed (今天 → 更早).
  const now = new Date();
  const grouped: { bucket: Bucket; rows: Row[] }[] = [];
  for (const r of rows ?? []) {
    const b = bucketOf(rowDate(r), now);
    const last = grouped[grouped.length - 1];
    if (last && last.bucket === b) last.rows.push(r);
    else grouped.push({ bucket: b, rows: [r] });
  }

  return (
    <div className="h-full min-h-0 flex-1 overflow-y-auto bg-mk-paper">
      <div className="mx-auto max-w-[960px] px-10 py-9">
        <header className="flex items-center gap-4">
          <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-mk-full bg-mk-accent-50 text-mk-accent-600">
            <Icon icon={GraduationCap} size={26} />
          </span>
          <div className="min-w-0">
            <div className="text-mk-label text-mk-muted">学习记录</div>
            <h1 className="mt-0.5 text-mk-h1 text-mk-ink">你走过的课程</h1>
            <p className="mt-1 text-mk-body text-mk-muted">按时间记着你学过、正在学的每一次——学习中的接着学，学完的回看当时的报告。</p>
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
          <div className="mt-6 flex flex-col gap-7">
            {grouped.map((g) => (
              <section key={g.bucket}>
                <div className="mb-2.5 text-mk-label font-bold uppercase tracking-wide text-mk-muted">{g.bucket}</div>
                <div className="flex flex-col gap-2.5">
                  {g.rows.map((r) => (
                    <HistoryRow key={r.attemptId || `${r.slug}#${r.updatedAt}`} row={r} onOpen={() => openRow(r)} />
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
