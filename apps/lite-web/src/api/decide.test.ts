import { describe, expect, it } from "vitest";
import { joinWhyNot, optionTag, optionTone } from "./decide";

// 🚨 「为什么放掉别的」是这件工具真正教的东西，所以她是一条一条分开答的，
// 而后端存的是一个字符串。拼错了，回灌给印记的就是一句认不出谁是谁的话。
describe("joinWhyNot", () => {
  it("keeps each rejected option attached to its own reason", () => {
    expect(
      joinWhyNot([
        { label: "改打饭的量", why: "只压住了症状" },
        { label: "改菜单", why: "食堂不归我们管" },
      ]),
    ).toBe("改打饭的量：只压住了症状；改菜单：食堂不归我们管");
  });

  // 她可以只答一条就先确认——没答的那条不该在句子里留下一个空壳
  //（「改菜单：」读起来像她说了什么，其实没有）。
  it("drops the ones she has not answered yet", () => {
    expect(
      joinWhyNot([
        { label: "改打饭的量", why: "  " },
        { label: "改菜单", why: "食堂不归我们管" },
      ]),
    ).toBe("改菜单：食堂不归我们管");
  });

  it("is empty when she has answered none", () => {
    expect(joinWhyNot([{ label: "改菜单", why: "" }])).toBe("");
    expect(joinWhyNot([])).toBe("");
  });

  it("trims so a stray space does not count as an answer", () => {
    expect(joinWhyNot([{ label: "  改菜单 ", why: " 太慢 " }])).toBe("改菜单：太慢");
  });
});

// 选项的颜色和编号要稳定：上面选中的那张、下面「放掉的」那张，是靠这两样认出
// 彼此的。循环也不能越界——印记给几个选项由它自己定。
// 🚨 「输给第一名的地方」比「为什么不选别的」问得具体得多，而它只有在她真的
// 排过序之后才问得出来。排序本身就是她把几条路放在一起比过的证据。
describe("joinWhyNot keeps working with a ranked list", () => {
  it("still pairs each rejected option with its own reason", () => {
    expect(
      joinWhyNot([
        { label: "教室走廊", why: "课间只有十分钟" },
        { label: "校门口", why: "老师要额外批准" },
      ]),
    ).toBe("教室走廊：课间只有十分钟；校门口：老师要额外批准");
  });
});

describe("optionTone / optionTag", () => {
  it("gives the same option the same colour and letter every time", () => {
    expect(optionTone(0)).toEqual(optionTone(0));
    expect(optionTag(0)).toBe("A");
    expect(optionTag(2)).toBe("C");
  });

  // 🚨 颜色必须是设计令牌，不是手写的十六进制——写死的颜色不跟暗色模式走。
  it("hands out design tokens, never a literal colour", () => {
    for (const v of Object.values(optionTone(99))) {
      expect(v).toMatch(/^var\(--mk-[a-z0-9-]+\)$/);
    }
    expect(optionTag(26)).toBe("A");
  });

  it("never gives two of the first five options the same colour", () => {
    const solids = [0, 1, 2, 3, 4].map((i) => optionTone(i).solid);
    expect(new Set(solids).size).toBe(5);
  });
});
