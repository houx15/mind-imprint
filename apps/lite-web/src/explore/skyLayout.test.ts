import { describe, expect, it } from "vitest";
import {
  MAX_WORDS,
  PLANET_SLOTS,
  STAGE,
  type SkyPlanet,
  type SkyWord,
  pickWords,
  placeWords,
  planetThreads,
  starsOverlap,
} from "./skyLayout";

/**
 * skyLayout.test.ts —— 这一屏上「人眼盯不住」的那几条。
 *
 * 和 `tree/roots.test.ts` 同一个理由：一张 1500px 的截图上，一颗压在星球边缘的
 * 词星看着像是「挨着」，而学生看到的是一行读不出来的字。她树上有几个词每天都
 * 不一样（一个到四十个），所以每一种条数都要对，截图只能看到今天这一种。
 */

const FIELDS = ["formal", "science", "making", "society", "humanities", "arts", "self"] as const;

function fakeWords(n: number): SkyWord[] {
  return Array.from({ length: n }, (_, i) => ({
    id: `k${i}`,
    zh: `词${i}`,
    field: FIELDS[i % FIELDS.length]!,
    interestId: `i${i}`,
    // 每三个共用一门学科，好让连线有东西可连。
    disciplineIds: [`d${i % 3}`, `d${i}`],
    strength: (i % 5) + 1,
  }));
}

function fakePlanets(): SkyPlanet[] {
  return [1, 2, 3, 4, 5].map((rank) => ({
    id: `p${rank}`,
    rank,
    disciplineId: `d${rank % 3}`,
    interestId: "",
    // 主枝留空：这几条测的是学科那一档，同枝那一档会把几乎所有词都连上，
    // 盖掉要测的东西。同枝那一档自己有一条测试。
    field: "" as const,
  }));
}

describe("词星在天上的位置", () => {
  // 🚨 这一条是这个文件存在的主要理由。1 号星球直径 236px，一颗落在它上面的
  // 词星，标签会印在星球的玻璃面上——看着「有东西」，读不出是什么。
  it("任何条数下都不会压在新闻星上", () => {
    for (let n = 1; n <= 16; n += 1) {
      for (const s of placeWords(fakeWords(n))) {
        for (const slot of Object.values(PLANET_SLOTS)) {
          const px = (slot.xPct / 100) * STAGE.w;
          const py = (slot.yPct / 100) * STAGE.h;
          const d = Math.hypot(s.x - px, s.y - py);
          expect(d, `${n} 条时「${s.zh}」压在星球上了`).toBeGreaterThan(slot.size / 2 + 40);
        }
      }
    }
  });

  it("任何条数下词星之间都不叠", () => {
    for (let n = 1; n <= 16; n += 1) {
      const placed = placeWords(fakeWords(n));
      for (let i = 0; i < placed.length; i += 1) {
        for (let j = i + 1; j < placed.length; j += 1) {
          expect(
            starsOverlap(placed[i]!, placed[j]!),
            `${n} 条时「${placed[i]!.zh}」和「${placed[j]!.zh}」叠了`,
          ).toBe(false);
        }
      }
    }
  });

  it("标签不出画框", () => {
    for (const s of Array.from({ length: 16 }, (_, i) => placeWords(fakeWords(i + 1))).flat()) {
      expect(s.x, `${s.zh} 顶出左边`).toBeGreaterThan(42);
      expect(s.x, `${s.zh} 顶出右边`).toBeLessThan(STAGE.w - 42);
      expect(s.y, `${s.zh} 顶出上边`).toBeGreaterThan(22);
      expect(s.y, `${s.zh} 顶出下边`).toBeLessThan(STAGE.h - 22);
    }
  });

  it("一个词都没有时什么也不摆", () => {
    expect(placeWords([])).toEqual([]);
  });

  it("十二条摆得下", () => {
    expect(placeWords(fakeWords(MAX_WORDS))).toHaveLength(MAX_WORDS);
  });

  it("同样的输入摆在同样的位置", () => {
    const a = placeWords(fakeWords(9)).map((s) => [s.id, s.x, s.y]);
    const b = placeWords(fakeWords(9)).map((s) => [s.id, s.x, s.y]);
    expect(b).toEqual(a);
  });

  // 同一根主枝的词相邻 = 这一圈读起来是七段颜色，和树上七根枝对得上。打散了摆
  // 只是一圈彩色的点。判据：每根主枝的词在环上占的是**连续**的一段。
  it("同一根主枝的词排在一起", () => {
    const placed = placeWords(fakeWords(14));
    const seen = new Set<string>();
    let prev = "";
    for (const s of placed) {
      if (s.field === prev) continue;
      expect(seen.has(s.field), `「${s.field}」在环上被切成了两段`).toBe(false);
      seen.add(s.field);
      prev = s.field;
    }
  });
});

