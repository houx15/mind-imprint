import { useRef, useState } from "react";

/**
 * CoachBoards —— 印记 递到她手上的两块**板**。
 *
 * # 为什么是板，不是表单
 *
 * 产品负责人 2026-09-10：
 *
 *   > the guiding, now only texts, or questions/choices/text input, is not
 *   > enough. we have great 透镜 interaction. and we can be richer.
 *   > we can make our ai be able to call out an interactive part and then back
 *
 * 结构没问题（印记 排步骤、领着走），缺的是**她手上能摆的东西**。透镜是唯一
 * 一件，而它一篇文章只用一两次。
 *
 * 🚨 而「交互」在这个产品里有确定的意思（2026-09-04）：**一块能用手摆的板**，
 * 拖是主要动词。所以这两块都是拖，不是勾选框、不是下拉菜单 —— 2026-09-01 被
 * 否掉的正是那种「form-like things」。
 *
 * # 两块板
 *
 *   标注板   文章里的几句话，各自拖进一个角色格子（主张/证据/限制/背景/对比）。
 *            这是一次**不问「你懂了吗」的理解检查**：贴不出来就是没读懂，
 *            而她一个字都不用写。
 *   生词板   这一段里的几个词，各自拖进「认识 / 不确定 / 不认识」。
 *            板上**没有释义** —— 她分完之后，印记 下一轮只讲后两格里的词。
 *            「讲哪几个词」这件事因此由她决定，不由模型猜。
 *
 * # 拖，而且不只能拖
 *
 * 拖用的是 pointer 事件（触屏和鼠标同一套代码，HTML5 的 dragstart 在手机上
 * 根本不触发）。但**点也能用**：先点一张卡片选中它，再点一个格子。
 * 理由不是无障碍的教条 —— 是一只手扶着手机的人拖不动一张卡片。
 * 两条路径的最终状态完全一样。
 *
 * # 板不判对错
 *
 * 和 CoachCard 同一条：没有 ✓、没有 ✗、没有分数。她摆完，摆的结果原样回灌给
 * 印记，由 印记 在下一轮里回应她 —— 「第 5 段那句我会标成限制而不是证据，
 * 因为……」。那是教，不是判分。所以这个文件里没有任何地方知道正确答案，
 * 服务端也不发。
 */

/** 一张待分类的卡片：它显示什么，以及回灌时它是谁。 */
export type BoardItem = {
  /** 板内唯一。 */
  id: string;
  /** 卡片上显示的字。 */
  text: string;
  /** 它出自哪一段（用来在回灌里说「第几段」）。 */
  blockId: string;
};

/** 她摆完之后的状态：itemId → 格子名。没摆的那些不在表里。 */
export type BoardPlacement = Record<string, string>;

// ---------------------------------------------------------------------------
// 拖 + 点，一套状态
// ---------------------------------------------------------------------------

/** 超过这么多像素才算「拖」，之内都算「点」。见 isDrag。 */
export const DRAG_SLOP = 6;

/**
 * 这一下是「拖」还是「点」。
 *
 * 🚨 它是一个独立的纯函数，不是写在事件处理里的一行，因为**它是这块板上唯一
 * 一处「读代码看不出对错」的逻辑**，而 jsdom 里没有 PointerEvent，事件那条路
 * 根本测不了。
 *
 * 原来那一行写的是「动了就算拖」（`> 0`）。手指按下去总会动一两个像素，鼠标
 * 也一样，于是绝大多数「点一下选中」都被当成一次拖动；而那次拖动的落点还在
 * 原地（未分类那一堆的容器 `data-board-bin=""`），于是卡片被「放回」原处，
 * 屏幕上什么都没发生。模拟学生走查里这块板出现了 52 步、她摆了 51 次，
 * 四张卡片一张都没进格子 —— 看上去像她不会用，其实是那一行。
 *
 * 距离从**按下的那个点**算，不累加每一帧的位移：累加的话，慢慢挪一圈再回到
 * 原处也会被算成拖了很远。
 */
export function isDrag(from: { x: number; y: number }, to: { x: number; y: number }): boolean {
  return Math.hypot(to.x - from.x, to.y - from.y) > DRAG_SLOP;
}

/**
 * 一块板的公共行为：选中一张卡、把它放进一个格子、以及拖动时的落点判定。
 *
 * 落点是用 `elementFromPoint` 找的，而不是靠每个格子各挂一个 pointerenter：
 * 拖动过程中指针被 `setPointerCapture` 捕获在卡片上，格子收不到任何
 * pointer 事件 —— 不捕获的话，手指一离开卡片这次拖动就断了。
 */
