import { describe, expect, it } from "vitest";
import { commentPointIsStale } from "./CommentPanel";

/**
 * 「这条意见说的是不是上一版」。
 *
 * 值得测的是两条边界，不是那个 includes —— 判错的方向是不对称的：
 * 把一条**还有效**的意见标成「上一版」，等于告诉她一件没做完的事不用做了；
 * 反过来只是多留一条旧话在屏幕上，点一下「再看一遍」就清掉。
 * 所以拿不准的时候一律「还算数」。
 */
describe("commentPointIsStale", () => {
  const text = "上周五我在食堂门口数了一下，有六个桶是满的。这说明浪费不是个别现象。";

  it("那句话还在，就还算数", () => {
    expect(commentPointIsStale("有六个桶是满的", text)).toBe(false);
  });

  it("那句话被她改掉了，就是上一版", () => {
    expect(commentPointIsStale("有很多桶是满的", text)).toBe(true);
  });

  it("🚨 没给现在的字（成稿那一步还没接）一律当还算数", () => {
    expect(commentPointIsStale("随便一句话", undefined)).toBe(false);
  });

  it("🚨 quote 是空的也当还算数 —— 空串是任何字符串的子串，不挡这一下结论会反过来", () => {
    expect(commentPointIsStale("", text)).toBe(false);
    expect(commentPointIsStale("   ", text)).toBe(false);
  });

  it("她把整段删空了，之前每一条都成了上一版", () => {
    expect(commentPointIsStale("有六个桶是满的", "")).toBe(true);
  });
});
