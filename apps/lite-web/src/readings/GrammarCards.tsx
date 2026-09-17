import type { ReadingGrammar, ReadingGrammarPart } from "../api/readingRoom";

/**
 * GrammarCards —— 语法那件工具，摆成一句拆开的句子加几张卡。
 *
 * # 它替掉了什么
 *
 * 原来是一大段散文。产品负责人 2026-09-17 逐字：
 *
 *   > grammar, I hope we can be better, like words, become a card. sentence
 *   > composition split? grammar points? with highlighting, knowledge point,
 *   > cases, etc. instead of a large paragraph.
 *
 * 他要的四样东西各有一个位置：
 *
 *   句子拆开 + 高亮   最上面那一句，每一块一个底色（splitSentenceByParts）
 *   每一块是什么      底色一样的一排小卡：成分名 + 那一块 + 它挂在哪
 *   语法点            一张一张的卡：名字、它在这句里干什么、一个新造的例句
 *   这一句的意思      最后一行
 *
 * 高亮能落下去，是因为服务端核对过：每一块的 text 逐字出现在这一句里
 * （reading_block_grammar.go 的 parseGrammarCard）。这里只做逐字查找，不猜。
 */

/** 每一块一个颜色，按出现顺序轮。都是现成的浅色 token —— 深色版由 token 自己管。 */
const PART_TONES = [
  "var(--mk-butter-bg)",
  "var(--mk-lake-bg)",
  "var(--mk-matcha-bg)",
  "var(--mk-peach-bg)",
  "var(--mk-taro-bg)",
];

export type SentenceSegment = { text: string; part: number | null };

/**
 * 把一句话按拆出来的那几块切开，每一段标上它属于第几块（不属于任何一块是 null）。
 *
 * 🚨 这是这个文件里唯一一处「读代码看不出对错」的逻辑，所以它是一个纯函数：
 *
 *   - 一块找不到（服务端核对过，正常不会发生，但老数据、改过的正文会）→ 跳过它，
 *     不能让整句渲染不出来。
 *   - 两块重叠 → 排在前面的那块赢，后一块整块不标（它在卡片列表里照样在）。
 *     一个字同时有两个底色是看不清的。
 *   - 同一块在句子里出现两次 → 标第一次出现、且不和已有块重叠的那一处。
 *   - 切出来的段拼回去**逐字等于原句**。
 */
export function splitSentenceByParts(sentence: string, parts: ReadingGrammarPart[]): SentenceSegment[] {
  const taken: { start: number; end: number; part: number }[] = [];
  parts.forEach((p, idx) => {
    if (!p.text) return;
    let from = 0;
    for (;;) {
      const at = sentence.indexOf(p.text, from);
      if (at < 0) return;
      const end = at + p.text.length;
      const overlaps = taken.some((t) => at < t.end && t.start < end);
      if (!overlaps) {
        taken.push({ start: at, end, part: idx });
        return;
      }
      from = at + 1;
    }
  });
  taken.sort((a, b) => a.start - b.start);
  const out: SentenceSegment[] = [];
  let cursor = 0;
  for (const t of taken) {
    if (t.start > cursor) out.push({ text: sentence.slice(cursor, t.start), part: null });
    out.push({ text: sentence.slice(t.start, t.end), part: t.part });
    cursor = t.end;
  }
  if (cursor < sentence.length) out.push({ text: sentence.slice(cursor), part: null });
  return out;
}

export function GrammarCards({ sentence, grammar }: { sentence: string; grammar: ReadingGrammar }) {
  const segments = splitSentenceByParts(sentence, grammar.parts);
  const tone = (i: number) => PART_TONES[i % PART_TONES.length];
  return (
    <div className="mk-grammar">
      {grammar.backbone && (
        <p className="mk-grammar__backbone">
          <span className="mk-grammar__label">主干</span>
          {grammar.backbone}
        </p>
      )}

      <p className="mk-grammar__sentence">
        {segments.map((seg, i) =>
          seg.part == null ? (
            <span key={i}>{seg.text}</span>
          ) : (
            <span
              key={i}
              className="mk-grammar__hl"
              style={{ background: tone(seg.part) }}
              data-part={seg.part}
            >
              {seg.text}
              <sup className="mk-grammar__hl-n">{seg.part + 1}</sup>
            </span>
          ),
        )}
      </p>

      <ol className="mk-grammar__parts">
        {grammar.parts.map((p, i) => (
          <li key={`${i}-${p.text}`} className="mk-grammar__part">
            <span className="mk-grammar__part-n" style={{ background: tone(i) }}>
              {i + 1}
            </span>
            <span className="mk-grammar__role">{p.role}</span>
            <span className="mk-grammar__part-text">{p.text}</span>
            {p.note && <span className="mk-grammar__part-note">{p.note}</span>}
          </li>
        ))}
      </ol>

      {grammar.points.length > 0 && (
        <>
          <p className="mk-grammar__label mk-grammar__label--block">语法点</p>
          <ul className="mk-word-cards">
            {grammar.points.map((pt) => (
              <li key={pt.name} className="mk-word-card">
                <p className="mk-word-card__head">
                  <span className="mk-grammar__point-name">{pt.name}</span>
                </p>
                {pt.why && <p className="mk-word-card__meaning">{pt.why}</p>}
                {pt.example && (
                  <p className="mk-word-card__example">
                    <span className="mk-word-card__example-en">{pt.example}</span>
                    {pt.exampleZh && <span className="mk-word-card__example-zh">{pt.exampleZh}</span>}
                  </p>
                )}
              </li>
            ))}
          </ul>
        </>
      )}

      {grammar.meaning && (
        <p className="mk-grammar__meaning">
          <span className="mk-grammar__label">句意</span>
          {grammar.meaning}
        </p>
      )}
    </div>
  );
}
