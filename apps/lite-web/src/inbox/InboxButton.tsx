import { useCallback, useEffect, useRef, useState } from "react";
import { Inbox } from "lucide-react";
import { Icon } from "@/ui";
import { InboxPanel } from "./InboxPanel";
import { useInbox } from "./useInbox";

/**
 * 收件箱 in the rail, directly above 设置. It carries the rail's `mt-auto`, so
 * the two stay pinned together at the foot. The panel is portaled out of the
 * rail because the rail is `overflow-hidden` and would clip it.
 */
export function InboxButton({ labelCls }: { labelCls: string }) {
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
    <>
      <button
        ref={buttonRef}
        type="button"
        aria-label={`收件箱，${unread} 条未读`}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => {
          if (!open) reload();
          setOpen(!open);
        }}
        className={[
          "mt-auto flex items-center gap-3 rounded-mk-md px-1.5 py-2 transition-colors duration-[120ms] ease-mk",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60",
          open ? "bg-white/15" : "hover:bg-white/10",
        ].join(" ")}
      >
        <span className="relative flex h-7 w-7 shrink-0 items-center justify-center">
          <Icon icon={Inbox} size={22} className={open ? "text-white" : "text-white/70"} />
          {unread > 0 && (
            <span
              aria-hidden="true"
              className="absolute right-0.5 top-0.5 h-2 w-2 rounded-mk-full"
              style={{ background: "var(--mk-danger)", boxShadow: "0 0 0 2px #fff" }}
            />
          )}
        </span>
        <span className={`${labelCls} ${open ? "font-semibold text-white" : "text-white/80"}`}>收件箱</span>
      </button>
      {open && <InboxPanel inbox={inbox} anchor={buttonRef} onClose={close} />}
    </>
  );
}
