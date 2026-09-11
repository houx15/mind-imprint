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

/**
 * 🚨 quote 判据一个人不够用，这一条记的就是它漏掉的那一格。
 *
 * 2026-09-11 第五轮线上走查，一个学生连着四步说同一件事：
 *   「我已经按它说的加了让步句，但下面还是显示缺，不知道是不是没刷新」
 * 那条意见说「缺让步」，她的做法是**新加一句**，被引的那句原封不动 ——
 * 于是 quote 还在，判据说「这条还算数」，屏幕接着说她没做。
 * 判错的方向恰好是最伤的那个：她做完了，产品说她没做。
 *
 * 所以「改过没有」要另外问，而且只有服务端答得上（它存了当时读的那一版）。
 */
describe("她新加了一句、没动被引的那句", () => {
  const before = "食堂每天倒掉的饭特别多。上周五我数了六个桶。";
  const after = before + "当然，也有人是真的吃不下。";

  it("被引的那句还在，所以这条意见本身不算「上一版」", () => {
    expect(commentPointIsStale("上周五我数了六个桶", after)).toBe(false);
  });

  it("但这一段确实动过了 —— 这件事 quote 答不出来，得靠存下来的那一版", () => {
    expect(after === before).toBe(false);
  });
});
