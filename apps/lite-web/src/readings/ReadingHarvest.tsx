import { Bookmark, ArrowUpRight, MessageSquareQuote } from "lucide-react";
import { Icon } from "@/ui";
import type { LiteAnnotation, LiteMessage, ReadingBlockNote, ReadingWord } from "@lite/api/readingRoom";
import { coachAnswerOf, coachCardOf } from "@lite/api/readingRoom";
import { ReadingOutcomes } from "@/studio/reading/ReadingOutcomes";
import type { ReadingOutcome } from "@/studio/reading/readingLoop";
import { WordCards } from "./WordCards";
import { GrammarCards } from "./GrammarCards";
import { BLOCK_TOOL_ANSWER, boardItems, parseBoardAnswer } from "./CoachCard";
import { parseOrderAnswer } from "./OrderBoard";

/**
 * ReadingHarvest —— 「阅读成果」那一页：她读这一篇时**真的产出**的东西。
 *
 * # 为什么重写
 *
 * 产品负责人 2026-09-18 逐字：
 *
 *   > reading notes is also: it should not only be lens. it should be the
 *   > grammar/words/my thoughts etc. it's my real reading 成果
 *
 * 这一页原来只摆透镜的成果（`ReadingOutcomes`）。而透镜 2026-09-17 已经被
 * 「你怎么看」顶掉了，读法库里一步都不再排它 —— 于是这一页对今天进来的每一个
 * 学生**永远是空的**，而她这一路上学过的词、拆过的句子、摆过的板、写下的话，
 * 一样都没留下。
 *
 * 五样，全部是她自己做过的事，按「学到的 → 判断过的 → 写下的」排：
 *
 *   我的摘抄  她在正文里划选后按「摘抄」留下的句子（2026-09-22）
 *   生词      她开过关键单词 / 查过的词，词卡原样
 *   语法      她拆过的句子和它的语法卡
 *   我摆的板  标注板（格子 + 她放进去的原话）、排序板（她排的先后）
 *   我写的    想一想 / 仿写 底下她自己写的那几段
 *   透镜成果  老数据还有，就照旧摆在最后
 *
 * 🚨 板上和词卡上的句子是**文章的原话**，不是她写的句子；「我写的」那一节才是
 * 她的话。每一节的标题因此说清是谁的（R4：报告和这一页上每一段引文都要说清
 * 是谁的话）。摘抄也是文章的原话——她做的判断是「这一句值得留下」。
 *
 * # 摘抄这一节上那颗「讨论这句」
 *
 * 产品负责人 2026-09-22：
 *
 *   > she can go to reading results part and chat with AI about those
 *
 * 摘抄的时候不追问为什么（那一问被明确否掉了）。想说的时候她到这一页来，
 * 点那一句旁边的「讨论这句」—— 那一句进印记那一栏的输入框，下一句话由她写。
 */

export type HarvestBoard = {
  kind: "label" | "order";
  prompt: string;
  /** 标注板：格子 → 她放进去的那几句。 */
  groups?: { bin: string; quotes: string[] }[];
  /** 排序板：她排的先后。 */
  order?: string[];
};

/** 她摆过的板，从对话里读回来。卡片在 印记 那条消息上，作答在她那条上。 */
export function harvestBoards(messages: LiteMessage[]): HarvestBoard[] {
  const sorted = [...messages].sort((a, b) => a.seq - b.seq);
  const cards = new Map<string, ReturnType<typeof coachCardOf>>();
  const out: HarvestBoard[] = [];
  for (const m of sorted) {
    const card = coachCardOf(m);
    if (card) {
      cards.set(card.prompt, card);
      continue;
    }
    const answer = coachAnswerOf(m);
    if (!answer || answer.type === BLOCK_TOOL_ANSWER) continue;
    const card2 = cards.get(answer.prompt);
    if (!card2) continue;
    const items = boardItems(card2);
    if (answer.type === "label_roles") {
      const placed = parseBoardAnswer(card2, answer.choice);
      const bins = card2.labels ?? [];
      const groups = bins
        .map((bin) => ({
          bin,
          quotes: items.filter((it) => placed[it.id] === bin).map((it) => it.text),
        }))
        .filter((g) => g.quotes.length > 0);
      if (groups.length > 0) out.push({ kind: "label", prompt: card2.prompt, groups });
    } else if (answer.type === "order_events") {
      const order = parseOrderAnswer(items, answer.choice)
        .map((id) => items.find((it) => it.id === id)?.text ?? "")
        .filter(Boolean);
      if (order.length > 0) out.push({ kind: "order", prompt: card2.prompt, order });
    }
  }
  return out;
}

/** 想一想 / 仿写 底下她自己写的那几段。印记 那一行单独留着，标清谁说的。 */
export function harvestWritings(messages: LiteMessage[]): { prompt: string; text: string }[] {
  const out: { prompt: string; text: string }[] = [];
  for (const m of [...messages].sort((a, b) => a.seq - b.seq)) {
    const answer = coachAnswerOf(m);
    if (!answer || answer.type !== BLOCK_TOOL_ANSWER) continue;
    const text = answer.choice.trim();
    if (text) out.push({ prompt: answer.prompt, text });
  }
  return out;
}

/** 她学过的词：关键单词挑的那些 + 她自己点的查词。同一个词只留一张。 */
export function harvestWords(notes: ReadingBlockNote[]): ReadingWord[] {
  const seen = new Set<string>();
  const out: ReadingWord[] = [];
  for (const n of notes) {
    for (const w of n.words ?? []) {
      const key = w.term.trim().toLowerCase();
      if (!key || seen.has(key)) continue;
      seen.add(key);
      out.push(w);
    }
  }
  return out;
}

