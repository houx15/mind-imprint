import { FIELDS } from "../tree/geometry";
import type { FieldId } from "../tree/types";

/**
 * skyLayout —— 这片天空上谁站在哪。
 *
 * # 两层，都是她的
 *
 * 近处是**今天的五颗新闻星**，大、亮、手摆的位置（`PLANET_SLOTS`）—— 五个物体
 * 在一块画布上是一次构图，构图赢过分布，所以不算。
 *
 * 远处是**她树上已经有的词**，小、暗，排在一圈椭圆环上。五颗新闻星向它们连线：
 * 一条线的意思是「今天这条新闻和你已经在意的这个词扎在同一门学问上」。
 *
 * # 这一层原来是推荐，2026-09-07 换掉了
 *
 * 上一版这圈星是「你可能还会感兴趣的」—— 从闭表算出来的推荐词。产品负责人打开
 * 它，读不出那是什么：
 *
 *   > I have a point under the five big balls, one is 手机 … is it a recommended
 *   > keyword? then we don't need to recommend keyword here. we just show how the
 *   > five dots connected with students' already existed nodes.
 *
 * 她是对的，而且理由不止「读不懂」。一颗推荐星是我们对她的一个猜测，它长得和
 * 她自己的词一模一样，摆在同一圈上 —— 这张图于是同时在说两件事（这是你的 /
 * 这是我们猜的），却没有任何东西把两者分开。现在这一圈只有一种东西：**她的**。
 * 点开一颗，是这个词的来历，不是一句「猜你喜欢」。
 *
 * # 环上的位置是挑出来的，不是均分的
 *
 * 环上先取 28 个候选点，再**把撞上新闻星的候选点删掉**，然后在剩下的里等距取。
 * 直接按词的条数均分整个椭圆，必然会有一颗压在某个 200px 的星球上 —— 那颗星的
 * 标签就读不出来了。
 *
 * 坐标系是一个 1180×640 的参考画框（`STAGE`），和 CSS 里那块场地的
 * `max-width` / `min-height` 一致；渲染时换算成百分比，所以窗口怎么缩都对。
 */

/** 参考画框。所有坐标都在这个尺寸里算，渲染时换成百分比。 */
export const STAGE = { w: 1180, h: 640 };

/** 五颗新闻星，按 rank（1 = 今天最重要的那条）。手摆的构图。 */
export const PLANET_SLOTS: Record<number, { xPct: number; yPct: number; size: number; drift: string }> = {
  1: { xPct: 27, yPct: 41, size: 236, drift: "exp-drift" },
  2: { xPct: 61, yPct: 24, size: 200, drift: "exp-drift-1" },
  3: { xPct: 79, yPct: 63, size: 178, drift: "exp-drift-2" },
  4: { xPct: 11, yPct: 80, size: 158, drift: "exp-drift-3" },
  5: { xPct: 46, yPct: 77, size: 150, drift: "exp-drift-4" },
};

/** 一颗新闻星在参考画框里的圆心与半径。 */
function planetDiscs(): { x: number; y: number; r: number }[] {
  return Object.values(PLANET_SLOTS).map((s) => ({
    x: (s.xPct / 100) * STAGE.w,
    y: (s.yPct / 100) * STAGE.h,
    // 星球会漂 ±40px（exp-float-*），所以按半径加一圈漂移余量算占位。
    r: s.size / 2 + 40,
  }));
}

/** 一颗词星要留多大地方：圆点加它下面那行标签。 */
const STAR_HALF_W = 42;
const STAR_HALF_H = 22;

/** 词星离新闻星至少这么远（从星球边缘算起）。 */
const CLEAR_OF_PLANET = 34;

/** 环上最多摆几颗。再多就不是一圈星，是一条项链。 */
export const MAX_WORDS = 12;

/** 她树上的一个词，摆上天之前的样子。 */
export interface SkyWord {
  /** 关键词行的 id（uuid）—— 点开抽屉时靠它回查那个词。 */
  id: string;
  zh: string;
  field: FieldId;
  /** interests.json 的 id。和新闻星的 interestId 对上就是「同一个词」。 */
  interestId: string;
  /** 这个词扎在哪几门学科上。连线靠它。 */
  disciplineIds: string[];
  /** 1..5。挑哪些词上天时用。 */
  strength: number;
}

export interface PlacedWord extends SkyWord {
  /** 参考画框里的坐标。 */
  x: number;
  y: number;
  /** 百分比，直接给 CSS。 */
  xPct: number;
  yPct: number;
  /** 五档漂移里的一档，让这一层也在动。 */
  drift: string;
}

