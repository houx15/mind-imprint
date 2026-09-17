import { describe, expect, it } from "vitest";
import {
  carryOverPlacement,
  composeBoardAnswer,
  boardItems,
  WORD_BINS,
  type CoachCardSpec,
} from "../src/readings/CoachCard";

/**
 * 同事 2026-09-17 报的第 3 条，逐字：「像是这种填入句子的卡片，很多时候会出现
 * 两三次交互，对其中的具体句子进行替换的情况（比如 1、4 句正确，2 和 3 重新
 * 填写），需要在 2、3 次调用的时候进行复用，保存上一次的填写结果，不用每一次
 * 都要重新填。」
 *
 * 这一份只测那个纯函数。板本身怎么画由眼睛看（AGENTS.md：UI 用真浏览器看），
 * 而「哪一句该带着哪一格过来」是读代码看不出对错的那一类。
 */

const BINS = ["主张", "证据", "限制", "背景", "对比"];

function labelCard(quotes: string[]): CoachCardSpec {
  return {
    type: "label_roles",
    prompt: "分析下列句子，判断它们各自属于哪一类论证成分。",
    labels: BINS,
    options: quotes.map((q, i) => ({ blockId: `b${i + 1}`, quote: q })),
  } as CoachCardSpec;
}

const A = "Aid groups say there are needs both in Gaza and Israel.";
const B = "They are begging to be allowed into Gaza to help Palestinians.";
const C = "Hundreds of people have been killed.";
const D = "Israel is a country in the Middle East.";

describe("一块板重发的时候", () => {
  it("她上一次摆好的那几句原样带过来", () => {
    const first = labelCard([A, B, C, D]);
    const choice = composeBoardAnswer(
      first,
      { o0: "主张", o1: "证据", o2: "证据", o3: "背景" },
      boardItems(first),
    );
    const second = labelCard([A, B, C, D]);
    const carried = carryOverPlacement({ card: first, choice }, second, BINS);
    expect(carried).toEqual({ o0: "主张", o1: "证据", o2: "证据", o3: "背景" });
  });

  it("按句子认，不按位置认 —— 两张卡上的 o0 常常不是同一句", () => {
    const first = labelCard([A, B, C, D]);
    const choice = composeBoardAnswer(first, { o0: "主张", o3: "背景" }, boardItems(first));
    // 印记 重发时换了顺序，还去掉了一句、添了一句。
    const second = labelCard([D, "The U.N. is worried about working in the area.", A]);
    const carried = carryOverPlacement({ card: first, choice }, second, BINS);
    // D 现在是 o0、A 现在是 o2，各自带着自己那一格过来；新加的那一句没有。
    expect(carried).toEqual({ o0: "背景", o2: "主张" });
  });

  it("上一次没摆的那几张，这一次仍然空着", () => {
    const first = labelCard([A, B, C]);
    const choice = composeBoardAnswer(first, { o0: "主张" }, boardItems(first));
    const carried = carryOverPlacement({ card: first, choice }, labelCard([A, B, C]), BINS);
    expect(carried).toEqual({ o0: "主张" });
  });

  it("这块板上没有的那一格不带过来 —— 带过去她会看到一张卡卡在不存在的格子里", () => {
    const first = labelCard([A, B]);
    const choice = composeBoardAnswer(first, { o0: "对比", o1: "证据" }, boardItems(first));
    const narrow = ["主张", "证据"];
    const second: CoachCardSpec = { ...labelCard([A, B]), labels: narrow } as CoachCardSpec;
    const carried = carryOverPlacement({ card: first, choice }, second, narrow);
    expect(carried).toEqual({ o1: "证据" });
  });

  it("两块板不是同一种，一个都不带", () => {
    const first = labelCard([A, B]);
    const choice = composeBoardAnswer(first, { o0: "主张" }, boardItems(first));
    const words: CoachCardSpec = {
      type: "word_bank",
      prompt: "这几个词你认得哪些？",
      words: [{ blockId: "b1", term: "scramble" }],
    } as CoachCardSpec;
    expect(carryOverPlacement({ card: first, choice }, words, WORD_BINS)).toEqual({});
  });

  it("生词板走同一条路", () => {
    const first: CoachCardSpec = {
      type: "word_bank",
      prompt: "这几个词你认得哪些？",
      words: [
        { blockId: "b1", term: "scramble" },
        { blockId: "b1", term: "intensify" },
      ],
    } as CoachCardSpec;
    const choice = composeBoardAnswer(first, { w0: "认识", w1: "不认识" }, boardItems(first));
    const second: CoachCardSpec = {
      type: "word_bank",
      prompt: "这几个词你认得哪些？",
      words: [
        { blockId: "b1", term: "intensify" },
        { blockId: "b2", term: "besiege" },
      ],
    } as CoachCardSpec;
    expect(carryOverPlacement({ card: first, choice }, second, WORD_BINS)).toEqual({
      w0: "不认识",
    });
  });
});
