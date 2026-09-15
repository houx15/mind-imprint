import { describe, expect, it } from "vitest";
import { highlightSegments, pickableSentences, quoteRanges } from "./gradingText";

const body = "学校后门那片空地一下雨就积水。去年秋天，我在那里摔过一跤。\n\n我读到城市里的雨水花园。";

describe("quoteRanges", () => {
  it("finds each quote, sorted by position, keeping the quote's index", () => {
    const ranges = quoteRanges(body, ["我读到城市里的雨水花园。", null, "学校后门那片空地一下雨就积水。"]);
    expect(ranges.map((r) => r.index)).toEqual([2, 0]);
    expect(body.slice(ranges[0]!.start, ranges[0]!.end)).toBe("学校后门那片空地一下雨就积水。");
  });
  it("skips quotes that are missing, blank or not in the text", () => {
    expect(quoteRanges(body, ["", "  ", "雨一直下。"])).toEqual([]);
  });
  it("gives a repeated quote the next occurrence instead of overlapping", () => {
    const text = "雨。雨。";
    expect(quoteRanges(text, ["雨。", "雨。"])).toEqual([
      { start: 0, end: 2, index: 0 },
      { start: 2, end: 4, index: 1 },
    ]);
  });
  it("drops a quote that only overlaps an earlier one", () => {
    expect(quoteRanges(body, ["去年秋天，我在那里", "我在那里摔过一跤。"]).map((r) => r.index)).toEqual([0]);
  });

  // Ruling: quotes are matched the way the server matches them
  // (quotematch.Normalize — whitespace/punctuation dropped, case folded), so
  // a quote the server accepted here is also found and highlighted.
  it("matches despite a half-width vs full-width punctuation difference", () => {
    const src = "我喜欢下雨，因为空气很干净。";
    expect(quoteRanges(src, ["我喜欢下雨,因为空气很干净。"])).toEqual([{ start: 0, end: src.length, index: 0 }]);
  });
  it("highlights across a paragraph break when the quote spans one", () => {
    const quote = "去年秋天，我在那里摔过一跤。我读到城市里的雨水花园。";
    const ranges = quoteRanges(body, [quote]);
    expect(ranges).toHaveLength(1);
    expect(body.slice(ranges[0]!.start, ranges[0]!.end)).toBe(
      "去年秋天，我在那里摔过一跤。\n\n我读到城市里的雨水花园。",
    );
  });
  it("never returns a wrong range: an all-punctuation or absent quote finds nothing", () => {
    expect(quoteRanges(body, ["……", "？！"])).toEqual([]);
    expect(quoteRanges(body, ["完全不存在的一句话"])).toEqual([]);
  });
});

describe("highlightSegments", () => {
  it("covers the whole text in order", () => {
    const segs = highlightSegments(body, quoteRanges(body, ["去年秋天，我在那里摔过一跤。"]));
    expect(segs.map((s) => s.text).join("")).toBe(body);
    expect(segs.filter((s) => s.index !== null).map((s) => s.text)).toEqual(["去年秋天，我在那里摔过一跤。"]);
  });
  it("returns one plain segment with no ranges", () => {
    expect(highlightSegments("雨", [])).toEqual([{ text: "雨", index: null }]);
  });
});

describe("pickableSentences", () => {
  it("only offers sentences that are literally in the text", () => {
    const got = pickableSentences(body);
    expect(got.length).toBeGreaterThan(1);
    for (const s of got) expect(body.includes(s)).toBe(true);
  });
});
