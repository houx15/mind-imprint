import { describe, it, expect } from "vitest";
import { segmentBlock } from "@/primitives/annotate/segment";
import type { AnnotateState } from "@mind-imprint/contracts";

const spans: AnnotateState["spans"] = [
  { id: "s1", block_ref: "b1", range: { start: 3, end: 7 }, tag: "authority", note: "who?", author: "ai" },
];

describe("segmentBlock", () => {
  it("returns one plain run when no spans touch the block", () => {
    expect(segmentBlock("b1", "hello world", [])).toEqual([{ text: "hello world", spanId: null, author: null }]);
  });
  it("splits a mid-block span into plain/marked/plain runs", () => {
    const runs = segmentBlock("b1", "abcXXXXdef", spans); // range 3..7 => "XXXX"
    expect(runs).toEqual([
      { text: "abc", spanId: null, author: null },
      { text: "XXXX", spanId: "s1", author: "ai" },
      { text: "def", spanId: null, author: null },
    ]);
  });
  it("ignores spans for other blocks", () => {
    expect(segmentBlock("bOther", "abcXXXXdef", spans)).toEqual([{ text: "abcXXXXdef", spanId: null, author: null }]);
  });
  it("marks the whole block when a span has no range", () => {
    const whole: AnnotateState["spans"] = [{ id: "s2", block_ref: "b1", tag: "t", note: "", author: "student" }];
    expect(segmentBlock("b1", "abc", whole)).toEqual([{ text: "abc", spanId: "s2", author: "student" }]);
  });

  // Offsets are RUNE (code point) indices (spec §7.1), matching Go's
  // utf8.RuneCountInString. These cases pin that segmentBlock slices by code
  // point rather than by UTF-16 code unit (String.prototype.slice), which
  // diverges from runes for CJK+non-BMP mixes and for any non-BMP block.
  it("marks a mid-string phrase in long realistic Chinese material by rune index", () => {
    // Real production content (studio/fixtures.ts, block b1) with the real
    // seeded anchor's rune offsets: 21..33 => "根据 NASA 卫星数据".
    const text =
      "过去二十年里发生了一件几乎没人注意到的事：根据 NASA 卫星数据，地球比 2000 年整整绿了一圈，而这背后最大的推手，是中国。";
    const cjkSpans: AnnotateState["spans"] = [
      { id: "s3", block_ref: "b1", range: { start: 21, end: 33 }, tag: "authority", note: "", author: "ai" },
    ];
    const runs = segmentBlock("b1", text, cjkSpans);
    const marked = runs.find((r) => r.spanId === "s3");
    expect(marked?.text).toBe("根据 NASA 卫星数据");
  });

  it("keeps CJK+emoji spans aligned when a non-BMP character precedes the marked run", () => {
    // "🎉" is a surrogate pair: 1 rune but 2 UTF-16 code units. A UTF-16
    // slice using rune indices would land one code unit short and shifted.
    const text = "今天🎉的天气真好，我们一起去散步吧。";
    const emojiSpans: AnnotateState["spans"] = [
      { id: "s4", block_ref: "b1", range: { start: 4, end: 8 }, tag: "t", note: "", author: "student" },
    ];
    const runs = segmentBlock("b1", text, emojiSpans);
    const marked = runs.find((r) => r.spanId === "s4");
    expect(marked?.text).toBe("天气真好");
  });

  it("pins Array.from-based code-point slicing for a block of only non-BMP characters", () => {
    // Each emoji here is its own surrogate pair; a UTF-16 slice(1,2) would
    // return one lone unpaired surrogate half instead of the middle emoji.
    const text = "🎉🎊🎈";
    const emojiOnlySpans: AnnotateState["spans"] = [
      { id: "s5", block_ref: "b1", range: { start: 1, end: 2 }, tag: "t", note: "", author: "ai" },
    ];
    const runs = segmentBlock("b1", text, emojiOnlySpans);
    const marked = runs.find((r) => r.spanId === "s5");
    expect(marked?.text).toBe("🎊");
  });
});
