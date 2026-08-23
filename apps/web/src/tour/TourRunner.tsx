import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Pebble, Button, Modal } from "@/ui";
import { Markdown } from "@/cards/Markdown";
import { useTour } from "./TourProvider";
import { resolveAnchor } from "./anchors";
import { clampToViewport } from "./viewport";
import { demoMockFor } from "./mocks";
import type { TourController, TourStep } from "./types";

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
    // A demoModal step is always centered on the mock — never spotlight an anchor.
    if (t.step.demoModal) return;
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

  // advance:"action" — advance when the user does the thing on ANY element matching
  // the selector. Delegated at the document in the CAPTURE phase (not bound to one
  // resolved node) for three reasons that a single-element listener gets wrong:
  //   1. "click any card" steps match MANY elements — `document.querySelector`
  //      would bind only the first, so clicking any other card navigated the app
  //      without advancing the tour (found in prod smoke).
  //   2. the target may mount after this effect runs (no poll needed here).
  //   3. the real click lands on a child (a card's title/image), so we match via
  //      `closest(selector)`; capture phase fires before the element's own onClick
  //      (which often navigates away), so the tour advances reliably.
  useEffect(() => {
    if (!t.running || !t.step || t.step.advance !== "action" || !t.step.actionEvent) return;
    const { selector, type } = t.step.actionEvent;
    let done = false;
    const handler = (e: Event) => {
      if (done) return;
      const target = e.target as Element | null;
      if (target && target.closest(selector)) { done = true; t.next(); }
    };
    document.addEventListener(type, handler, true);
    return () => document.removeEventListener(type, handler, true);
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

  // A demoModal step swaps the whole spotlight+popover apparatus for a real
  // `@/ui` Modal hosting a static UI mock — the mock IS the focus, so there is
  // no anchor cutout here. The 印记 explanation + controls reuse the exact
  // same markup as the normal bubble (BubbleHeader/BubbleControls below),
  // placed in the Modal's footer per the ruling — one focused surface, not
  // two overlapping popovers.
  if (t.step.demoModal) {
    const { kind, title } = t.step.demoModal;
    return (
      <Modal
        open
        onClose={t.stop}
        title={title ?? null}
        footer={
          <div className="w-full">
            <BubbleHeader step={t.step} />
            <BubbleControls t={t} isAction={isAction} />
          </div>
        }
      >
        {demoMockFor(kind)}
      </Modal>
    );
  }

  return createPortal(
    // Root is click-through (pointer-events:none) so an `action` step's real
    // anchor stays reachable; only the popover (and, on non-action steps, the
    // click-blocker + spotlight) opt back into catching clicks.
    <div
      className="pointer-events-none fixed inset-0 z-[100]"
      style={{ pointerEvents: "none" }}
      role="dialog"
      aria-modal="true"
      aria-label="新手引导"
    >
      {/* Dim overlay. With a rect, a box-shadow cutout creates the spotlight.
          On `action` steps it is click-through so the highlighted element itself
          receives the click; on non-action steps it swallows anchor clicks
          (highlight-but-inert). */}
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
                pointerEvents: isAction ? "none" : "auto",
              }
            : { background: "rgba(15,23,42,0.55)", pointerEvents: isAction ? "none" : "auto" }
        }
      />
      {/* Full-screen click-blocker for NON-action steps only: keeps the modal
          feel (dimmed area swallows stray clicks). Omitted on `action` steps so
          the whole page stays reachable during "you try it". */}
      {!isAction && (
        <div data-testid="tour-blocker" className="pointer-events-auto absolute inset-0" />
      )}
      {/* 印记 popover. Centered when no anchor; otherwise near the anchor, clamped
          inside the viewport so its controls (下一步/结束/跳过本节) never land
          off-screen when the anchor sits near an edge. Tall content scrolls
          internally rather than overflowing the viewport. */}
      <div
        ref={popRef}
        className="pointer-events-auto fixed w-max min-w-[280px] max-w-[400px] overflow-y-auto rounded-mk-md bg-mk-surface p-4 shadow-mk-lg ring-1 ring-mk-border"
        style={{
          maxHeight: `calc(100vh - ${VIEWPORT_MARGIN * 2}px)`,
          ...(centered
            ? { top: "50%", left: "50%", transform: "translate(-50%,-50%)" }
            : clampState && clampState.rect === rect && clampState.placement === placement
              ? { top: clampState.top, left: clampState.left }
              : positionNear(rect!, t.step.placement)),
        }}
      >
        <BubbleHeader step={t.step} />
        <BubbleControls t={t} isAction={isAction} />
      </div>
    </div>,
    document.body,
  );
}

/** 印记 header + title + explanation Markdown — shared verbatim between the
 *  normal anchored/centered popover and a `demoModal` step's Modal footer, so
 *  the two surfaces never drift apart. */
function BubbleHeader({ step }: { step: TourStep }) {
  return (
    <>
      <div className="mb-2 flex items-center gap-2">
        <span className="mk-pebble-bounce inline-flex">
          <Pebble size={22} />
        </span>
        <span className="text-mk-small font-semibold text-mk-accent-700">印记</span>
      </div>
      {step.title && <div className="mb-1 text-mk-body font-semibold text-mk-ink">{step.title}</div>}
      <div className="text-mk-body leading-relaxed text-mk-ink"><Markdown text={step.text} /></div>
    </>
  );
}

/** 结束/跳过本节/上一步/下一步(-or-action pill) footer controls — shared
 *  verbatim between the normal popover and a `demoModal` step's Modal footer. */
function BubbleControls({ t, isAction }: { t: TourController; isAction: boolean }) {
  return (
    <div className="mt-3 flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
      <Button variant="link" size="sm" onClick={t.stop}>结束</Button>
      <div className="flex items-center gap-2">
        <Button variant="link" size="sm" onClick={t.skipSegment}>跳过本节</Button>
        {t.stepIndex > 0 || t.segmentIndex > 0 ? (
          <Button variant="secondary" size="sm" onClick={t.prev}>上一步</Button>
        ) : null}
        {isAction ? (
          <span className="rounded-mk-full bg-mk-accent-50 px-3 py-1 text-mk-small font-semibold text-mk-accent-700">点亮处可点 →</span>
        ) : (
          <Button variant="primary" size="sm" onClick={t.next}>下一步</Button>
        )}
      </div>
    </div>
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
