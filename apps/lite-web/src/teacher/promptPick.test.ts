import { describe, expect, it } from "vitest";
import { promptBody, wordsOf } from "./PromptLibraryPicker";
import { promptQueryString } from "../api/writingPrompts";
import type { WritingPrompt } from "../api/writingPrompts";

const p = (extra: Partial<WritingPrompt> = {}): WritingPrompt => ({
  id: "X-1",
  category: "高考语文",
  lang: "zh",
  year: 2026,
  type: "真题",
  source: "2026年全国一卷",
  taskType: "材料作文",
  text: "阅读下面的材料，根据要求写作。",
  topics: [],
  difficulty: 2,
  diffLabel: "进阶",
  ...extra,
});

/**
 * 🚨 目标字数是会写进作业、学生那边照着它算进度的数。
 * 猜错了她会被一个不存在的要求追着跑，所以取不到就**返回 null**，不猜。
 */
describe("wordsOf", () => {
  it("中英两种写法都认", () => {
    expect(wordsOf("不少于800字")).toBe(800);
    expect(wordsOf("at least 250 words")).toBe(250);
    expect(wordsOf("80词左右")).toBe(80);
    expect(wordsOf("不超过60词")).toBe(60);
  });

  // 区间取上限：「120-150词」老师要的是 150 那一档。
  it("区间取上限", () => {
    expect(wordsOf("120-150词")).toBe(150);
    expect(wordsOf("600-800之间")).toBe(800);
  });

  it("取不到就是 null，不猜一个", () => {
    expect(wordsOf(undefined)).toBeNull();
    expect(wordsOf("")).toBeNull();
    expect(wordsOf("字数不限")).toBeNull();
    // 🚨 太小的数字不是字数要求（「二选一」里的 2、「满分60分」里的分值）。
    expect(wordsOf("二选一")).toBeNull();
  });
});

describe("promptBody", () => {
  it("题面在前，出处缀在最后一行", () => {
    const body = promptBody(p({ wordLimit: "不少于800字" }));
    expect(body.startsWith("阅读下面的材料")).toBe(true);
    expect(body).toContain("2026年全国一卷 · 不少于800字");
  });

  it("没有出处和字数时不留一个空括号", () => {
    expect(promptBody(p({ source: "", wordLimit: undefined }))).toBe(
      "阅读下面的材料，根据要求写作。",
    );
  });
});

/**
 * 🚨 查询串要**省掉空值**。带上 `?lang=&topic=&page=1` 的话，
 * 每一次筛选都会生成一个不同的 URL，缓存和「回到上一次」都对不上。
 */
describe("promptQueryString", () => {
  it("什么都不筛就是空串", () => {
    expect(promptQueryString({})).toBe("");
    expect(promptQueryString({ lang: "", topic: "", q: "   " })).toBe("");
  });

  it("第 1 页不写进去", () => {
    expect(promptQueryString({ page: 1 })).toBe("");
    expect(promptQueryString({ page: 3 })).toBe("?page=3");
  });

  it("0 值不写进去（难度 0 = 不筛）", () => {
    expect(promptQueryString({ difficulty: 0, year: 0 })).toBe("");
    expect(promptQueryString({ difficulty: 2 })).toBe("?difficulty=2");
  });

  it("搜索词去掉两头空格", () => {
    expect(promptQueryString({ q: "  短视频  " })).toBe("?q=%E7%9F%AD%E8%A7%86%E9%A2%91");
  });
});
