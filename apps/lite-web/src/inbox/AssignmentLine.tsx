import { useEffect, useState } from "react";
import { getAssignmentForAtom, type AssignmentForAtom } from "../api/assignments";
import { formatDeadline } from "../shared/deadline";
import { effectiveDueAt, isReturnOpen } from "../writings/finishedWriting";

/**
 * 作业 · 截止 … — shown in a room whose atom was started from an
 * assignment. Renders nothing when the atom has no assignment or the lookup
 * fails: a room she opened herself must look exactly as it did before.
 *
 * `className` defaults to muted small text; the reading room passes `""` so the
 * line inherits its meta row's styling next to 来源 · ….
 */
export function AssignmentLine({ atomId, className = "text-mk-small text-mk-muted" }: { atomId: string; className?: string }) {
  const [expanded, setExpanded] = useState(false);
  const [assignment, setAssignment] = useState<AssignmentForAtom | null>(null);

  useEffect(() => {
    let cancelled = false;
    setAssignment(null);
    setExpanded(false);
    getAssignmentForAtom(atomId)
      .then((a) => {
        if (!cancelled) setAssignment(a);
      })
      .catch(() => {
        if (!cancelled) setAssignment(null);
      });
    return () => {
      cancelled = true;
    };
  }, [atomId]);

  if (!assignment) return null;
  // 退回修改: the deadline that applies is the return deadline, and she
  // revises with the teacher's reason in view (before 2026-09-17 the room
  // showed the original deadline and no note).
  const returned = isReturnOpen(assignment);
  const due = effectiveDueAt(assignment);
  return <span className={className}>
    {returned ? "作业 · 已退回" : "作业"}{due && <> · 截止 {formatDeadline(due)}</>}
    {returned && assignment.returnNote?.trim() && (
      <span className="mt-1 block whitespace-pre-wrap text-mk-secondary">退回说明：{assignment.returnNote}</span>
    )}
    {assignment.instructions.trim() && <>
      <button type="button" aria-expanded={expanded} onClick={() => setExpanded(value => !value)} className="ml-2 underline">{expanded ? "收起任务说明" : "查看任务说明"}</button>
      {expanded && <span className="mt-2 block whitespace-pre-wrap rounded-lg border border-mk-border bg-mk-surface p-3">{assignment.instructions}</span>}
    </>}
  </span>;
}
