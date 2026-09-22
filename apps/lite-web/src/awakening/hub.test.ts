import { describe, expect, it } from "vitest";

import { HUB } from "./content";
import { hubCards } from "./hub";

const base = {
  turnsDone: 0,
  threadCount: 0,
  resume: "terminal" as const,
  navigator: "",
  hasEnergy: false,
};

describe("hubCards", () => {
  // 一个空的库是一扇通向空屋子的门。
  it("还没有线索时不摆线索库", () => {
    const cards = hubCards(base);
    expect(cards.map((c) => c.key)).toEqual(["continue", "energy", "navigator", "story"]);
    expect(cards[0]!.title).toBe(HUB.restart);
  });

  it("有线索时摆出线索库，并说清里面有几条", () => {
    const cards = hubCards({ ...base, turnsDone: 3, threadCount: 2 });
    expect(cards.map((c) => c.key)).toEqual([
      "continue",
      "library",
      "energy",
      "navigator",
      "story",
    ]);
    expect(cards[0]!.title).toBe(HUB.held);
    expect(cards[0]!.body).toContain("3");
    expect(cards[1]!.body).toContain("2");
  });

  // 🚨 这一屏上不该再有任何一个「清空」。2026-09-21 之前「新的探索」按下去
  // 删掉的是她自己写的回答，而线索库存在的理由正是那件事不该发生。
  it("入口上没有任何一个会删掉她回答的动作", () => {
    const cards = hubCards({ ...base, turnsDone: 3, threadCount: 2 });
    for (const c of cards) {
      expect(`${c.title}${c.body}`).not.toContain("清空");
    }
  });

  it("她停在探询之前那几屏时，第一张卡说的是那一屏，不是探索", () => {
    const cards = hubCards({ ...base, turnsDone: 0, resume: "energy" });
    expect(cards[0]!.title).toBe(HUB.continueRun);
    expect(cards[0]!.body).toContain("能量");
    expect(cards.some((c) => c.key === "library")).toBe(false);
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
