import { describe, expect, it } from "vitest";
import { reviewTodo, splitByMarks, type ReviewMark } from "./review";

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

describe("reviewTodo", () => {
  // 🚨 审核这件事的产出只有一个：她的判断。
  //
  // 原来这里要她把印记划出来的每一句、列出来的每一个方面全填完才准点完成
  // （「还有 3 句话没回答，2 个方面没说」）——那是一张作业卷子，而
  // 「form-like things」正是产品负责人 2026-09-01 明确否掉的东西。
  it("只差她的判断，不数还有几格没填", () => {
    expect(reviewTodo(false, "")).toBe("结论");
    expect(reviewTodo(false, "第二段站不住")).toBe("");
    // 留了意见本身就是判断：结论只会是「执行修改」。
    expect(reviewTodo(true, "")).toBe("");
  });
});
