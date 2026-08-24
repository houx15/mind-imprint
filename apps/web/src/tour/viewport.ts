export interface ViewportPoint { top: number; left: number }
export interface ViewportSize { w: number; h: number }
export interface ViewportBounds { vw: number; vh: number }

/**
 * Clamp a box's top-left position so it stays fully within the viewport, with
 * `margin` px of breathing room on every edge. Pure math — no DOM — so the
 * 印记 tour popover (and its tests) can reason about placement without a real
 * layout. Never returns a coordinate below `margin`, even in the degenerate
 * case where the box itself is bigger than the viewport minus margins (it
 * will then overflow past the far edge rather than go negative).
 */
export function clampToViewport(
  pos: ViewportPoint,
  size: ViewportSize,
  viewport: ViewportBounds,
  margin = 8,
): ViewportPoint {
  const maxLeft = Math.max(margin, viewport.vw - size.w - margin);
  const maxTop = Math.max(margin, viewport.vh - size.h - margin);
  return {
    left: Math.min(Math.max(pos.left, margin), maxLeft),
    top: Math.min(Math.max(pos.top, margin), maxTop),
  };
}
