import { describe, it, expect } from "vitest";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { compileCardEnvelope } from "@/studio/compileCard";

// The artifact a completed writing card leaves in 片段 (#7/item C). A plain
// projection of the STUDENT's answers (铁律① — never authorship): a flowing
// paragraph of her own words, NOT a "**label**\ntext" dump — the card's name
// is only a subtle provenance prefix, not part of the paragraph itself.
describe("compileCardEnvelope", () => {
  const toulmin = CARD_REGISTRY["toulmin"]!;

  it("prefixes the card's name and includes only filled fields, as flowing prose (not a labeled dump)", () => {
    const out = compileCardEnvelope(toulmin, {
      claim: "中国的可持续贡献是实质性的",
      warrant: "",
      evidence: "NASA 卫星数据显示植被覆盖增加",
    });
    expect(out).toContain(`【${toulmin.name}】`);
    expect(out).toContain("中国的可持续贡献是实质性的。");
    expect(out).toContain("NASA 卫星数据显示植被覆盖增加。");
    // no markdown label/value dump — it reads as one paragraph
    expect(out).not.toContain("**");
    expect(out).not.toMatch(/^-\s/m);
    // the empty warrant field contributes nothing — its label never appears
    const warrantLabel = toulmin.steps.find((s) => s.key === "warrant")!.fields[0]!.label;
    expect(out).not.toContain(warrantLabel);
  });

  it("joins multiple answers into one continuous paragraph, in field order, without labels", () => {
    const out = compileCardEnvelope(toulmin, {
      claim: "中国的可持续贡献是实质性的",
      evidence: "NASA 卫星数据显示植被覆盖增加",
    });
    const [header, prose] = out.split("\n");
    expect(header).toBe(`【${toulmin.name}】`);
    expect(prose).toBe("中国的可持续贡献是实质性的。NASA 卫星数据显示植被覆盖增加。");
  });

  it("does not double-punctuate an answer that already ends in terminal punctuation", () => {
    const out = compileCardEnvelope(toulmin, { claim: "这是我的主张。", evidence: "这是证据！" });
    expect(out).toContain("这是我的主张。这是证据！");
    expect(out).not.toContain("。。");
  });

  it("returns empty string when nothing (or only whitespace) was filled", () => {
    expect(compileCardEnvelope(toulmin, {})).toBe("");
    expect(compileCardEnvelope(toulmin, { claim: "", warrant: "   " })).toBe("");
  });

  it("joins array values and flattens one level of object values", () => {
    const out = compileCardEnvelope(toulmin, {
      claim: ["气候", "能源"],
      evidence: { text: "证据", source: "来源" },
    });
    expect(out).toContain("气候、能源");
    expect(out).toContain("证据 · 来源");
  });

  it("falls back to raw keys for values a custom renderer stored off-schema", () => {
    // a key the card doesn't declare must still surface, not be silently lost
    const out = compileCardEnvelope(toulmin, { some_custom_slot: "别丢了这条" });
    expect(out).toContain("别丢了这条");
  });
});
