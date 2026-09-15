import { describe, expect, it } from "vitest";
import {
  determinedUnmarkedQuotes,
  highlightSegments,
  notFoundQuotes,
  pickableSentences,
  quoteRanges,
  rangeForQuote,
  unmarkedPointQuotes,
} from "./gradingText";

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

  // Fix round 1: "İ" (U+0130) lowercases to TWO UTF-16 units ("i" + a
  // combining dot above). A map that pushed only one entry per original
  // character shifted every later match's offsets by one.
  it("keeps offsets aligned after a lowercase expansion (İ → two UTF-16 units)", () => {
    const text = "İstanbul is great. 今天天气很好。";
    const quote = "今天天气很好。";
    expect(quoteRanges(text, [quote])).toEqual([{ start: text.indexOf("今天"), end: text.length, index: 0 }]);
  });
});

describe("unmarkedPointQuotes", () => {
  it("names the point whose quote only overlapped an earlier one", () => {
    const quotes = ["去年秋天，我在那里", "我在那里摔过一跤。"];
    expect(unmarkedPointQuotes(quotes, quoteRanges(body, quotes))).toEqual([1]);
  });
  it("names a point whose quote was never found", () => {
    const quotes = ["完全不存在的一句话"];
    expect(unmarkedPointQuotes(quotes, quoteRanges(body, quotes))).toEqual([0]);
  });
  it("does not flag a point with no quote at all", () => {
    expect(unmarkedPointQuotes([null, "  "], [])).toEqual([]);
  });
  it("flags nothing once every quote got a range", () => {
    const quotes = ["去年秋天，我在那里摔过一跤。"];
    expect(unmarkedPointQuotes(quotes, quoteRanges(body, quotes))).toEqual([]);
  });
});

describe("rangeForQuote", () => {
  it("finds one quote on its own, keeping the index it is given", () => {
    const r = rangeForQuote(body, "我读到城市里的雨水花园。", 3);
    expect(r).not.toBeNull();
    expect(r!.index).toBe(3);
    expect(body.slice(r!.start, r!.end)).toBe("我读到城市里的雨水花园。");
  });
  // FB-3: two points may quote overlapping sentences. The student clicks the
  // second one; its highlight must not depend on the first point's range.
  it("highlights a quote that overlaps another point's quote", () => {
    const r = rangeForQuote(body, "我在那里摔过一跤。", 1);
    expect(r).not.toBeNull();
    expect(body.slice(r!.start, r!.end)).toBe("我在那里摔过一跤。");
  });
  it("matches the way the server does (punctuation dropped) and absorbs trailing punctuation", () => {
    const r = rangeForQuote(body, "去年秋天我在那里摔过一跤", 0);
    expect(body.slice(r!.start, r!.end)).toBe("去年秋天，我在那里摔过一跤。");
  });
  it("is null for a quote that is blank, all punctuation, or not in the text", () => {
    expect(rangeForQuote(body, null, 0)).toBeNull();
    expect(rangeForQuote(body, "  ", 0)).toBeNull();
    expect(rangeForQuote(body, "……", 0)).toBeNull();
    expect(rangeForQuote(body, "雨一直下。", 0)).toBeNull();
  });
});

describe("notFoundQuotes", () => {
  it("does not flag a quote that only overlaps another point's quote", () => {
    expect(notFoundQuotes(body, ["去年秋天，我在那里", "我在那里摔过一跤。"])).toEqual([]);
  });
  it("does not flag the same sentence quoted twice", () => {
    expect(notFoundQuotes(body, ["学校后门那片空地一下雨就积水。", "学校后门那片空地一下雨就积水。"])).toEqual([]);
  });
  it("flags a quote that is not in the text, and skips points with no quote", () => {
    expect(notFoundQuotes(body, [null, "完全不存在的一句话", "  ", "我读到城市里的雨水花园。"])).toEqual([1]);
  });
});

describe("determinedUnmarkedQuotes", () => {
  it("is undetermined (null) when the body has not loaded yet", () => {
    expect(determinedUnmarkedQuotes(undefined, ["去年秋天，我在那里摔过一跤。"])).toBeNull();
  });
  it("is undetermined (null) when the body's preload failed — same input as not loaded, by design (see the doc comment: there is no separate 'failed' value at this layer)", () => {
    expect(determinedUnmarkedQuotes(undefined, ["完全不存在的一句话"])).toBeNull();
  });
  // FB-3: the student side flags only quotes that are not found at all; an
  // overlap with another point's quote is not something she can act on.
  it("flags only quotes that are not found, once the body is known", () => {
    expect(determinedUnmarkedQuotes(body, ["去年秋天，我在那里", "我在那里摔过一跤。"])).toEqual(new Set());
    expect(determinedUnmarkedQuotes(body, ["去年秋天，我在那里", "完全不存在的一句话"])).toEqual(new Set([1]));
  });
  it("flags nothing once every quote is found", () => {
    expect(determinedUnmarkedQuotes(body, ["去年秋天，我在那里摔过一跤。"])).toEqual(new Set());
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
