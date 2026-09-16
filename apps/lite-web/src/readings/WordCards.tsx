import type { ReadingWord } from "../api/readingRoom";

/**
 * WordCards —— 关键单词，摆成一组卡片。
 *
 * # 它替掉了什么
 *
 * 这件工具原来产出一段散文（「词 — 意思 — 一个短例子」，三样挤在一行里）。
 * 产品负责人 2026-09-16 的原话：
 *
 *   > please, let's make the words like a set of cards. for each word, we have
 *   > 词性, meaning. explanation, example sentences.
 *
 * 一段散文除了读起来累，还有两件事做不到：它没法**一个词一个词地**看过去，
 * 也没法回到正文里把那个词标出来 —— 要标，就得知道哪几个字是那个词。所以
 * 产物从 markdown 变成了一个数组，而 `term` 是服务端拿回段落里逐字核对过的
 * 那一份写法。荧光笔（`articleHighlight.ts`）和这组卡片读的是同一个字段。
 *
 * # 卡片上的四行，各自在回答一个不同的问题
 *
 *   词 + 词性     这是个什么词
 *   意思          它在**这一句里**是什么意思（不是词典里的第一条）
 *   讲解          为什么它值得学：词根 / 和近义词的差别 / 常见搭配
 *   例句          换一个场景它长什么样（新造的，不是原文那一句）
 *
 * 后两行可能是空的（模型没写满就不凑），空了整行不显示 —— 半行有字比整行
 * 没有更糟。
 */
export function WordCards({ words }: { words: ReadingWord[] }) {
  if (words.length === 0) return null;
  return (
    <ul className="mk-word-cards">
      {words.map((w) => (
        <li key={w.term} className="mk-word-card">
          <p className="mk-word-card__head">
            <span className="mk-word-card__term">{w.term}</span>
            {w.pos && <span className="mk-word-card__pos">{w.pos}</span>}
          </p>
          <p className="mk-word-card__meaning">{w.meaning}</p>
          {w.note && <p className="mk-word-card__note">{w.note}</p>}
          {w.example && (
            <p className="mk-word-card__example">
              <span className="mk-word-card__example-en">{w.example}</span>
              {w.exampleZh && (
                <span className="mk-word-card__example-zh">{w.exampleZh}</span>
              )}
            </p>
          )}
        </li>
      ))}
    </ul>
  );
}
