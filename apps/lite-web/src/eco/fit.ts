import { useEffect, useState, type RefObject } from "react";

/**
 * How much to shrink fixed-size overlay content so it still fits.
 *
 * Both stages in this prototype scale their DIAGRAM with the viewport (the
 * world's star map, the tree's SVG) while their LABELS stay a fixed pixel
 * size. On a short screen the diagram shrinks and the labels do not, so they
 * start colliding — which is exactly how the tree became unreadable at 700px
 * tall while looking fine at 900.
 *
 * This returns a factor in `[min, 1]`, the element's own height against the
 * height the layout was designed at. Multiply fixed sizes by it. It returns 1
 * before the first measurement and wherever `ResizeObserver` is missing, so
 * the design height is always the safe default.
 *
 * Its own module (not `ui.tsx`) because it is behaviour, not a component, and
 * both stages import it without pulling in the primitive set.
 */
export function useFitScale(
  ref: RefObject<HTMLElement | null>,
  designHeight: number,
  min = 0.72,
): number {
  const [scale, setScale] = useState(1);
  useEffect(() => {
    const el = ref.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver((entries) => {
      const h = entries[0]?.contentRect.height ?? designHeight;
      setScale(Math.min(1, Math.max(min, h / designHeight)));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref, designHeight, min]);
  return scale;
}
