import interestsJSON from "@mind-imprint/contracts/interests";
import disciplinesJSON from "@mind-imprint/contracts/disciplines";
import type { FieldId } from "../tree/types";

/**
 * recommend —— 「你可能还会感兴趣的」是怎么算出来的。
 *
 * # 为什么这件事不需要模型
 *
 * 领域词表（249 条）里每一条都写好了它扎在哪几门学科上。她树上的每个词也扎在
 * 那几门学科上。**两个领域共用一门学科，就是一条真实的、写在表里的关系** ——
 * 「电池」和「电动车」都扎在能源系统上，这件事不需要问模型，查表就有。
 *
 * 因此推荐是一个纯函数：输入她的词，输出表里其他词的排名。没有调用、没有延迟、
 * 没有钱，而且**每一条都能说清为什么**（`via` / `because` 两个字段就是那句话的
 * 原料）。一次模型调用给出的推荐说不清自己的理由，只能补一句编出来的解释。
 *
 * # 罕见的学科算得重
 *
 * 直接数共用学科的个数不行：经济学挂着 42 条领域、视觉设计 41 条，几乎任何两个
 * 词都在这两门上「共用」。那样排出来的前几名全是被大学科串起来的巧合。
 *
 * 所以每门学科按它挂了多少条领域折价：`1 / log(1 + 条数)`。能源系统挂 12 条，
 * 共用它是一件有信息量的事；经济学挂 42 条，共用它几乎什么都不说明。实测的
 * 差别很大 —— 未加权时「电池 + 太阳能」的邻居第一名是被经济学串上来的
 * 「咖啡」，加权之后是「电动车」。
 *
 * # 七根枝都要有
 *
 * 不设上限的话前十名会挤在一两根枝上（她的词扎在哪，邻居就都在哪）。而这一屏
 * 的用处恰恰是**指出她还没走过的方向**，所以每根枝最多 `perField` 条。
 */

type RawInterest = {
  id: string;
  field: string;
  zh: string;
  en: string;
  disciplines: string[];
};

const TABLE = interestsJSON as RawInterest[];

const DISCIPLINE_ZH = new Map(
  (disciplinesJSON as { id: string; zh: string }[]).map((d) => [d.id, d.zh]),
);

/** 每门学科挂了多少条领域。表是固定的，所以这个只算一次。 */
const FANOUT = (() => {
  const n = new Map<string, number>();
  for (const it of TABLE) {
    for (const d of it.disciplines) n.set(d, (n.get(d) ?? 0) + 1);
  }
  return n;
})();

/** 一门学科被共用时值多少分。挂得越少的越值钱。 */
function weightOf(disciplineId: string): number {
  return 1 / Math.log(1 + (FANOUT.get(disciplineId) ?? 1));
}

/** 她树上一个词，推荐这一侧只需要这两样。 */
export interface MyInterest {
  /** interests.json 的 id。空字符串表示这一行还没有 interest_id，跳过。 */
  interestId: string;
  zh: string;
  disciplineIds: string[];
}

export interface Recommendation {
  id: string;
  zh: string;
  en: string;
  field: FieldId;
  /** 共用的学科 id，最值钱的在前。 */
  via: string[];
  /** 共用学科的中文名，给面板直接用。 */
  viaZh: string[];
  /** 她自己哪几个词把这几门学科种下的。面板上那句「你的『电池』『太阳能』」。 */
  because: string[];
  /** 排序用。没有单位，只在同一次调用内可比。 */
  score: number;
}

export interface RecommendOptions {
  /** 她按过「不感兴趣」的 interest id。 */
  dismissed?: readonly string[];
  /** 总共给几条。 */
  limit?: number;
  /** 每根主枝最多几条。 */
  perField?: number;
}

/**
 * 按她树上的词，从闭表里排出她还没碰过的领域。
 *
 * 她一个词都没有时返回空数组 —— 这一屏该显示的是兴趣测试的入口，不是一批
 * 凭空挑的词。**没有依据的推荐不是推荐**，那是把闭表的前十条摆出来假装认识她。
 */
export function recommend(
  mine: readonly MyInterest[],
  opts: RecommendOptions = {},
): Recommendation[] {
  const { dismissed = [], limit = 12, perField = 3 } = opts;

  const have = new Set(mine.map((m) => m.interestId).filter(Boolean));
  const skip = new Set([...have, ...dismissed]);

  // 每门学科上，她有哪几个词。既是权重（有几个词），也是理由（是哪几个词）。
  const seededBy = new Map<string, string[]>();
  for (const m of mine) {
    for (const d of m.disciplineIds) {
      const list = seededBy.get(d);
      if (list) list.push(m.zh);
      else seededBy.set(d, [m.zh]);
    }
  }
  if (seededBy.size === 0) return [];

  const scored: Recommendation[] = [];
  for (const it of TABLE) {
    if (skip.has(it.id)) continue;
    const hits = it.disciplines.filter((d) => seededBy.has(d));
    if (hits.length === 0) continue;

    hits.sort((a, b) => weightOf(b) - weightOf(a));
    const score = hits.reduce((sum, d) => sum + seededBy.get(d)!.length * weightOf(d), 0);

    // 理由只列**最值钱的那两门**学科种下它的词：三门学科全列会拉出六七个词，
    // 那句话就没人读了。
    const because: string[] = [];
    for (const d of hits.slice(0, 2)) {
      for (const zh of seededBy.get(d)!) if (!because.includes(zh)) because.push(zh);
    }

    scored.push({
      id: it.id,
      zh: it.zh,
      en: it.en,
      field: it.field as FieldId,
      via: hits,
      viaZh: hits.map((d) => DISCIPLINE_ZH.get(d) ?? d),
      because,
      score,
    });
  }

  // 分数相同的按 id 排，这样同样的输入永远给同样的输出 —— 一屏推荐每次刷新都
  // 换一批，读起来就是随机的。
  scored.sort((a, b) => b.score - a.score || (a.id < b.id ? -1 : 1));

  const perFieldCount = new Map<string, number>();
  const out: Recommendation[] = [];
  for (const r of scored) {
    if (out.length >= limit) break;
    const n = perFieldCount.get(r.field) ?? 0;
    if (n >= perField) continue;
    perFieldCount.set(r.field, n + 1);
    out.push(r);
  }
  return out;
}

/** 面板上那句「为什么给你这个」。 */
export function reasonSentence(r: Recommendation): string {
  const words = r.because.map((w) => `「${w}」`).join("");
  const discs = r.viaZh.slice(0, 2).join(" · ");
  return `你的${words}扎在 ${discs} 上，${r.zh} 也扎在那里。`;
}
