import { useCallback, useEffect, useRef, useState } from "react";
import { getInbox, type InboxItemDTO } from "../api/assignments";
import { useAlive } from "../shared/useAlive";
import { errorMessage } from "./inboxLogic";

const INBOX_RELOAD_EVENT = "lite-inbox-reload";

/**
 * Ask every mounted inbox to refetch. Each `useInbox` call holds its own
 * state (the rail button, a landing strip), so a page that changes what the
 * inbox shows — a report page marking itself seen — cannot reach their
 * `reload` directly.
 */
export function requestInboxReload(): void {
  window.dispatchEvent(new Event(INBOX_RELOAD_EVENT));
}

export interface InboxState {
  items: InboxItemDTO[];
  unread: number;
  status: "loading" | "error" | "ready";
  error: string | null;
  reload: () => void;
}

/**
 * The student's inbox. Loads once, and again when `reload` is called or the
 * window regains focus (a teacher may have assigned something while she was
 * in another tab).
 *
 * Only the newest request's answer is applied: a slow earlier response that
 * lands after a newer one, or after unmount, is dropped. A refetch keeps the
 * items already on screen; only a retry after an error shows 加载中 again.
 */
export function useInbox(): InboxState {
  const [state, setState] = useState<Omit<InboxState, "reload">>({
    items: [],
    unread: 0,
    status: "loading",
    error: null,
  });
  const alive = useAlive();
  const seq = useRef(0);

  const reload = useCallback(() => {
    const mine = ++seq.current;
    setState((s) => (s.status === "error" ? { ...s, status: "loading", error: null } : s));
    getInbox()
      .then((r) => {
        if (!alive.current || mine !== seq.current) return;
        setState({ items: r.items, unread: r.unread, status: "ready", error: null });
      })
      .catch((err: unknown) => {
        if (!alive.current || mine !== seq.current) return;
        setState((s) => ({ ...s, status: "error", error: errorMessage(err) || "没有更多信息" }));
      });
  }, [alive]);

  useEffect(() => {
    reload();
    window.addEventListener("focus", reload);
    window.addEventListener(INBOX_RELOAD_EVENT, reload);
    return () => {
      window.removeEventListener("focus", reload);
      window.removeEventListener(INBOX_RELOAD_EVENT, reload);
    };
  }, [reload]);

  return { ...state, reload };
}
