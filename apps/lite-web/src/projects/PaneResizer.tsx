import { useEffect, useRef } from "react";

/**
 * 右栏左边那道可以拖的缝。
 *
 * 拖的时候把宽度直接算成"从窗口右边到指针"的距离——不是累加位移。累加会在
 * 指针跑出窗口再回来之后漂掉，而她拖到边上正是常事。
 *
 * setPointerCapture：指针滑到 iframe、滑出窗口，事件仍然回到这道缝上，不会
 * 拖到一半突然断掉。
 */
export function PaneResizer({
  onResize,
  onDoubleClick,
}: {
  onResize: (width: number) => void;
  /** 双击回到默认宽度——拖坏了总要有条退路。 */
  onDoubleClick: () => void;
}) {
  const dragging = useRef(false);

  useEffect(() => {
    return () => {
      // 组件在拖动中途被卸掉（她收起了工具），把全局那两样还回去。
      if (dragging.current) {
        document.body.style.userSelect = "";
        document.body.style.cursor = "";
      }
    };
  }, []);

  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label="调整宽度"
      onDoubleClick={onDoubleClick}
      onPointerDown={(e) => {
        e.preventDefault();
        e.currentTarget.setPointerCapture(e.pointerId);
        dragging.current = true;
        // 拖的时候别把两边的字选中了。
        document.body.style.userSelect = "none";
        document.body.style.cursor = "col-resize";
      }}
      onPointerMove={(e) => {
        if (!dragging.current) return;
        onResize(window.innerWidth - e.clientX);
      }}
      onPointerUp={(e) => {
        dragging.current = false;
        e.currentTarget.releasePointerCapture(e.pointerId);
        document.body.style.userSelect = "";
        document.body.style.cursor = "";
      }}
      className="group absolute left-0 top-0 z-10 hidden h-full w-1.5 -translate-x-1/2 cursor-col-resize lg:block"
    >
      {/* 平时是一条几乎看不见的线，指到才显出来——它不该在她读东西的时候抢注意。 */}
      <div
        className="mx-auto h-full w-0.5 rounded-mk-full transition-colors group-hover:bg-mk-accent-500"
        style={{ background: "transparent" }}
      />
    </div>
  );
}
