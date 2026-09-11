import { describe, expect, it } from "vitest";
import { countWords } from "@/workspace/blocks/wordcount";
import { wordUnit } from "./wordUnit";

/**
 * 数对了、单位说错了，她一样没法判断自己到没到。
 *
 * 2026-09-12 第十二轮线上走查，英文那个学生连着四步在算自己超没超，
 * 而屏幕上写的是「已保存 · 167 字」—— 数字是词数，单位是字。
 */
describe("wordUnit", () => {
  it("英文篇按词", () => {
    expect(wordUnit("en")).toBe("词");
  });

  it("中文篇按字", () => {
    expect(wordUnit("zh")).toBe("字");
  });

  it("🚨 拿不准一律按中文 —— 这个房间默认是中文，lang 缺失不该冒出一个「词」", () => {
    expect(wordUnit(null)).toBe("字");
    expect(wordUnit(undefined)).toBe("字");
    expect(wordUnit("")).toBe("字");
  });

  // 单位跟着 countWords 的口径走，两者必须说的是同一件事：
  // 中文一个字算一个，英文一个词算一个。
  it("单位和数出来的东西对得上", () => {
    expect(countWords("食堂每天倒掉很多饭")).toBe(9); // 9 个字
    expect(countWords("The canteen wastes food every day")).toBe(6); // 6 个词
  });
});
