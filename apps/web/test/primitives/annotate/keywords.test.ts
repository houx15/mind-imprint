import { describe, expect, it } from "vitest";
import { splitByKeywords } from "../../../src/primitives/annotate/keywords";

/**
 * 荧光笔的切分。这里测的全是「读代码看不出对错」的那几条：重叠、大小写、
 * 以及那条最要紧的不变量 —— 切完拼回去必须**逐字**等于原文。
 */

const joined = (text: string, terms: string[]) =>
  splitByKeywords(text, terms)
    .map((r) => r.text)
    .join("");

describe("splitByKeywords", () => {
  it("没有词可标时原样返回一段", () => {
    const runs = splitByKeywords("The tide turned.", []);
    expect(runs).toEqual([{ text: "The tide turned.", term: null }]);
  });

  it("命中的那几段带上 term，其余为 null", () => {
    const runs = splitByKeywords("The ice sheet is retreating fast.", ["ice sheet"]);
    expect(runs.map((r) => r.term)).toEqual([null, "ice sheet", null]);
    expect(runs[1]!.text).toBe("ice sheet");
  });

  it("同一个词出现几次就标几次", () => {
    const runs = splitByKeywords("scrambling, then scrambling again", ["scrambling"]);
    expect(runs.filter((r) => r.term !== null)).toHaveLength(2);
  });

  // 🚨 句首那个词在卡片上常常是小写的。按大小写严格匹配，她会看着卡片上有、
  // 正文里没有，只能得出「这个功能坏了」的结论。
  it("大小写不敏感地找，但显示的是正文里的那一份写法", () => {
    const runs = splitByKeywords("Retreating ice is the story.", ["retreating"]);
    const hit = runs.find((r) => r.term !== null);
    expect(hit?.text).toBe("Retreating");
    expect(hit?.term).toBe("Retreating");
  });

  // 🚨 短的先标会把长的切碎成三段：carbon | footprint 之间那个空格成了普通
  // 文本，于是屏幕上是两块分开的荧光，而卡片上写着一个词组。
  it("长的词先标，短的不把它切碎", () => {
    const runs = splitByKeywords("Our carbon footprint doubled.", ["carbon", "carbon footprint"]);
    const hits = runs.filter((r) => r.term !== null);
    expect(hits).toHaveLength(1);
    expect(hits[0]!.text).toBe("carbon footprint");
  });

  // 重叠的区间在 DOM 上没法表达，所以只能标一个。测的是「只标一个」和
  // 「每次标的都是同一个」—— 具体是哪一个由长度、再由字典序定，是个随便定的
  // 顺序；重要的是它不随渲染次数变（React 的 key 会跟着跳）。
  it("重叠的两个词只标一个，而且每次都是同一个", () => {
    const first = splitByKeywords("sea level rise", ["sea level", "level rise"]);
    const again = splitByKeywords("sea level rise", ["level rise", "sea level"]);
    expect(first.filter((r) => r.term !== null)).toHaveLength(1);
    expect(again).toEqual(first);
  });

  // 🚨 这是这个文件里最重要的一条。标注的锚点是这一段文本里的偏移 ——
  // 正文里多一个或少一个字符，之前存下来的每一条标注就都错位了。
  it("切完拼回去逐字等于原文", () => {
    const cases: [string, string[]][] = [
      ["The ice sheet is retreating fast.", ["ice sheet", "retreating"]],
      ["scrambling, then scrambling again", ["scrambling"]],
      ["Our carbon footprint doubled.", ["carbon", "carbon footprint"]],
      ["没有命中任何词的一段中文。", ["retreating"]],
      ["term 就在最前面", ["term"]],
      ["最后一个字是 term", ["term"]],
      ["", ["term"]],
    ];
    for (const [text, terms] of cases) {
      expect(joined(text, terms)).toBe(text);
    }
  });

  it("空白和空串的词被忽略，不会切出空段", () => {
    const runs = splitByKeywords("The tide turned.", ["", "   ", "tide"]);
    expect(runs.every((r) => r.text.length > 0)).toBe(true);
    expect(runs.filter((r) => r.term !== null).map((r) => r.text)).toEqual(["tide"]);
  });

  it("正文里找不到的词就是没有 —— 不猜、不做模糊匹配", () => {
    // 服务端已经把这种卡片整张丢掉了；这里守的是「万一漏过来也不乱标」。
    const runs = splitByKeywords("The tide turned.", ["scramble"]);
    expect(runs).toEqual([{ text: "The tide turned.", term: null }]);
  });
});
