import { useState } from "react";
import { reportAssignmentIssue, type AssignmentKind } from "../api/assignments";
import { formatDeadline, STATUS_LABEL, type AssignmentStatus } from "../shared/deadline";
import { useAlive } from "../shared/useAlive";
import { statusChipStyle } from "../teacher/assignmentLogic";
import { kindLabel } from "../teacher/format";
import { openItemsForKind, startButtonLabel, stripDueAt, unreadGradings } from "./inboxLogic";
import { openAssignment, openGrading } from "./openAssignment";
import { useInbox } from "./useInbox";

/** Status chip, same colours as the teacher end. */
export function AssignmentStatusChip({ status, label, needsReadingReview = false }: { status: AssignmentStatus; label: string; needsReadingReview?: boolean }) {
  return (
    <span
      className="inline-block whitespace-nowrap rounded-mk-full px-2 py-0.5 text-mk-label font-semibold"
      style={needsReadingReview ? { background: "color-mix(in srgb, var(--mk-warning) 14%, var(--mk-surface))", color: "var(--mk-warning-ink, var(--mk-ink))" } : statusChipStyle(status)}
    >
      {label || STATUS_LABEL[status]}
    </span>
  );
}

/**
 * 作业 on a landing: this kind's open assignments (未开始 / 进行中 / 已逾期 /
 * 已退回). Renders nothing when there are none, so a student with no
 * assignments sees the landing exactly as before. `className` carries the
 * outer spacing, so the hidden strip leaves no margin behind.
 *
 * Without `kind` (the home page) it lists every kind, each row labelled.
 */
