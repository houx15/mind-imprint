import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { Icon } from "@/ui";
import { api, ApiError } from "@/api";
import type { MeUser } from "../api/auth";
import {
  getStudentPage,
  getStudentTree,
  type ItemRow,
  type StudentAssignmentRow,
  type StudentPage as StudentPageData,
} from "../api/teacher";
import { formatDeadline, STATUS_LABEL, type AssignmentStatus } from "../shared/deadline";
import { StatusChip } from "./AssignmentDetailPage";
import { formatMinutes, itemStatusLabel, kindLabel } from "./format";
import { WeekSummaryCard } from "./WeekSummaryCard";
import { TreeView } from "../tree/TreeView";
import { useInterestTree } from "../tree/useInterestTree";

/**
 * StudentPage — one student, from the teacher's side: the roster stats she
 * already saw as a row in `ClassPage`, her reading/writing/project items,
 * and her interest tree — read-only (`TreeView`'s `readOnly`, Task 9).
 *
 * 上周表现总结 (`WeekSummaryCard`, plan 3) sits between the stat tiles and
 * the assignment and item lists. It owns its own loading, keyed by student.
 *
 * Async load hygiene: every fetch here carries a `cancelled` flag (the same
 * pattern `useInterestTree` uses) so a late response from the PREVIOUS
 * student never lands after `classId`/`userId` has moved on, and every piece
 * of fetched state is reset to `null` at the top of its effect so a newly
 * opened student never briefly shows the last one's numbers.
 */
export function StudentPage({
  classId,
  userId,
  onBack,
  onOpenItem,
}: {
  classId: string;
  userId: string;
  onBack: () => void;
  onOpenItem: (atomId: string) => void;
}) {
  const [page, setPage] = useState<StudentPageData | null>(null);
  const [pageError, setPageError] = useState<string | null>(null);
  const [pageNonce, setPageNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setPage(null);
    setPageError(null);
    getStudentPage(classId, userId)
      .then((p) => {
        if (!cancelled) setPage(p);
      })
      .catch((e: unknown) => {
        if (!cancelled) setPageError(e instanceof ApiError ? e.message : String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [classId, userId, pageNonce]);

  // 班级名字，只用来喂给 TreeView 头部——失败时静默留空，`user.classes` 就是
  // `[]`，TreeView 本来就把它当「没有班级」处理（见 TreeView.tsx ~143-147），
  // 不值得为这一行装饰性文字单独起一套加载/报错 UI。
  const [className, setClassName] = useState<string | null>(null);
  useEffect(() => {
    let cancelled = false;
    setClassName(null);
    api
      .getClass(classId)
      .then((d) => {
        if (!cancelled) setClassName(d.class.name);
      })
      .catch(() => {
        if (!cancelled) setClassName(null);
      });
    return () => {
      cancelled = true;
    };
  }, [classId]);

  // TreeView 需要一个 MeUser；这里搭一个最小的站位对象，只填 TreeView 真的会
  // 读的两个字段（`display_name`、`classes[0].name`，见 TreeView.tsx 的
  // controller notes），其余留空值而不是编造。
  const studentUser: MeUser = useMemo(
    () => ({
      id: userId,
      email: "",
      display_name: page?.student.displayName ?? "",
      role: "student",
      avatar_color: page?.student.avatarColor ?? "",
      page_background: "",
      onboarded_at: null,
      school: { id: "", name: "" },
      classes: className ? [{ id: classId, name: className, role_in_class: "student" }] : [],
    }),
    [userId, page, className, classId],
  );

  const treeFetcher = useCallback(() => getStudentTree(classId, userId), [classId, userId]);
  const live = useInterestTree(treeFetcher);

  const readings = page?.items.filter((i) => i.kind === "reading") ?? [];
  const writings = page?.items.filter((i) => i.kind === "writing") ?? [];
  const projects = page?.items.filter((i) => i.kind === "project") ?? [];

  return (
    <div className="min-h-full">
      <div className="mx-auto max-w-[980px] px-4 pb-16 pt-8 sm:px-8">
        <button
          type="button"
          onClick={onBack}
          className="flex items-center gap-1.5 rounded-mk-sm text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
        >
          <Icon icon={ArrowLeft} size={15} />
          返回
        </button>

        {pageError ? (
          <div className="mt-4 text-mk-small font-semibold text-mk-danger">
            加载失败：{pageError}{" "}
            <button
              type="button"
              onClick={() => setPageNonce((n) => n + 1)}
              className="cursor-pointer underline"
            >
              重试
            </button>
          </div>
        ) : page === null ? (
          <div className="mt-4 text-mk-body text-mk-muted">加载中…</div>
        ) : (
          <>
            <h1 className="mt-4 text-mk-h1 tracking-tight text-mk-ink">{page.student.displayName}</h1>

            <div className="mt-4 flex flex-wrap gap-3">
              <StatTile label="累计时长" value={formatMinutes(page.student.minutesTotal)} />
              <StatTile label="本周时长" value={formatMinutes(page.student.minutesThisWeek)} />
              <StatTile label="本周活跃天数" value={`${page.student.activeDaysThisWeek} 天`} />
              <StatTile label="对话轮次" value={`${page.student.turns} 轮`} />
              <StatTile label="阅读" value={`${page.student.readingsDone}/${page.student.readingsTotal}`} />
              <StatTile label="写作" value={`${page.student.writingsDone}/${page.student.writingsTotal}`} />
              <StatTile label="项目" value={`${page.student.projectsDone}/${page.student.projectsTotal}`} />
            </div>

            <WeekSummaryCard key={`${classId}:${userId}`} classId={classId} userId={userId} />

            <AssignmentSection rows={page.assignments} onOpenItem={onOpenItem} />

            <ItemSection kind="reading" rows={readings} onOpenItem={onOpenItem} />
            <ItemSection kind="writing" rows={writings} onOpenItem={onOpenItem} />
            <ItemSection kind="project" rows={projects} onOpenItem={onOpenItem} />

            <section className="mt-10">
              <h2 className="text-mk-h3 text-mk-ink">兴趣树</h2>
              {live.status === "loading" ? (
                <p className="mt-2 text-mk-body text-mk-muted">加载中…</p>
              ) : live.status === "error" ? (
                <p className="mt-2 text-mk-small font-semibold text-mk-danger">兴趣树加载失败：{live.error}</p>
              ) : live.status === "empty" ? (
                <p className="mt-2 text-mk-body text-mk-muted">暂无兴趣关键词</p>
              ) : (
                <div className="-mx-4 mt-2 sm:-mx-8">
                  <TreeView user={studentUser} live={live} readOnly />
                </div>
              )}
            </section>
          </>
        )}
      </div>
    </div>
  );
}

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-[104px] rounded-mk-md border border-mk-border bg-mk-surface px-3.5 py-2.5">
      <div className="text-mk-h3 tabular-nums text-mk-ink">{value}</div>
      <div className="mt-0.5 text-mk-label text-mk-muted">{label}</div>
    </div>
  );
}

/** `M月D日`，或没有时间的 `—`。这一页自己的小写法——同样的取舍见
 *  `ClassPage.tsx` 的 `lastActiveLabel`，那个不导出，这里就地重写一份。 */
function shortDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return `${d.getMonth() + 1}月${d.getDate()}日`;
}

