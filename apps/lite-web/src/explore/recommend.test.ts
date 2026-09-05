import interestsJSON from "@mind-imprint/contracts/interests";
import { describe, expect, it } from "vitest";
import { type MyInterest, reasonSentence, recommend } from "./recommend";

/**
 * recommend.test.ts —— 推荐是一个纯函数，所以它该被真正测出来，而不是靠看一屏
 * 星星「觉得还行」。
 *
 * 值得测的四件事，都是读代码看不出对错的：
 *  1. 排序真的把大学科折价了（不加权时第一名是被经济学串上来的巧合）；
 *  2. 她已经有的词、按过「不感兴趣」的词，一条都不能回来；
 *  3. 每根主枝的配额真的在挡；
 *  4. 同样的输入给同样的输出（否则每次刷新换一批，学生只会读成随机）。
 */

const TABLE = interestsJSON as { id: string; zh: string; disciplines: string[] }[];
const byZh = new Map(TABLE.map((it) => [it.zh, it]));

function her(...zh: string[]): MyInterest[] {
  return zh.map((z) => {
    const it = byZh.get(z);
    if (!it) throw new Error(`词表里没有「${z}」，测试的前提变了`);
    return { interestId: it.id, zh: it.zh, disciplineIds: it.disciplines };
  });
}

describe("推荐", () => {
  it("一个词都没有时不推荐任何东西", () => {
    expect(recommend([])).toEqual([]);
    // 有词但都还没有 interest_id（0134 之前的旧行）也一样：没有依据。
    expect(recommend([{ interestId: "", zh: "旧词", disciplineIds: [] }])).toEqual([]);
  });

  // 🚨 这一条是这个文件存在的主要理由。经济学挂着 42 条领域、视觉设计 41 条，
  // 几乎任何两个词都在这两门上「共用」。不折价的话前几名全是被大学科串起来的
  // 巧合 —— 实测「电池 + 太阳能」不加权的第一名是「咖啡」。
  it("能源那一路推出来的是能源那一路的东西", () => {
    const out = recommend(her("电池", "太阳能"), { limit: 6 });
    const zhs = out.map((r) => r.zh);
    expect(zhs).toContain("电动车");
    // 只靠经济学 / 视觉设计这类大学科连上来的词不该进前六。
    for (const r of out) {
      const rare = r.via.some((d) => d === "energy-systems" || d === "materials");
      expect(rare, `${r.zh} 只是被大学科串上来的`).toBe(true);
    }
  });

  it("媒介那一路推出来的是媒介那一路的东西", () => {
    const zhs = recommend(her("假消息", "社交媒体"), { limit: 8 }).map((r) => r.zh);
    expect(zhs.some((z) => ["舆论", "民调", "广告", "隐私"].includes(z))).toBe(true);
  });

  it("她已经有的词不会被推回来", () => {
    const mine = her("电池", "太阳能", "专注");
    const ids = recommend(mine, { limit: 40 }).map((r) => r.id);
    for (const m of mine) expect(ids).not.toContain(m.interestId);
  });

  it("按过不感兴趣的词不会再出现", () => {
    const mine = her("电池", "太阳能");
    const first = recommend(mine, { limit: 6 });
    const dropped = first[0]!.id;
    const again = recommend(mine, { limit: 6, dismissed: [dropped] });
    expect(again.map((r) => r.id)).not.toContain(dropped);
    // 挡掉一条之后还是给满六条，不是给五条。
    expect(again).toHaveLength(6);
  });

  it("每根主枝有配额，不会全挤在一根上", () => {
    const out = recommend(her("电池", "太阳能", "假消息", "社交媒体", "专注"), {
      limit: 14,
      perField: 2,
    });
    const perField = new Map<string, number>();
    for (const r of out) perField.set(r.field, (perField.get(r.field) ?? 0) + 1);
    for (const [f, n] of perField) expect(n, `${f} 超配额了`).toBeLessThanOrEqual(2);
    expect(perField.size, "全挤在一两根枝上").toBeGreaterThanOrEqual(3);
  });

  it("同样的输入给同样的输出", () => {
    const mine = her("电池", "太阳能", "专注");
    const a = recommend(mine, { limit: 10 }).map((r) => r.id);
    const b = recommend([...mine].reverse(), { limit: 10 }).map((r) => r.id);
    expect(b).toEqual(a);
  });

  // 理由必须提到她**自己的**词，不然那句话就是一句通用的场面话。
  it("每一条都说得出它是从她哪个词来的", () => {
    const mine = her("电池", "太阳能");
    for (const r of recommend(mine, { limit: 8 })) {
      expect(r.because.length).toBeGreaterThan(0);
      for (const w of r.because) expect(["电池", "太阳能"]).toContain(w);
      expect(r.viaZh.length).toBe(r.via.length);
      expect(reasonSentence(r)).toContain(r.zh);
    }
  });
});
