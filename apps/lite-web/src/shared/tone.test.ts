import { describe, expect, it } from "vitest";
import { DONE, TODO, toneAt, toneNameAt, tone } from "./tone";

// 🚨 工具面上不许再出现手写的十六进制。
//
// 产品负责人 2026-09-03：「please also follow our design tokens」。写死的颜色
// 不跟暗色模式走——令牌在暗色下整组被换掉，十六进制不会，于是暗色里那几块底色
// 会亮得刺眼。这条测试盯的就是"有没有人又手写了一个颜色"。
describe("palette", () => {
  it("only ever hands out CSS variables, never a literal colour", () => {
    const all = [
      ...[0, 1, 2, 3, 4, 5, 6, 7].flatMap((i) => Object.values(toneAt(i))),
      ...Object.values(tone("mist")),
      ...Object.values(DONE),
      ...Object.values(TODO),
    ];
    for (const v of all) {
      expect(v).toMatch(/^var\(--mk-[a-z0-9-]+\)$/);
      expect(v).not.toMatch(/#[0-9a-f]{3,8}/i);
    }
  });

  // 每一档都要三个值都在：只给 solid 就会有人拿实色当整块背景，压上去的字读不了。
  it("gives every tone a solid, a background and a foreground", () => {
    const t = tone("taro");
    expect(t.solid).toBe("var(--mk-taro)");
    expect(t.bg).toBe("var(--mk-taro-bg)");
    expect(t.fg).toBe("var(--mk-taro-fg)");
  });

  // 同一个位置永远是同一个色：上面选中的那张卡和下面「放掉的」那张，是靠颜色
  // 认出彼此的。
  it("is stable and wraps instead of running off the end", () => {
    expect(toneNameAt(0)).toBe(toneNameAt(0));
    expect(toneNameAt(7)).toBe(toneNameAt(0));
    expect(toneNameAt(-1)).toMatch(/^[a-z]+$/);
  });

  it("never repeats a colour inside one cycle", () => {
    const names = [0, 1, 2, 3, 4, 5, 6].map(toneNameAt);
    expect(new Set(names).size).toBe(names.length);
  });
});
