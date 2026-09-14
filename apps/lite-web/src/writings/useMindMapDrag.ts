import { useRef, useState } from "react";
import type React from "react";
import { isDrag } from "../readings/CoachBoards";
import type { OutlineMoveMode } from "./outlineMove";

/**
 * 拖动图上的节点：按住一张卡片，把它放到另一张上面，它就挂到那一张底下。
 *
 * 产品负责人 2026-09-12：「写作部分的右侧视觉树无法拖动节点或调整逻辑关系」。
 * 这块图在这之前只能加、改字、删 —— 一条理由挂错了论点，唯一的办法是删掉重说。
 *
 * 和 AGENTS.md 铁律②、[[interaction-means-a-board]] 是同一件事：
 * **板是主角，拖是主要动词。**
 *
 * # 为什么用 pointer 事件，而且点也能用
 *
 * 照抄阅读室那两块板的做法（CoachBoards.tsx，那边已经被走查打磨过三轮）：
 *
 *  - pointer 事件，不是 HTML5 的 dragstart —— 后者在手机上根本不触发。
 *  - `setPointerCapture`：不捕获的话，手指一离开卡片这次拖动就断了。
 *  - 落点用 `elementFromPoint` 找，不给每张卡挂 pointerenter —— 拖动期间
 *    指针被捕获在被拖的那张上，别的卡片收不到任何 pointer 事件。
 *  - 超过 `DRAG_SLOP` 才算拖。手指按下去总会动一两个像素；
 *    「动了就算拖」那一版让阅读室的板 51 次摆放一次都没成功。
 *
 * 🚨 `isDrag` 是从阅读室那个文件导入的**几何判断**，不是文案。
 * [[shared-component-module-scope-copy-2026-09-12]] 那条讲的是「只对一个房间
 * 成立的话不能放共用作用域」；一个「移动超过 6 像素算不算拖」的纯函数
 * 换个房间照样成立，共用它正是该做的事 —— 而且它那 6 像素是被走查打下来的，
 * 再抄一份就是把那次教训扔掉。
 */
export type MindMapDrag = {
  /** 这张图现在能不能拖（没给 onMove 就不能，比如只读的地方）。 */
  enabled: boolean;
  /** 正在被拖的那张卡。没有就是 null。 */
  draggingId: string | null;
  /** 指针此刻悬在哪张卡上（它就是落点）。 */
  hoverId: string | null;
  /** 挂在每张卡片上的那几个事件。 */
  handlers: (id: string) => {
    onPointerDown: (e: React.PointerEvent<HTMLElement>) => void;
    onPointerMove: (e: React.PointerEvent<HTMLElement>) => void;
    onPointerUp: (e: React.PointerEvent<HTMLElement>) => void;
  };
  /**
   * 刚才那一下到底是不是拖。
   *
   * 🚨 卡片上的字本来就点得开（改这一条），而一次拖动几乎总是从字上起手。
   * 不问这一句的话，每拖完一次都会弹出一个「改一下这一条」的框 ——
   * 她刚把一条理由挪到别处，屏幕却问她要不要改它的文字。
   */
  justDragged: () => boolean;
};

export function useMindMapDrag(
  onMove?: (draggedId: string, targetId: string, mode: OutlineMoveMode) => void,
): MindMapDrag {
  const [draggingId, setDraggingId] = useState<string | null>(null);
  const [hoverId, setHoverId] = useState<string | null>(null);
  const downAtRef = useRef<{ x: number; y: number } | null>(null);
  const movedRef = useRef(false);
  // 同 CoachBoards：ref 在同一个事件循环里就是最新值，state 要等渲染。
  // 一次快速点按的 down 和 up 落在同一个 React 批次里，只读 state 会漏掉。
  const draggingRef = useRef<string | null>(null);
  /** 这一次有没有已经捕获过指针。见 onPointerDown 那段。 */
  const capturedRef = useRef(false);

  function nodeAt(x: number, y: number): string | null {
    const el = document.elementFromPoint(x, y);
    const card = el?.closest?.("[data-outline-node]");
    return card ? card.getAttribute("data-outline-node") : null;
  }

  function handlers(id: string) {
    return {
      onPointerDown(e: React.PointerEvent<HTMLElement>) {
        if (!onMove) return;
        // 只接主键/单指。右键和第二根手指不该开始一次拖动。
        if (e.button !== 0) return;
        // 🚨 **这里故意不捕获指针。**
        //
        // 捕获之后，随后那个 click 事件会被派发到**捕获的那个元素**（这张卡），
        // 而不是她真正点到的那个 span —— 于是卡片上「点字改这一条」整个失效。
        // 这不是推出来的：2026-09-12 的真浏览器检查里，拖动本身通过了、
        // 库里也改对了，唯独点字不再打开改字框。jsdom 里看不见这件事，
        // 因为它没有真的 pointer capture。
        //
        // 所以改成**真的开始拖了再捕获**（见 onPointerMove）。一次普通的点击
        // 从头到尾没有捕获发生，click 照常落在 span 上。
        downAtRef.current = { x: e.clientX, y: e.clientY };
        movedRef.current = false;
        capturedRef.current = false;
        draggingRef.current = id;
        setDraggingId(id);
      },
      onPointerMove(e: React.PointerEvent<HTMLElement>) {
        if (!draggingRef.current) return;
        const from = downAtRef.current;
        if (from && isDrag(from, { x: e.clientX, y: e.clientY })) movedRef.current = true;
        if (!movedRef.current) return;
        // 越过门槛的第一帧才捕获：不捕获的话，手指一离开这张卡，
        // 后面的 move 和 up 就都收不到了，这次拖动会断在半路。
        if (!capturedRef.current) {
          e.currentTarget.setPointerCapture(e.pointerId);
          capturedRef.current = true;
        }
        const over = nodeAt(e.clientX, e.clientY);
        setHoverId(over === draggingRef.current ? null : over);
      },
      onPointerUp(e: React.PointerEvent<HTMLElement>) {
        if (draggingRef.current !== id) return;
        if (capturedRef.current) e.currentTarget.releasePointerCapture?.(e.pointerId);
        capturedRef.current = false;
        const over = movedRef.current ? nodeAt(e.clientX, e.clientY) : null;
        draggingRef.current = null;
        setDraggingId(null);
        setHoverId(null);
        downAtRef.current = null;
        // 放在自己身上、或者放在空白处 = 什么都不做。把一次落空的拖动
        // 当成一次「挂到最近的那张上」，是在替她做一个她没做的决定。
        if (over && over !== id) onMove?.(id, over, "child");
      },
    };
  }

  return {
    enabled: Boolean(onMove),
    draggingId,
    hoverId,
    handlers,
    justDragged: () => movedRef.current,
  };
}
