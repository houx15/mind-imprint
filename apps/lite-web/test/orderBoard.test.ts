import { describe, expect, it } from "vitest";
import { composeOrderAnswer, moveItem, parseOrderAnswer } from "../src/readings/OrderBoard";

// 排序板上读代码看不出对错的只有这三件：挪一张之后的顺序、交上去的那段字、
// 以及从那段字读回来的顺序（刷新之后屏幕上摆的就是它）。

const items = [
  { id: "o0", text: "The storm reached the coast on Friday.", blockId: "b1", where: "第1段" },
  { id: "o1", text: "Officials ordered an evacuation on Wednesday.", blockId: "b2", where: "第2段" },
  { id: "o2", text: "Power was restored by Sunday.", blockId: "b3", where: "第3段" },
];

describe("moveItem", () => {
  it("往前、往后、越界", () => {
    expect(moveItem(["a", "b", "c"], 2, 0)).toEqual(["c", "a", "b"]);
    expect(moveItem(["a", "b", "c"], 0, 2)).toEqual(["b", "c", "a"]);
    const same = ["a", "b"];
    expect(moveItem(same, 0, 5)).toBe(same);
    expect(moveItem(same, 1, 1)).toBe(same);
  });
});

describe("排序板的作答", () => {
  it("序号一行、原文一行，读回来是同一个顺序", () => {
    const order = ["o1", "o0", "o2"];
    const text = composeOrderAnswer(order, items);
    expect(text.split("\n")).toEqual([
      "第1：",
      "Officials ordered an evacuation on Wednesday.",
      "第2：",
      "The storm reached the coast on Friday.",
      "第3：",
      "Power was restored by Sunday.",
    ]);
    expect(parseOrderAnswer(items, text)).toEqual(order);
  });

  it("认不出来的行跳过，不编顺序", () => {
    expect(parseOrderAnswer(items, "我排好了")).toEqual([]);
    expect(parseOrderAnswer(items, "第1：\n文章里没有的一句")).toEqual([]);
  });
});