function Section({ title, hint, children }: { title: string; hint?: string; children: React.ReactNode }) {
  return (
    <section className="mk-harvest__sec">
      <h3 className="mk-harvest__title">{title}</h3>
      {hint && <p className="mk-harvest__hint">{hint}</p>}
      {children}
    </section>
  );
}

export function ReadingHarvest({
  notes,
  messages,
  outcomes,
  excerpts = [],
  onLocate,
  onDiscuss,
  ordinalOf,
}: {
  notes: ReadingBlockNote[];
  messages: LiteMessage[];
  outcomes: ReadingOutcome[];
  /** 她摘抄过的句子，最早的在前（服务端按 created_at 排，就是她读的顺序）。 */
  excerpts?: LiteAnnotation[];
  onLocate: (blockId: string) => void;
  /** 把这一句放进印记那一栏的输入框。 */
  onDiscuss?: (blockId: string, quote: string) => void;
  /** 段 id → 第几段。段号是她屏幕上唯一认得的坐标。 */
  ordinalOf?: (blockId: string) => number;
}) {
  const words = harvestWords(notes);
  const grammar = notes.filter((n) => n.grammar && n.subject);
  const boards = harvestBoards(messages);
  const writings = harvestWritings(messages);
  const empty =
    excerpts.length === 0 &&
    words.length === 0 && grammar.length === 0 && boards.length === 0 && writings.length === 0 && outcomes.length === 0;

  if (empty) {
    // 🚨 空状态说的是**接下来会有什么**，不是「你还没做」。这一页在她刚进房间
    // 时本来就是空的，把它写成一句记账，读起来就是一条对她的催promise。
    return (
      <p className="mk-harvest__empty">
        这里汇总本篇的阅读成果：摘抄、词汇、句子分析、工具卡和笔记。
      </p>
    );
  }

  return (
    <div className="mk-harvest">
      {excerpts.length > 0 && (
        <Section title="我的摘抄" hint={`已保存 ${excerpts.length} 条原文摘抄`}>
          {excerpts.map((e) => {
            const ord = ordinalOf?.(e.blockId) ?? 0;
            return (
              <div key={e.id} className="mk-harvest__item mk-harvest__clipping">
                <div className="mk-harvest__source"><Icon icon={Bookmark} size={13} />原文摘抄{ord > 0 && <span>第 {ord} 段</span>}</div>
                <blockquote className="mk-harvest__excerpt">{e.quote}</blockquote>
                <div className="mk-harvest__acts">
                  <button type="button" className="mk-harvest__locate" onClick={() => onLocate(e.blockId)}>
                    <Icon icon={ArrowUpRight} size={13} />
                    {ord > 0 ? `回到第 ${ord} 段` : "回到原文"}
                  </button>
                  {onDiscuss && (
                    <button
                      type="button"
                      className="mk-harvest__locate"
                      onClick={() => onDiscuss(e.blockId, e.quote)}
                    >
                      <Icon icon={MessageSquareQuote} size={13} />
                      讨论这句
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </Section>
      )}
      {words.length > 0 && (
        <Section title="生词" hint="关键单词和你点开查过的词。">
          <WordCards words={words} />
        </Section>
      )}
      {grammar.length > 0 && (
        <Section title="拆开的句子" hint="你在段落工具里拆过的句子。">
          {grammar.map((n, i) => (
            <div key={`${n.blockId}-${i}`} className="mk-harvest__item">
              <GrammarCards sentence={n.subject ?? ""} grammar={n.grammar!} />
              <button type="button" className="mk-harvest__locate" onClick={() => onLocate(n.blockId)}>
                回到原文
              </button>
            </div>
          ))}
        </Section>
      )}
      {boards.length > 0 && (
        <Section title="我摆的板" hint="板上的句子是文章的原话，位置是你的判断。">
          {boards.map((b, i) => (
            <div key={i} className="mk-harvest__item">
              <p className="mk-harvest__q">{b.prompt}</p>
              {b.kind === "label" ? (
                <ul className="mk-harvest__bins">
                  {(b.groups ?? []).map((g) => (
                    <li key={g.bin}>
                      <span className="mk-harvest__bin">{g.bin}</span>
                      <ul>
                        {g.quotes.map((q, j) => (
                          <li key={j}>{q}</li>
                        ))}
                      </ul>
                    </li>
                  ))}
                </ul>
              ) : (
                <ol className="mk-harvest__order">
                  {(b.order ?? []).map((q, j) => (
                    <li key={j}>{q}</li>
                  ))}
                </ol>
              )}
            </div>
          ))}
        </Section>
      )}
      {writings.length > 0 && (
        <Section title="我写的" hint="想一想和仿写底下，你自己写的那几段。">
          {writings.map((w, i) => (
            <div key={i} className="mk-harvest__item">
              <p className="mk-harvest__q">{w.prompt}</p>
              <p className="mk-harvest__mine">{w.text}</p>
            </div>
          ))}
        </Section>
      )}
      {outcomes.length > 0 && (
        <Section title="透镜成果">
          <ReadingOutcomes outcomes={outcomes} onLocate={() => onLocate(outcomes[0]?.blockId ?? "")} />
        </Section>
      )}
    </div>
  );
}
