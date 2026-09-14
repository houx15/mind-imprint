import { useEffect, useState } from "react";
import { getAssignmentForAtom, type AssignmentForAtom } from "../api/assignments";
import { formatDeadline } from "../shared/deadline";

/**
 * 老师布置 · 截止 … — shown in a room whose atom was started from an
 * assignment. Renders nothing when the atom has no assignment or the lookup
 * fails: a room she opened herself must look exactly as it did before.
 *
 * `className` defaults to muted small text; the reading room passes `""` so the
 * line inherits its meta row's styling next to 来源 · ….
 */
export function AssignmentLine({ atomId, className = "text-mk-small text-mk-muted" }: { atomId: string; className?: string }) {
  const [assignment, setAssignment] = useState<AssignmentForAtom | null>(null);

  useEffect(() => {
    let cancelled = false;
    setAssignment(null);
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

  if (!assignment || !assignment.dueAt) return null;
  return <span className={className}>老师布置 · 截止 {formatDeadline(assignment.dueAt)}</span>;
}
