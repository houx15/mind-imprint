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
  /**
   * 悬在那张卡上会发生哪一种：挂到它底下，还是放到它旁边。
   * 空白处是 `"root"`（升到最上层）。
   *
   * 🚨 她必须**在松手之前**看见会发生哪一种。一次看不见结果的拖动，
   * 和 2026-09-12 那个「拖完弹出一个改字框」是同一类问题：
   * 她做了一个动作，屏幕给的反馈不是她预期的那件事。
   */
  hoverMode: OutlineMoveMode | null;
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
  const [hoverMode, setHoverMode] = useState<OutlineMoveMode | null>(null);
  // 同 draggingRef：松手那一刻要读最新值，state 要等渲染。
  const hoverModeRef = useRef<OutlineMoveMode | null>(null);
  const downAtRef = useRef<{ x: number; y: number } | null>(null);
  const movedRef = useRef(false);
  // 同 CoachBoards：ref 在同一个事件循环里就是最新值，state 要等渲染。
  // 一次快速点按的 down 和 up 落在同一个 React 批次里，只读 state 会漏掉。
  const draggingRef = useRef<string | null>(null);
  /** 这一次有没有已经捕获过指针。见 onPointerDown 那段。 */
  const capturedRef = useRef(false);

  function cardAt(x: number, y: number): Element | null {
    const el = document.elementFromPoint(x, y);
    return el?.closest?.("[data-outline-node]") ?? null;
  }

  function nodeAt(x: number, y: number): string | null {
    return cardAt(x, y)?.getAttribute("data-outline-node") ?? null;
  }

  /**
   * 卡片上三分之一 = 放到它旁边（兄弟），其余 = 挂到它底下（孩子）。
   *
   * 为什么是上三分之一而不是一半：**挂到底下是常用的那一个**（她大部分时间
   * 在给一条理由补材料），所以它该占大头；「放到旁边」是纠正一次挂错的动作，
   * 占一条窄边就够，而且窄边更难误触。
   */
  function modeFor(card: Element, y: number): OutlineMoveMode {
    const box = card.getBoundingClientRect();
    return y - box.top < box.height / 3 ? "after" : "child";
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
        hoverModeRef.current = null;
        setDraggingId(id);
        setHoverMode(null);
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
        const card = cardAt(e.clientX, e.clientY);
        const over = card?.getAttribute("data-outline-node") ?? null;
        if (card && over && over !== draggingRef.current) {
          setHoverId(over);
          const mode = modeFor(card, e.clientY);
          hoverModeRef.current = mode;
          setHoverMode(mode);
        } else {
          setHoverId(null);
          // 空白画布上：升到最上层。
          hoverModeRef.current = "root";
          setHoverMode("root");
        }
      },
      onPointerUp(e: React.PointerEvent<HTMLElement>) {
        if (draggingRef.current !== id) return;
        if (capturedRef.current) e.currentTarget.releasePointerCapture?.(e.pointerId);
        capturedRef.current = false;
        const card = movedRef.current ? cardAt(e.clientX, e.clientY) : null;
        const over = card?.getAttribute("data-outline-node") ?? null;
        const dragged = movedRef.current;
        const mode = hoverModeRef.current;
        draggingRef.current = null;
        hoverModeRef.current = null;
        setDraggingId(null);
        setHoverId(null);
        setHoverMode(null);
        downAtRef.current = null;
        if (!dragged) return;
        if (over && over !== id) {
          onMove?.(id, over, card ? modeFor(card, e.clientY) : "child");
          return;
        }
        // 🚨 **落在空白画布上 = 升到最上层。**
        //
        // 这之前是空操作，理由写着「把一次落空的拖动当成一次『挂到最近的那张
        // 上』，是在替她做一个她没做的决定」—— 那句话是对的，但它得出的结论
        // 错了：什么都不做，等于一条理由被挂进子层之后**再也出不来**
        //（同事 2026-09-20 的意见 1）。空白处不是「没有落点」，它是一个真的
        // 落点，而且是唯一一个能把节点提出来的那个。
        if (mode === "root" || !over) onMove?.(id, "", "root");
      },
    };
  }

  return {
    enabled: Boolean(onMove),
    draggingId,
    hoverId,
    hoverMode,
    handlers,
    justDragged: () => movedRef.current,
  };
}
