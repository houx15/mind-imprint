import { useCallback, useRef, useState } from "react";

/**
 * useCanvasDrag —— 在一块自由的板上挪一张纸，以及把一张纸放到另一张上。
 *
 * 和 useZoneDrag 是两件事，别合并：那个回答「落进了哪一格」（格子是给定的几块
 * 地方），这个回答「落在了哪个坐标」（位置本身就是她的判断）。两种板产品负责人
 * 2026-09-03 的四张图里都有——四象限是前者，想法画布是后者。
 *
 * 🚨 落下才写库。拖的过程里每一帧发一次请求没有必要，而且她还没决定放哪儿。
 *
 * 🚨 没挪动就是一次点击。对她来说「碰一下这张纸」本来就是一件事，两个动作合在
 * 一个手势里；分成"拖把手 + 点正文"只会让触屏上两个都不好按。
 */

export interface CanvasDragApi {
  /** 正在被拖的那张。null = 没人在拖。 */
  dragging: string | null;
  /** 拖过程中悬在哪张别的纸上（放上去 = 合并）。 */
  over: string | null;
  /** 纸的 onPointerDown。 */
  start: (id: string, at: { x: number; y: number }, e: React.PointerEvent) => void;
  /** 每张纸的 ref，用来做"放到另一张上"的命中判断。 */
  itemRef: (id: string) => (el: HTMLElement | null) => void;
}

export function useCanvasDrag(opts: {
  /** 板子本身。坐标相对它算。 */
  boardRef: React.RefObject<HTMLElement | null>;
  width: number;
  height: number;
  /** 板子当前的高度，用来夹住下边界。 */
  boardHeight: number;
  /** 拖动过程中的预览位置。只改内存，不写库。 */
  onPreview: (id: string, x: number, y: number) => void;
  /** 松手，落在空地上。这里才写库。 */
  onDrop: (id: string, x: number, y: number) => void;
  /**
   * 松手，落在另一张纸上。
   *
   * 🚨 一并把**出发点**给回去。落在别人身上不是一次移动，是一次提议（合并要先
   * 问一句），所以这张纸得回到原位——不回去的话它就停在目标那张底下，被完全
   * 盖住。截图里就是这样：三条办法只看得见两条，第三条压在别人后面，看起来像
   * 刚才那一条凭空没了。
   */
  onDropOn: (id: string, targetId: string, from: { x: number; y: number }) => void;
  onTap?: (id: string) => void;
}): CanvasDragApi {
  const [dragging, setDragging] = useState<string | null>(null);
  const [over, setOver] = useState<string | null>(null);
  const items = useRef(new Map<string, HTMLElement>());
  const cb = useRef(opts);
  cb.current = opts;

  const itemRef = useCallback(
    (id: string) => (el: HTMLElement | null) => {
      if (el) items.current.set(id, el);
      else items.current.delete(id);
    },
    [],
  );

  const start = useCallback(
    (id: string, at: { x: number; y: number }, e: React.PointerEvent) => {
      if (e.button !== 0) return;
      const board = cb.current.boardRef.current;
      if (!board) return;
      const rect = board.getBoundingClientRect();
      const grabX = e.clientX - rect.left - at.x;
      const grabY = e.clientY - rect.top - at.y;
      let moved = false;
      let last = at;

      /** 指针底下有没有别的纸。自己不算。 */
      const hitOther = (x: number, y: number): string | null => {
        let found: string | null = null;
        items.current.forEach((node, other) => {
          if (other === id || found) return;
          const r = node.getBoundingClientRect();
          if (x >= r.left && x <= r.right && y >= r.top && y <= r.bottom) found = other;
        });
        return found;
      };

      setDragging(id);

      const onMove = (ev: PointerEvent) => {
        if (!moved && Math.abs(ev.clientX - e.clientX) + Math.abs(ev.clientY - e.clientY) < 5) return;
        moved = true;
        const { width, height, boardHeight } = cb.current;
        const x = Math.max(0, Math.min(rect.width - width, ev.clientX - rect.left - grabX));
        const y = Math.max(0, Math.min(boardHeight - height, ev.clientY - rect.top - grabY));
        last = { x, y };
        cb.current.onPreview(id, x, y);
        setOver(hitOther(ev.clientX, ev.clientY));
      };

      const onUp = (ev: PointerEvent) => {
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onUp);
        window.removeEventListener("pointercancel", onUp);
        setDragging(null);
        setOver(null);
        if (!moved) {
          cb.current.onTap?.(id);
          return;
        }
        const target = hitOther(ev.clientX, ev.clientY);
        if (target) cb.current.onDropOn(id, target, at);
        else cb.current.onDrop(id, last.x, last.y);
      };

      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onUp);
      // 触屏上系统一接管手势只发 cancel 不发 up。漏了它，纸会一直粘在手上。
      window.addEventListener("pointercancel", onUp);
    },
    [],
  );

  return { dragging, over, start, itemRef };
}
