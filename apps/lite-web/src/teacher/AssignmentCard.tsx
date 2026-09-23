import type { CSSProperties } from "react";
import { ArrowRight } from "lucide-react";
import { Icon } from "@/ui";
import type { AssignmentSummaryDTO } from "../api/assignments";
import { studentArtwork } from "../learning/StudentArtwork";
import { formatDeadline } from "../shared/deadline";
import { assignmentFileName, progressFromCounts, type ProgressCounts } from "./assignmentLogic";
import { kindLabel } from "./format";

const KIND_ART = { reading: "reading", writing: "writing", project: "project" } as const;

/** Colours of the progress bar and the count tiles, one per group. */
export const PROGRESS_HUE = {
  done: "var(--mk-success)",
  inProgress: "var(--mk-accent-500)",
  overdue: "var(--mk-danger)",
  notStarted: "var(--mk-muted)",
  toGrade: "var(--mk-warning)",
} as const;

/** A bar split into 已完成 / 进行中 / 已逾期; the rest is the track (未开始). */
export function ProgressBar({ p, reviewCount = 0 }: { p: ProgressCounts; reviewCount?: number }) {
  const pct = (n: number) => (p.total > 0 ? `${(n / p.total) * 100}%` : "0%");
  return (
    <div
      className="teacher-progress"
      role="img"
      aria-label={`已完成 ${Math.max(0, p.done - reviewCount)}，阅读待继续 ${reviewCount}，进行中 ${p.inProgress}，已逾期 ${p.overdue}，未开始 ${p.notStarted}，共 ${p.total} 人`}
    >
      <span style={{ width: pct(Math.max(0, p.done - reviewCount)), "--seg-hue": PROGRESS_HUE.done } as CSSProperties} />
      {reviewCount > 0 && <span style={{ width: pct(reviewCount), "--seg-hue": PROGRESS_HUE.toGrade } as CSSProperties} />}
      {(["inProgress", "overdue"] as const).map((k) => <span key={k} style={{ width: pct(p[k]), "--seg-hue": PROGRESS_HUE[k] } as CSSProperties} />)}
    </div>
  );
}

/**
 * One homework as a card, as the student end shows a task (`.learning-task`):
 * art band, 类型 · 截止, title, progress, and a 查看 button. The whole card
 * opens the homework for a mouse; the button is the keyboard target.
 */
export function AssignmentCard({ assignment: a, onOpen, compact = false }: { assignment: AssignmentSummaryDTO; onOpen: () => void; compact?: boolean }) {
  const p = progressFromCounts(a.counts);
  const file = assignmentFileName(a);
  if (compact) return <button type="button" className="teacher-assignment-row" onClick={onOpen}>
    <span className="teacher-assignment-row-art"><img src={studentArtwork[KIND_ART[a.kind] ?? "ideas"]} alt="" /></span>
    <span className="teacher-assignment-row-main"><small>{kindLabel(a.kind)} · 截止 {formatDeadline(a.dueAt)}</small><strong>{a.title}</strong>{file && <small>{file}</small>}</span>
    <span className="teacher-assignment-row-count"><b>{Math.max(0, p.done - a.needsReadingReview)}/{p.total}</b><small>已完成</small></span>
    <span className="teacher-assignment-row-alert" data-alert={a.issueCount > 0 || a.toGrade > 0 || a.needsReadingReview > 0 || p.overdue > 0 ? "true" : undefined}>{a.issueCount > 0 ? `材料问题 ${a.issueCount}` : a.toGrade > 0 ? `待批改 ${a.toGrade}` : a.needsReadingReview > 0 ? `阅读待继续 ${a.needsReadingReview}` : p.overdue > 0 ? `已逾期 ${p.overdue}` : p.inProgress > 0 ? `进行中 ${p.inProgress}` : ""}</span>
    <Icon icon={ArrowRight} size={16} />
  </button>;
  return (
    <article className="teacher-task teacher-assignment-card cursor-pointer" onClick={onOpen}>
      <div className="teacher-task-art">
        <img src={studentArtwork[KIND_ART[a.kind] ?? "ideas"]} alt="" />
      </div>
      <p className="teacher-task-eyebrow">
        <span>{kindLabel(a.kind)}</span>
        <span aria-hidden="true">·</span>
        <span>截止 {formatDeadline(a.dueAt)}</span>
      </p>
      <h3>{a.title}</h3>
      {file && <p className="teacher-task-file">{file}</p>}
      <div className="teacher-task-completion"><strong>{Math.max(0, p.done - a.needsReadingReview)}<small> / {p.total} 人</small></strong><span>已完成</span></div>
      <ProgressBar p={p} reviewCount={a.needsReadingReview} />
      <p className="teacher-task-detail">
        进行中 {p.inProgress} · 未开始 {p.notStarted}
        {p.overdue > 0 && (
          <>
            {" · "}
            <span className="teacher-chip" style={{ background: `color-mix(in srgb, ${PROGRESS_HUE.overdue} 14%, var(--mk-surface))`, color: `color-mix(in srgb, ${PROGRESS_HUE.overdue} 55%, var(--mk-ink))` }}>
              已逾期 {p.overdue}
            </span>
          </>
        )}
      </p>
      <div className="teacher-task-foot">
        {a.toGrade > 0 ? (
          <span
            className="teacher-chip"
            style={{ background: `color-mix(in srgb, ${PROGRESS_HUE.toGrade} 16%, var(--mk-surface))`, color: `color-mix(in srgb, ${PROGRESS_HUE.toGrade} 55%, var(--mk-ink))` }}
          >
            待批改 {a.toGrade}
          </span>
        ) : (
          <span />
        )}
        <button
          type="button"
          className="teacher-cta"
          aria-label={`查看作业：${a.title}`}
          onClick={(e) => {
            e.stopPropagation();
            onOpen();
          }}
        >
          查看
          <Icon icon={ArrowRight} size={15} />
        </button>
      </div>
    </article>
  );
}
