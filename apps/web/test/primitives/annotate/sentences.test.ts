import { describe, it, expect } from "vitest";
import { segmentSentences, sentenceAtOffset } from "@/primitives/annotate/sentences";

describe("segmentSentences", () => {
  it("splits Chinese sentences on 。！？ and keeps the terminator", () => {
    const segs = segmentSentences("中国碳排放全球第一。这是要正面处理的反例！你怎么看？");
    expect(segs.map((s) => s.text)).toEqual([
      "中国碳排放全球第一。",
      "这是要正面处理的反例！",
      "你怎么看？",
    ]);
    // Rune offsets are contiguous and cover the string.
    expect(segs[0]!.start).toBe(0);
    expect(segs[2]!.end).toBe([...("中国碳排放全球第一。这是要正面处理的反例！你怎么看？")].length);
  });

  it("splits English sentences and skips the inter-sentence space", () => {
    const segs = segmentSentences("Vegetation rose. Emissions also grew. Is that sustainable?");
    expect(segs.map((s) => s.text)).toEqual([
      "Vegetation rose.",
      "Emissions also grew.",
      "Is that sustainable?",
    ]);
  });

  it("does NOT over-split on decimals or abbreviations", () => {
    const segs = segmentSentences("The U.S. emitted 3.5 Gt in 2023. That is a lot.");
    expect(segs.map((s) => s.text)).toEqual([
      "The U.S. emitted 3.5 Gt in 2023.",
      "That is a lot.",
    ]);
  });

  it("returns the whole text as one sentence when there is no terminator", () => {
    const segs = segmentSentences("一句没有句号的话");
    expect(segs).toHaveLength(1);
    expect(segs[0]!.text).toBe("一句没有句号的话");
  });

  it("keeps a trailing closing quote with its sentence", () => {
    const segs = segmentSentences("他说「这不可持续」。然后走了。");
    expect(segs[0]!.text).toBe("他说「这不可持续」。");
    expect(segs[1]!.text).toBe("然后走了。");
  });

  it("splits an English sentence ending in a closing quote (.\")", () => {
    const segs = segmentSentences(`He said "go home." Then he left.`);
    expect(segs.map((s) => s.text)).toEqual([`He said "go home."`, "Then he left."]);
  });
});

describe("sentenceAtOffset", () => {
  const segs = segmentSentences("第一句。第二句。第三句。");
  it("returns the sentence containing the offset", () => {
    expect(sentenceAtOffset(segs, 0)!.text).toBe("第一句。");
    expect(sentenceAtOffset(segs, 4)!.text).toBe("第二句。"); // offset 4 = '第' of 第二句
    expect(sentenceAtOffset(segs, 9)!.text).toBe("第三句。");
  });
  it("clamps to the first/last for out-of-range offsets", () => {
    expect(sentenceAtOffset(segs, -5)!.text).toBe("第一句。");
    expect(sentenceAtOffset(segs, 999)!.text).toBe("第三句。");
  });
});
