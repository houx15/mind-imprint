import { describe, expect, it } from "vitest";
import { transcriptLines } from "./transcriptLines";
import type { LiteMessage } from "@lite/api/readingRoom";

const m = (seq: number, role: string, content: string, payload?: unknown): LiteMessage =>
  ({ seq, role, content, createdAt: "2026-09-16T00:00:00Z", payload } as LiteMessage);

describe("transcriptLines", () => {
  it("keeps her words and 印记's words apart, in seq order", () => {
    const got = transcriptLines([m(2, "ai", "你为什么这么说？"), m(1, "student", "我觉得人均排放更重要。")]);
    expect(got.map((l) => [l.kind, "who" in l ? l.who : null])).toEqual([
      ["said", "student"],
      ["said", "coach"],
    ]);
  });

  it("never truncates her words", () => {
    // 🚨 2026-09-12：她自己写的字被切到 400，她跟印记说了三次「我的字被截断了」
    // 然后重打了整段。3364 是那次真实的长度。
    const long = "我".repeat(3364);
    const got = transcriptLines([m(1, "student", long)]);
    expect(got[0]).toMatchObject({ kind: "said", text: long });
  });

  it("renders a card message as one static line and absorbs her answer", () => {
    const got = transcriptLines([
      m(1, "ai", "给你一张卡片", { card: { type: "choose_span", prompt: "挑一句", options: [{ quote: "第三句" }] } }),
      m(2, "student", "第三句"),
      m(3, "ai", "为什么挑它？"),
    ]);
    expect(got[0]).toEqual({ kind: "card", seq: 1, label: "挑句子", answer: "第三句" });
    expect(got).toHaveLength(2);
  });

  it("drops system messages — they are bookkeeping, not anything anyone said", () => {
    expect(transcriptLines([m(1, "system", "工具已了结")])).toEqual([]);
  });

  it("leaves the answer empty when she never answered the card", () => {
    const got = transcriptLines([
      m(1, "ai", "给你一张卡片", { card: { type: "word_bank", prompt: "生词", words: [{ term: "capacity" }] } }),
    ]);
    expect(got[0]).toEqual({ kind: "card", seq: 1, label: "生词板", answer: "" });
  });

  it("keeps a message whose payload is not a card she could use", () => {
    // 半张卡片（choose_span 没有选项）被 coachCardOf 判成不是卡片。那条消息
    // 本身仍然是印记说过的话，回看里不能凭空少一条。
    const got = transcriptLines([m(1, "ai", "我们换个方式看这一段。", { card: { type: "choose_span", prompt: "挑一句" } })]);
    expect(got).toEqual([{ kind: "said", who: "coach", text: "我们换个方式看这一段。", seq: 1 }]);
  });
});
