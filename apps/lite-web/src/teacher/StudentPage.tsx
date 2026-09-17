import { StudentLearningSnapshot } from "./LearningSnapshot";
import { BackLink, SectionHead, StudioEmpty, StudioError, StudioHeading, StudioLoading } from "./StudioArtwork";
import { StudentGenderField } from "./StudentGenderField";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowRight } from "lucide-react";
import { Button, Icon } from "@/ui";
import { api } from "@/api";
import { ApiError } from "../api/client";
import type { MeUser } from "../api/auth";
import {
  getStudentPage,
  getStudentTree,
  type ItemRow,
  type StudentAssignmentRow,
  type StudentPage as StudentPageData,
} from "../api/teacher";
import { listStudentParentReports, type ParentReportSummary } from "../api/parentReports";
import { publishedMonthDay, rangeLabel } from "../parentReport/range";
import { formatDeadline, STATUS_LABEL, type AssignmentStatus } from "../shared/deadline";
import { StatusChip } from "./AssignmentDetailPage";
import { errorText } from "./assignmentLogic";
import { formatMinutes, itemStatusLabel, kindLabel } from "./format";
import { GenerateParentReportDialog } from "./GenerateParentReportDialog";
import { rememberDraftError } from "./parentReportLogic";
import { TeacherPage } from "./TeacherPage";
import { WeekSummaryCard } from "./WeekSummaryCard";
import { TreeView } from "../tree/TreeView";
import { studentArtwork } from "../learning/StudentArtwork";

const KIND_ART: Record<string, "reading" | "writing" | "project"> = { reading: "reading", writing: "writing", project: "project" };
import { useInterestTree } from "../tree/useInterestTree";

