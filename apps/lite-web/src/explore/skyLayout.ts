import type { Recommendation } from "./recommend";

/**
 * skyLayout —— 这片天空上谁站在哪。
 *
 * # 两层深度
 *
 * 近处是**今天的五颗新闻星**，大、亮、手摆的位置（`PLANET_SLOTS`）—— 五个物体
 * 在一块画布上是一次构图，构图赢过分布，所以不算。
 *
 * 远处是**她还没走过的领域**，小、暗、排在一圈椭圆环上。这一层不能手摆：它有
 * 几颗取决于她树上有什么，今天三颗明天九颗，位置必须算。
 *
 * 两层的大小与亮度差就是「近 / 远」这件事本身：今天的事在眼前，没去过的地方在
 * 更远处等着。
 *
 * # 环上的位置是挑出来的，不是均分的
 *
 * 环上先取 28 个候选点，再**把撞上新闻星的候选点删掉**，然后在剩下的里等距取。
 * 直接按推荐条数均分整个椭圆，必然会有一颗压在某个 200px 的星球上 —— 那颗星的
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

/** 一颗推荐星要留多大地方：圆点加它下面那行标签。 */
const STAR_HALF_W = 42;
const STAR_HALF_H = 22;

/** 推荐星离新闻星至少这么远（从星球边缘算起）。 */
const CLEAR_OF_PLANET = 34;

export interface PlacedStar extends Recommendation {
  /** 参考画框里的坐标。 */
  x: number;
  y: number;
  /** 百分比，直接给 CSS。 */
  xPct: number;
  yPct: number;
  /** 五档漂移里的一档，让这一层也在动。 */
  drift: string;
}

const DRIFTS = ["exp-drift", "exp-drift-1", "exp-drift-2", "exp-drift-3", "exp-drift-4"];

/**
 * 环上的候选点。从正上方开始顺时针，28 个。
 *
 * 椭圆的半径留出了标签的余量：`rx` 520 加半个标签 42 是 562，画框半宽 590，
 * 两边各剩 28px。
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
 * 把推荐排到环上。
 *
 * 返回的条数可能少于传进来的：环上放不下就少放几颗。**少放好过压上** ——
 * 一颗压在星球上的推荐等于没有这颗推荐，而且它还毁掉了那颗星球。
 */
export function placeRecommendations(input: readonly Recommendation[]): PlacedStar[] {
  if (input.length === 0) return [];

  const recs = chainByKinship(input);
  const free = ringCandidates().filter((p) => !collidesWithPlanet(p));
  if (free.length === 0) return [];

  // 先按等距挑一个理想位置（取前 n 个会把它们全挤在环的一段上），**再从那里
  // 往后找第一个不和已放的星叠在一起的空位**。
  //
  // 只按等距挑是不够的：椭圆被星球咬掉几段之后，剩下的空位不是均匀的，等距下标
  // 会挑到两个挨着的点。实测 8 条时前两颗就叠上了 —— 这是 `skyLayout.test.ts`
  // 抓出来的，一张截图上它俩只是「有点近」。
  const step = free.length / Math.min(recs.length, free.length);
  const used = new Set<number>();
  const out: PlacedStar[] = [];
  for (let i = 0; i < recs.length; i += 1) {
    const wanted = Math.floor(i * step);
    let slot: { x: number; y: number } | null = null;
    for (let k = 0; k < free.length; k += 1) {
      const idx = (wanted + k) % free.length;
      if (used.has(idx)) continue;
      const cand = free[idx]!;
      const placed: PlacedStar = { ...recs[i]!, ...cand, xPct: 0, yPct: 0, drift: "" };
      if (out.some((s) => starsOverlap(s, placed))) continue;
      used.add(idx);
      slot = cand;
      break;
    }
    // 环上再也找不到放得下的位置。少放好过压上。
    if (!slot) break;
    out.push({
      ...recs[i]!,
      x: slot.x,
      y: slot.y,
      xPct: (slot.x / STAGE.w) * 100,
      yPct: (slot.y / STAGE.h) * 100,
      drift: DRIFTS[i % DRIFTS.length]!,
    });
  }
  return out;
}

/** 两条推荐共用几门学科。 */
function kinship(a: Recommendation, b: Recommendation): number {
  return a.via.filter((d) => b.via.includes(d)).length;
}

/**
 * 把推荐排成一条链：关系近的排在一起。
 *
 * **摆的顺序决定星座长什么样。** 按分数直接摆上环，关系最近的两颗可能落在环的
 * 两端，那条连线就成了横穿整张图的一条弦 —— 实测第一版就是这样，十二颗星拉出
 * 一把交叉在星球上的长线，读起来是噪音而不是星座。
 *
 * 贪心：从分最高的那条起，每次接上「和刚放下这条共用学科最多」的那一条，同分
 * 按分数、再按 id。这样环上相邻的两颗多半是有关系的，连线就是沿着环的一段短弧。
 */
function chainByKinship(recs: readonly Recommendation[]): Recommendation[] {
  const rest = [...recs];
  const chain: Recommendation[] = [rest.shift()!];
  while (rest.length > 0) {
    const last = chain[chain.length - 1]!;
    let bestIdx = 0;
    let bestKey: [number, number, string] = [-1, -1, ""];
    rest.forEach((r, i) => {
      const key: [number, number, string] = [kinship(last, r), r.score, r.id];
      if (
        key[0] > bestKey[0] ||
        (key[0] === bestKey[0] && key[1] > bestKey[1]) ||
        (key[0] === bestKey[0] && key[1] === bestKey[1] && key[2] < bestKey[2])
      ) {
        bestKey = key;
        bestIdx = i;
      }
    });
    chain.push(rest.splice(bestIdx, 1)[0]!);
  }
  return chain;
}

/** 一条连线最长这么长。超过就不画。 */
const MAX_EDGE = 230;

/**
 * 星座连线：只连**环上相邻、挨得够近、并且真的共用学科**的两颗。
 *
 * 「关系」是共用学科的条数 —— 和根系上那条线是同一个关系，所以这两屏说的是同一
 * 件事：两个领域连在一起，是因为它们扎在同一门学问上。
 *
 * 只连相邻的两颗，所以最多 n-1 条短弧，连成一条沿着环的链。两两都连的话十二颗
 * 星会拉出四五十条线，那是一张网；连「关系最近的那一颗」而不限相邻，会拉出横穿
 * 整张图的长弦。两种都试过，都不是星座。
 */
export function constellationEdges(
  stars: readonly PlacedStar[],
): { from: PlacedStar; to: PlacedStar; shared: number }[] {
  const edges: { from: PlacedStar; to: PlacedStar; shared: number }[] = [];
  for (let i = 0; i + 1 < stars.length; i += 1) {
    const a = stars[i]!;
    const b = stars[i + 1]!;
    const shared = kinship(a, b);
    if (shared === 0) continue;
    // 太长的不画。全连上会连成一个闭合的多边形，而那读起来是一张图表的边框，
    // 不是星座 —— 真实的星座都是断开的几段。
    if (Math.hypot(a.x - b.x, a.y - b.y) > MAX_EDGE) continue;
    edges.push({ from: a, to: b, shared });
  }
  return edges;
}

/** 两颗推荐星的标签会不会叠在一起。测试用，也给以后调环参数时用。 */
export function starsOverlap(a: PlacedStar, b: PlacedStar): boolean {
  return Math.abs(a.x - b.x) < STAR_HALF_W * 2 && Math.abs(a.y - b.y) < STAR_HALF_H * 2;
}
