import { describe, expect, it } from "vitest";
import { toolsForSelection } from "@lite/readings/SelectionTools";
import type { ReadingBlockTool } from "@lite/api/readingRoom";

/**
 * 划选之后那条工具条，在中文这边该摆哪几件。
 *
 * 🚨 2026-09-24 之前中文那边**一件都没有**：查词和语法是 Lang "en" 的，
 * 所以她在一篇文言文里划几个字，工具条上只有「摘抄 / 放入对话框 / 关闭」。
 * 产品负责人：「students may select some texts and need the
 * explanation/translation」。
 *
 * 现在中文有了三件（查字 = 词一级，白话翻译 / 字词释义 / 句法 = 句一级），
 * 于是「这几个字算一个词还是一小句」这条判据第一次承重。
 */

/** 文言文上服务端会发的那几件（照 readingBlockToolsFor("zh","classical")）。 */
const ZH_CLASSICAL: ReadingBlockTool[] = [
  { id: "classical_translate", label: "白话翻译", subject: "sentence" },
  { id: "classical_word", label: "查字", subject: "word" },
  { id: "classical_words", label: "字词释义", subject: "sentence" },
  { id: "classical_syntax", label: "句法", subject: "sentence" },
  { id: "rhetoric", label: "成语修辞" },
  { id: "classical_shape", label: "全篇章法", scope: "article" },
];

describe("中文划选工具条", () => {
  it("点一个字只给查字 —— 一个字的「句法」是没有意义的按钮", () => {
    expect(toolsForSelection(ZH_CLASSICAL, "蓑").map((t) => t.id)).toEqual(["classical_word"]);
    expect(toolsForSelection(ZH_CLASSICAL, "俄而").map((t) => t.id)).toEqual(["classical_word"]);
  });

  it("划一整句只给句一级的三件", () => {
    const ids = toolsForSelection(ZH_CLASSICAL, "俄而雪骤，公欣然曰").map((t) => t.id);
    expect(ids).toEqual(["classical_translate", "classical_words", "classical_syntax"]);
  });

  /**
   * 🚨 三到四个汉字**两级都给**。
   *
   * 光看这几个字分不出她要哪一个，而工具条上没有体裁这一位：
   *
   *	热岛效应   四个字，是一个词（现代汉语）
   *	俄而雪骤   四个字，是一整句（文言）
   *
   * 所以两级都摆，由她说要哪个 —— 选区是「对哪几个字」，工具条是「做什么」。
   */
  it("三到四个汉字两级都给，因为分不出是一个词还是一小句", () => {
    const ids = toolsForSelection(ZH_CLASSICAL, "俄而雪骤").map((t) => t.id);
    expect(ids).toContain("classical_word");
    expect(ids).toContain("classical_translate");
    const three = toolsForSelection(ZH_CLASSICAL, "蓑笠翁").map((t) => t.id);
    expect(three).toContain("classical_word");
    expect(three).toContain("classical_translate");
  });

  it("整篇那几件永远不在划选工具条上 —— 它们读的是全文", () => {
    for (const quote of ["蓑", "蓑笠翁", "俄而雪骤，公欣然曰"]) {
      expect(toolsForSelection(ZH_CLASSICAL, quote).map((t) => t.id)).not.toContain(
        "classical_shape",
      );
    }
  });

  it("没有 subject 的整段工具也不在 —— 它讲的是整段，不是她划的这几个字", () => {
    expect(toolsForSelection(ZH_CLASSICAL, "俄而雪骤，公欣然曰").map((t) => t.id)).not.toContain(
      "rhetoric",
    );
  });

  /** 英文那一侧一个字节都没变：空白切词，词和句互斥。 */
  it("英文不受影响", () => {
    const en: ReadingBlockTool[] = [
      { id: "lookup", label: "查词", subject: "word" },
      { id: "grammar", label: "句子解析", subject: "sentence" },
    ];
    expect(toolsForSelection(en, "prolonged").map((t) => t.id)).toEqual(["lookup"]);
    expect(toolsForSelection(en, "The storm reached the coast.").map((t) => t.id)).toEqual([
      "grammar",
    ]);
    // 四个字母的英文词仍然只是一个词 —— 三到四个字那条规则只认汉字。
    expect(toolsForSelection(en, "rain").map((t) => t.id)).toEqual(["lookup"]);
  });
});
