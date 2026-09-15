// inbox/inboxLogic.ts — pure rules behind the student inbox and the 作业
// strips. No React and no network, so each rule is testable on its own.

import type { AssignmentInboxItem, AssignmentKind } from "../api/assignments";
import type { AssignmentStatus } from "../shared/deadline";

const START_PREFIX = "开始失败：";

/**
 * The start endpoint already prefixes some of its messages (a failed link
 * fetch comes back as 「开始失败：链接无法读取正文，请告知老师更换阅读材料」).
 * Those are shown as they are; anything else gets the prefix once.
 */
export function startErrorText(message: string): string {
  const m = message.trim();
  if (m.startsWith(START_PREFIX)) return m;
  return `${START_PREFIX}${m || "没有更多信息"}`;
}

/** The message of whatever a failed call threw, without any prefix. */
export function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  return typeof err === "string" ? err : "";
}

const OPEN_STATUSES: readonly AssignmentStatus[] = ["not_started", "in_progress", "overdue"];

/** What a landing's 作业 strip lists: this kind, and still open. */
export function openItemsForKind(items: readonly AssignmentInboxItem[], kind: AssignmentKind): AssignmentInboxItem[] {
  return items.filter((it) => it.kind === kind && OPEN_STATUSES.includes(it.status));
}

/** Unread first; the server's order is kept inside each group. */
export function sortUnreadFirst<T extends { unread: boolean }>(items: readonly T[]): T[] {
  return [...items.filter((it) => it.unread), ...items.filter((it) => !it.unread)];
}

export const INBOX_PANEL_WIDTH = 360;
const INBOX_PANEL_GAP = 8;
const INBOX_PANEL_MARGIN = 16;

/**
 * The inbox panel's `left`, in px. It opens just right of the rail's resting
 * edge (`railRight`), so it does not cover the rail foot or the button that
 * opened it — the rail rests at 220px on a wide landing and 56–68px
 * elsewhere, so a fixed offset cannot fit both. When the viewport is too
 * narrow for that, it slides left and keeps a 16px margin on the right,
 * and never goes past 16px on the left.
 */
export function inboxPanelLeft(railRight: number, viewportWidth: number): number {
  const beside = railRight + INBOX_PANEL_GAP;
  const lastFit = viewportWidth - INBOX_PANEL_WIDTH - INBOX_PANEL_MARGIN;
  return Math.max(INBOX_PANEL_MARGIN, Math.min(beside, lastFit));
}

/** 开始 when there is nothing to go back to yet (never started, including an
 * overdue assignment she never opened); 继续 once a room exists. */
export function startButtonLabel(item: Pick<AssignmentInboxItem, "status" | "atomId">): "开始" | "继续" {
  return item.status === "not_started" || item.atomId === null ? "开始" : "继续";
}