export function AssignmentStrip({ kind = null, className = "" }: { kind?: AssignmentKind | null; className?: string }) {
  const inbox = useInbox();
  const alive = useAlive();
  const [openingId, setOpeningId] = useState<string | null>(null);
  const [startError, setStartError] = useState<string | null>(null);
  const [issueItemId, setIssueItemId] = useState<string | null>(null);
  const [issueState, setIssueState] = useState<"idle" | "sending" | "sent">("idle");

  const items = openItemsForKind(inbox.items, kind);
  // Only the home page (every kind) lists new gradings.
  const gradings = kind === null ? unreadGradings(inbox.items) : [];
  if (items.length === 0 && gradings.length === 0) return null;

  async function openSent(id: string) {
    const g = gradings.find((it) => it.id === id);
    if (!g || openingId) return;
    setOpeningId(id);
    try {
      await openGrading(g, inbox.reload);
    } finally {
      if (alive.current) setOpeningId(null);
    }
  }

  async function open(id: string) {
    const item = items.find((it) => it.id === id);
    if (!item || openingId) return;
    setOpeningId(id);
    setStartError(null);
    setIssueItemId(null);
    setIssueState("idle");
    const err = await openAssignment(item, inbox.reload);
    if (!alive.current) return;
    setOpeningId(null);
    if (err) { setStartError(err); if (item.kind === "reading") setIssueItemId(item.id); }
  }

  return (
    <section
      aria-label="作业"
      className={`rounded-mk-lg border border-mk-border bg-mk-surface p-3 shadow-mk-xs ${className}`}
    >
      <h2 className="px-1 text-mk-small font-semibold text-mk-secondary">作业</h2>
      <ul className="mt-1 flex flex-col">
        {gradings.map((g) => (
          <li
            key={g.id}
            className="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-mk-border px-1 py-2.5 first:border-t-0"
          >
            <div className="min-w-0 flex-1">
              <p className="flex min-w-0 items-center gap-2 text-mk-body font-semibold text-mk-ink">
                <span
                  className="shrink-0 rounded-mk-full px-2 py-0.5 text-mk-label font-normal text-mk-accent-700"
                  style={{ background: "color-mix(in srgb, var(--mk-accent-500) 12%, var(--mk-surface))" }}
                >
                  批改
                </span>
                <span className="truncate">{g.writingTitle}</span>
              </p>
              <p className="mt-0.5 text-mk-small text-mk-muted">老师已批改 · {formatDeadline(g.sentAt)}</p>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <span
                className="inline-block whitespace-nowrap rounded-mk-full px-2 py-0.5 text-mk-label font-semibold"
                style={{ background: "color-mix(in srgb, var(--mk-danger) 12%, var(--mk-surface))", color: "var(--mk-danger)" }}
              >
                未读
              </span>
              <button
                type="button"
                disabled={openingId !== null}
                onClick={() => void openSent(g.id)}
                className="whitespace-nowrap rounded-mk-full px-3.5 py-1.5 text-mk-small font-semibold text-white transition-opacity duration-[120ms] ease-mk disabled:opacity-40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                style={{ background: "var(--mk-accent-500)" }}
              >
                {openingId === g.id ? "处理中" : "查看"}
              </button>
            </div>
          </li>
        ))}
        {items.map((item) => (
          <li
            key={item.id}
            className="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-mk-border px-1 py-2.5 first:border-t-0"
          >
            <div className="min-w-0 flex-1">
              <p className="flex min-w-0 items-center gap-2 text-mk-body font-semibold text-mk-ink">
                {kind === null && (
                  <span
                    className="shrink-0 rounded-mk-full px-2 py-0.5 text-mk-label font-normal text-mk-accent-700"
                    style={{ background: "color-mix(in srgb, var(--mk-accent-500) 12%, var(--mk-surface))" }}
                  >
                    {kindLabel(item.kind)}
                  </span>
                )}
                <span className="truncate">{item.title}</span>
              </p>
              {item.instructions.trim() && (
                <p className="mt-0.5 line-clamp-2 text-mk-small text-mk-muted">{item.instructions}</p>
              )}
              {item.dueAt && (
                <p className="mt-0.5 text-mk-small text-mk-muted">截止 {formatDeadline(stripDueAt(item))}</p>
              )}
              {item.status === "returned" && item.returnNote && (
                <p className="mt-0.5 line-clamp-2 text-mk-small text-mk-secondary">退回说明：{item.returnNote}</p>
              )}
              {item.needsReadingReview && <p className="mt-0.5 text-mk-small font-semibold text-mk-accent-700">阅读计划尚未完成，可返回原阅读继续。</p>}
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <AssignmentStatusChip status={item.status} label={item.needsReadingReview ? "阅读待继续" : item.statusLabel} needsReadingReview={item.needsReadingReview} />
              <button
                type="button"
                disabled={openingId !== null}
                onClick={() => void open(item.id)}
                className="whitespace-nowrap rounded-mk-full px-3.5 py-1.5 text-mk-small font-semibold text-white transition-opacity duration-[120ms] ease-mk disabled:opacity-40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                style={{ background: "var(--mk-accent-500)" }}
              >
                {openingId === item.id ? "处理中" : item.needsReadingReview ? "查看并继续" : startButtonLabel(item)}
              </button>
            </div>
          </li>
        ))}
      </ul>
      {startError && (
        <div role="alert" className="flex flex-wrap items-center gap-2 px-1 pt-2 text-mk-small" style={{ color: "var(--mk-danger)" }}><span>{startError}</span>{issueItemId && <button type="button" className="rounded-mk-full border border-mk-border px-3 py-1 font-semibold text-mk-accent-700" disabled={issueState !== "idle"} onClick={() => { setIssueState("sending"); void reportAssignmentIssue(issueItemId, startError.slice(0, 450)).then(() => setIssueState("sent")).catch((e: unknown) => { setIssueState("idle"); setStartError(`反馈失败：${e instanceof Error ? e.message : String(e)}`); }); }}>{issueState === "sent" ? "已通知老师" : issueState === "sending" ? "发送中" : "反馈给老师"}</button>}</div>
      )}
    </section>
  );
}
