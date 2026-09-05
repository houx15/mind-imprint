import { describe, expect, it } from "vitest";
import type { Recommendation } from "./recommend";
import {
  PLANET_SLOTS,
  STAGE,
  constellationEdges,
  placeRecommendations,
  starsOverlap,
} from "./skyLayout";

/**
 * skyLayout.test.ts —— 这一屏上「人眼盯不住」的那几条。
 *
 * 和 `tree/roots.test.ts` 同一个理由：一张 1500px 的截图上，一颗压在星球边缘的
 * 推荐星看着像是「挨着」，而学生看到的是一行读不出来的字。推荐的条数每天都不
 * 一样（三条到十二条），所以每一种条数都要对，截图只能看到今天这一种。
 */

const FIELDS = ["formal", "science", "making", "society", "humanities", "arts", "self"] as const;

function fakeRecs(n: number): Recommendation[] {
  return Array.from({ length: n }, (_, i) => ({
    id: `i${i}`,
    zh: `词${i}`,
    en: `Word ${i}`,
    field: FIELDS[i % FIELDS.length]!,
    // 每三个共用一门学科，好让星座连线有东西可连。
    via: [`d${i % 3}`, `d${i}`],
    viaZh: [`学科${i % 3}`, `学科${i}`],
    because: ["电池"],
    score: 10 - i,
  }));
}

describe("推荐星在天上的位置", () => {
  // 🚨 这一条是这个文件存在的主要理由。1 号星球直径 236px，一颗落在它上面的
  // 推荐星，标签会印在星球的玻璃面上——看着「有东西」，读不出是什么。
  it("任何条数下都不会压在新闻星上", () => {
    for (let n = 1; n <= 16; n += 1) {
      for (const s of placeRecommendations(fakeRecs(n))) {
        for (const slot of Object.values(PLANET_SLOTS)) {
          const px = (slot.xPct / 100) * STAGE.w;
          const py = (slot.yPct / 100) * STAGE.h;
          const d = Math.hypot(s.x - px, s.y - py);
          expect(d, `${n} 条时「${s.zh}」压在星球上了`).toBeGreaterThan(slot.size / 2 + 40);
        }
      }
    }
  });

  it("任何条数下推荐星之间都不叠", () => {
    for (let n = 1; n <= 16; n += 1) {
      const placed = placeRecommendations(fakeRecs(n));
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
    for (const s of Array.from({ length: 16 }, (_, i) => placeRecommendations(fakeRecs(i + 1))).flat()) {
      expect(s.x, `${s.zh} 顶出左边`).toBeGreaterThan(42);
      expect(s.x, `${s.zh} 顶出右边`).toBeLessThan(STAGE.w - 42);
      expect(s.y, `${s.zh} 顶出上边`).toBeGreaterThan(22);
      expect(s.y, `${s.zh} 顶出下边`).toBeLessThan(STAGE.h - 22);
    }
  });

  it("一条推荐都没有时什么也不摆", () => {
    expect(placeRecommendations([])).toEqual([]);
  });

  // 十二条要摆得下。摆不下不是错误（少放好过压上），但今天的默认上限是十二，
  // 如果环上装不下十二颗，那是环的参数该调，不是默认值该降。
  it("十二条摆得下", () => {
    expect(placeRecommendations(fakeRecs(12))).toHaveLength(12);
  });

  it("同样的输入摆在同样的位置", () => {
    const a = placeRecommendations(fakeRecs(9)).map((s) => [s.id, s.x, s.y]);
    const b = placeRecommendations(fakeRecs(9)).map((s) => [s.id, s.x, s.y]);
    expect(b).toEqual(a);
  });
});

describe("星座连线", () => {
  it("每颗星最多连一条，且不重复连同一对", () => {
    const stars = placeRecommendations(fakeRecs(9));
    const edges = constellationEdges(stars);
    expect(edges.length).toBeLessThanOrEqual(stars.length);
    const keys = edges.map((e) => [e.from.id, e.to.id].sort().join("|"));
    expect(new Set(keys).size).toBe(keys.length);
  });

  it("只连真的共用学科的两颗", () => {
    for (const e of constellationEdges(placeRecommendations(fakeRecs(9)))) {
      expect(e.from.via.some((d) => e.to.via.includes(d))).toBe(true);
      expect(e.shared).toBeGreaterThan(0);
    }
  });

  // 🚨 实测抓出来的：第一版按「关系最近的那一颗」连，关系最近的两颗常常落在环的
  // 两端，连线成了横穿整张图、盖在星球上的长弦。摆的顺序（chainByKinship）决定
  // 星座长什么样，而这件事在代码里看不出来，只有量线长才知道。
  it("是弧，不是横穿整张图的弦", () => {
    const limit = Math.hypot(STAGE.w, STAGE.h) / 2;
    for (const n of [4, 8, 12]) {
      for (const e of constellationEdges(placeRecommendations(fakeRecs(n)))) {
        const len = Math.hypot(e.from.x - e.to.x, e.from.y - e.to.y);
        expect(len, `${n} 条时「${e.from.zh}」到「${e.to.zh}」拉了一条横穿图的线`).toBeLessThan(
          limit,
        );
      }
    }
  });

  it("谁都不共用学科时一条线都不画", () => {
    const lonely: Recommendation[] = [0, 1, 2].map((i) => ({
      id: `x${i}`,
      zh: `孤${i}`,
      en: `Lone ${i}`,
      field: "science",
      via: [`only${i}`],
      viaZh: [`只有${i}`],
      because: ["电池"],
      score: 1,
    }));
    expect(constellationEdges(placeRecommendations(lonely))).toEqual([]);
  });
});