/**
 * StudentPage — one student, from the teacher's side: the roster stats she
 * already saw as a row in `ClassPage`, her reading/writing/project items,
 * and her interest tree — read-only (`TreeView`'s `readOnly`, Task 9).
 *
 * 上周表现总结 (`WeekSummaryCard`, plan 3) sits between the stat tiles and
 * the assignment and item lists. It owns its own loading, keyed by student.
 *
 * 生成家长报告 (plan 4) sits in the header; this student's parent reports are
 * listed below 作业. A created report opens in the editor.
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
  onOpenParentReport,
}: {
  classId: string;
  userId: string;
  onBack: () => void;
  onOpenItem: (atomId: string) => void;
  onOpenParentReport: (reportId: string) => void;
}) {
  const [page, setPage] = useState<StudentPageData | null>(null);
  const [pageError, setPageError] = useState<string | null>(null);
  const [pageNonce, setPageNonce] = useState(0);
  const [generating, setGenerating] = useState(false);

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

  const closeGenerate = useCallback(() => setGenerating(false), []);

  const readings = page?.items.filter((i) => i.kind === "reading") ?? [];
  const writings = page?.items.filter((i) => i.kind === "writing") ?? [];
  const projects = page?.items.filter((i) => i.kind === "project") ?? [];

  const noItems = readings.length + writings.length + projects.length === 0;

  return (
    <TeacherPage>
      <BackLink label="返回班级" onClick={onBack} />

      {pageError ? (
        <StudioError message={pageError} onRetry={() => setPageNonce((n) => n + 1)} />
      ) : page === null ? (
        <StudioLoading />
      ) : (
        <>
          <StudioHeading
            kicker={`${className ?? "学生"} · 学习档案`}
            title={page.student.displayName}
            description="学习时长、作业、学习成果与兴趣树。对话内容不向教师展示。"
            kind="reading"
            actions={
              <Button variant="primary" onClick={() => setGenerating(true)}>
                生成家长报告
              </Button>
            }
          />
          <StudentGenderField classId={classId} userId={userId} initial={page.student.gender} />

          <div className="teacher-stat-strip">
            <StatTile label="累计时长" value={formatMinutes(page.student.minutesTotal)} />
            <StatTile label="本周时长" value={formatMinutes(page.student.minutesThisWeek)} />
            <StatTile label="对话轮次" value={`${page.student.turns} 轮`} />
          </div>
          <StudentLearningSnapshot student={page.student} />
          <p className="mb-6 mt-3 text-mk-small text-mk-muted">阅读、写作和项目的数量均为「已完成 / 总数」；时长仅统计在平台内的学习活动。</p>
          <WeekSummaryCard key={`${classId}:${userId}`} classId={classId} userId={userId} />

          <AssignmentSection rows={page.assignments} onOpenItem={onOpenItem} />

          <ParentReportSection
            key={`parent-reports:${classId}:${userId}`}
            classId={classId}
            userId={userId}
            onOpen={onOpenParentReport}
            onGenerate={() => setGenerating(true)}
          />

          <section>
            <SectionHead
              title="学习成果"
              aside={noItems ? undefined : `阅读 ${readings.length} · 写作 ${writings.length} · 项目 ${projects.length}`}
            />
            {noItems ? (
              <StudioEmpty kind="reading" title="暂无学习记录">
                学生开始阅读、写作或项目后，记录会显示在这里。可以通过布置作业让学生开始。
              </StudioEmpty>
            ) : (
              <div className="flex flex-col gap-6">
                <ItemGroup kind="reading" rows={readings} onOpenItem={onOpenItem} />
                <ItemGroup kind="writing" rows={writings} onOpenItem={onOpenItem} />
                <ItemGroup kind="project" rows={projects} onOpenItem={onOpenItem} />
              </div>
            )}
          </section>

          <section>
            <SectionHead title="兴趣树" />
            {live.status === "loading" ? (
              <StudioLoading />
            ) : live.status === "error" ? (
              <StudioError verb="兴趣树加载" message={live.error} />
            ) : live.status === "empty" ? (
              <StudioEmpty kind="discovery" title="暂无兴趣关键词" compact>
                学生在阅读和写作中记下的关键词会显示在这里。
              </StudioEmpty>
            ) : (
              <div className="teacher-tree">
                <TreeView user={studentUser} live={live} readOnly />
              </div>
            )}
          </section>

          {generating && (
            <GenerateParentReportDialog
              classId={classId}
              userId={userId}
              studentName={page.student.displayName}
              onClose={closeGenerate}
              onCreated={(reportId, draftError) => {
                rememberDraftError(reportId, draftError);
                setGenerating(false);
                onOpenParentReport(reportId);
              }}
            />
          )}
        </>
      )}
    </TeacherPage>
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

/** 她在这个班里的作业：状态标签 + 截止时间。开始过的（有 atomId）点开进单项页。 */
function AssignmentSection({
  rows,
  onOpenItem,
}: {
  rows: StudentAssignmentRow[];
  onOpenItem: (atomId: string) => void;
}) {
  return (
    <section>
      <SectionHead title="作业" aside={rows.length > 0 ? `${rows.length} 份` : undefined} />
      {rows.length === 0 ? (
        <StudioEmpty kind="writing" title="暂无作业" compact>
          布置给该学生的作业会显示在这里。
        </StudioEmpty>
      ) : (
        <div className="teacher-row-list">
          {rows.map((row) => {
            const label = row.statusLabel || STATUS_LABEL[row.status as AssignmentStatus] || row.status;
            const body = (
              <>
                <img src={studentArtwork[KIND_ART[row.kind] ?? "ideas"]} alt="" />
                <span className="teacher-row-card-copy">
                  <strong>{row.title}</strong>
                  <small>
                    <span>{kindLabel(row.kind)}</span>
                    <span>截止 {row.dueAt ? formatDeadline(row.dueAt) : "—"}</span>
                  </small>
                  <span className="mt-2 block">
                    <StatusChip status={row.status} label={label} />
                  </span>
                </span>
              </>
            );
            const atomId = row.atomId;
            return atomId ? (
              <button key={row.id} type="button" onClick={() => onOpenItem(atomId)} className="teacher-row-card" aria-label={`查看作业：${row.title}`}>
                {body}
                <Icon icon={ArrowRight} size={18} />
              </button>
            ) : (
              <div key={row.id} className="teacher-row-card">
                {body}
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}

/** This student's parent reports in this class. Loads on its own, so a slow
 * or failed list never holds up the rest of the page. */
function ParentReportSection({
  classId,
  userId,
  onOpen,
  onGenerate,
}: {
  classId: string;
  userId: string;
  onOpen: (reportId: string) => void;
  onGenerate: () => void;
}) {
  const [rows, setRows] = useState<ParentReportSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setRows(null);
    setError(null);
    listStudentParentReports(classId, userId)
      .then((list) => {
        if (!cancelled) setRows(list);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [classId, userId, nonce]);

  return (
    <section>
      <SectionHead title="家长报告" aside={rows && rows.length > 0 ? `${rows.length} 份` : undefined} />
      {error ? (
        <StudioError message={error} onRetry={() => setNonce((n) => n + 1)} />
      ) : rows === null ? (
        <StudioLoading />
      ) : rows.length === 0 ? (
        <StudioEmpty kind="keepsake" title="暂无家长报告" compact action={{ label: "生成家长报告", onClick: onGenerate }}>
          家长报告按所选日期汇总学习记录，导出前可修改。
        </StudioEmpty>
      ) : (
        <div className="teacher-row-list">
          {rows.map((row) => (
            <button key={row.id} type="button" onClick={() => onOpen(row.id)} className="teacher-row-card">
              <img src={studentArtwork.keepsake} alt="" />
              <span className="teacher-row-card-copy">
                <strong>{rangeLabel(row.rangeStart, row.rangeEnd) || "—"}</strong>
                <small>
                  <span>创建于 {publishedMonthDay(row.createdAt) || "—"}</span>
                </small>
              </span>
              <Icon icon={ArrowRight} size={18} />
            </button>
          ))}
        </div>
      )}
    </section>
  );
}

function ItemGroup({
  kind,
  rows,
  onOpenItem,
}: {
  kind: "reading" | "writing" | "project";
  rows: ItemRow[];
  onOpenItem: (atomId: string) => void;
}) {
  return (
    <div>
      <h3 className="teacher-group-title">
        {kindLabel(kind)}
        <span>{rows.length}</span>
      </h3>
      {rows.length === 0 ? (
        <p className="teacher-group-empty">暂无{kindLabel(kind)}记录</p>
      ) : (
        <div className="teacher-task-grid mt-3">
          {rows.map((row) => (
            <article key={row.atomId} className="teacher-task cursor-pointer" data-item-row onClick={() => onOpenItem(row.atomId)}>
              <div className="teacher-task-art">
                <img src={studentArtwork[kind === "reading" ? "keepsake" : kind]} alt="" />
              </div>
              <p className="teacher-task-eyebrow">
                <span>{kindLabel(kind)}</span>
                <span aria-hidden="true">·</span>
                <span>{itemStatusLabel(row.kind, row.status)}</span>
                {row.kind === "reading" && row.level !== null ? (
                  <>
                    <span aria-hidden="true">·</span>
                    <span>第 {row.level} 档</span>
                  </>
                ) : null}
              </p>
              <h3>{row.title}</h3>
              <p className="teacher-task-detail">
                时长 {formatMinutes(row.minutes)} · 对话 {row.turns} 轮 · 最近活跃 {shortDate(row.lastActiveAt)}
              </p>
              <div className="teacher-task-foot">
                <span />
                <button
                  type="button"
                  className="teacher-cta"
                  aria-label={`查看成果：${row.title}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    onOpenItem(row.atomId);
                  }}
                >
                  查看成果
                  <Icon icon={ArrowRight} size={15} />
                </button>
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}