describe("挑哪些词上天", () => {
  it("今天有星球连着的排在前面", () => {
    // d0/d1/d2 是星球挂的学科；下标 ≥ 3 的词里只有 i%3 命中的才连得上。
    const words = fakeWords(20);
    const picked = pickWords(words, fakePlanets(), 6);
    const planets = fakePlanets();
    const linked = (w: SkyWord) =>
      planets.some((p) => w.disciplineIds.includes(p.disciplineId) || p.interestId === w.interestId);
    // 挑出来的里，连得上的必须排在连不上的前面。
    let sawUnlinked = false;
    for (const w of picked) {
      if (!linked(w)) sawUnlinked = true;
      else expect(sawUnlinked, `「${w.zh}」连得上，却排在一个连不上的后面`).toBe(false);
    }
  });

  it("最多给上限那么多", () => {
    expect(pickWords(fakeWords(40), fakePlanets(), MAX_WORDS)).toHaveLength(MAX_WORDS);
  });

  it("一个词都没有时挑不出东西", () => {
    expect(pickWords([], fakePlanets())).toEqual([]);
  });
});

describe("新闻星连到她的词上", () => {
  const placed = placeWords(pickWords(fakeWords(20), fakePlanets()));

  // 🚨 一门大学科（经济学挂着 42 条领域）会让一颗星球连上她半棵树。三条之内还
  // 读得出「今天这条和我在意的那几个词有关」；十二条是一团毛线，而在一张截图上
  // 毛线和星图的差别只是「有点密」。
  it("每颗星球最多三条", () => {
    const perPlanet = new Map<string, number>();
    for (const t of planetThreads(fakePlanets(), placed)) {
      perPlanet.set(t.planetId, (perPlanet.get(t.planetId) ?? 0) + 1);
    }
    for (const [id, n] of perPlanet) {
      expect(n, `星球 ${id} 连了 ${n} 条`).toBeLessThanOrEqual(3);
    }
  });

  it("只连真的有关系的", () => {
    const byId = new Map(fakePlanets().map((p) => [p.id, p]));
    for (const t of planetThreads(fakePlanets(), placed)) {
      const p = byId.get(t.planetId)!;
      const shares =
        t.to.disciplineIds.includes(p.disciplineId) ||
        (p.interestId !== "" && p.interestId === t.to.interestId);
      expect(shares, `星球 ${t.planetId} 连到了没关系的「${t.to.zh}」`).toBe(true);
      expect(t.strength).toBeGreaterThan(0);
    }
  });

  // 同一颗星球的三条里，取的是**短的**。一条真的关系拉成横穿整张图的线，读起来
  // 仍然是噪音 —— 上一版的星座连线就是这么废掉的。
  it("同强度时先连近的", () => {
    const planets = fakePlanets();
    const threads = planetThreads(planets, placed);
    for (const p of planets) {
      const mine = threads.filter((t) => t.planetId === p.id);
      if (mine.length === 0) continue;
      const takenMax = Math.max(...mine.map((t) => Math.hypot(t.from.x - t.to.x, t.from.y - t.to.y)));
      const dropped = placed
        .filter((w) => !mine.some((t) => t.to.id === w.id))
        .filter((w) => w.disciplineIds.includes(p.disciplineId))
        .map((w) => Math.hypot(mine[0]!.from.x - w.x, mine[0]!.from.y - w.y));
      for (const d of dropped) {
        expect(d, `星球 ${p.id} 放着近的不连，连了远的`).toBeGreaterThanOrEqual(takenMax - 0.001);
      }
    }
  });

  it("同一个领域的那条比同一门学科的粗", () => {
    const same: SkyPlanet[] = [
      { id: "px", rank: 1, disciplineId: "d0", interestId: "i3", field: "" },
    ];
    const threads = planetThreads(same, placed);
    const exact = threads.find((t) => t.to.interestId === "i3");
    expect(exact?.strength).toBe(3);
  });

  // 🚨 实测加的那一档：只认「同一门学科」时，真实数据下**一条线都没有**
  // （今天的新闻挂 logic-proof，她的词扎在 energy-systems 上，一处都不重合）。
  // 同一根主枝是最弱但仍然为真的关系，而它就是两屏共用的那套颜色。
  it("学科对不上时，同一根主枝也算一条关系", () => {
    const noShared: SkyPlanet[] = [
      { id: "pf", rank: 1, disciplineId: "完全对不上", interestId: "", field: "science" },
    ];
    const threads = planetThreads(noShared, placed);
    expect(threads.length).toBeGreaterThan(0);
    for (const t of threads) {
      expect(t.to.field).toBe("science");
      expect(t.strength).toBe(1);
    }
  });

  it("她一个词都没有时一条线都不画", () => {
    expect(planetThreads(fakePlanets(), [])).toEqual([]);
  });
});
