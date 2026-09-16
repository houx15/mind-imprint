import { useEffect, useState } from "react";
import { getAssignmentForAtom, type AssignmentForAtom } from "../api/assignments";
import { formatDeadline } from "../shared/deadline";

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
  return <span className={className}>
    作业{assignment.dueAt && <> · 截止 {formatDeadline(assignment.dueAt)}</>}
    {assignment.instructions.trim() && <>
      <button type="button" aria-expanded={expanded} onClick={() => setExpanded(value => !value)} className="ml-2 underline">{expanded ? "收起任务说明" : "查看任务说明"}</button>
      {expanded && <span className="mt-2 block whitespace-pre-wrap rounded-lg border border-mk-border bg-mk-surface p-3">{assignment.instructions}</span>}
    </>}
  </span>;
}
