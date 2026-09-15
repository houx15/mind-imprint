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
