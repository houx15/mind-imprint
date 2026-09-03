import { describe, expect, it } from "vitest";
import { reframeSentence, seedHmw } from "./reframe";

/**
 * 只测「读代码看不出对错」的东西。
 *
 * seedHmw 起的那句头会被原样交到她手上当草稿，所以它拼错了不会报错——只会
 * 让她拿到一句读不通的话，然后以为是自己写错了。这正是值得钉住的一类。
 */
describe("seedHmw", () => {
  it("把她自己写的两句拼成一个问句", () => {
    expect(seedHmw("中午最后一批来打饭的人", "知道菜还剩不剩")).toBe(
      "我们可以怎样帮助中午最后一批来打饭的人，让他能够知道菜还剩不剩？",
    );
  });

  it("空着的格子留「……」，看得出缺在哪儿", () => {
    // 🚨 不是留空。一句被删干净的话看起来只是没写，一句缺了词的话才看得出
    // 还差哪一格。
    expect(seedHmw("", "知道菜还剩不剩")).toContain("帮助……，");
    expect(seedHmw("值日生", "")).toContain("能够……？");
  });

  it("前后空白不带进句子里", () => {
    expect(seedHmw("  值日生  ", " 少倒几盒 ")).toBe("我们可以怎样帮助值日生，让他能够少倒几盒？");
  });
});

describe("reframeSentence", () => {
  it("她的答案已经以「因为」开头时，模板不再补一个", () => {
    // 问的是「为什么这对他重要？」，答案几乎总是「因为…」开头。补两次就成了
    // 「因为 因为课间只有十分钟」。
    expect(reframeSentence({ who: "值日生", needs: "少倒几盒", why: "因为课间只有十分钟" })).toBe(
      "值日生需要少倒几盒，因为课间只有十分钟。",
    );
  });

  it("三格全空时给空串，而不是一句全是省略号的话", () => {
    expect(reframeSentence({ who: "", needs: "", why: "" })).toBe("");
  });
});
