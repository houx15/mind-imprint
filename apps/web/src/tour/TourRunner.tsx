import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Pebble } from "@/ui";
import { Markdown } from "@/cards/Markdown";
import { useTour } from "./TourProvider";
import { resolveAnchor } from "./anchors";

const PAD = 8; // spotlight padding around the anchor

export function TourRunner() {
  const t = useTour();
  const [rect, setRect] = useState<DOMRect | null>(null);

  // Resolve + spotlight the anchor whenever the step changes.
  useEffect(() => {
    if (!t.running || !t.step) { setRect(null); return; }
    if (!t.step.anchor || t.step.placement === "center") { setRect(null); return; }
    let cancelled = false;
    void resolveAnchor(t.step.anchor).then((el) => {
      if (cancelled) return;
      if (!el) { setRect(null); return; }
      el.scrollIntoView({ block: "center", behavior: "smooth" });
      setRect(el.getBoundingClientRect());
    });
    return () => { cancelled = true; };
  }, [t.running, t.step]);

  // advance:"action" — advance when the user does the thing on the target element.
  useEffect(() => {
    if (!t.running || !t.step || t.step.advance !== "action" || !t.step.actionEvent) return;
    const { selector, type } = t.step.actionEvent;
    let cleanup = () => {};
    const attach = (el: HTMLElement) => {
      const handler = () => t.next();
      el.addEventListener(type, handler, { once: true });
      cleanup = () => el.removeEventListener(type, handler);
    };
    // Attach synchronously when the target is already mounted (common case) so no
    // microtask tick is needed before the listener is live; otherwise poll for it.
    const immediate = document.querySelector<HTMLElement>(selector);
    if (immediate) attach(immediate);
    else void resolveAnchor(selector).then((el) => { if (el) attach(el); });
    return () => cleanup();
  }, [t.running, t.step, t]);

  // Keyboard: Esc ends; Enter/→ advances a "next" step.
  useEffect(() => {
    if (!t.running) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") { e.preventDefault(); t.stop(); }
      else if ((e.key === "Enter" || e.key === "ArrowRight") && t.step?.advance === "next") { e.preventDefault(); t.next(); }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [t]);

  if (!t.running || !t.step) return null;

  const isAction = t.step.advance === "action";
  const centered = !rect;

  return createPortal(
    <div className="fixed inset-0 z-[100]" role="dialog" aria-modal="true" aria-label="新手引导">
      {/* Dim overlay. With a rect, a box-shadow cutout creates the spotlight. */}
      <div
        className="absolute inset-0"
        style={
          rect
            ? {
                top: rect.top - PAD, left: rect.left - PAD,
                width: rect.width + PAD * 2, height: rect.height + PAD * 2,
                position: "fixed", borderRadius: 12,
                boxShadow: "0 0 0 9999px rgba(15,23,42,0.55)",
                transition: "all 160ms ease",
              }
            : { background: "rgba(15,23,42,0.55)" }
        }
      />
      {/* 印记 popover. Centered when no anchor; otherwise near the anchor. */}
      <div
        className="fixed max-w-[360px] rounded-mk-md bg-mk-surface p-4 shadow-mk-lg ring-1 ring-mk-border"
        style={
          centered
            ? { top: "50%", left: "50%", transform: "translate(-50%,-50%)" }
            : positionNear(rect!, t.step.placement)
        }
      >
        <div className="mb-2 flex items-center gap-2">
          <Pebble size={22} />
          <span className="text-mk-small font-semibold text-mk-accent-700">印记</span>
        </div>
        {t.step.title && <div className="mb-1 text-mk-body font-semibold text-mk-ink">{t.step.title}</div>}
        <div className="text-mk-body text-mk-ink"><Markdown text={t.step.text} /></div>

        <div className="mt-3 flex items-center justify-between">
          <button type="button" className="text-mk-small text-mk-muted hover:text-mk-ink" onClick={t.stop}>结束</button>
          <div className="flex items-center gap-2">
            <button type="button" className="text-mk-small text-mk-muted hover:text-mk-ink" onClick={t.skipSegment}>跳过本节</button>
            {t.stepIndex > 0 || t.segmentIndex > 0 ? (
              <button type="button" className="rounded-mk-full px-3 py-1 text-mk-small text-mk-ink ring-1 ring-mk-border hover:bg-mk-paper" onClick={t.prev}>上一步</button>
            ) : null}
            {isAction ? (
              <span className="rounded-mk-full bg-mk-accent-50 px-3 py-1 text-mk-small font-semibold text-mk-accent-700">试试看 →</span>
            ) : (
              <button type="button" className="rounded-mk-full bg-mk-accent px-3 py-1 text-mk-small font-semibold text-white hover:opacity-90" onClick={t.next}>下一步</button>
            )}
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}

function positionNear(rect: DOMRect, placement: TourPlacementLocal = "bottom"): React.CSSProperties {
  const gap = 12;
  switch (placement) {
    case "top":    return { top: rect.top - gap, left: rect.left, transform: "translateY(-100%)" };
    case "left":   return { top: rect.top, left: rect.left - gap, transform: "translateX(-100%)" };
    case "right":  return { top: rect.top, left: rect.right + gap };
    case "bottom":
    default:       return { top: rect.bottom + gap, left: rect.left };
  }
}
type TourPlacementLocal = "top" | "bottom" | "left" | "right" | "center";
