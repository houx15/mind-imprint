import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";

// teacher/controls/Popover.tsx — the floating panel under a field (Select's
// list, DateField's calendar). It is portalled to <body> so a scrolling
// canvas or a dialog's `overflow` does not clip it, positioned from the
// trigger's rect, flipped above when there is no room below, and kept in
// place while anything scrolls. The lite theme's tokens are on
// `body.lite-teacher-theme`, so the portal keeps them.

const GAP = 6;
const MARGIN = 12;

export function Popover({
  anchor,
  open,
  onClose,
  minWidth,
  width: fixedWidth,
  children,
}: {
  anchor: RefObject<HTMLElement>;
  open: boolean;
  onClose: () => void;
  /** At least the trigger's width; this raises it (the calendar). */
  minWidth?: number;
  /** A panel with its own layout (the calendar): this width, whatever the
   *  trigger's. */
  width?: number;
  children: ReactNode;
}) {
  const panelRef = useRef<HTMLDivElement>(null);
  const [style, setStyle] = useState<CSSProperties>({ visibility: "hidden" });

  useLayoutEffect(() => {
    if (!open) return;
    let frame = 0;
    const place = () => {
      const a = anchor.current?.getBoundingClientRect();
      const panel = panelRef.current;
      if (!a || !panel) return;
      const width = Math.min(fixedWidth ?? Math.max(a.width, minWidth ?? 0), window.innerWidth - 2 * MARGIN);
      const height = panel.offsetHeight;
      const below = window.innerHeight - a.bottom - GAP - MARGIN;
      const above = a.top - GAP - MARGIN;
      const top = height <= below || below >= above ? a.bottom + GAP : Math.max(MARGIN, a.top - GAP - height);
      const left = Math.min(Math.max(MARGIN, a.left), window.innerWidth - MARGIN - width);
      const maxHeight = Math.max(160, top > a.top ? below : above);
      setStyle({ top, left, width, maxHeight });
    };
    place();
    const onMove = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(place);
    };
    window.addEventListener("scroll", onMove, true);
    window.addEventListener("resize", onMove);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("scroll", onMove, true);
      window.removeEventListener("resize", onMove);
      setStyle({ visibility: "hidden" });
    };
  }, [open, anchor, minWidth, fixedWidth]);

  // A press outside both the trigger and the panel closes it.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node;
      if (panelRef.current?.contains(t) || anchor.current?.contains(t)) return;
      onClose();
    };
    document.addEventListener("pointerdown", onDown, true);
    return () => document.removeEventListener("pointerdown", onDown, true);
  }, [open, anchor, onClose]);

  if (!open) return null;
  return createPortal(
    <div ref={panelRef} className="tc-popover" style={style}>
      {children}
    </div>,
    document.body,
  );
}