/** 她在这个班里的作业：状态标签 + 截止时间。开始过的（有 atomId）点开进单项页。 */
function AssignmentSection({
  rows,
  onOpenItem,
}: {
  rows: StudentAssignmentRow[];
  onOpenItem: (atomId: string) => void;
}) {
  return (
    <section className="mt-8">
      <h2 className="text-mk-h3 text-mk-ink">作业</h2>
      {rows.length === 0 ? (
        <p className="mt-2 text-mk-small text-mk-muted">暂无作业</p>
      ) : (
        <div className="mt-2 flex flex-col gap-2">
          {rows.map((row) => {
            const label = row.statusLabel || STATUS_LABEL[row.status as AssignmentStatus] || row.status;
            const body = (
              <>
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="text-mk-small font-bold text-mk-ink">{row.title}</span>
                  <StatusChip status={row.status} label={label} />
                </div>
                <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-1 text-mk-small text-mk-muted">
                  <span>{kindLabel(row.kind)}</span>
                  <span>截止时间 {row.dueAt ? formatDeadline(row.dueAt) : "—"}</span>
                </div>
              </>
            );
            const atomId = row.atomId;
            return atomId ? (
              <button
                key={row.id}
                type="button"
                onClick={() => onOpenItem(atomId)}
                className="w-full rounded-mk-md border border-mk-border bg-mk-surface p-3 text-left transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
              >
                {body}
              </button>
            ) : (
              <div key={row.id} className="rounded-mk-md border border-mk-border bg-mk-surface p-3">
                {body}
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}

function ItemSection({
  kind,
  rows,
  onOpenItem,
}: {
  kind: "reading" | "writing" | "project";
  rows: ItemRow[];
  onOpenItem: (atomId: string) => void;
}) {
  return (
    <section className="mt-8">
      <h2 className="text-mk-h3 text-mk-ink">{kindLabel(kind)}</h2>
      {rows.length === 0 ? (
        <p className="mt-2 text-mk-small text-mk-muted">暂无{kindLabel(kind)}记录</p>
      ) : (
        <div className="mt-2 flex flex-col gap-2">
          {rows.map((row) => (
            <button
              key={row.atomId}
              type="button"
              onClick={() => onOpenItem(row.atomId)}
              className="w-full rounded-mk-md border border-mk-border bg-mk-surface p-3 text-left transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="text-mk-small font-bold text-mk-ink">{row.title}</span>
                <span className="text-mk-small text-mk-muted">{itemStatusLabel(row.kind, row.status)}</span>
              </div>
              <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-1 text-mk-small text-mk-muted">
                <span>时长 {formatMinutes(row.minutes)}</span>
                <span>对话 {row.turns} 轮</span>
                <span>最近活跃 {shortDate(row.lastActiveAt)}</span>
                {row.kind === "reading" && row.level !== null ? <span>第 {row.level} 档</span> : null}
              </div>
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
