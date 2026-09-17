import { useState } from "react";
import type { ReadingGrammar, ReadingGrammarPoint, ReadingGrammarSpan } from "../api/readingRoom";

/**
 * GrammarCards —— 语法那件工具的卡片。
 *
 * # 第二版（产品负责人 2026-09-17 晚些）
 *
 *   > grammar includes 词法，句法，时态. we highlight the key words when we want
 *   > to illustrate 词法; we split the 句子成分, what is 主句，what is从句 during
 *   > 句法; we analyzes the 时态 when we need that. I hope we can use different
 *   > colors to highlight different parts.
 *
 * 一句话，分几层看，一层一个标签页，每一层给同一句话换一套颜色：
 *
 *   主从句    不属于任何从句的部分 = 主句（一个颜色），每个从句各一个颜色
 *   句子成分  主语、谓语、宾语……一个成分一个固定颜色（换一句也不变，好记）
 *   词法      关键词加底色加粗，下面逐个说词性、词形
 *   时态      谓语加底色，下面说时态名、为什么用它、一个例句
 *
 * 不是每张卡都有四层（owner 2026-09-17：「only need to highlight those key
 * points」）：模型只交这一句的重点那一两层，空的那层不出标签；只有一层时不摆
 * 标签栏，换成一个层名。每一层下面有图例和逐条说明；句意固定在最下面。
 *
 * 高亮能落下去，是因为服务端核对过：每一段 text 逐字出现在这一句里
 * （reading_block_grammar.go 的 parseGrammarCard）。这里只做逐字查找，不猜。
 *
 * 第一版的卡片（backbone / parts[].role / points，没有 clauses/words/tenses）
 * 走 LegacyGrammar，照原来的样子摆。
 */

type Tone = { bg: string; fg: string };
const tone = (name: string): Tone => ({ bg: `var(--mk-${name}-bg)`, fg: `var(--mk-${name}-fg)` });

const MAIN_TONE = tone("butter");
const CLAUSE_TONES = [tone("lake"), tone("taro"), tone("berry"), tone("matcha")];
const WORD_TONE = tone("matcha");
const TENSE_TONE = tone("peach");
const LEGACY_TONES = ["butter", "lake", "matcha", "peach", "taro"].map(tone);

/** 句子成分的颜色按名字固定，不按出现顺序轮 —— 主语在哪一句里都是同一个颜色。 */
const PART_TONE_BY_LABEL: Record<string, Tone> = {
  主语: tone("lake"),
  谓语: tone("peach"),
  宾语: tone("matcha"),
  表语: tone("matcha"),
  补语: tone("berry"),
  状语: tone("taro"),
  定语: tone("butter"),
  同位语: tone("mist"),
  插入语: tone("mist"),
};
const partTone = (label: string): Tone => {
  // 「间接宾语」「时间状语」这类带前缀的，按结尾那个成分名取色。
  const key = Object.keys(PART_TONE_BY_LABEL).find((k) => label.endsWith(k));
  return (key && PART_TONE_BY_LABEL[key]) || tone("mist");
};

const labelOf = (s: ReadingGrammarSpan) => s.label || s.role || "";

export type SentenceSegment = { text: string; part: number | null };

/**
 * 把一句话按标出来的那几段切开，每一段标上它属于第几段（不属于任何一段是 null）。
 *
 * 🚨 这是这个文件里「读代码看不出对错」的逻辑，所以它是一个纯函数，有测试：
 *
 *   - 一段找不到（老数据、改过的正文）→ 跳过它，不能让整句渲染不出来。
 *   - 两段重叠 → 排在前面的那段赢，后一段整段不标（它在下面的列表里照样在）。
 *     一个字同时有两个底色是看不清的。
 *   - 同一段在句子里出现两次 → 标第一次出现、且不和已有段重叠的那一处。
 *   - 切出来的段拼回去**逐字等于原句**。
 */
