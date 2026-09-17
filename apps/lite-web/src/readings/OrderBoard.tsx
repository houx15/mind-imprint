import { useRef, useState } from "react";
import { DRAG_SLOP, type BoardItem } from "./CoachBoards";

/**
 * OrderBoard —— 排序板：几件事，她按**发生的先后**从上到下排好。
 *
 * 来自同事 2026-09-17 的阅读模块 PRD：新闻报道「搭建事件时间线」，记叙文
 * 「事件卡排序；切换发生顺序／讲述顺序」。服务端只在这两种体裁上发它
 * （`reading_genre.go`）。
 *
 * 卡片开局按**原文顺序**摆（服务端排的，见 `orderByArticle`）——那就是讲述
 * 顺序；每张卡片前面的段号一直留着，她排完之后仍然看得出每件事是在第几段
 * 讲的。「讲述顺序和发生顺序哪里不一样」正是这块板要她看出来的东西。
 *
 * 三条路径，最终状态一样：拖（pointer 事件，触屏和鼠标同一套）、先点一张再点
 * 一个位置、每一行右边的上移/下移。理由和 CoachBoard 一样：一只手扶着手机的人
 * 拖不动一张卡片，而键盘用户根本拖不了。
 *
 * 和 CoachBoard 同一条：**没有对错**。服务端不发正确顺序，这里也不知道。
 */

/** 把 `from` 那一张挪到 `to` 的位置上。越界就原样返回。 */
export function moveItem<T>(list: T[], from: number, to: number): T[] {
  if (from === to || from < 0 || to < 0 || from >= list.length || to >= list.length) return list;
  const out = list.slice();
  const [it] = out.splice(from, 1);
  out.splice(to, 0, it as T);
  return out;
}

/**
 * 她排好之后，这块板变成一段什么话。
 *
 * 🚨 格式和标注板（`composeBoardAnswer`）一样：序号单独一行、原文单独一行。
 * 服务端逐行核对原文并加上 `> ` 前缀 —— 文章的句子不能以她的话的身份进语料。
 */
export function composeOrderAnswer(order: string[], items: BoardItem[]): string {
  const byId = new Map(items.map((it) => [it.id, it]));
  const lines: string[] = [];
  order.forEach((id, i) => {
    const it = byId.get(id);
    if (!it) return;
    lines.push(`第${i + 1}：`);
    lines.push(it.text);
  });
  return lines.join("\n");
}

/** 从她那条作答里读回她排的顺序（item id）。认不出来的行跳过。 */
export function parseOrderAnswer(items: BoardItem[], choice: string): string[] {
  const byText = new Map(items.map((it) => [it.text.trim(), it.id]));
  const lines = choice.split("\n");
  const out: string[] = [];
  for (let i = 0; i + 1 < lines.length; i++) {
    if (!/^第\d+：$/.test(lines[i]!.trim())) continue;
    const id = byText.get(lines[i + 1]!.trim());
    if (id && !out.includes(id)) out.push(id);
  }
  return out;
}

