import { useState } from "react";
import { ChevronDown, ChevronUp, GripVertical } from "lucide-react";
import { Icon } from "@/ui";
import { OUTLINE_KIND_DEPTH, outlineKindLabel, outlineKindOf } from "./outlineKind";
import type { OutlineMoveMode } from "./outlineMove";
import type { WritingOutlineItem } from "../api/writingRoom";

/**
 * FlowOrderList —— 行文那一步的顺序清单：正文那几块，从上到下就是文章的顺序。
 *
 * # 它替掉了什么（同事 2026-09-22 的意见 4 与 5）
 *
 * 原来这一屏有两块：一张思维导图（说明文字写着「拖一条到另一条旁边」），
 * 底下一张清单，每行左边一个 `GripVertical` 图标、右边一个下拉「打算怎么证明」。
 *
 * 两块都是坏的：
 *
 *   1. **那个抓手是画上去的。** 那一行没有任何一个拖动事件 —— 图标承诺了
 *      一件它做不到的事。同事：「这三个论点的顺序无法拖动改变」。
 *      顺序其实只能在上面那张图里改，而且要拖到卡片的**上三分之一**才算换
 *      顺序、拖到别处就变成挂进子层 —— 一条没人讲过的规矩。
 *   2. **那个下拉喂给了空气。** `writing_outline.method` 存下来之后，
 *      没有任何一条提示词读它（全仓 grep 只有存和取两处）。她从十一个
 *      「论证方法」里挑一个，那一下什么都不影响。产品负责人 2026-09-22：
 *      「this is awkward this selection... how would you think that students
 *      can use this to write? they would hate this.」
 *
 * 所以这一步只剩一件事：**把正文那几块排成文章的顺序**。一列从上往下的清单
 * 本来就是文章的样子，比一张树状图更贴近「行文」这两个字。
 *
 * # 两条路都给
 *
 * 拖动是主路。但 HTML5 的拖放在触摸屏上不工作，键盘也走不了，所以每一行
 * 还有一对上下按钮 —— 它们不是「多余的按钮」，它们是这件事在另一半设备上
 * 唯一的做法。
 */

/** 能排顺序的是正文那一层（深度 1）：分论点、反方观点，记叙文的场景、转折、感悟。 */
function isFlowBlock(item: WritingOutlineItem): boolean {
  return OUTLINE_KIND_DEPTH[outlineKindOf(item)] === 1;
}

export function FlowOrderList({
  items,
  onMove,
}: {
  items: WritingOutlineItem[];
  onMove: (draggedId: string, targetId: string, mode: OutlineMoveMode) => void;
}) {
  const [dragging, setDragging] = useState<string | null>(null);
  const [over, setOver] = useState<{ id: string; mode: "before" | "after" } | null>(null);

  const ordered = items.slice().sort((a, b) => a.position - b.position);
  const blocks = ordered.filter(isFlowBlock);

  /** 这一块底下挂着的东西 —— 排顺序的时候它们跟着走，所以要看得见。 */
  function childrenOf(id: string): WritingOutlineItem[] {
    const at = ordered.findIndex((n) => n.id === id);
    if (at < 0) return [];
    const head = ordered[at];
    if (!head) return [];
    const out: WritingOutlineItem[] = [];
    for (let i = at + 1; i < ordered.length; i++) {
      const n = ordered[i];
      if (!n || n.depth <= head.depth) break;
      out.push(n);
    }
    return out;
  }

  if (blocks.length === 0) return null;

  return (
    <ul className="flex list-none flex-col gap-2">
      {blocks.map((o, i) => {
        const kids = childrenOf(o.id);
        const isOver = over?.id === o.id && dragging !== null && dragging !== o.id;
        return (
          <li
            key={o.id}
            draggable
            onDragStart={(e) => {
              setDragging(o.id);
              e.dataTransfer.effectAllowed = "move";
              // Firefox 不设 data 就不开始拖。
              e.dataTransfer.setData("text/plain", o.id);
            }}
            onDragEnd={() => {
              setDragging(null);
              setOver(null);
            }}
            onDragOver={(e) => {
              if (dragging === null || dragging === o.id) return;
              e.preventDefault();
              e.dataTransfer.dropEffect = "move";
              const box = e.currentTarget.getBoundingClientRect();
              setOver({ id: o.id, mode: e.clientY - box.top < box.height / 2 ? "before" : "after" });
            }}
            onDragLeave={() => setOver((s) => (s?.id === o.id ? null : s))}
            onDrop={(e) => {
              e.preventDefault();
              const mode = over?.id === o.id ? over.mode : "after";
              const dragged = dragging;
              setDragging(null);
              setOver(null);
              if (dragged && dragged !== o.id) onMove(dragged, o.id, mode);
            }}
            className="flex cursor-grab items-start gap-3 rounded-mk-md border px-3 py-2"
            style={{
              borderColor: isOver ? "var(--mk-accent-500)" : "var(--mk-border)",
              background: dragging === o.id ? "var(--mk-accent-50)" : "var(--mk-surface)",
              // 落点在上边还是下边，用一条边说出来 —— 否则她松手之前不知道
              // 它会落到哪里去。
              boxShadow: isOver
                ? over?.mode === "before"
                  ? "inset 0 3px 0 0 var(--mk-accent-500)"
                  : "inset 0 -3px 0 0 var(--mk-accent-500)"
                : undefined,
            }}
          >
            <Icon icon={GripVertical} size={14} className="mt-1 shrink-0 text-mk-faint" />
            <span className="flex min-w-0 flex-1 flex-col gap-1">
              <span className="text-mk-label text-mk-accent-700">
                {outlineKindLabel(outlineKindOf(o))}
              </span>
              <span className="text-mk-body text-mk-ink">{o.text}</span>
              {kids.length > 0 && (
                <span className="flex flex-col gap-0.5 pt-1">
                  {kids.map((k) => (
                    <span key={k.id} className="truncate text-mk-small text-mk-muted">
                      {outlineKindLabel(outlineKindOf(k))} · {k.text}
                    </span>
                  ))}
                </span>
              )}
            </span>
            <span className="flex shrink-0 items-center gap-1">
              <button
                type="button"
                aria-label={`把「${o.text}」往前挪`}
                disabled={i === 0}
                onClick={() => {
                  const prev = blocks[i - 1];
                  if (prev) onMove(o.id, prev.id, "before");
                }}
                className="rounded-mk-sm p-1 text-mk-muted transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 hover:text-mk-accent-700 disabled:opacity-30 disabled:hover:bg-transparent"
              >
                <Icon icon={ChevronUp} size={16} />
              </button>
              <button
                type="button"
                aria-label={`把「${o.text}」往后挪`}
                disabled={i === blocks.length - 1}
                onClick={() => {
                  const next = blocks[i + 1];
                  if (next) onMove(o.id, next.id, "after");
                }}
                className="rounded-mk-sm p-1 text-mk-muted transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 hover:text-mk-accent-700 disabled:opacity-30 disabled:hover:bg-transparent"
              >
                <Icon icon={ChevronDown} size={16} />
              </button>
            </span>
          </li>
        );
      })}
    </ul>
  );
}
