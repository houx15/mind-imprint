import { describe, expect, it } from "vitest";
import { pageNumbers } from "./Pagination";

/**
 * 页码条的形状。
 *
 * 值得测，是因为它属于「读代码看不出对错」的那一类：边界（第 1 页、最后一页、
 * 只有三页）错了不会报错，只会让页码条在她点的时候跳来跳去。
 * 0 是省略号。
 */
describe("pageNumbers", () => {
  it("页数少的时候全部列出来", () => {
    expect(pageNumbers(1, 1)).toEqual([1]);
    expect(pageNumbers(2, 3)).toEqual([1, 2, 3]);
    expect(pageNumbers(4, 7)).toEqual([1, 2, 3, 4, 5, 6, 7]);
  });

  it("页数多的时候固定形状：头、尾、当前页左右各一", () => {
    expect(pageNumbers(1, 30)).toEqual([1, 2, 0, 30]);
    expect(pageNumbers(15, 30)).toEqual([1, 0, 14, 15, 16, 0, 30]);
    expect(pageNumbers(30, 30)).toEqual([1, 0, 29, 30]);
  });

  // 🚨 长度要稳。一条会变长变短的页码条会把下面的卡片顶来顶去，
  // 她点「下一页」的时候按钮会从指针底下跑掉。
  it("中间那几页的长度不变", () => {
    const widths = new Set<number>();
    for (let p = 4; p <= 27; p++) widths.add(pageNumbers(p, 30).length);
    expect([...widths]).toEqual([7]);
  });

  it("省略号只在真的跳过了页码时出现", () => {
    // 第 3 页：1,2,3,4 是连着的，前面不该有省略号。
    expect(pageNumbers(3, 30)).toEqual([1, 2, 3, 4, 0, 30]);
    expect(pageNumbers(2, 30)).toEqual([1, 2, 3, 0, 30]);
  });

  it("不出现重复的页码", () => {
    for (const [p, n] of [
      [1, 30],
      [2, 30],
      [29, 30],
      [30, 30],
      [5, 9],
    ] as const) {
      const nums = pageNumbers(p, n).filter((x) => x !== 0);
      expect(new Set(nums).size).toBe(nums.length);
    }
  });
});
