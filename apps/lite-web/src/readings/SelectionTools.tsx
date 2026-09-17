import { createPortal } from "react-dom";
import type { ReadingBlockTool } from "@lite/api/readingRoom";

/**
 * SelectionTools —— 她在正文里划出几个字之后，贴着那几个字出现的一条工具条。
 *
 * # 为什么有它
 *
 * 产品负责人 2026-09-18 逐字：
 *
 *   > when I select a word or a sentence, my selection disappears and cannot
 *   > ask a word's meaning
 *
 * 两件事，一个原因。划选原来只做一件事：把那几个字变成一条引用，然后**清掉
 * 选区**（`removeAllRanges`）。于是她划一个词，屏幕上那个词当场不再高亮，而她
 * 想做的那件事（查这个词是什么意思）没有任何入口 —— 查词和语法当时只能从段落
 * 工具条进去，再在面板里**重新**点一次那个词。
 *
 * 划选是一个还没说完的动作。选区就是「对哪几个字」，工具条就是「做什么」。
 *
 * # 摆哪几件工具由服务端的目录决定
 *
 * 不写死。`subject === "word"` 的工具（查词）只在她划的是**一个词**时出现，
 * `subject === "sentence"` 的（语法）只在她划的**不止一个词**时出现 —— 划了
 * 半句话去「查词」，讲出来的不是一张词卡。中文文章上这两件工具本来就不在目录里
 * （它们是 Lang "en" 的），那时候这条工具条一个按钮都没有，整个不渲染。
 */

/** 她划的这几个字算不算「一个词」。英文按空白切；中文没有词边界，四个字以内算。 */
export function isOneWord(quote: string): boolean {
  const q = quote.trim();
  if (!q) return false;
  if (/\s/.test(q)) return false;
  // 连字符和撇号是词的一部分（well-being、don't）；句读不是。
  return [...q].length <= 4 || /^[A-Za-z][A-Za-z'’-]*$/.test(q);
}

/** 这条工具条上该有哪几件工具。一件都没有就不该渲染它。 */
export function toolsForSelection(tools: ReadingBlockTool[], quote: string): ReadingBlockTool[] {
  const oneWord = isOneWord(quote);
  return tools.filter((t) =>
    t.subject === "word" ? oneWord : t.subject === "sentence" ? !oneWord : false,
  );
}

export function SelectionTools({
  quote,
  at,
  tools,
  onPick,
  onDismiss,
}: {
  quote: string;
  /** 选区下边缘的中点，viewport 坐标。 */
  at: { x: number; y: number };
  tools: ReadingBlockTool[];
  onPick: (toolId: string) => void;
  onDismiss: () => void;
}) {
  const picks = toolsForSelection(tools, quote);
  if (picks.length === 0) return null;
  // position: fixed + portal：正文在一个会滚动的容器里，工具条不该被它裁掉。
  return createPortal(
    <div
      className="mk-seltools"
      role="group"
      aria-label="对选中的文字"
      style={{ left: Math.max(8, Math.min(at.x, window.innerWidth - 8)), top: at.y + 8 }}
      // 🚨 按下去不能让浏览器先把选区收掉 —— 选区就是「对哪几个字」。
      onMouseDown={(e) => e.preventDefault()}
    >
      {picks.map((t) => (
        <button key={t.id} type="button" className="mk-seltools__btn" onClick={() => onPick(t.id)}>
          {t.label}
        </button>
      ))}
      <button type="button" className="mk-seltools__btn mk-seltools__btn--quiet" onClick={onDismiss}>
        关闭
      </button>
    </div>,
    document.body,
  );
}
