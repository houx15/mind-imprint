import { useCallback, useEffect, useRef, useState } from "react";
import { Inbox } from "lucide-react";
import { InboxPanel } from "./InboxPanel";
import { useInbox } from "./useInbox";

/**
 * 收件箱 in the rail's foot, directly above 设置. It sits in a
 * `learning-nav-links` group so it takes the same row style as the tabs and
 * its label folds with theirs (learning.css hides `.learning-nav-links span`
 * while the rail is folded). The unread dot is an `<i>`, not a `<span>`, for
 * that reason: a span would be hidden along with the label.
 *
 * The panel is portaled out of the rail because the rail is `overflow: hidden`
 * and would clip it.
 */
export function InboxButton() {
  const inbox = useInbox();
  const { reload, unread } = inbox;
  const [open, setOpen] = useState(false);
  const buttonRef = useRef<HTMLButtonElement>(null);

  // `navigate` dispatches popstate, so this covers in-app navigation as well
  // as Back/Forward: close the panel, and refresh the unread count (opening an
  // assignment marks it seen just before the room loads).
  useEffect(() => {
    function onPopState() {
      setOpen(false);
      reload();
    }
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, [reload]);

  const close = useCallback((restoreFocus: boolean) => {
    setOpen(false);
    if (restoreFocus) buttonRef.current?.focus();
  }, []);

  return (
    <div className="learning-nav-links" style={{ marginBottom: "var(--mk-space-16)" }}>
      <button
        ref={buttonRef}
        type="button"
        aria-label={`收件箱，${unread} 条未读`}
        aria-haspopup="dialog"
        aria-expanded={open}
        style={open ? { background: "var(--mk-accent-50)", color: "var(--mk-accent-500)" } : undefined}
        onClick={() => {
          if (!open) reload();
          setOpen(!open);
        }}
      >
        <i className="relative flex shrink-0 not-italic" aria-hidden="true">
          <Inbox size={21} strokeWidth={1.7} />
          {unread > 0 && (
            <i
              className="absolute -right-0.5 -top-0.5 h-2 w-2 rounded-mk-full"
              style={{ background: "var(--mk-danger)", boxShadow: "0 0 0 2px var(--mk-paper)" }}
            />
          )}
        </i>
        <span>收件箱</span>
      </button>
      {open && <InboxPanel inbox={inbox} anchor={buttonRef} onClose={close} />}
    </div>
  );
}