export function splitSentenceByParts<T extends { text: string }>(sentence: string, parts: T[]): SentenceSegment[] {
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

/** 这一截有没有字（主句的颜色只给有字的那几截，一个逗号、一个空格不上色）。 */
export function hasWords(text: string): boolean {
  return /[\p{L}\p{N}]/u.test(text);
}

/** 主句 = 从句之外的那几截，按原顺序接起来，每一截去掉两头的逗号和空格。
 *  拿来在列表里写「主句：…」。 */
export function mainClauseText(sentence: string, clauses: { text: string }[]): string {
  return splitSentenceByParts(sentence, clauses)
    .filter((s) => s.part == null && hasWords(s.text))
    .map((s) => s.text.replace(/^[\s,;:，；：]+|[\s,;:，；：]+$/gu, ""))
    .join(" … ");
}

type Layer = "clauses" | "parts" | "words" | "tenses";
const LAYER_NAMES: Record<Layer, string> = {
  clauses: "主从句",
  parts: "句子成分",
  words: "词法",
  tenses: "时态",
};

export function GrammarCards({ sentence, grammar }: { sentence: string; grammar: ReadingGrammar }) {
  const clauses = grammar.clauses ?? [];
  const parts = grammar.parts ?? [];
  const words = grammar.words ?? [];
  const tenses = grammar.tenses ?? [];
  // 第二版的回话没有 backbone/points（服务端 omitempty 会把空的 clauses 也省掉，
  // 所以不能拿「有没有 clauses 这个键」来判）。
  const isV2 = !grammar.backbone && !(grammar.points?.length);
  const layers: Layer[] = [];
  if (clauses.length > 0) layers.push("clauses");
  if (parts.length > 0) layers.push("parts");
  if (words.length > 0) layers.push("words");
  if (tenses.length > 0) layers.push("tenses");
  const [picked, setPicked] = useState<Layer | null>(null);
  const layer = picked && layers.includes(picked) ? picked : layers[0];

  if (!isV2) return <LegacyGrammar sentence={sentence} grammar={grammar} />;

  return (
    <div className="mk-grammar">
      {layers.length > 1 ? (
        <div className="mk-grammar__tabs" role="tablist" aria-label="语法分层">
          {layers.map((l) => (
            <button
              key={l}
              type="button"
              role="tab"
              aria-selected={l === layer}
              className="mk-grammar__tab"
              onClick={() => setPicked(l)}
            >
              {LAYER_NAMES[l]}
            </button>
          ))}
        </div>
      ) : layer ? (
        <p className="mk-grammar__label mk-grammar__label--block">{LAYER_NAMES[layer]}</p>
      ) : null}

      {layer === "clauses" && <ClauseLayer sentence={sentence} clauses={clauses} />}
      {layer === "parts" && <PartLayer sentence={sentence} parts={parts} />}
      {layer === "words" && <SpanLayer sentence={sentence} spans={words} tone={WORD_TONE} bold />}
      {layer === "tenses" && <SpanLayer sentence={sentence} spans={tenses} tone={TENSE_TONE} />}

      {grammar.meaning && (
        <p className="mk-grammar__meaning">
          <span className="mk-grammar__label">句意</span>
          {grammar.meaning}
        </p>
      )}
    </div>
  );
}

function Highlight({ text, t, bold }: { text: string; t: Tone; bold?: boolean }) {
  return (
    <span
      className={bold ? "mk-grammar__hl mk-grammar__hl--bold" : "mk-grammar__hl"}
      style={{ background: t.bg, color: t.fg }}
    >
      {text}
    </span>
  );
}

function Swatch({ t, label }: { t: Tone; label: string }) {
  return (
    <span className="mk-grammar__chip" style={{ background: t.bg, color: t.fg }}>
      {label}
    </span>
  );
}

/** 主句的一截：两头的逗号、空格留在底色外面，只给字上色。 */
function MainPiece({ text }: { text: string }) {
  const m = /^([\s,;:，；：]*)([\s\S]*?)([\s,;:，；：]*)$/u.exec(text);
  const [pre, core, post] = m ? [m[1], m[2], m[3]] : ["", text, ""];
  return (
    <>
      {pre}
      <Highlight text={core ?? text} t={MAIN_TONE} bold />
      {post}
    </>
  );
}

function ClauseLayer({ sentence, clauses }: { sentence: string; clauses: ReadingGrammarSpan[] }) {
  const segments = splitSentenceByParts(sentence, clauses);
  const clauseTone = (i: number): Tone => CLAUSE_TONES[i % CLAUSE_TONES.length] ?? MAIN_TONE;
  const main = mainClauseText(sentence, clauses);
  return (
    <>
      <p className="mk-grammar__sentence">
        {segments.map((seg, i) =>
          seg.part != null ? (
            <Highlight key={i} text={seg.text} t={clauseTone(seg.part)} />
          ) : hasWords(seg.text) ? (
            <MainPiece key={i} text={seg.text} />
          ) : (
            <span key={i}>{seg.text}</span>
          ),
        )}
      </p>
      <ul className="mk-grammar__list">
        {main && (
          <li className="mk-grammar__item">
            <Swatch t={MAIN_TONE} label="主句" />
            <span className="mk-grammar__item-text">{main}</span>
            <span className="mk-grammar__item-note">句子的主干，去掉所有从句后剩下的部分。</span>
          </li>
        )}
        {clauses.map((c, i) => (
          <li key={`${i}-${c.text}`} className="mk-grammar__item">
            <Swatch t={clauseTone(i)} label={labelOf(c)} />
            <span className="mk-grammar__item-text">{c.text}</span>
            {c.note && <span className="mk-grammar__item-note">{c.note}</span>}
          </li>
        ))}
      </ul>
    </>
  );
}

function PartLayer({ sentence, parts }: { sentence: string; parts: ReadingGrammarSpan[] }) {
  const segments = splitSentenceByParts(sentence, parts);
  return (
    <>
      <p className="mk-grammar__sentence">
        {segments.map((seg, i) =>
          seg.part == null ? (
            <span key={i}>{seg.text}</span>
          ) : (
            <Highlight key={i} text={seg.text} t={partTone(labelOf(parts[seg.part] ?? { text: "" }))} />
          ),
        )}
      </p>
      <ul className="mk-grammar__list">
        {parts.map((p, i) => (
          <li key={`${i}-${p.text}`} className="mk-grammar__item">
            <Swatch t={partTone(labelOf(p))} label={labelOf(p)} />
            <span className="mk-grammar__item-text">{p.text}</span>
            {p.note && <span className="mk-grammar__item-note">{p.note}</span>}
          </li>
        ))}
      </ul>
    </>
  );
}

/** 词法与时态：一层一个颜色，句子里标出那几段，下面逐条说明（时态带例句）。 */
function SpanLayer({
  sentence,
  spans,
  tone: t,
  bold,
}: {
  sentence: string;
  spans: ReadingGrammarSpan[];
  tone: Tone;
  bold?: boolean;
}) {
  const segments = splitSentenceByParts(sentence, spans);
  return (
    <>
      <p className="mk-grammar__sentence">
        {segments.map((seg, i) =>
          seg.part == null ? (
            <span key={i}>{seg.text}</span>
          ) : (
            <Highlight key={i} text={seg.text} t={t} bold={bold} />
          ),
        )}
      </p>
      <ul className="mk-grammar__list">
        {spans.map((s, i) => (
          <li key={`${i}-${s.text}`} className="mk-grammar__item">
            <span className="mk-grammar__item-word" style={{ background: t.bg, color: t.fg }}>
              {s.text}
            </span>
            <span className="mk-grammar__item-label">{labelOf(s)}</span>
            {s.note && <span className="mk-grammar__item-note">{s.note}</span>}
            {s.example && (
              <span className="mk-grammar__item-example">
                <span className="mk-word-card__example-en">{s.example}</span>
                {s.exampleZh && <span className="mk-word-card__example-zh">{s.exampleZh}</span>}
              </span>
            )}
          </li>
        ))}
      </ul>
    </>
  );
}

/** 第一版的卡片：主干 + 按序号标色的几块 + 语法点。 */
function LegacyGrammar({ sentence, grammar }: { sentence: string; grammar: ReadingGrammar }) {
  const legacyParts = grammar.parts ?? [];
  const segments = splitSentenceByParts(sentence, legacyParts);
  const t = (i: number): Tone => LEGACY_TONES[i % LEGACY_TONES.length] ?? MAIN_TONE;
  const points: ReadingGrammarPoint[] = grammar.points ?? [];
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
            <span key={i} className="mk-grammar__hl" style={{ background: t(seg.part).bg }}>
              {seg.text}
              <sup className="mk-grammar__hl-n">{seg.part + 1}</sup>
            </span>
          ),
        )}
      </p>
      <ol className="mk-grammar__parts">
        {legacyParts.map((p, i) => (
          <li key={`${i}-${p.text}`} className="mk-grammar__part">
            <span className="mk-grammar__part-n" style={{ background: t(i).bg }}>
              {i + 1}
            </span>
            <span className="mk-grammar__role">{labelOf(p)}</span>
            <span className="mk-grammar__part-text">{p.text}</span>
            {p.note && <span className="mk-grammar__part-note">{p.note}</span>}
          </li>
        ))}
      </ol>
      {points.length > 0 && (
        <>
          <p className="mk-grammar__label mk-grammar__label--block">语法点</p>
          <ul className="mk-word-cards">
            {points.map((pt) => (
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
