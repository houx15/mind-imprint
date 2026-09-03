import { describe, expect, it } from "vitest";

import { toBlocks } from "./Says";

/** 取第 n 块并断定它是列表——只为让类型收窄，读起来还是「第几块是列表」。 */
function listAt(content: string, n: number) {
  const b = toBlocks(content)[n];
  if (b?.kind !== "list") throw new Error(`第 ${n} 块不是列表`);
  return b;
}

// 这个函数值得测，因为它坏掉的样子是「看着还在，只是挤成了一行」——
// 线上就是这么坏的：模型分了三条要点，气泡把换行折没了。
describe("toBlocks", () => {
  it("把连着的要点收进同一个列表", () => {
    const blocks = toBlocks(
      "你记了一句别人的原话。\n\n· 里面黑——车棚本身的问题\n· 早读快迟到——时间压力\n· 还得走回来——多走一段路\n\n哪一件最可能是原因？",
    );
    expect(blocks.map((b) => b.kind)).toEqual(["p", "list", "p"]);
    const list = listAt(
      "你记了一句别人的原话。\n\n· 里面黑——车棚本身的问题\n· 早读快迟到——时间压力\n· 还得走回来——多走一段路\n\n哪一件最可能是原因？",
      1,
    );
    expect(list.items).toHaveLength(3);
    expect(list.items[0]?.text).toBe("里面黑——车棚本身的问题");
  });

  it("认 -、*、1. 这几种写法", () => {
    const list = listAt("- 一\n* 二\n1. 三\n2、四", 0);
    expect(list.items.map((i) => i.text)).toEqual(["一", "二", "三", "四"]);
    // 编号要留住她看到的那个数字，符号则统一显示成「·」。
    expect(list.items[2]?.marker).toBe("1.");
  });

  it("中间隔了一段话就另起一个列表", () => {
    const blocks = toBlocks("· 一\n中间说了句话\n· 二");
    expect(blocks.map((b) => b.kind)).toEqual(["list", "p", "list"]);
  });

  it("单独一句话不拆", () => {
    const blocks = toBlocks("那这些车是怎么停的？");
    expect(blocks).toEqual([{ kind: "p", text: "那这些车是怎么停的？" }]);
  });

  it("小数不会被当成要点", () => {
    // 「3.5 倍」开头的一句话，如果放行 `.` 后面不带空格，就会被拆成一条要点。
    expect(toBlocks("3.5 倍于去年的量")).toEqual([{ kind: "p", text: "3.5 倍于去年的量" }]);
  });

  it("年份开头的句子不会被当成要点", () => {
    expect(toBlocks("2026 年的数据还没回来")).toEqual([
      { kind: "p", text: "2026 年的数据还没回来" },
    ]);
  });

  it("破折号开头的句子不是要点", () => {
    // 「——这是时间压力」这种续写不能被当成要点，否则一句话会被拆成两块。
    const blocks = toBlocks("里面黑——这是车棚本身的问题");
    expect(blocks).toEqual([{ kind: "p", text: "里面黑——这是车棚本身的问题" }]);
  });
});
