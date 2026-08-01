import { describe, it, expect } from "vitest";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { compileCardEnvelope } from "@/studio/compileCard";

// The artifact a completed writing card leaves in 片段 (#7). A plain projection
// of the STUDENT's answers (铁律① — never authorship), so we assert it carries
// her words, titles the block with the card, and drops empty fields.
describe("compileCardEnvelope", () => {
  const toulmin = CARD_REGISTRY["toulmin"]!;

  it("titles the block with the card and includes only filled fields", () => {
    const out = compileCardEnvelope(toulmin, {
      claim: "中国的可持续贡献是实质性的。",
      warrant: "",
      evidence: "NASA 卫星数据显示植被覆盖增加。",
    });
    expect(out).toContain(`【${toulmin.name}】`);
    expect(out).toContain("中国的可持续贡献是实质性的。");
    expect(out).toContain("NASA 卫星数据显示植被覆盖增加。");
    // the empty warrant field contributes nothing — its label never appears
    const warrantLabel = toulmin.steps.find((s) => s.key === "warrant")!.fields[0]!.label;
    expect(out).not.toContain(warrantLabel);
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