/** 今天的一颗新闻星，只留连线要用的那几个字段。 */
export interface SkyPlanet {
  id: string;
  rank: number;
  /** 这条新闻挂在哪门学科上（可能为空）。 */
  disciplineId: string;
  /** 这条新闻落在哪个领域上（可能为空）。 */
  interestId: string;
  /** 七根主枝之一 —— 连线最弱的那一档靠它。 */
  field: FieldId | "";
}

const DRIFTS = ["exp-drift", "exp-drift-1", "exp-drift-2", "exp-drift-3", "exp-drift-4"];

/**
 * 环上的候选点。从正上方开始顺时针，28 个。
 */
function ringCandidates(): { x: number; y: number }[] {
  const cx = STAGE.w / 2;
  const cy = STAGE.h / 2;
  // rx 留够标签的余量：最外一档 500 + 34 = 534，加半个标签 42 是 576，画框半宽
  // 590 —— 两边各剩 14px。改这两个数之前先看 `skyLayout.test.ts` 那条「标签不出
  // 画框」。
  const rx = 500;
  const ry = 268;
  // 一圈完美的椭圆读起来是一张图表。按下标循环给一点深浅，星就散在一条带子里
  // 而不是钉在一条线上。**是循环不是随机**：每次刷新换一批位置的星空，学生只
  // 会读成随机。
  const DEPTH = [0, -34, 16, -14, 34, -24, 8];
  const out: { x: number; y: number }[] = [];
  for (let i = 0; i < 28; i += 1) {
    const a = -Math.PI / 2 + (i / 28) * Math.PI * 2;
    const d = DEPTH[i % DEPTH.length]!;
    out.push({ x: cx + Math.cos(a) * (rx + d), y: cy + Math.sin(a) * (ry + d) });
  }
  return out;
}

/** 这个候选点会不会压在某颗新闻星上。 */
function collidesWithPlanet(p: { x: number; y: number }): boolean {
  return planetDiscs().some((d) => Math.hypot(p.x - d.x, p.y - d.y) < d.r + CLEAR_OF_PLANET);
}

/**
 * 今天这一屏摆哪几个词。
 *
 * 环上装得下十二颗，而她的树上可能有四十个词。挑的顺序是：
 *
 *  1. **今天有星球连着的先上。** 这一屏的主语是今天那五条新闻，一个和今天毫无
 *     关系的词摆上去只是占位。
 *  2. 然后按强度。强度是来源条数推出来的 —— 她真的花过时间的那些词。
 *  3. 再按 id，把并列的顺序钉死（同一份数据每次摆在同一处）。
 */
export function pickWords(
  words: readonly SkyWord[],
  planets: readonly SkyPlanet[],
  limit: number = MAX_WORDS,
): SkyWord[] {
  const linked = new Set<string>();
  for (const p of planets) {
    for (const w of words) {
      if (relatedness(p, w) > 0) linked.add(w.id);
    }
  }
  return [...words]
    .sort((a, b) => {
      const la = linked.has(a.id) ? 1 : 0;
      const lb = linked.has(b.id) ? 1 : 0;
      if (la !== lb) return lb - la;
      if (a.strength !== b.strength) return b.strength - a.strength;
      return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
    })
    .slice(0, limit);
}

/**
 * 把词排到环上。
 *
 * 返回的条数可能少于传进来的：环上放不下就少放几颗。**少放好过压上** ——
 * 一颗压在星球上的词等于没有这颗词，而且它还毁掉了那颗星球。
 */
export function placeWords(input: readonly SkyWord[]): PlacedWord[] {
  if (input.length === 0) return [];

  const words = byBranch(input);
  const free = ringCandidates().filter((p) => !collidesWithPlanet(p));
  if (free.length === 0) return [];

  // 先按等距挑一个理想位置（取前 n 个会把它们全挤在环的一段上），**再从那里
  // 往后找第一个不和已放的星叠在一起的空位**。
  //
  // 只按等距挑是不够的：椭圆被星球咬掉几段之后，剩下的空位不是均匀的，等距下标
  // 会挑到两个挨着的点。实测 8 条时前两颗就叠上了 —— 这是 `skyLayout.test.ts`
  // 抓出来的，一张截图上它俩只是「有点近」。
  const step = free.length / Math.min(words.length, free.length);
  const used = new Set<number>();
  const out: PlacedWord[] = [];
  for (let i = 0; i < words.length; i += 1) {
    const wanted = Math.floor(i * step);
    let slot: { x: number; y: number } | null = null;
    for (let k = 0; k < free.length; k += 1) {
      const idx = (wanted + k) % free.length;
      if (used.has(idx)) continue;
      const cand = free[idx]!;
      const placed: PlacedWord = { ...words[i]!, ...cand, xPct: 0, yPct: 0, drift: "" };
      if (out.some((s) => starsOverlap(s, placed))) continue;
      used.add(idx);
      slot = cand;
      break;
    }
    // 环上再也找不到放得下的位置。少放好过压上。
    if (!slot) break;
    out.push({
      ...words[i]!,
      x: slot.x,
      y: slot.y,
      xPct: (slot.x / STAGE.w) * 100,
      yPct: (slot.y / STAGE.h) * 100,
      drift: DRIFTS[i % DRIFTS.length]!,
    });
  }
  return out;
}

