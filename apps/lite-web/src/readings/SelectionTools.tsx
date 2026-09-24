import { useLayoutEffect, useRef } from "react";
import { BookmarkPlus, Check, MessageSquareQuote, X } from "lucide-react";
import { Icon } from "@/ui";
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
 * # 摘抄 与 放入对话框（2026-09-22）
 *
 * 产品负责人报的第 1、2 条：
 *
 *   > 可以加个句子划线、加入摘抄本的功能
 *   > 有时候划线是为了辅助阅读，但是一划线(select texts)句子就被收到右下角，
 *   > 还得一个个删除，可以划线后加一个"放入印记对话框"按键
 *
 * 所以这条工具条现在承担四件事里的两件新的，而**划选本身不再自动做任何事**。
 * 这是它最要紧的一条：她划一句只是为了读顺一点，那一句就不该跑到任何地方去。
 * 引用到对话框从「副作用」改成「一颗按钮」。
 *
 * 摘抄是记下来：它在正文上留一道下划线、进「阅读成果」、进报告。不追问为什么
 * —— 产品负责人明确说了不要那一问（「actually I don't think we need this ask」），
 * 想说的时候她自己去阅读成果那一页跟印记说。
 *
 * 🚨 **摘抄是正文定下来之后才有的事**（产品负责人 2026-09-22：「摘抄 is a
 * feature that after the text is decided」）。只有摘要的那一篇（`excerptOnly`）
 * 上这颗按钮不出现 —— 摘抄记的是字偏移，而她随时会把全文粘进来换掉这份摘要，
 * 换完之后那个偏移指的是另一段话。服务端同样拦着（`excerpt_only_source`），
 * 这里只是不把一颗按下去会报错的按钮摆给她看。
 *
 * # 摆哪几件工具由服务端的目录决定
 *
 * 查词 / 语法这两件不写死。`subject === "word"` 的工具（查词）只在她划的是
 * **一个词**时出现，`subject === "sentence"` 的（语法）只在她划的**不止一个词**
 * 时出现 —— 划了半句话去「查词」，讲出来的不是一张词卡。中文文章上这两件工具
 * 本来就不在目录里（它们是 Lang "en" 的）。
 *
 * 放入对话框不跟着目录走：它对任何一段选中的文字都成立，所以这条工具条永远
 * 至少有它和「关闭」两颗，不再有「一个按钮都没有」那种情况。
 */

