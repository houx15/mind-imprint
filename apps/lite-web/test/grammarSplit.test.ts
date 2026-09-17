import { describe, expect, it } from "vitest";
import { mainClauseText, splitSentenceByParts } from "../src/readings/GrammarCards";

/**
 * 语法卡最上面那一句的切法。这是 GrammarCards 里唯一一处读代码看不出对错的
 * 逻辑：高亮落在哪、重叠了怎么办、拼回去是不是原句。样子本身用眼睛看。
 */

const S = "These feathers, which likely helped insulate the birds, might be the key.";
const join = (segs: { text: string }[]) => segs.map((s) => s.text).join("");

describe("splitSentenceByParts", () => {
  it("拼回去逐字等于原句", () => {
    const segs = splitSentenceByParts(S, [
      { text: "These feathers", role: "主语" },
      { text: "which likely helped insulate the birds", role: "定语从句" },
      { text: "might be the key", role: "谓语" },
    ]);
    expect(join(segs)).toBe(S);
    expect(segs.filter((s) => s.part !== null).map((s) => s.part)).toEqual([0, 1, 2]);
  });

  it("块的顺序和句子里的顺序不同时，照句子里的顺序摆，但编号跟着块", () => {
    const segs = splitSentenceByParts(S, [
      { text: "might be the key", role: "谓语" },
      { text: "These feathers", role: "主语" },
    ]);
    const marked = segs.filter((s) => s.part !== null);
    expect(marked.map((s) => s.text)).toEqual(["These feathers", "might be the key"]);
    expect(marked.map((s) => s.part)).toEqual([1, 0]);
  });

  it("找不到的那一块跳过，整句照常渲染", () => {
    const segs = splitSentenceByParts(S, [
      { text: "keep the birds warm", role: "改写过的" },
      { text: "These feathers", role: "主语" },
    ]);
    expect(join(segs)).toBe(S);
    expect(segs.filter((s) => s.part !== null)).toHaveLength(1);
  });

  it("两块重叠，排在前面的赢，后一块不标 —— 一个字不能有两个底色", () => {
    const segs = splitSentenceByParts(S, [
      { text: "These feathers, which", role: "a" },
      { text: "feathers", role: "b" },
    ]);
    expect(join(segs)).toBe(S);
    expect(segs.filter((s) => s.part !== null).map((s) => s.part)).toEqual([0]);
  });

  it("同一块出现两次，标第一处不冲突的", () => {
    const s = "the cold and the cold";
    const segs = splitSentenceByParts(s, [
      { text: "the cold and", role: "a" },
      { text: "the cold", role: "b" },
    ]);
    expect(join(segs)).toBe(s);
    const b = segs.find((x) => x.part === 1);
    expect(b).toBeDefined();
    // 第二块落在后面那个 "the cold" 上，而不是被第一块占掉的那一处
    expect(segs.indexOf(b!)).toBeGreaterThan(0);
  });

  it("没有块就是整句一段", () => {
    expect(splitSentenceByParts(S, [])).toEqual([{ text: S, part: null }]);
  });
});

// 🚨 第二版：主句由界面反推（从句之外的部分），模型不给。反推错了，「主从句」那一层
// 的颜色就全是错的。
describe("mainClauseText", () => {
  const long =
    "These feathers, which likely helped insulate the birds from the cold, might be the key to why hesperornithiforms did not survive.";
  const clauses = [
    { text: "which likely helped insulate the birds from the cold" },
    { text: "why hesperornithiforms did not survive" },
  ];

  it("主句是从句之外有字的那几截，两头的逗号去掉", () => {
    expect(mainClauseText(long, clauses)).toBe("These feathers … might be the key to");
  });

  it("没有从句时整句都是主句", () => {
    expect(mainClauseText(S, [])).toBe(S);
  });
});