/**
 * 环上的顺序：**同一根主枝的词排在一起**。
 *
 * 词星的颜色就是它在树上那根枝的颜色，所以同色相邻时这一圈读起来是七段颜色，
 * 和树上七根枝是同一件事。打散了摆则是一圈彩色的点 —— 好看，但什么也没说。
 *
 * 主枝内部按强度，再按 id：同一份数据每次摆在同一处。
 */
function byBranch(words: readonly SkyWord[]): SkyWord[] {
  const order = new Map(FIELDS.map((f, i) => [f.id, i]));
  return [...words].sort((a, b) => {
    const fa = order.get(a.field) ?? 99;
    const fb = order.get(b.field) ?? 99;
    if (fa !== fb) return fa - fb;
    if (a.strength !== b.strength) return b.strength - a.strength;
    return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
  });
}

/**
 * 一颗新闻星和一个词有多近。0 = 没关系，不连。
 *
 * 三档，全部是查表得到的事实，不是模型判断的：
 *
 *  3. **同一个领域**（interestId 相同）：今天这条新闻讲的就是这个词。
 *  2. **同一门学科**：它们扎在同一门学问上 —— 和根系上那条线是同一个关系，
 *     所以这两屏说的是同一件事。
 *  1. **同一根主枝**：最弱的一档，但仍然是真的 —— 而且它就是这两屏共用的那套
 *     颜色，一条连到同色词的线不需要解释。
 *
 * 🚨 第三档是实测加的（2026-09-07）。只认「同一门学科」时，实际打开地图**一条
 * 线都没有**：42 门学科铺得很开，今天五条新闻挂的是 logic-proof / anthropology
 * / genetics / public-health，而她树上那六个词扎在 energy-systems /
 * media-studies / statistics 上，一处都不重合。一张承诺「连线是它们和今天的
 * 关系」却一条线都没有的图，比没有这句话更糟。
 */
function relatedness(p: SkyPlanet, w: SkyWord): number {
  if (p.interestId && p.interestId === w.interestId) return 3;
  if (p.disciplineId && w.disciplineIds.includes(p.disciplineId)) return 2;
  if (p.field && p.field === w.field) return 1;
  return 0;
}

/** 一颗星球最多连几条。 */
const MAX_THREADS_PER_PLANET = 3;

export interface Thread {
  planetId: string;
  /** 起点：这颗星球在参考画框里的圆心。 */
  from: { x: number; y: number };
  to: PlacedWord;
  /** 3 = 同一个领域，2 = 同一门学科，1 = 同一根主枝。线的粗细按它分。 */
  strength: number;
}

/**
 * 五颗新闻星连到她的词上。
 *
 * **每颗星最多三条。** 一门大学科（经济学挂着 42 条领域）会让一颗星球连上她
 * 半棵树，那不是一张星图，是一团毛线。三条之内还读得出「今天这条和我在意的
 * 那几个词有关」，那正是这一屏要说的全部。
 *
 * 排序：关系强的在前，同强度取**线短的** —— 一条横穿整张图的线即使是真的，
 * 读起来也是噪音。并列再按 id 定序，同一份数据每次连出同一张图。
 */
export function planetThreads(
  planets: readonly SkyPlanet[],
  words: readonly PlacedWord[],
): Thread[] {
  const out: Thread[] = [];
  for (const p of planets) {
    const slot = PLANET_SLOTS[p.rank] ?? PLANET_SLOTS[5]!;
    const from = { x: (slot.xPct / 100) * STAGE.w, y: (slot.yPct / 100) * STAGE.h };
    const scored = words
      .map((w) => ({ w, s: relatedness(p, w), d: Math.hypot(from.x - w.x, from.y - w.y) }))
      .filter((c) => c.s > 0)
      .sort((a, b) => (a.s !== b.s ? b.s - a.s : a.d !== b.d ? a.d - b.d : a.w.id < b.w.id ? -1 : 1))
      .slice(0, MAX_THREADS_PER_PLANET);
    for (const c of scored) {
      out.push({ planetId: p.id, from, to: c.w, strength: c.s });
    }
  }
  return out;
}

/** 两颗词星的标签会不会叠在一起。测试用，也给以后调环参数时用。 */
export function starsOverlap(a: PlacedWord, b: PlacedWord): boolean {
  return Math.abs(a.x - b.x) < STAR_HALF_W * 2 && Math.abs(a.y - b.y) < STAR_HALF_H * 2;
}