/** 她划的这几个字算不算「一个词」。英文按空白切；中文没有词边界，四个字以内算。 */
export function isOneWord(quote: string): boolean {
  const q = quote.trim();
  if (!q) return false;
  if (/\s/.test(q)) return false;
  // 连字符和撇号是词的一部分（well-being、don't）；句读不是。
  return [...q].length <= 4 || /^[A-Za-z][A-Za-z'’-]*$/.test(q);
}

const HAN = /[㐀-䶿一-鿿]/;

/**
 * 这一段汉字选区短到**分不出**她要的是「一个词」还是「一小句」。
 *
 * 🚨 2026-09-24：中文这边第一次同时有了词一级（查字）和句一级（白话翻译 /
 * 字词释义 / 句法）的工具，于是 isOneWord 那条「四个字以内算一个词」的线
 * 第一次承重，而它在两种中文上给出的答案是反的：
 *
 *	热岛效应   四个字，是一个词（现代汉语）
 *	俄而雪骤   四个字，是一整句（文言）
 *
 * 光看这四个字分不出来，而工具条上又没有体裁这一位。所以三到四个字的汉字
 * 选区**两级工具都摆**，让她自己说要哪一个 —— 这正是这条工具条存在的理由：
 * 选区是「对哪几个字」，工具条是「做什么」。
 *
 * 一两个字不在其中：一个字的「句法」是没有意义的按钮。
 */
function ambiguousHanSpan(quote: string): boolean {
  const chars = [...quote.trim()];
  if (chars.length < 3 || chars.length > 4) return false;
  return chars.every((c) => HAN.test(c));
}

/** 这条工具条上该有哪几件**服务端目录里的**工具。 */
export function toolsForSelection(tools: ReadingBlockTool[], quote: string): ReadingBlockTool[] {
  const oneWord = isOneWord(quote);
  const alsoPhrase = ambiguousHanSpan(quote);
  return tools.filter((t) =>
    // 整篇那几件不在这里 —— 它们读的是全文，和她划的这几个字无关。
    t.scope === "article"
      ? false
      : t.subject === "word"
        ? oneWord
        : t.subject === "sentence"
          ? !oneWord || alsoPhrase
          : false,
  );
}

export function SelectionTools({
  quote,
  at,
  tools,
  excerpted,
  excerptable,
  onPick,
  onExcerpt,
  onSendToCoach,
  onDismiss,
}: {
  quote: string;
  /** 选区下边缘的中点，viewport 坐标。 */
  at: { x: number; y: number };
  tools: ReadingBlockTool[];
  /** 这一句已经在摘抄本里了。按钮据此换成「已摘抄」并且不可再按 —— 同一句
   *  摘两遍，报告里就是两条一模一样的。 */
  excerpted: boolean;
  /** 这一篇的正文定下来了（不是只有摘要），可以摘抄。false 时整颗按钮不出现。 */
  excerptable: boolean;
  onPick: (toolId: string) => void;
  onExcerpt: () => void;
  onSendToCoach: () => void;
  onDismiss: () => void;
}) {
  const picks = toolsForSelection(tools, quote);
  const barRef = useRef<HTMLDivElement>(null);
  // Measure the whole toolbar, not just its anchor: selections near either
  // edge and the last visible line must keep every action reachable.
  useLayoutEffect(() => {
    function place() {
      const bar = barRef.current;
      if (!bar) return;
      const { width, height } = bar.getBoundingClientRect();
      bar.style.left = `${Math.max(8, Math.min(at.x - width / 2, window.innerWidth - width - 8))}px`;
      const below = at.y + 8;
      bar.style.top = `${Math.max(8, Math.min(below, window.innerHeight - height - 8))}px`;
    }
    place();
    window.addEventListener("resize", place);
    return () => window.removeEventListener("resize", place);
  }, [at.x, at.y, quote, excerptable, excerpted, tools]);
  // position: fixed + portal：正文在一个会滚动的容器里，工具条不该被它裁掉。
  return createPortal(
    <div
      ref={barRef}
      className="mk-seltools"
      role="group"
      aria-label="对选中的文字"
      onKeyDown={(e) => { if (e.key === "Escape") onDismiss(); }}
      style={{ left: Math.max(8, Math.min(at.x, window.innerWidth - 8)), top: at.y + 8 }}
      // 🚨 按下去不能让浏览器先把选区收掉 —— 选区就是「对哪几个字」。
      onMouseDown={(e) => e.preventDefault()}
    >
      {excerptable && (
        <button
          type="button"
          className="mk-seltools__btn mk-seltools__btn--mark"
          onClick={onExcerpt}
          disabled={excerpted}
        >
          <Icon icon={excerpted ? Check : BookmarkPlus} size={14} />
          {excerpted ? "已摘抄" : "摘抄"}
        </button>
      )}
      <button type="button" className="mk-seltools__btn" onClick={onSendToCoach}>
        <Icon icon={MessageSquareQuote} size={14} />
        放入对话框
      </button>
      {picks.map((t) => (
        <button key={t.id} type="button" className="mk-seltools__btn" onClick={() => onPick(t.id)}>
          {t.label}
        </button>
      ))}
      <button type="button" className="mk-seltools__btn mk-seltools__btn--quiet" aria-label="关闭" title="关闭" onClick={onDismiss}>
        <Icon icon={X} size={14} />
      </button>
    </div>,
    document.body,
  );
}