function useBoard(initial: BoardPlacement = {}) {
  const [placed, setPlaced] = useState<BoardPlacement>(initial);
  const [picked, setPicked] = useState<string | null>(null);
  const [hoverBin, setHoverBin] = useState<string | null>(null);
  const [dragging, setDragging] = useState<string | null>(null);
  const [ghost, setGhost] = useState<{ x: number; y: number } | null>(null);
  // 按下去的那个点，用来判断这到底是一次「点」还是一次「拖」。
  const downAtRef = useRef<{ x: number; y: number } | null>(null);
  const movedRef = useRef(false);

  function binAt(x: number, y: number): string | null {
    const el = document.elementFromPoint(x, y);
    const bin = el?.closest?.("[data-board-bin]");
    return bin ? bin.getAttribute("data-board-bin") : null;
  }

  function place(itemId: string, bin: string | null) {
    setPlaced((prev) => {
      const next = { ...prev };
      if (bin) next[itemId] = bin;
      else delete next[itemId];
      return next;
    });
  }

  function onItemPointerDown(itemId: string, e: React.PointerEvent<HTMLElement>) {
    // 只接主键/单指。右键和第二根手指不该开始一次拖动。
    if (e.button !== 0) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    movedRef.current = false;
    downAtRef.current = { x: e.clientX, y: e.clientY };
    setDragging(itemId);
    setGhost({ x: e.clientX, y: e.clientY });
  }

  function onItemPointerMove(e: React.PointerEvent<HTMLElement>) {
    if (!dragging) return;
    // 抖动不算拖动，门槛见 isDrag。
    const from = downAtRef.current;
    if (from && isDrag(from, { x: e.clientX, y: e.clientY })) movedRef.current = true;
    setGhost({ x: e.clientX, y: e.clientY });
    setHoverBin(binAt(e.clientX, e.clientY));
  }

  function onItemPointerUp(itemId: string, e: React.PointerEvent<HTMLElement>) {
    if (!dragging) return;
    e.currentTarget.releasePointerCapture?.(e.pointerId);
    const bin = binAt(e.clientX, e.clientY);
    setDragging(null);
    setGhost(null);
    setHoverBin(null);
    if (movedRef.current) {
      // 拖到格子外面松手 = 把它拿回来，不是把它丢进最近的格子。
      place(itemId, bin);
      setPicked(null);
      return;
    }
    downAtRef.current = null;
    // 没动过 = 这是一次点击：选中 / 取消选中。
    setPicked((prev) => (prev === itemId ? null : itemId));
  }

  /** 点一个格子：把选中的那张放进去。没有选中的就什么也不做。 */
  function onBinClick(bin: string) {
    if (!picked) return;
    place(picked, bin);
    setPicked(null);
  }

  return { placed, picked, hoverBin, dragging, ghost, place, setPicked, onItemPointerDown, onItemPointerMove, onItemPointerUp, onBinClick };
}

// ---------------------------------------------------------------------------
// 板
// ---------------------------------------------------------------------------

export function CoachBoard({
  items,
  bins,
  itemLabel,
  submitLabel,
  busy,
  onSubmit,
}: {
  items: BoardItem[];
  /** 格子的名字，按屏幕顺序。 */
  bins: string[];
  /** 未分类那一堆上面的一行说明。 */
  itemLabel: string;
  submitLabel: string;
  busy?: boolean;
  onSubmit: (placement: BoardPlacement) => void;
}) {
  const b = useBoard();
  const loose = items.filter((it) => !b.placed[it.id]);
  const done = loose.length === 0;

  return (
    <div className="mk-board">
      <div className="mk-board__loose" data-board-bin="">
        <p className="mk-board__hint">
          {loose.length > 0 ? itemLabel : "都摆好了。"}
        </p>
        <div className="mk-board__chips">
          {loose.map((it) => (
            <Chip
              key={it.id}
              item={it}
              picked={b.picked === it.id}
              dragging={b.dragging === it.id}
              board={b}
            />
          ))}
        </div>
      </div>

      <div className="mk-board__bins">
        {bins.map((bin) => {
          const inside = items.filter((it) => b.placed[it.id] === bin);
          return (
            <div
              key={bin}
              data-board-bin={bin}
              onClick={() => b.onBinClick(bin)}
              className={`mk-board__bin${b.hoverBin === bin ? " is-over" : ""}${b.picked ? " is-armed" : ""}`}
            >
              <span className="mk-board__binname">{bin}</span>
              <div className="mk-board__chips">
                {inside.map((it) => (
                  <Chip
                    key={it.id}
                    item={it}
                    picked={b.picked === it.id}
                    dragging={b.dragging === it.id}
                    board={b}
                  />
                ))}
              </div>
            </div>
          );
        })}
      </div>

      <div className="mk-board__foot">
        <button
          type="button"
          disabled={busy || !done}
          onClick={() => onSubmit(b.placed)}
          className="mk-board__submit"
        >
          {submitLabel}
        </button>
      </div>

      {/* 跟着手指走的那一张。`position: fixed`，所以它不受任何祖先的
          overflow 裁剪 —— 板本身在一个会滚动的面板里。 */}
      {b.dragging && b.ghost && (
        <span
          aria-hidden="true"
          className="mk-board__ghost"
          style={{ left: b.ghost.x, top: b.ghost.y }}
        >
          {items.find((it) => it.id === b.dragging)?.text}
        </span>
      )}
    </div>
  );
}

function Chip({
  item,
  picked,
  dragging,
  board,
}: {
  item: BoardItem;
  picked: boolean;
  dragging: boolean;
  board: ReturnType<typeof useBoard>;
}) {
  return (
    <button
      type="button"
      aria-pressed={picked}
      className={`mk-board__chip${picked ? " is-picked" : ""}${dragging ? " is-dragging" : ""}`}
      onPointerDown={(e) => board.onItemPointerDown(item.id, e)}
      onPointerMove={board.onItemPointerMove}
      onPointerUp={(e) => board.onItemPointerUp(item.id, e)}
      onPointerCancel={(e) => board.onItemPointerUp(item.id, e)}
      // 点击已经由 pointerup 处理了；再让浏览器合成一次 click，会连着
      // 冒泡到格子上，把刚选中的那张立刻放进去。
      onClick={(e) => e.stopPropagation()}
    >
      {item.text}
    </button>
  );
}
