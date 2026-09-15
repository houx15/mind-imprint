import { StudentLearningSnapshot } from "./LearningSnapshot";
import { StudioEmpty, StudioHeading } from "./StudioArtwork";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { Icon } from "@/ui";
import { api, ApiError } from "@/api";
import type { MeUser } from "../api/auth";
import { getStudentPage, getStudentTree, type ItemRow, type StudentPage as StudentPageData } from "../api/teacher";
import { formatMinutes, itemStatusLabel, kindLabel } from "./format";
import { TreeView } from "../tree/TreeView";
import { useInterestTree } from "../tree/useInterestTree";

/**
 * StudentPage — one student, from the teacher's side: the roster stats she
 * already saw as a row in `ClassPage`, her reading/writing/project items,
 * and her interest tree — read-only (`TreeView`'s `readOnly`, Task 9).
 *
 * 上周表现总结 is deliberately NOT stubbed here — plan 3 adds that card, and
 * a placeholder that says 「下一版本提供」 would just be a promise this build
 * doesn't keep.
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
      <div className="teacher-page teacher-student-page">
        <button
          type="button"
          onClick={onBack}
          className="flex items-center gap-1.5 rounded-mk-sm text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
        >
          <Icon icon={ArrowLeft} size={15} />
          返回学生名单
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
            <StudioHeading label={`${className ?? "学生"} · 学习档案`} title={page.student.displayName} kind="reading" />

            <div className="teacher-stat-strip">
              <StatTile label="累计时长" value={formatMinutes(page.student.minutesTotal)} />
              <StatTile label="本周时长" value={formatMinutes(page.student.minutesThisWeek)} />
              <StatTile label="对话轮次" value={`${page.student.turns} 轮`} />
            </div>

            <StudentLearningSnapshot student={page.student} />
            <p className="mt-3 mb-3 text-mk-small text-mk-muted">阅读、写作和项目的数量均为「已完成 / 总数」；时长仅统计在平台内的学习活动。</p>
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
                <StudioEmpty kind="discovery">暂无兴趣关键词</StudioEmpty>
              ) : (
                <div className="teacher-tree">
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
    <div className="teacher-stat">
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
    <section className="teacher-item-section">
      <div className="teacher-item-label"><h2>{kindLabel(kind)}</h2><p>{rows.length} 项学习记录</p><span aria-hidden="true">↘</span></div>
      {rows.length === 0 ? (
        <StudioEmpty kind={kind}>暂无{kindLabel(kind)}记录</StudioEmpty>
      ) : (
        <div className="mt-2 flex flex-col gap-2">
          {rows.map((row) => (
            <button
              key={row.atomId}
              type="button"
              onClick={() => onOpenItem(row.atomId)}
              className="teacher-item-row"
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="text-mk-small font-bold text-mk-ink">{row.title}</span>
                <span className="text-mk-small text-mk-muted">{itemStatusLabel(row.kind, row.status)} <span aria-hidden="true">↗</span></span>
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
