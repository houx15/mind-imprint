import { useMemo, useState } from "react";
import type { ReadingBlock } from "../api/readings";

/**
 * 文章顶上的查找与跳转。
 *
 * 产品负责人 2026-09-12：「在文章顶部增加关键词查找和段落跳转入口，为每段标注
 * 段落编号，方便定位内容。」段号在 Annotate 那边（`data-n` + CSS ::before）。
 *
 * 🚨 查到的词**不往正文里塞高亮**。正文里已经有她自己的标注（`<mark>`），而
 * 标注的锚点是这一段文本里的字节偏移 —— 为了查找再插一层标签，等于在她的
 * 标注底下动地基。这里只做一件事：告诉她命中在第几段，点一下滚过去。
 * 定位比高亮更贴近她的问题（「方便定位内容」）。
 */
export function ArticleFinder({
  blocks,
  onJump,
}: {
  blocks: ReadingBlock[];
  /** 滚到这一段（ReadingRoom 里那个按 data-block-id 找元素的函数）。 */
  onJump: (blockId: string) => void;
}) {
  const [q, setQ] = useState("");
  const [at, setAt] = useState(0);

  const hits = useMemo(() => {
    const needle = q.trim().toLowerCase();
    if (!needle) return [];
    const out: { n: number; id: string }[] = [];
    blocks.forEach((b, i) => {
      if (b.text.toLowerCase().includes(needle)) out.push({ n: i + 1, id: b.id });
    });
    return out;
  }, [blocks, q]);

  function jumpTo(i: number) {
    if (hits.length === 0) return;
    const next = ((i % hits.length) + hits.length) % hits.length;
    setAt(next);
    onJump(hits[next]!.id);
  }

  /** 「跳到第 N 段」。数字之外的输入一律不动作。 */
  function jumpToOrdinal(raw: string) {
    const n = Number(raw.trim());
    if (!Number.isInteger(n) || n < 1 || n > blocks.length) return;
    onJump(blocks[n - 1]!.id);
  }

  return (
    <div className="mk-article-finder">
      <input
        className="mk-article-finder__q"
        value={q}
        onChange={(e) => {
          setQ(e.target.value);
          setAt(0);
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") jumpTo(hits.length === 0 ? 0 : at === 0 && q ? 0 : at + 1);
        }}
        placeholder="查找关键词"
        aria-label="查找关键词"
      />
      {q.trim() !== "" && (
        <span className="mk-article-finder__count">
          {hits.length === 0 ? "没有找到" : `第 ${at + 1} / ${hits.length} 处`}
        </span>
      )}
      {hits.length > 0 && (
        <>
          <button type="button" onClick={() => jumpTo(at - 1)} aria-label="上一处">
            上一处
          </button>
          <button type="button" onClick={() => jumpTo(at + 1)} aria-label="下一处">
            下一处
          </button>
        </>
      )}
      <span className="mk-article-finder__sep" aria-hidden="true" />
      <input
        className="mk-article-finder__n"
        inputMode="numeric"
        placeholder="段号"
        aria-label="跳到第几段"
        onKeyDown={(e) => {
          if (e.key === "Enter") jumpToOrdinal((e.target as HTMLInputElement).value);
        }}
      />
      <span className="mk-article-finder__total">共 {blocks.length} 段</span>
    </div>
  );
}