export function OrderBoard({
  items,
  busy = false,
  onSubmit,
}: {
  items: BoardItem[];
  busy?: boolean;
  onSubmit: (order: string[]) => void;
}) {
  const initial = items.map((it) => it.id);
  const [order, setOrder] = useState<string[]>(initial);
  const [picked, setPicked] = useState<string | null>(null);
  const [dragging, setDragging] = useState<string | null>(null);
  const [overIndex, setOverIndex] = useState<number | null>(null);
  const start = useRef<{ id: string; x: number; y: number; moved: boolean } | null>(null);
  const byId = new Map(items.map((it) => [it.id, it]));
  const touched = order.some((id, i) => id !== initial[i]);

  function move(from: number, to: number) {
    setOrder((prev) => moveItem(prev, from, to));
  }

  function indexAt(x: number, y: number): number | null {
    const el = document.elementFromPoint(x, y)?.closest("[data-order-index]");
    if (!el) return null;
    const n = Number(el.getAttribute("data-order-index"));
    return Number.isFinite(n) ? n : null;
  }

  function onPointerDown(id: string, e: React.PointerEvent) {
    if (busy) return;
    start.current = { id, x: e.clientX, y: e.clientY, moved: false };
    (e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId);
  }

  function onPointerMove(e: React.PointerEvent) {
    const s = start.current;
    if (!s) return;
    if (!s.moved && Math.hypot(e.clientX - s.x, e.clientY - s.y) > DRAG_SLOP) {
      s.moved = true;
      setDragging(s.id);
      setPicked(null);
    }
    if (s.moved) setOverIndex(indexAt(e.clientX, e.clientY));
  }

  function onPointerUp(id: string, e: React.PointerEvent) {
    const s = start.current;
    start.current = null;
    setDragging(null);
    setOverIndex(null);
    if (!s || busy) return;
    if (s.moved) {
      const to = indexAt(e.clientX, e.clientY);
      if (to !== null) move(order.indexOf(s.id), to);
      return;
    }
    // 点一下：没有选中的 → 选中它；已经选中了另一张 → 把那一张挪到这里。
    if (picked === null) setPicked(id);
    else if (picked === id) setPicked(null);
    else {
      move(order.indexOf(picked), order.indexOf(id));
      setPicked(null);
    }
  }

  return (
    <div className="mk-board mk-order">
      <p className="mk-board__hint">
        请按事情发生的先后，从上到下排列。可以拖动卡片，也可以先点一张、再点它该去的位置，或者用右侧的箭头。卡片上的段号是文章讲述的顺序。
      </p>
      <ol className="mk-order__list">
        {order.map((id, i) => {
          const it = byId.get(id);
          if (!it) return null;
          return (
            <li
              key={id}
              data-order-index={i}
              className={`mk-order__row${overIndex === i ? " is-over" : ""}${picked !== null && picked !== id ? " is-armed" : ""}`}
            >
              <span className="mk-order__num" aria-hidden="true">
                {i + 1}
              </span>
              <button
                type="button"
                aria-pressed={picked === id}
                className={`mk-board__chip mk-order__chip${picked === id ? " is-picked" : ""}${dragging === id ? " is-dragging" : ""}`}
                onPointerDown={(e) => onPointerDown(id, e)}
                onPointerMove={onPointerMove}
                onPointerUp={(e) => onPointerUp(id, e)}
                onPointerCancel={() => {
                  start.current = null;
                  setDragging(null);
                  setOverIndex(null);
                }}
              >
                {it.where && <span className="mk-board__chipwhere">{it.where}</span>}
                {it.text}
              </button>
              <span className="mk-order__arrows">
                <button
                  type="button"
                  aria-label={`上移第${i + 1}件`}
                  disabled={busy || i === 0}
                  onClick={() => move(i, i - 1)}
                  className="mk-order__arrow"
                >
                  ↑
                </button>
                <button
                  type="button"
                  aria-label={`下移第${i + 1}件`}
                  disabled={busy || i === order.length - 1}
                  onClick={() => move(i, i + 1)}
                  className="mk-order__arrow"
                >
                  ↓
                </button>
              </span>
            </li>
          );
        })}
      </ol>
      <div className="mk-board__foot">
        <button
          type="button"
          disabled={busy || !touched}
          onClick={() => {
            setOrder(initial);
            setPicked(null);
          }}
          className="mk-board__undo"
        >
          恢复原文顺序
        </button>
        <button type="button" disabled={busy} onClick={() => onSubmit(order)} className="mk-board__submit">
          排好了
        </button>
      </div>
      {dragging && (
        <span className="sr-only" aria-live="polite">
          正在移动
        </span>
      )}
    </div>
  );
}

/** 排好之后的那块板，只读：同样的编号、同样的卡片。 */
export function OrderBoardRecap({ items, order }: { items: BoardItem[]; order: string[] }) {
  const byId = new Map(items.map((it) => [it.id, it]));
  return (
    <ol className="mk-board mk-order mk-order__list is-done" aria-label="你排好的顺序">
      {order.map((id, i) => {
        const it = byId.get(id);
        if (!it) return null;
        return (
          <li key={id} className="mk-order__row">
            <span className="mk-order__num" aria-hidden="true">
              {i + 1}
            </span>
            <span className="mk-board__chip is-readonly mk-order__chip">
              {it.where && <span className="mk-board__chipwhere">{it.where}</span>}
              {it.text}
            </span>
          </li>
        );
      })}
    </ol>
  );
}
