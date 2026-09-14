import { useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";
import { createPortal } from "react-dom";
import type { InboxItemDTO } from "../api/assignments";
import { markParentReportSeen } from "../api/parentReports";
import { navigate } from "../routing";
import { formatDeadline } from "../shared/deadline";
import { useAlive } from "../shared/useAlive";
import { AssignmentStatusChip } from "./AssignmentStrip";
import {
  INBOX_PANEL_WIDTH,
  inboxChipLabel,
  inboxPanelLeft,
  inboxTargetPath,
  publishedLabel,
  sortUnreadFirst,
} from "./inboxLogic";
import { openAssignment } from "./openAssignment";
import type { InboxState } from "./useInbox";

/**
 * The 收件箱 panel. Portaled to `document.body` because the rail it opens
 * from is `overflow: hidden`. The student theme is mirrored onto `body`
 * (`lite-student-theme`), so the portal keeps the same tokens.
 *
 * Position: just right of the rail's resting edge, measured from
 * `.learning-nav-slot` (not the nav itself, which widens on hover). See
 * `inboxPanelLeft` for the narrow-screen rule.
 *
 * Closes on Escape (focus returns to the button), a pointer-down outside the
 * panel and its button, and navigation (handled by `InboxButton`).
 *
 * Two kinds of row, told apart by `type` before any kind-specific field is
 * read: an assignment (chip = its kind, 说明, 截止, status; opening starts it)
 * and a published parent report (chip 报告, class, 发布于 M月D日; opening marks
 * it seen and goes to its page). Order and unread come from the server.
 */
export function InboxPanel({
  inbox,
  anchor,
  onClose,
}: {
  inbox: InboxState;
  anchor: RefObject<HTMLElement>;
  onClose: (restoreFocus: boolean) => void;
}) {
  const panelRef = useRef<HTMLDivElement>(null);
  const alive = useAlive();
  const [openingId, setOpeningId] = useState<string | null>(null);
  const [startError, setStartError] = useState<string | null>(null);
  const [left, setLeft] = useState(16);

  // Measured before paint, so the panel never flashes at the fallback spot.
  useLayoutEffect(() => {
    const rail = anchor.current?.closest(".learning-nav-slot") ?? anchor.current;
    const railRight = rail ? rail.getBoundingClientRect().right : 0;
    setLeft(inboxPanelLeft(railRight, window.innerWidth));
  }, [anchor]);

  useEffect(() => {
    panelRef.current?.focus();
  }, []);

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose(true);
    }
    function onPointerDown(e: PointerEvent) {
      const target = e.target as Node | null;
      if (!target) return;
      if (panelRef.current?.contains(target) || anchor.current?.contains(target)) return;
      onClose(false);
    }
    document.addEventListener("keydown", onKeyDown);
    document.addEventListener("pointerdown", onPointerDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.removeEventListener("pointerdown", onPointerDown);
    };
  }, [anchor, onClose]);

  const items = sortUnreadFirst(inbox.items);

  async function open(item: InboxItemDTO) {
    if (openingId) return;
    const key = rowKey(item);
    setOpeningId(key);
    setStartError(null);
    const path = inboxTargetPath(item);
    if (path) {
      // Seen before navigating, so the reload `InboxButton` runs on popstate
      // already sees it. A failed seen call must not keep her off the page.
      await markParentReportSeen(item.id).catch(() => undefined);
      if (!alive.current) return;
      setOpeningId(null);
      navigate(path);
      return;
    }
    if (item.type !== "assignment") return;
    const err = await openAssignment(item, inbox.reload);
    if (!alive.current) return;
    setOpeningId(null);
    if (err) setStartError(err);
  }

  return createPortal(
    <div
      ref={panelRef}
      role="dialog"
      aria-label="收件箱"
      tabIndex={-1}
      className="fixed z-50 flex flex-col overflow-hidden rounded-mk-lg border border-mk-border bg-mk-surface text-mk-ink shadow-mk-lg outline-none"
      style={{
        left,
        bottom: 16,
        width: INBOX_PANEL_WIDTH,
        maxWidth: "calc(100vw - 32px)",
        maxHeight: "calc(100vh - 32px)",
      }}
    >
      <div className="border-b border-mk-border px-4 py-3">
        <h2 className="text-mk-body font-semibold text-mk-ink">收件箱</h2>
        <p className="mt-0.5 text-mk-small text-mk-muted">老师布置的作业与发布的报告会出现在这里。</p>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-2">
        {inbox.status === "error" && (
          <div role="alert" className="flex flex-wrap items-center gap-2 px-2 py-2 text-mk-small">
            <span style={{ color: "var(--mk-danger)" }}>加载失败：{inbox.error}</span>
            <button
              type="button"
              onClick={inbox.reload}
              className="rounded-mk-full border border-mk-border px-2.5 py-0.5 text-mk-small text-mk-secondary hover:border-mk-accent-200 hover:text-mk-accent-700"
            >
              重试
            </button>
          </div>
        )}

        {items.length === 0 && inbox.status === "loading" && (
          <p className="px-2 py-6 text-center text-mk-small text-mk-muted">加载中</p>
        )}
        {items.length === 0 && inbox.status === "ready" && (
          <p className="px-2 py-6 text-center text-mk-small text-mk-muted">暂无消息</p>
        )}

        {items.length > 0 && (
          <ul className="flex flex-col gap-1">
            {items.map((item) => (
              <li key={rowKey(item)}>
                <button
                  type="button"
                  disabled={openingId !== null}
                  onClick={() => void open(item)}
                  className="flex w-full flex-col gap-1 rounded-mk-md px-2.5 py-2 text-left transition-colors duration-[120ms] ease-mk hover:bg-mk-paper disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                >
                  <span className="flex items-center gap-2">
                    <span
                      className="shrink-0 rounded-mk-full px-2 py-0.5 text-mk-label text-mk-accent-700"
                      style={{ background: "color-mix(in srgb, var(--mk-accent-500) 12%, var(--mk-surface))" }}
                    >
                      {inboxChipLabel(item)}
                    </span>
                    {item.unread && (
                      <span
                        aria-label="未读"
                        className="h-2 w-2 shrink-0 rounded-mk-full"
                        style={{ background: "var(--mk-danger)", boxShadow: "0 0 0 2px var(--mk-surface)" }}
                      />
                    )}
                    <span
                      className={`min-w-0 flex-1 truncate text-mk-body text-mk-ink ${item.unread ? "font-semibold" : ""}`}
                    >
                      {item.title}
                    </span>
                  </span>
                  {item.type === "assignment" && item.instructions.trim() && (
                    <span className="line-clamp-2 text-mk-small text-mk-muted">{item.instructions}</span>
                  )}
                  <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-mk-small text-mk-muted">
                    {item.className && <span>{item.className}</span>}
                    {item.type === "assignment" ? (
                      <>
                        {item.dueAt && <span>截止 {formatDeadline(item.dueAt)}</span>}
                        <span className="ml-auto">
                          {openingId === rowKey(item) ? (
                            <span className="text-mk-small text-mk-muted">处理中</span>
                          ) : (
                            <AssignmentStatusChip status={item.status} label={item.statusLabel} />
                          )}
                        </span>
                      </>
                    ) : (
                      <>
                        {publishedLabel(item.publishedAt) && <span>{publishedLabel(item.publishedAt)}</span>}
                        {openingId === rowKey(item) && <span className="ml-auto">处理中</span>}
                      </>
                    )}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {startError && (
        <p role="alert" className="border-t border-mk-border px-4 py-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {startError}
        </p>
      )}
    </div>,
    document.body,
  );
}

/** Assignment and report ids come from different tables; the row key keeps
 * them apart. */
function rowKey(item: InboxItemDTO): string {
  return `${item.type}:${item.id}`;
}
