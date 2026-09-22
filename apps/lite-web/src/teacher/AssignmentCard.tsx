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
export function ProgressBar({ p }: { p: ProgressCounts }) {
  const pct = (n: number) => (p.total > 0 ? `${(n / p.total) * 100}%` : "0%");
  return (
    <div
      className="teacher-progress"
      role="img"
      aria-label={`已完成 ${p.done}，进行中 ${p.inProgress}，已逾期 ${p.overdue}，未开始 ${p.notStarted}，共 ${p.total} 人`}
    >
      {(["done", "inProgress", "overdue"] as const).map((k) => (
        <span key={k} style={{ width: pct(p[k]), "--seg-hue": PROGRESS_HUE[k] } as CSSProperties} />
      ))}
    </div>
  );
}

/**
 * One homework as a card, as the student end shows a task (`.learning-task`):
 * art band, 类型 · 截止, title, progress, and a 查看 button. The whole card
 * opens the homework for a mouse; the button is the keyboard target.
 */
export function AssignmentCard({ assignment: a, onOpen }: { assignment: AssignmentSummaryDTO; onOpen: () => void }) {
  const p = progressFromCounts(a.counts);
  const file = assignmentFileName(a);
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
      <div className="teacher-task-completion"><strong>{p.done}<small> / {p.total} 人</small></strong><span>已完成</span></div>
      <ProgressBar p={p} />
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
