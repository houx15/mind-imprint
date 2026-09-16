import { describe, expect, it } from "vitest";
import { draftQuoteMatch, quoteClickableInRoom, quoteFoundInDraft } from "./draftQuote";
import { splitOnHighlight } from "./ProseSurface";

/** Draft/quote pairs where the quote SHOULD be findable — reused by both the
 *  gate tests below and the combined gate+highlighter test, so the two never
 *  quietly drift apart from each other. */
const FOUND_FIXTURES: { name: string; draft: string; quote: string }[] = [
  { name: "逐字能找到", draft: "中国的碳排放全球第一，这是她论证要面对的反例。", quote: "中国的碳排放全球第一" },
  {
    // 🚨 这是复现 CRITICAL 那个 bug 的原始用例：引文结尾是句号，正文里
    // 对应位置是全角空格 + 逗号——归一化判据说找得到，字面 indexOf 找不到。
    name: "标点、空白不算数——沿用同一套归一化规则",
    draft: "中国的碳排放全球第一　，后面接着写别的。",
    quote: "中国的碳排放全球第一。",
  },
  {
    name: "大小写不算数——英文稿同理",
    draft: "China leads the world in carbon emissions today.",
    quote: "CHINA leads the world in carbon emissions",
  },
  {
    name: "点评的引文本来就是逐字子串——位置匹配不该让这条路径变了行为",
    draft: "她站在收残台旁边数了一下。后来才发现浪费有多严重。",
    quote: "她站在收残台旁边数了一下。",
  },
];

describe("quoteFoundInDraft", () => {
  for (const f of FOUND_FIXTURES) {
    it(f.name, () => {
      expect(quoteFoundInDraft(f.draft, f.quote)).toBe(true);
    });
  }

  it("她已经改掉那句话，就找不到了", () => {
    expect(quoteFoundInDraft("她后来把这句整个删掉，换成了别的论证。", "中国的碳排放全球第一")).toBe(false);
  });

  it("引文是空的/纯标点，永远不可点", () => {
    expect(quoteFoundInDraft("随便什么正文。", "")).toBe(false);
    expect(quoteFoundInDraft("随便什么正文。", "   ")).toBe(false);
    expect(quoteFoundInDraft("随便什么正文。", "……")).toBe(false);
    expect(quoteFoundInDraft("随便什么正文。", null)).toBe(false);
  });

  it("草稿是空的，什么都找不到", () => {
    expect(quoteFoundInDraft("", "中国的碳排放全球第一")).toBe(false);
  });

  it("引文只是草稿里另一句话的子串误判——必须真的出现，不是巧合前缀", () => {
    expect(quoteFoundInDraft("她讨论的是碳排放增长趋势，不是总量第一。", "中国的碳排放全球第一")).toBe(false);
  });
});

/**
 * 🚨 CRITICAL（review round 1）：判据用的是归一化匹配（`rangeForQuote`），
 * 但 `ProseSurface` 的镜像高亮层做的是**字面** `text.indexOf`——它是一块
 * textarea 的镜像层，不是一次搜索。第一版直接把老师的原话喂给它，能通过
 * 「可点」这道判据，却在 `ProseSurface` 里字面找不到，什么都不会高亮、
 * 不会滚动。`draftQuoteMatch` 返回的是**草稿里真正匹配到的那一段原文**，
 * 它在计算出来的那一刻本来就是 `draft` 的字面子串，所以永远能字面匹配上
 * 自己——这里把「判据说可点」和「高亮层真的找得到」钉在同一个测试里，
 * 这正是本该在第一轮就抓住这个 bug 的那个测试。
 */
describe("draftQuoteMatch feeds ProseSurface's literal highlighter", () => {
  for (const f of FOUND_FIXTURES) {
    it(`${f.name} → 高亮真的找得到，不是只在判据里找得到`, () => {
      const match = draftQuoteMatch(f.draft, f.quote);
      expect(match).not.toBeNull();
      const split = splitOnHighlight(f.draft, match);
      expect(split).not.toBeNull();
      // 中间那一段必须非空——否则「找到了」只是找到了一个空字符串，
      // `indexOf("")` 永远为 0，会把「没匹配」悄悄伪装成「匹配上了」。
      expect(split?.[1]).not.toBe("");
    });
  }

  it("点评（CommentPanel）的引文是服务端校验过的逐字子串——不经过这个模块，也不受影响", () => {
    // CommentPanel 的 onTrace 直接把 point.quote 原样交给 ProseSurface，
    // 从不走 draftQuoteMatch；这里只确认「逐字子串」这条路径本身在字面
    // indexOf 下天然成立，验证改成位置匹配没有让这条已有路径变了行为。
    const draft = "她站在收残台旁边数了一下。后来才发现浪费有多严重。";
    const quote = "她站在收残台旁边数了一下。";
    const split = splitOnHighlight(draft, quote);
    expect(split?.[1]).toBe(quote);
  });
});

describe("quoteClickableInRoom", () => {
  it("成稿阶段、草稿里找得到 → 可点", () => {
    expect(quoteClickableInRoom("draft", "中国的碳排放全球第一。", "中国的碳排放全球第一")).toBe(true);
  });

  it("段落阶段——即使草稿里找得到，也不可点：那一步没有草稿正文可以跳", () => {
    expect(quoteClickableInRoom("snippets", "中国的碳排放全球第一。", "中国的碳排放全球第一")).toBe(false);
  });

  it("结构阶段同理，不可点", () => {
    expect(quoteClickableInRoom("outline", "中国的碳排放全球第一。", "中国的碳排放全球第一")).toBe(false);
  });

  it("finished 阶段：`ComposeStage` 会渲染，但这里按字面约定仍然不可点", () => {
    expect(quoteClickableInRoom("finished", "中国的碳排放全球第一。", "中国的碳排放全球第一")).toBe(false);
  });

  it("成稿阶段但草稿里已经找不到——不可点", () => {
    expect(quoteClickableInRoom("draft", "她把这句整个删掉了。", "中国的碳排放全球第一")).toBe(false);
  });
});
