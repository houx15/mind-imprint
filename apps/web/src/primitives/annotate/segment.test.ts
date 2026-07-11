import { describe, it, expect } from "vitest";
import { segmentBlock } from "./segment";
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
});
