/**
 * sentences.ts —— 把她写的一段拆成句子。
 *
 * 标注板（`RoleBoard`）上的每一张卡片都是她自己写的一句话，而**卡片是拆出来
 * 的，不是模型挑出来的**。这一点是承重的：
 *
 *  - 铁律① 因此是结构性成立的，不是靠 prompt 恳求。板上不可能出现一个她没写
 *    过的字——拆分只会切，不会生成。
 *  - 也就没有「引文核对」这一关要过。阅读室那两块板必须逐字回文章里核对
 *    （模型会编一句读着很像的话），这里连编的机会都没有。
 *  - 顺带省掉一次模型调用：板是即时出现的，不用等，也不会失败。
 *
 * 🚨 拆句是那种「读代码看不出对错」的纯函数，所以它单独一个文件、带一组测试。
 * 拆错的后果不是报错，是板上多出一张只有两个字的卡片，或者两句话挤在一张卡上
 * ——两种都只有人眼能发现。
 */

/** 板上的一句话。`index` 是它在这一段里的第几句（从 1 起，给她看的）。 */
export type Sentence = { id: string; index: number; text: string };

/**
 * 句末标点。中文的句号/问号/感叹号/分号，以及省略号。
 *
 * 🚨 逗号（，）**不是**句末标点，哪怕中文里一个长句常常靠逗号串很多小句。
 * 按逗号切会把「因为食堂每天倒掉很多饭，我觉得这件事值得写」切成两张卡，
 * 而那本来是一个完整的因果——标注板要她分辨的恰好是这种关系，切开就没了。
 */
const ZH_ENDERS = "。！？；!?;";

/**
 * 英文缩写：这些词后面的点号不是句末。
 *
 * 不求全，只挡真的会出现在学生作文里的那几个。漏掉一个的后果是多切一刀，
 * 不是崩掉。
 */
const ABBREVIATIONS = [
  "mr", "mrs", "ms", "dr", "prof", "st", "vs", "etc", "e.g", "i.e", "fig", "no",
  "jan", "feb", "mar", "apr", "jun", "jul", "aug", "sep", "sept", "oct", "nov", "dec",
];

function endsWithAbbreviation(buf: string): boolean {
  // 取最后一个空白之后的那一小段，去掉结尾的点，比对小写。
  const tail = buf.slice(Math.max(0, buf.length - 12)).split(/\s/).pop() ?? "";
  const word = tail.replace(/\.$/, "").toLowerCase();
  return ABBREVIATIONS.includes(word);
}

/**
 * 拆句。保留句末标点，丢掉纯空白的片段。
 *
 * 规则：
 *  - 中文句末标点后面直接断。
 *  - 英文的 `.` 只有在**后面跟空白或到结尾**、而且前面不是缩写、也不是小数点
 *    时才断。`3.5` 和 `Dr. Chen` 因此不会被切开。
 *  - 连续的句末标点算一个（`？！` 一起收尾）。
 *  - 换行也断句：她按回车分开的两句，在她眼里本来就是两句。
 */
export function splitSentences(text: string): string[] {
  const out: string[] = [];
  let buf = "";

  const flush = () => {
    const t = buf.trim();
    if (t) out.push(t);
    buf = "";
  };

  // `at` 而不是 `text[i]`：开了 noUncheckedIndexedAccess 之后下标取出来是
  // `string | undefined`，而这个循环到处都在看前一个/后一个字符。一个
  // 「越界就是空串」的取字符函数比每处加一个 `?? ""` 干净得多。
  const at = (i: number): string => (i >= 0 && i < text.length ? (text[i] as string) : "");
  const CLOSERS = `”"’'）)」』`;

  for (let i = 0; i < text.length; i++) {
    const ch = at(i);
    buf += ch;

    if (ch === "\n") {
      flush();
      continue;
    }

    if (ZH_ENDERS.includes(ch)) {
      // 把紧跟着的其他句末标点和右引号一起收进来。
      while (i + 1 < text.length && (ZH_ENDERS.includes(at(i + 1)) || CLOSERS.includes(at(i + 1)))) {
        buf += at(++i);
      }
      flush();
      continue;
    }

    if (ch === ".") {
      const prev = at(i - 1);
      const next = at(i + 1);
      // 小数点：两边都是数字。
      if (prev && next && /\d/.test(prev) && /\d/.test(next)) continue;
      // 省略号 `...`：等最后一个点再断。
      if (next === ".") continue;
      // 缩写。
      if (endsWithAbbreviation(buf)) continue;
      // 句末的点后面必须是空白或者到头了，否则它是 URL / 文件名里的点。
      if (next !== "" && !/\s/.test(next)) continue;
      while (i + 1 < text.length && CLOSERS.includes(at(i + 1))) buf += at(++i);
      flush();
      continue;
    }
  }
  flush();
  return out;
}

/** 板上最少要几张卡才成一块板。一句话的段落没有「角色」可分。 */
export const ROLE_BOARD_MIN = 2;

/**
 * 板上最多摆几张卡。
 *
 * 八张已经是一屏的极限；再多这块板就从「分辨」变成「录入」了。超出的时候
 * **只取前八句**，而且界面上要说出来——悄悄少几句会让她以为自己写的东西丢了。
 */
export const ROLE_BOARD_MAX = 8;

/**
 * 把一段变成板上的卡片。
 *
 * 返回 `{ sentences, truncated }`：`truncated` 是「这一段比板装得下的更长」，
 * 调用方要把这件事显示出来。
 */
export function sentencesForBoard(text: string): { sentences: Sentence[]; truncated: boolean } {
  const all = splitSentences(text);
  const kept = all.slice(0, ROLE_BOARD_MAX);
  return {
    sentences: kept.map((t, i) => ({ id: `s${i}`, index: i + 1, text: t })),
    truncated: all.length > kept.length,
  };
}
