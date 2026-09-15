// writings/finishedWriting.ts — pure rules behind the finished-writing page:
// which chip it shows, which deadline applies, and how a version is listed.

import type { AssignmentForAtom } from "../api/assignments";
import type { WritingVersionSummary } from "../api/writings";
import { formatDeadline } from "../shared/deadline";
import { wordUnit } from "./wordUnit";

export type FinishedChip = "已完成" | "已提交" | "已退回" | "已锁定";

/** The teacher returned it and she has not submitted since. */
export function isReturnOpen(a: AssignmentForAtom): boolean {
  return a.returnedAt !== null && !a.resubmitted;
}

/** Same rule as liteassign.EffectiveDue: the return deadline once returned. */
export function effectiveDueAt(a: AssignmentForAtom): string {
  return a.returnedAt !== null && a.returnDueAt !== null ? a.returnDueAt : a.dueAt;
}

export function finishedChip(input: { assignment: AssignmentForAtom | null; locked: boolean }): FinishedChip {
  if (input.locked) return "已锁定";
  if (!input.assignment) return "已完成";
  if (isReturnOpen(input.assignment)) return "已退回";
  return "已提交";
}

/** Whether the for-atom assignment lookup that feeds `finishedChip` has come
 *  back yet, and how: `getAssignmentForAtom` resolves `null` on a genuine
 *  200 for a writing that is not homework, so a THROWN error is a real
 *  failure and must never be folded into "not homework" — that would show
 *  已完成 for a submitted/returned homework whose lookup just failed. */
export type AssignmentLoadState = "loading" | "loaded" | "failed";

/** The chip to render, or `null` while it cannot yet be known honestly:
 *  loading (nothing decided yet) or failed (a real error, not "no
 *  homework"). Only once the assignment state is `"loaded"` does this defer
 *  to `finishedChip`. */
export function chipToShow(
  assignmentState: AssignmentLoadState,
  input: { assignment: AssignmentForAtom | null; locked: boolean },
): FinishedChip | null {
  return assignmentState === "loaded" ? finishedChip(input) : null;
}

/** The stage 修改 should force the room onto before she starts revising, or
 *  `null` when no switch is needed. Without this, a writing finished while
 *  still recorded on 结构 (`stage` is independent of `status`) would reopen
 *  into PlanningView's full-screen 结构 conversation instead of the write
 *  view the revising strip lives in — controller ruling: 修改 always opens
 *  the compose/write view, even from 结构. */
export function stageAfterRevise(stage: string): "draft" | null {
  return stage === "outline" ? "draft" : null;
}

export function chipHue(chip: FinishedChip): string {
  switch (chip) {
    case "已退回":
      return "var(--mk-warning)";
    case "已锁定":
      return "var(--mk-muted)";
    default:
      return "var(--mk-success)";
  }
}

/** `v3 · 9月15日 14:20 · 812 字` — Beijing time, 词 for an English piece. */
export function versionLine(v: WritingVersionSummary, lang: string): string {
  return `v${v.number} · ${formatDeadline(v.submittedAt)} · ${v.wordCount} ${wordUnit(lang)}`;
}
