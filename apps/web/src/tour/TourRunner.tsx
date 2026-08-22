import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Pebble } from "@/ui";
import { Markdown } from "@/cards/Markdown";
import { useTour } from "./TourProvider";
import { resolveAnchor } from "./anchors";
import { clampToViewport } from "./viewport";

const PAD = 8; // spotlight padding around the anchor
const VIEWPORT_MARGIN = 8; // breathing room the popover keeps from every viewport edge

// The clamped position is tagged with the (rect, placement) it was computed
// for. When either changes we fall back to the unclamped `positionNear`
// guess for that one render — until the layout effect below measures the
// freshly-positioned popover and replaces it with a clamped one — instead of
// showing a stale clamp computed for the previous anchor.
type ClampState = { rect: DOMRect; placement: string; top: number; left: number };

export function TourRunner() {
  const t = useTour();
  const [rect, setRect] = useState<DOMRect | null>(null);
  const popRef = useRef<HTMLDivElement | null>(null);
  const [clampState, setClampState] = useState<ClampState | null>(null);

  // Resolve + spotlight the anchor whenever the step changes.
  useEffect(() => {
    // Clear the previous step's spotlight immediately on every step change, so a
    // stale cutout never lingers over an old anchor while the next one resolves
    // (up to a ~2s poll) or while the next step turns out to be centered/anchor-less.
    setRect(null);
    if (!t.running || !t.step) return;
    if (!t.step.anchor || t.step.placement === "center") return;
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
    let cancelled = false;
    const attach = (el: HTMLElement) => {
      const handler = () => t.next();
      el.addEventListener(type, handler, { once: true });
      cleanup = () => el.removeEventListener(type, handler);
    };
    // Attach synchronously when the target is already mounted (common case) so no
    // microtask tick is needed before the listener is live; otherwise poll for it.
    // The `cancelled` guard prevents a late resolve from attaching (and leaking) a
    // listener after this effect has already torn down (e.g. the step changed).
    const immediate = document.querySelector<HTMLElement>(selector);
    if (immediate) attach(immediate);
    else void resolveAnchor(selector).then((el) => { if (!cancelled && el) attach(el); });
    return () => { cancelled = true; cleanup(); };
  }, [t.running, t.step, t]);

  // After the popover renders (unclamped) near its anchor, measure its actual
  // box and clamp it fully inside the viewport. Runs synchronously before
  // paint, so there's no flash of an off-screen popover — the layout effect
  // corrects position in the same commit the browser paints.
  const placement = t.step?.placement ?? "bottom";
  useLayoutEffect(() => {
    if (!rect) return;
    const el = popRef.current;
    if (!el) return;
    const box = el.getBoundingClientRect();
    const { top, left } = clampToViewport(
      { top: box.top, left: box.left },
      { w: box.width, h: box.height },
      { vw: window.innerWidth, vh: window.innerHeight },
      VIEWPORT_MARGIN,
    );
    setClampState({ rect, placement, top, left });
  }, [rect, placement]);

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
      {/* 印记 popover. Centered when no anchor; otherwise near the anchor, clamped
          inside the viewport so its controls (下一步/结束/跳过本节) never land
          off-screen when the anchor sits near an edge. Tall content scrolls
          internally rather than overflowing the viewport. */}
      <div
        ref={popRef}
        className="fixed max-w-[360px] overflow-y-auto rounded-mk-md bg-mk-surface p-4 shadow-mk-lg ring-1 ring-mk-border"
        style={{
          maxHeight: `calc(100vh - ${VIEWPORT_MARGIN * 2}px)`,
          ...(centered
            ? { top: "50%", left: "50%", transform: "translate(-50%,-50%)" }
            : clampState && clampState.rect === rect && clampState.placement === placement
              ? { top: clampState.top, left: clampState.left }
              : positionNear(rect!, t.step.placement)),
        }}
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
