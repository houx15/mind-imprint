import { describe, expect, it } from "vitest";
import type { Anchor } from "@mind-imprint/contracts";
import { toReadingOutcomes, type LiteCard } from "@lite/api/readingRoom";

/**
 * readingOutcomes.test.ts — 铁律①: the AI never writes the student's prose,
 * and that includes never being QUOTED BACK to her as if she had written it.
 *
 * `toReadingOutcomes` rebuilds 阅读成果 from the persisted card rows after a
 * reload. A card carries up to two anchors: the AI's example sentence (author
 * "ai", written at summon time to show her what the lens is for) and the
 * sentence she picked herself (author "student", written at submit). It used
 * to fall back from the second to the first — so a card with no student anchor
 * rendered the AI's own example as HER finding, and it flowed onward into the
 * takeaway draft's key quotes. Mis-attribution is worse than omission.
 */

const evalFixture = {
  verdict: "strong" as const,
  verdictLabel: "高度匹配",
  verdictReason: "你抓到了全篇的支点。",
  checks: [
    { key: "target", label: "找对对象", status: "pass" as const, evidence: "碳排放总量", explanation: "找对了。" },
  ],
  finding: "这句给出了与全文乐观基调相反的事实。",
  judgment: "文章回避了排放总量。",
  support: "碳排放总量仍居全球第一。",
  caveat: "总量不等于人均。",
  nextStep: "去查人均排放。",
  spanIds: ["sel0"],
};

function anchor(author: "ai" | "student", quote: string): Anchor {
  return {
    id: author === "ai" ? "ex0" : "sel0",
    material_id: "m1",
    block_id: "b2",
    start: 0,
    end: quote.length,
    quote,
    dimension: "currency",
    author,
    question: "",
    answer: "",
  };
}

function card(anchors: Anchor[]): LiteCard {
  return {
    id: "card-1",
    cardId: "craap",
    blockId: "b2",
    status: "submitted",
    origin: "student",
    anchors,
    fieldValues: {},
    eventTrace: [],
    framework: evalFixture,
    createdAt: "2026-08-26T00:00:00Z",
    submittedAt: "2026-08-26T00:05:00Z",
  };
}

const aiExample = "中国的太阳能装机量在过去十年增长了十倍。";
const herPick = "但同一时期，中国的碳排放总量仍居全球第一。";

describe("toReadingOutcomes", () => {
  it("uses the sentence SHE picked, even when the AI's example is stored first", () => {
    const out = toReadingOutcomes([card([anchor("ai", aiExample), anchor("student", herPick)])]);
    expect(out).toHaveLength(1);
    expect(out[0]?.quote).toBe(herPick);
  });

  it("skips a card with no student anchor rather than showing the AI's example as her finding", () => {
    expect(toReadingOutcomes([card([anchor("ai", aiExample)])])).toEqual([]);
  });

  it("skips a card with no anchors at all", () => {
    expect(toReadingOutcomes([card([])])).toEqual([]);
  });
});
