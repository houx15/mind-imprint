import { describe, expect, it } from "vitest";
import { partProgress, splitByMarks, splitIntoParts, type ReviewMark } from "./review";

function mark(id: string, quote: string, extra: Partial<ReviewMark> = {}): ReviewMark {
  return {
    id,
    part: "",
    partNote: "",
    quote,
    question: "这句站得住吗？",
    answer: "",
    sessionId: null,
    ordinal: 0,
    mine: false,
    ...extra,
  };
}

const TEXT = "中国的碳排放总量全球第一。人均排放低于美国。这两句都对。";

describe("splitByMarks", () => {
  it("keeps the highlight inside the original sentence", () => {
    const segs = splitByMarks(TEXT, [mark("m1", "人均排放低于美国。")]);
    expect(segs.map((s) => s.text).join("")).toBe(TEXT);
    expect(segs.find((s) => s.mark)?.text).toBe("人均排放低于美国。");
  });

  // 划线的顺序不该取决于服务端返回的顺序。
  it("orders segments by position in the text, not by input order", () => {
    const segs = splitByMarks(TEXT, [mark("late", "这两句都对。"), mark("early", "中国的碳排放总量全球第一。")]);
    const marked = segs.filter((s) => s.mark).map((s) => s.mark!.id);
    expect(marked).toEqual(["early", "late"]);
    expect(segs.map((s) => s.text).join("")).toBe(TEXT);
  });

  // 🚨 两条线叠在同一句上时，把文字切碎会让两条都读不通。原文必须原样拼回来。
  it("drops an overlapping mark rather than shredding the text", () => {
    const segs = splitByMarks(TEXT, [
      mark("wide", "人均排放低于美国。这两句都对。"),
      mark("inner", "这两句都对。"),
    ]);
    expect(segs.map((s) => s.text).join("")).toBe(TEXT);
    expect(segs.filter((s) => s.mark)).toHaveLength(1);
  });

  // 印记划的句子可能来自另一段。找不到就当没划，不能把文字弄丢。
  it("ignores a quote that is not in this paragraph", () => {
    const segs = splitByMarks(TEXT, [mark("elsewhere", "完全不在这一段里的一句话")]);
    expect(segs).toHaveLength(1);
    expect(segs[0]?.mark).toBeNull();
    expect(segs[0]?.text).toBe(TEXT);
  });

  it("returns the text unchanged when nothing is marked", () => {
    expect(splitByMarks(TEXT, []).map((s) => s.text).join("")).toBe(TEXT);
  });

  // 空引用不该把每个字之间都插一个空高亮。
  it("ignores an empty quote", () => {
    const segs = splitByMarks(TEXT, [mark("blank", "   ")]);
    expect(segs).toHaveLength(1);
    expect(segs[0]?.mark).toBeNull();
  });
});

// 🚨 一整篇摊在那里，她能做的只有从头划到尾——那是"读过了"，不是"审过了"。
// 产品负责人 2026-09-03：「we must go into texts, instead of presenting a large
// text」。设计文档要的是「explanations for each part so that we know what we
// should care about in each part」。
describe("splitIntoParts", () => {
  const paras = ["开头这一段。", "中间讲做法。", "结尾收一下。"];

  it("starts a new part where a mark names one", () => {
    const parts = splitIntoParts(paras, [
      mark("m1", "开头这一段。", { part: "开头", partNote: "第一句决定别人读不读下去" }),
      mark("m2", "中间讲做法。", { part: "做法", partNote: "步骤要能照着做" }),
    ]);
    expect(parts.map((p) => p.name)).toEqual(["开头", "做法"]);
    // 没有划线的结尾段跟着它前面那一部分走，不会凭空丢掉。
    expect(parts[1]?.paragraphs).toEqual(["中间讲做法。", "结尾收一下。"]);
    expect(parts[0]?.note).toBe("第一句决定别人读不读下去");
  });

  // 印记没分段的时候不能崩，也不能把每一段拆成一部分。
  it("falls back to one part when no mark names one", () => {
    const parts = splitIntoParts(paras, [mark("m1", "中间讲做法。")]);
    expect(parts).toHaveLength(1);
    expect(parts[0]?.paragraphs).toEqual(paras);
    expect(parts[0]?.marks.map((m) => m.id)).toEqual(["m1"]);
  });

  it("files each mark under the part its sentence lives in", () => {
    const parts = splitIntoParts(paras, [
      mark("m1", "开头这一段。", { part: "开头" }),
      mark("m2", "中间讲做法。", { part: "做法" }),
      mark("m3", "结尾收一下。", { part: "做法" }),
    ]);
    expect(parts[0]?.marks.map((m) => m.id)).toEqual(["m1"]);
    expect(parts[1]?.marks.map((m) => m.id)).toEqual(["m2", "m3"]);
  });

  // 🚨 引文在原文里找不到（模型抄错一个字）时，那条问题不能就此消失——
  // 她仍然该看见印记问了什么。
  it("keeps a question whose quote does not appear in the text", () => {
    const parts = splitIntoParts(paras, [mark("lost", "这句原文里没有")]);
    expect(parts[0]?.marks.map((m) => m.id)).toEqual(["lost"]);
  });

  it("handles an empty document without throwing", () => {
    expect(splitIntoParts([], [])).toEqual([]);
  });
});

describe("partProgress", () => {
  it("counts only the answered ones", () => {
    const parts = splitIntoParts(["一段。"], [
      mark("a", "一段。", { answer: "答了" }),
      mark("b", "一段。"),
    ]);
    expect(partProgress(parts[0]!)).toEqual({ done: 1, total: 2 });
  });
});
