import type { ReactNode } from "react";
import type { ZoneDrag } from "./useZoneDrag";

/**
 * 拖着的那张纸本身。
 *
 * 🚨 position: fixed。卡片住在会滚动的区里，原地位移会被父级的 overflow 裁掉，
 * 拖到区外就消失了——见 useZoneDrag 顶上的注释。
 *
 * 歪一点、抬高一点：手里拿着的那张和躺在板上的那些要看得出区别，否则拖的过程中
 * 屏幕上会有两张一模一样的纸，谁也说不清哪张是活的。
 */
export function DragGhost({ drag, children }: { drag: ZoneDrag | null; children: ReactNode }) {
  if (!drag) return null;
  return (
    <div
      aria-hidden
      style={{
        position: "fixed",
        left: drag.left,
        top: drag.top,
        width: drag.width,
        pointerEvents: "none",
        zIndex: 60,
        transform: "rotate(-2.5deg) scale(1.03)",
        filter: "drop-shadow(0 8px 16px rgb(0 0 0 / 0.18))",
      }}
    >
      {children}
    </div>
  );
}
