/**
 * 荧光笔：把一段文本按「要标出来的词」切成若干段。
 *
 * # 它是给谁用的
 *
 * 轻量版阅读室的「关键单词」把一段里 3–5 个值得学的词做成卡片，产品负责人
 * 2026-09-16 要求这些词同时在正文里被标出来（「with 荧光笔 feeling」）。
 * 服务端已经保证每个 term 都**逐字**出现在那一段里（对不上的卡片整张丢掉），
 * 所以这里不必做任何模糊匹配 —— 找不到就是没有，不要去猜。
 *
 * # 三条规则，都是判错的代价决定的
 *
 * 1. **大小写不敏感地找，但原样显示。** 句首那个词在卡片上常常是小写的；
 *    按大小写严格匹配会让它在正文里标不出来，而她看着卡片上有、正文里没有，
 *    只会以为这个功能坏了。
 * 2. **长的词先标。** 「carbon footprint」和「carbon」同时要标时，先标长的，
 *    否则短的会把长的切碎成三段。
 * 3. **标出来的段落之间不重叠。** 一个字符只属于一个词 —— 重叠的区间在 DOM
 *    上没有办法表达，硬来会把文本顺序弄乱。
 *
 * 切出来的段落**首尾相接、拼回去逐字等于原文**：正文里少一个字符，之前存下来
 * 的每一条标注就都错位了（标注的锚点是字节偏移）。
 */

export type KeywordRun = {
  text: string;
  /** 命中的那个词（卡片上的 term）。`null` = 这一段不是关键词。 */
  term: string | null;
};

/**
 * 把 text 按 terms 切开。terms 为空、或一个都没命中时，返回单独一段原文。
 *
 * 返回的每一段都非空，且 `runs.map(r => r.text).join("") === text`。
 */
export function splitByKeywords(text: string, terms: string[]): KeywordRun[] {
  const wanted = terms.map((t) => t.trim()).filter((t) => t.length > 0);
  if (wanted.length === 0 || text.length === 0) return [{ text, term: null }];

  // 长的先来（规则 2）。长度相同的按字典序，只为让结果稳定 —— 同一段文本
  // 每次切出来都一样，否则 React 的 key 会在重渲染之间跳。
  const ordered = [...new Set(wanted)].sort((a, b) =>
    b.length - a.length || a.localeCompare(b),
  );

  const lower = text.toLowerCase();
  // hit[i] 是从 i 开始的那个词的长度；0 = 这里不开始任何词。
  const hit = new Int32Array(text.length);
  const claimed = new Uint8Array(text.length);

  for (const term of ordered) {
    const needle = term.toLowerCase();
    let from = 0;
    for (;;) {
      const at = lower.indexOf(needle, from);
      if (at < 0) break;
      // 规则 3：任何一个字符已经被别的词占了，这一处就跳过。
      let free = true;
      for (let i = at; i < at + needle.length; i++) {
        if (claimed[i]) {
          free = false;
          break;
        }
      }
      if (free) {
        hit[at] = needle.length;
        claimed.fill(1, at, at + needle.length);
      }
      from = at + needle.length;
    }
  }

  const runs: KeywordRun[] = [];
  let plainFrom = 0;
  for (let i = 0; i < text.length; ) {
    // noUncheckedIndexedAccess: Int32Array 的下标读回来是 `number | undefined`，
    // 而 i 永远在界内。`?? 0` 是给类型看的，不是给运行时看的。
    const len = hit[i] ?? 0;
    if (len === 0) {
      i++;
      continue;
    }
    if (i > plainFrom) runs.push({ text: text.slice(plainFrom, i), term: null });
    // term 用的是**正文里的那一份写法**，不是卡片上的 —— 正文一个字都不能改。
    runs.push({ text: text.slice(i, i + len), term: text.slice(i, i + len) });
    i += len;
    plainFrom = i;
  }
  if (plainFrom < text.length) runs.push({ text: text.slice(plainFrom), term: null });
  if (runs.length === 0) return [{ text, term: null }];
  return runs;
}
