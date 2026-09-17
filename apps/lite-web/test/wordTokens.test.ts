import { describe, expect, it } from "vitest";
import { wordTokens } from "../src/readings/BlockToolsPanel";

// 「查词」的选择器就是这一段本身。切法读代码看不出对错：撇号、连字符、
// 数字、拼回去是不是原文。
describe("wordTokens", () => {
  const text = "Jingmai O’Connor, a paleontologist, used non-invasive methods 66 million years later — it's odd.";
  const toks = wordTokens(text);
  const words = toks.filter((t) => t.word).map((t) => t.text);

  it("拼回去逐字等于原文", () => {
    expect(toks.map((t) => t.text).join("")).toBe(text);
  });

  it("撇号和连字符留在词里", () => {
    expect(words).toContain("O’Connor");
    expect(words).toContain("non-invasive");
    expect(words).toContain("it's");
  });

  it("数字和标点不是词", () => {
    expect(words).not.toContain("66");
    for (const w of words) {
      expect(w.includes(",") || w.includes(".") || w.includes("—")).toBe(false);
    }
  });

  it("词尾的连字符不算进词", () => {
    expect(wordTokens("well- known").filter((t) => t.word).map((t) => t.text)).toEqual(["well", "known"]);
  });
});
