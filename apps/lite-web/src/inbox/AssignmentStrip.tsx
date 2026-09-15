import { useState } from "react";
import type { AssignmentKind } from "../api/assignments";
import { formatDeadline, STATUS_LABEL, type AssignmentStatus } from "../shared/deadline";
import { useAlive } from "../shared/useAlive";
import { statusChipStyle } from "../teacher/assignmentLogic";
import { openItemsForKind, startButtonLabel, stripDueAt } from "./inboxLogic";
import { openAssignment } from "./openAssignment";
import { useInbox } from "./useInbox";

/** Status chip, same colours as the teacher end. */
export function AssignmentStatusChip({ status, label }: { status: AssignmentStatus; label: string }) {
  return (
    <span
      className="inline-block whitespace-nowrap rounded-mk-full px-2 py-0.5 text-mk-label font-semibold"
      style={statusChipStyle(status)}
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
 */
export function AssignmentStrip({ kind, className = "" }: { kind: AssignmentKind; className?: string }) {
  const inbox = useInbox();
  const alive = useAlive();
  const [openingId, setOpeningId] = useState<string | null>(null);
  const [startError, setStartError] = useState<string | null>(null);

  const items = openItemsForKind(inbox.items, kind);
  if (items.length === 0) return null;

  async function open(id: string) {
    const item = items.find((it) => it.id === id);
    if (!item || openingId) return;
    setOpeningId(id);
    setStartError(null);
    const err = await openAssignment(item, inbox.reload);
    if (!alive.current) return;
    setOpeningId(null);
    if (err) setStartError(err);
  }

  return (
    <section
      aria-label="作业"
      className={`rounded-mk-lg border border-mk-border bg-mk-surface p-3 shadow-mk-xs ${className}`}
    >
      <h2 className="px-1 text-mk-small font-semibold text-mk-secondary">作业</h2>
      <ul className="mt-1 flex flex-col">
        {items.map((item) => (
          <li
            key={item.id}
            className="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-mk-border px-1 py-2.5 first:border-t-0"
          >
            <div className="min-w-0 flex-1">
              <p className="truncate text-mk-body font-semibold text-mk-ink">{item.title}</p>
              {item.instructions.trim() && (
                <p className="mt-0.5 line-clamp-2 text-mk-small text-mk-muted">{item.instructions}</p>
              )}
              {item.dueAt && (
                <p className="mt-0.5 text-mk-small text-mk-muted">截止 {formatDeadline(stripDueAt(item))}</p>
              )}
              {item.status === "returned" && item.returnNote && (
                <p className="mt-0.5 line-clamp-2 text-mk-small text-mk-secondary">退回说明：{item.returnNote}</p>
              )}
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <AssignmentStatusChip status={item.status} label={item.statusLabel} />
              <button
                type="button"
                disabled={openingId !== null}
                onClick={() => void open(item.id)}
                className="whitespace-nowrap rounded-mk-full px-3.5 py-1.5 text-mk-small font-semibold text-white transition-opacity duration-[120ms] ease-mk disabled:opacity-40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                style={{ background: "var(--mk-accent-500)" }}
              >
                {openingId === item.id ? "处理中" : startButtonLabel(item)}
              </button>
            </div>
          </li>
        ))}
      </ul>
      {startError && (
        <p role="alert" className="px-1 pt-1 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {startError}
        </p>
      )}
    </section>
  );
}
