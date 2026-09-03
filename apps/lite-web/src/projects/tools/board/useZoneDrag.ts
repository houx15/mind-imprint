import { useCallback, useRef, useState } from "react";

/**
 * useZoneDrag —— 把一张卡片拖到某一个区里。
 *
 * 产品负责人 2026-09-03，看完线上的十件工具：「you said you have finished
 * gamification, but in my view, no」，附四张图。四张图里的动作是同一个：
 * **用手把一张纸放到一个位置上**——观察拖进「谁 / 需要什么 / 为什么」、
 * 线索拖向 A 或 B、便签拖到一起合并、障碍拖到最左表示最要紧。
 *
 * 🚨 这件事和「选一个下拉框」不是同一件事，虽然存进库的值一样。放的位置是一句
 * 判断，而且它发生在她能把这句话说出口之前——Board.tsx 那条注释已经说过一次，
 * 这里把它抽成所有工具面共用的原语。
 *
 * 为什么用「幽灵」而不是原地 transform：卡片住在会滚动的区里，原地位移会被
 * 父级的 overflow 裁掉，拖到区外就消失了。幽灵是 position:fixed，谁也裁不到它。
 *
 * 为什么用 pointer 事件而不是 HTML5 drag-and-drop：HTML5 那套在触屏上根本
 * 不触发，而这是给中学生用的产品。Board.tsx 走的也是 pointer，同一条路。
 */

export interface ZoneDrag {
  /** 正在被拖的那张卡。 */
  id: string;
  /** 幽灵的屏幕坐标与宽度。 */
  left: number;
  top: number;
  width: number;
  /** 指针此刻悬在哪个区上方；null = 悬在空处，松手等于放回原处。 */
  over: string | null;
}

export interface ZoneDragApi {
  /** null = 没人在拖。 */
  drag: ZoneDrag | null;
  /** 卡片的 onPointerDown。 */
  start: (id: string, e: React.PointerEvent) => void;
  /** 区的 ref：`ref={zoneRef("who")}`。 */
  zoneRef: (zone: string) => (el: HTMLElement | null) => void;
}

export function useZoneDrag(opts: {
  /** 松手。zone = null 表示没落在任何区里。 */
  onDrop: (id: string, zone: string | null) => void;
  /** 没挪动就松手——那是一次点击。不给就什么都不做。 */
  onTap?: (id: string) => void;
}): ZoneDragApi {
  const [drag, setDrag] = useState<ZoneDrag | null>(null);
  const zones = useRef(new Map<string, HTMLElement>());
  // 🚨 回调放进 ref。onDrop 每一帧都是新函数，直接闭进 start 里会让拖到一半的
  // 那次手势拿着上一帧的状态收尾——这类 bug 只在她拖得慢的时候出现。
  const cb = useRef(opts);
  cb.current = opts;

  const zoneRef = useCallback(
    (zone: string) => (el: HTMLElement | null) => {
      if (el) zones.current.set(zone, el);
      else zones.current.delete(zone);
    },
    [],
  );

  const start = useCallback((id: string, e: React.PointerEvent) => {
    // 右键不起拖；卡片里的输入框自己 stopPropagation。
    if (e.button !== 0) return;
    const box = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const grabX = e.clientX - box.left;
    const grabY = e.clientY - box.top;
    const fromX = e.clientX;
    const fromY = e.clientY;
    let moved = false;

    /** 指针底下是哪个区。 */
    const hit = (x: number, y: number): string | null => {
      let found: string | null = null;
      let best = Infinity;
      zones.current.forEach((node, zone) => {
        const r = node.getBoundingClientRect();
        if (x < r.left || x > r.right || y < r.top || y > r.bottom) return;
        // 🚨 面积最小的那个赢。卡片本身也可以注册成一个区（拖到一起 = 合并），
        // 那时候它嵌在大区里面，按注册顺序取会永远选中外面那个大区。
        const area = r.width * r.height;
        if (area < best) {
          best = area;
          found = zone;
        }
      });
      return found;
    };

    setDrag({ id, left: box.left, top: box.top, width: box.width, over: null });

    const onMove = (ev: PointerEvent) => {
      // 手指抖三五像素不算拖。不设这个阈值，每一次点选都会闪一下幽灵。
      if (!moved && Math.abs(ev.clientX - fromX) + Math.abs(ev.clientY - fromY) < 5) return;
      moved = true;
      setDrag({
        id,
        left: ev.clientX - grabX,
        top: ev.clientY - grabY,
        width: box.width,
        over: hit(ev.clientX, ev.clientY),
      });
    };

    const onUp = (ev: PointerEvent) => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      window.removeEventListener("pointercancel", onUp);
      setDrag(null);
      if (!moved) {
        cb.current.onTap?.(id);
        return;
      }
      cb.current.onDrop(id, hit(ev.clientX, ev.clientY));
    };

    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
    // 🚨 pointercancel 也要收尾。触屏上系统一接管手势（下拉刷新、返回手势）
    // 只发 cancel 不发 up——漏了它，幽灵会永远挂在屏幕上。
    window.addEventListener("pointercancel", onUp);
  }, []);

  return { drag, start, zoneRef };
}
