import { describe, expect, it } from "vitest";
import { quoteFoundInDraft } from "./draftQuote";

describe("quoteFoundInDraft", () => {
  it("找得到就可点", () => {
    expect(quoteFoundInDraft("中国的碳排放全球第一，这是她论证要面对的反例。", "中国的碳排放全球第一")).toBe(true);
  });

  it("她已经改掉那句话，就找不到了", () => {
    expect(quoteFoundInDraft("她后来把这句整个删掉，换成了别的论证。", "中国的碳排放全球第一")).toBe(false);
  });

  it("标点、空白不算数——沿用同一套归一化规则", () => {
    // 服务端引的是「中国的碳排放全球第一。」，正文里那句话结尾换了逗号，
    // 中间多了个全角空格：同一条 quotematch.Normalize 规则下仍然算找到。
    expect(quoteFoundInDraft("中国的碳排放全球第一　，后面接着写别的。", "中国的碳排放全球第一。")).toBe(true);
  });

  it("大小写不算数——英文稿同理", () => {
    expect(quoteFoundInDraft("China leads the world in carbon emissions today.", "CHINA leads the world in carbon emissions")).toBe(
      true,
    );
  });

  it("引文是空的/纯标点，永远不可点", () => {
    expect(quoteFoundInDraft("随便什么正文。", "")).toBe(false);
    expect(quoteFoundInDraft("随便什么正文。", "   ")).toBe(false);
    expect(quoteFoundInDraft("随便什么正文。", "……")).toBe(false);
    expect(quoteFoundInDraft("随便什么正文。", null)).toBe(false);
  });

  it("草稿是空的，什么都找不到", () => {
    expect(quoteFoundInDraft("", "中国的碳排放全球第一")).toBe(false);
  });

  it("引文只是草稿里另一句话的子串误判——必须真的出现，不是巧合前缀", () => {
    expect(quoteFoundInDraft("她讨论的是碳排放增长趋势，不是总量第一。", "中国的碳排放全球第一")).toBe(false);
  });
});
