// teacher/parentReportLogic.ts — pure decisions behind the teacher's parent
// report pages: how server failures read, where they show, and the labels a
// report row carries. No React here.
//
// Messages come from apps/api/internal/api/lite_parent_report.go (plan 4 T3).

import { ApiError } from "../api/client";
import { SECTION_LABELS } from "../parentReport/view";
import { errorText } from "./assignmentLogic";

/** Longest section the server accepts, in runes (`section_too_long`). */
export const SECTION_MAX_RUNES = 2000;

export function runeCount(text: string): number {
  return [...text].length;
}

/**
 * How a generate or redraft `draftError` reads. The server can send a line
 * that already names its own failure (「保存草稿失败：请求编号 …」); that one is
 * shown as it is. Anything else gets 草稿生成失败： once (plan 4 Ruling 13,
 * the same rule as plan 2's `startErrorText`).
 */
export function draftErrorText(draftError: string | null | undefined): string | null {
  const m = (draftError ?? "").trim();
  if (!m) return null;
  return m.includes("失败：") ? m : `草稿生成失败：${m}`;
}

const TRAILING_KEY = /([：:]\s*)([a-z_]+)\s*$/;

/** `section_too_long` / `invalid_section` messages end with the raw section
 * key (「每部分不超过 2000 字：overview」). A known key is replaced by its label;
 * anything else is left as the server wrote it. */
export function sectionKeysToLabels(message: string): string {
  return message.replace(TRAILING_KEY, (whole, sep: string, key: string) => {
    const label = SECTION_LABELS[key];
    return label ? `${sep}${label}` : whole;
  });
}

/** The autosave failure line under a section. */
export function saveErrorText(e: unknown): string {
  return `保存失败：${sectionKeysToLabels(errorText(e))}`;
}

/** The server's code, when the failure came from the API. */
export function errorCode(e: unknown): string | null {
  return e instanceof ApiError ? e.code : null;
}

/** A range the server refused shows under the date inputs; any other
 * generate failure shows next to the buttons. */
export function generateErrorPlacement(e: unknown): "range" | "form" {
  const code = errorCode(e);
  return code === "range_before_start" || code === "invalid_range" ? "range" : "form";
}

/** Whether every section of a body is empty after trim. The server fills a
 * blank body on redraft without `replaceBody` (plan 4 Ruling 12). */
export function isBodyBlank(body: Readonly<Record<string, string>>): boolean {
  return Object.values(body).every((v) => !v.trim());
}

/**
 * Whether the editor shows 暂无草稿，请重新生成草稿: no stored model draft,
 * every section blank, and no draftError banner already saying why (after a
 * reload the in-memory message is gone).
 */
export function showsNoDraftHint(
  hasDraft: boolean,
  texts: Readonly<Record<string, string>>,
  draftMessage: string | null,
): boolean {
  return !hasDraft && isBodyBlank(texts) && !draftMessage;
}

/** The line above a section whose stored text still quotes a hidden 金句 or
 * keyword (`hiddenMentions`). */
export function hiddenMentionText(texts: readonly string[]): string {
  return `这一段仍引用了已隐藏的内容：${texts.join("、")}，请修改`;
}

export const EXPORT_BLOCKED_TEXT = "导出失败：正文仍引用已隐藏的内容，请先修改";

/**
 * Why 导出图片 must not run, or null when it may. A picture that still quotes
 * a hidden item would hand parents exactly what the teacher chose to keep out,
 * so any section listed in `hiddenMentions` blocks the export. The editor calls
 * this on the report returned by the LAST save, after flushing pending saves.
 */
export function exportBlockedReason(report: { hiddenMentions: Readonly<Record<string, readonly string[]>> }): string | null {
  const blocked = Object.values(report.hiddenMentions).some((texts) => texts.length > 0);
  return blocked ? EXPORT_BLOCKED_TEXT : null;
}

/**
 * The section keys a save may send: those in the report's current `sections`.
 * While every keyword is hidden `interests` is not there, and PATCH rejects it
 * (400 `invalid_section`); its local text is kept, not sent.
 */
export function savableSectionKeys(
  sections: readonly string[],
  texts: Readonly<Record<string, string>>,
): string[] {
  return Object.keys(texts).filter((key) => sections.includes(key));
}

// A generate that returned 201 with a `draftError` navigates to the editor,
// which reloads the report by id and no longer has the response. The message
// is handed over here, in memory. It is not deleted on read, so React's
// StrictMode double mount still finds it; a successful redraft clears it.
const pendingDraftErrors = new Map<string, string>();

export function rememberDraftError(reportId: string, draftError: string | null): void {
  if (draftError && draftError.trim()) pendingDraftErrors.set(reportId, draftError);
  else pendingDraftErrors.delete(reportId);
}

export function recalledDraftError(reportId: string): string | null {
  return pendingDraftErrors.get(reportId) ?? null;
}
