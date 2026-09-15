// writings/finishedWriting.ts — pure rules behind the finished-writing page:
// which chip it shows, which deadline applies, and how a version is listed.

import type { AssignmentForAtom } from "../api/assignments";
import { ApiError } from "../api/client";
import { isRevising, isWritingFinished, type Writing, type WritingVersionSummary } from "../api/writings";
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

/** Controller ruling: the locked page's sentence, verbatim. */
export const LOCKED_TEXT = "已过截止时间，作业已锁定";

/** The finished page's body when the writing has no submitted version (it
 *  was finished by the old API during the 0153 deploy). 修改 then 完成这篇
 *  creates version 1. */
export const NO_VERSION_TEXT = "暂无提交版本。请点击「修改」，修改完成后点击「完成这篇」提交。";

/** The revising strip's copy — names the version already submitted and the
 *  room's real button (完成这篇, not the generic 完成). `null` = no version
 *  exists yet, so there is no version number to name. */
export function revisingStripText(n: number | null): string {
  if (n === null) return "修改完成后请点击「完成这篇」提交";
  return `已提交 v${n} · 修改完成后请再次点击「完成这篇」提交新版本`;
}

/** What 放弃修改 does, said before she confirms it. With no version there is
 *  nothing to restore: the server only ends revising. */
export function discardConfirmText(n: number | null): string {
  if (n === null) return "将退出修改，正文与标题保持不变";
  return `正文与标题将恢复为 v${n}`;
}

/** The finished page's headline: the latest version's title. A rename made
 *  while revising is not submitted until 完成这篇, and the body shown under
 *  the headline is the version's. The live title is used while the list
 *  loads, when there is no version, or when the version title is blank. */
export function finishedHeadline(liveTitle: string, versions: readonly WritingVersionSummary[] | null): string {
  const t = versions?.[0]?.title ?? "";
  return t.trim() !== "" ? t : liveTitle;
}

/** What the finished page's left column shows. */
export type FinishedBodyState = "loading" | "error" | "no_versions" | "ready";

export function finishedBodyState(input: {
  listLoaded: boolean;
  versionCount: number;
  loadError: string | null;
  bodyLoaded: boolean;
}): FinishedBodyState {
  if (input.loadError !== null) return "error";
  if (!input.listLoaded) return "loading";
  if (input.versionCount === 0) return "no_versions";
  return input.bodyLoaded ? "ready" : "loading";
}

/** The 已退回 block's headline: the return, and the deadline it carries. */
export function returnedLine(a: AssignmentForAtom): string {
  return `已退回 · 截止 ${formatDeadline(effectiveDueAt(a))}`;
}

/** Finished page when finished and not revising — or revising but locked,
 *  because a locked writing cannot be edited and shows its latest version. */
export function showFinishedPage(w: Pick<Writing, "status" | "finishedAt" | "revisingAt">, locked: boolean): boolean {
  return isWritingFinished(w) && (!isRevising(w) || locked);
}

/**
 * Whether a thrown value is the server refusing a room write because the
 * writing is locked (403 `writing_locked` — the deadline passed while she
 * was revising). An ordinary input error is shown as a message; this one
 * means no write can succeed any more, so the room reloads into the locked
 * finished page instead.
 */
export function isWritingLockedError(err: unknown): boolean {
  return err instanceof ApiError && err.status === 403 && err.code === "writing_locked";
}

/**
 * Whether a thrown value means the room is out of date: 403 `writing_locked`
 * (deadline passed while revising) or 403 `writing_finished` (no longer
 * revising, e.g. 完成这篇 or 放弃修改 in another tab). Both refuse every
 * retry the same way, so the room reloads instead of showing the message.
 */
export function isWritingClosedError(err: unknown): boolean {
  return err instanceof ApiError && err.status === 403 && (err.code === "writing_locked" || err.code === "writing_finished");
}

/** Verbatim, on every sent 批改 the student reads. */
export const AI_ATTRIBUTION = "由 AI 起草，老师审阅后发送";

/** 「针对 v{n}」 when the grading is about a version other than the one on the page. */
export function gradingVersionLine(gradingVersion: number, shownVersion: number | null): string | null {
  return gradingVersion === shownVersion ? null : `针对 v${gradingVersion}`;
}
