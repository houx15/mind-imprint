import { describe, expect, it } from "vitest";
import { boardItems, composeBoardAnswer, type CoachCardSpec } from "@lite/readings/CoachCard";
import { DRAG_SLOP, isDrag } from "@lite/readings/CoachBoards";

/**
 * 板摆完之后变成的那段话。
 *
 * 只测这一件事，因为只有这一件事「读代码看不出对错」：**引文必须单独成行**。
 *
 * 服务端的 `composeCardAnswerMessage`（reading_coach.go）会拿她这一轮的每一行
 * 回文章里做字面核对，是原文的那几行带上 `> ` 前缀。这是 R4 的第三道闸：
 * 文章的句子绝不能以「她说的话」的身份进语料，否则它有资格被印在一张可以分享
 * 的报告图片上、署她的名。
 *
 * 「证据：「原文」」写成一行的话，那一行既不是纯粹的原文（前面多了三个字），
 * 核对就过不了，于是整句文章的话落进了她的语料。屏幕上完全看不出来 ——
 * 报告是几天之后才生成的。
 */

const SPANS: CoachCardSpec = {
  type: "label_roles",
  prompt: "这几句在文章里各自在干什么？",
  labels: ["主张", "证据", "限制"],
  options: [
    { blockId: "b2", quote: "The agency said it had delivered 40 trucks of supplies." },
    { blockId: "b3", quote: "Officials cautioned that the figure could not be verified." },
  ],
};

const WORDS: CoachCardSpec = {
  type: "word_bank",
  prompt: "这几个词，哪些你已经认识？",
  words: [
    { blockId: "b1", term: "scrambling" },
    { blockId: "b2", term: "delivered" },
    { blockId: "b3", term: "cautioned" },
  ],
};

describe("板摆完之后的那段话", () => {
  it("标注板：标签一行，引文自己一行，逐字不动", () => {
    const items = boardItems(SPANS);
    const text = composeBoardAnswer(SPANS, { [items[0]!.id]: "证据", [items[1]!.id]: "限制" }, items);
    const lines = text.split("\n");
    expect(lines).toEqual([
      "证据：",
      "The agency said it had delivered 40 trucks of supplies.",
      "限制：",
      "Officials cautioned that the figure could not be verified.",
    ]);
    // 🚨 这是这条测试存在的理由：每一句引文所在的那一行，必须**整行**逐字等于
    // 文章里的那句话。前面粘上标签，服务端就核对不上，文章的句子会以她的话的
    // 身份进语料。
    for (const o of SPANS.options ?? []) {
      expect(lines, `引文没有单独成行：${o.quote}`).toContain(o.quote);
    }
  });

  it("没摆进格子的那几张不进这段话", () => {
    const items = boardItems(SPANS);
    const text = composeBoardAnswer(SPANS, { [items[0]!.id]: "证据" }, items);
    expect(text).not.toContain("Officials cautioned");
  });

  it("生词板：一个词一行，词不是句子所以不必单独成行", () => {
    const items = boardItems(WORDS);
    const text = composeBoardAnswer(
      WORDS,
      { [items[0]!.id]: "不认识", [items[1]!.id]: "认识", [items[2]!.id]: "不确定" },
      items,
    );
    expect(text.split("\n")).toEqual([
      "scrambling — 不认识",
      "delivered — 认识",
      "cautioned — 不确定",
    ]);
  });

  it("一张都没摆 = 一段空话，调用方据此不发这一轮", () => {
    const items = boardItems(WORDS);
    expect(composeBoardAnswer(WORDS, {}, items)).toBe("");
  });

  it("boardItems 按卡片的形状取东西，取错了板上就是空的", () => {
    expect(boardItems(SPANS).map((i) => i.blockId)).toEqual(["b2", "b3"]);
    expect(boardItems(WORDS).map((i) => i.text)).toEqual(["scrambling", "delivered", "cautioned"]);
    // 三种老卡片没有任何可摆的东西 —— 不是崩，是空。
    expect(boardItems({ type: "short_text", prompt: "说说看" })).toEqual([]);
  });
});

describe("这一下是点还是拖", () => {
  it("手抖几个像素仍然是「点」", () => {
    // 🚨 原来的判据是「动了就算拖」。手指按下去总会动一两个像素，于是几乎每次
    // 「点一下选中」都被当成一次落在原地的拖动（落点还在未分类那一堆里），
    // 卡片被放回去，屏幕上什么都没发生。模拟学生走查里这块板出现了 52 步、
    // 她摆了 51 次，四张卡片一张都没进格子 —— 看上去像她不会用，其实是这一行。
    for (const d of [0, 1, 2, 5, DRAG_SLOP]) {
      expect(isDrag({ x: 100, y: 100 }, { x: 100 + d, y: 100 }), `位移 ${d}px 不该算拖`).toBe(false);
    }
  });

  it("真的拖开了就是「拖」", () => {
    expect(isDrag({ x: 100, y: 100 }, { x: 140, y: 100 })).toBe(true);
    expect(isDrag({ x: 100, y: 100 }, { x: 100, y: 60 })).toBe(true);
  });

  it("算的是直线距离，不是横竖各自的位移", () => {
    // 斜着挪 5 和 5，直线距离是 7.07，超过门槛。分别判 x、y 会漏掉这一种。
    expect(isDrag({ x: 0, y: 0 }, { x: 5, y: 5 })).toBe(true);
  });

  it("绕一圈回到原处，仍然是「点」", () => {
    // 距离从**按下的那个点**算，不累加每一帧的位移 —— 累加的话，慢慢挪一圈
    // 再回到原处也会被算成拖了很远，而她手上什么都没发生。
    expect(isDrag({ x: 100, y: 100 }, { x: 100, y: 100 })).toBe(false);
  });
});
