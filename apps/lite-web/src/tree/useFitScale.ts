import { useEffect, useState, type RefObject } from "react";

/**
 * How much to shrink fixed-size overlay content so it still fits.
 *
 * 两块画布都让**图**随视口缩放（兴趣树的 SVG，原型里世界的星图）而**标签**
 * 保持固定像素。屏幕一矮，图缩了标签没缩，于是它们开始互相压——兴趣树在
 * 900px 高时好好的、到 700px 就读不出来，就是这么来的。
 *
 * This returns a factor in `[min, 1]`, the element's own height against the
 * height the layout was designed at. Multiply fixed sizes by it. It returns 1
 * before the first measurement and wherever `ResizeObserver` is missing, so
 * the design height is always the safe default.
 *
 * 自成一个模块（而不是塞进 `ui.tsx`）因为它是行为不是组件；两块画布都能单独
 * 引它，不必把整套原语拖进来。原型的世界视图也引这里——依赖方向是
 * **原型 → 真代码**，反过来绝不。
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
