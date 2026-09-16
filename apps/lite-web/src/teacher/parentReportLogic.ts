// teacher/parentReportLogic.ts — pure decisions behind the teacher's parent
// report pages: how server failures read, where they show, and the labels a
// report row carries. No React here.
//
// Messages come from apps/api/internal/api/lite_parent_report.go (plan 4 T3).

import { ApiError } from "../api/client";
import type { ParentReport, TeacherParentReport } from "../api/parentReports";
import { SECTION_LABELS, visibleFacts } from "../parentReport/view";
import { errorText } from "./assignmentLogic";

/**
 * What goes into the exported picture, decided from ONE server snapshot (the
 * GET made after every save has landed): its stored body restricted to its
 * `sections`, and its facts without the hidden items. Never local textarea
 * text and never an earlier response — the snapshot is the body the server
 * checked `hiddenMentions` against, so the check and the picture agree.
 */
export function posterReportFrom(snapshot: TeacherParentReport): ParentReport {
  const { view } = snapshot;
  const body: Record<string, string> = {};
  for (const key of view.sections) {
    const text = view.body[key];
    if (typeof text === "string") body[key] = text;
  }
  return { ...view, facts: visibleFacts(view.facts, snapshot.hidden), body };
}

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

// ── The conversation beside the editor (§12.6, D3) ─────────────────────────

/**
 * The `artifact` a parentReport turn sends: the text of each section the
 * report shows now, as the editor holds it (unsaved typing included), each cut
 * to SECTION_MAX_RUNES. Only the listed sections go out, so neither a section
 * hidden with its keywords nor text past the limit reaches the request (the
 * server applies the same two rules; the request is capped at 256KB). An
 * explicit projection: nothing else from the editor's state is sent.
 */
export function reportArtifactPayload(
  texts: Readonly<Record<string, string>>,
  sections: readonly string[],
): { body: Record<string, string> } {
  const body: Record<string, string> = {};
  for (const key of sections) {
    body[key] = [...(texts[key] ?? "")].slice(0, SECTION_MAX_RUNES).join("");
  }
  return { body };
}

/** A turn's `patch` (`{body: {<section>: <text>}}`) as section texts. A value
 * that is not a string is dropped. */
export function reportPatchBody(patch: Readonly<Record<string, unknown>>): Record<string, string> {
  const body = patch.body;
  if (typeof body !== "object" || body === null || Array.isArray(body)) return {};
  const out: Record<string, string> = {};
  for (const [key, text] of Object.entries(body as Record<string, unknown>)) {
    if (typeof text === "string") out[key] = text;
  }
  return out;
}

export interface ReportPatchPlan {
  /** The editor's texts after the patch. Sections not written are unchanged. */
  texts: Record<string, string>;
  /** Sections to save, in report order. Each goes through the editor's one
   * queue and its section save. */
  write: string[];
  /** Sections she changed while the turn was in flight. Her text stays and
   * the patched text is dropped (the rule of `applyPatch`). */
  kept: string[];
}

/**
 * Which patched sections to write and which to keep. `live` is the editor's
 * text NOW (read from its ref when the reply lands, not from the last
 * render), `snapshot` is the text sent with the turn, and `sections` is the
 * report's sections now.
 *
 * - A section the report does not show now (hidden while the turn ran), or
 *   one the editor has no text entry for, is dropped: there is no textarea to
 *   show it in, and the section save would skip it anyway.
 * - A section whose live text differs from the snapshot is kept.
 * - A section whose patched text equals the live text is neither written nor
 *   kept: nothing changes.
 */
export function planReportPatch(
  live: Readonly<Record<string, string>>,
  snapshot: Readonly<Record<string, string>>,
  patch: Readonly<Record<string, string>>,
  sections: readonly string[],
): ReportPatchPlan {
  const texts = { ...live };
  const write: string[] = [];
  const kept: string[] = [];
  for (const key of sections) {
    if (!(key in patch) || !(key in live)) continue;
    if ((live[key] ?? "") !== (snapshot[key] ?? "")) {
      kept.push(key);
      continue;
    }
    const next = patch[key]!;
    if (next === live[key]) continue;
    texts[key] = next;
    write.push(key);
  }
  return { texts, write, kept };
}

/** The line above a section whose patched text was dropped. */
export function keptSectionText(key: string): string {
  return `${SECTION_LABELS[key] ?? "该段落"} 已保留你的修改`;
}

/**
 * Why the conversation is closed, or null. The server refuses a turn for a
 * student who has left the class with the same 409 `student_left` it gives a
 * redraft, so the reason shown is the server's own words, whichever of the two
 * requests was refused first. A verb prefix is removed so the line reads the
 * same in both cases.
 */
export function studentLeftReason(e: unknown): string | null {
  if (errorCode(e) !== "student_left") return null;
  return errorText(e).replace(/^(对话|重新生成)失败：/, "");
}
