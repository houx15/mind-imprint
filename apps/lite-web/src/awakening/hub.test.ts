import { describe, expect, it } from "vitest";

import { HUB } from "./content";
import { hubCards } from "./hub";

const base = { turnsDone: 0, resume: "terminal" as const, navigator: "", hasEnergy: false };

describe("hubCards", () => {
  it("没有保留下来的回答时不摆「新的探索」", () => {
    const cards = hubCards(base);
    expect(cards.map((c) => c.key)).toEqual(["continue", "energy", "navigator", "story"]);
    // 这时第一张卡本身就是新的探索，所以它说的是「开始」。
    expect(cards[0]!.title).toBe(HUB.restart);
  });

  it("有保留下来的回答时，继续和新的探索各占一张", () => {
    const cards = hubCards({ ...base, turnsDone: 3 });
    expect(cards.map((c) => c.key)).toEqual([
      "continue",
      "fresh",
      "energy",
      "navigator",
      "story",
    ]);
    expect(cards[0]!.title).toBe(HUB.held);
    expect(cards[0]!.body).toContain("3");
    // 🚨 清空几段必须说出来 —— 删的是她自己写下的字。
    expect(cards[1]!.body).toContain("3");
  });

  it("她停在探询之前那几屏时，第一张卡说的是那一屏，不是探索", () => {
    const cards = hubCards({ ...base, turnsDone: 0, resume: "energy" });
    expect(cards[0]!.title).toBe(HUB.continueRun);
    expect(cards[0]!.body).toContain("能量");
    expect(cards.some((c) => c.key === "fresh")).toBe(false);
  });

  it("还没选过助手时那张卡说的是「选择」，不是「重新选择」", () => {
    expect(hubCards(base).find((c) => c.key === "navigator")?.title).toBe(HUB.navigatorFirst);
    expect(
      hubCards({ ...base, navigator: "NOVA" }).find((c) => c.key === "navigator")?.title,
    ).toBe(HUB.navigator);
  });

  it("做过一次能量测试之后那张卡照实说", () => {
    expect(hubCards({ ...base, hasEnergy: true }).find((c) => c.key === "energy")?.body).toContain(
      HUB.energyDone,
    );
  });
});
