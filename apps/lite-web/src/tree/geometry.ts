import type { Field, FieldId, Keyword } from "./types";

/**
 * tree/geometry — 那棵树的形状。
 *
 * 主枝是 1000×780 viewBox 里的三次贝塞尔曲线，树干在 x=500。一个关键词的
 * `at.t` 是它沿枝的位置（0 = 贴着树干，1 = 梢尖），`at.spread` 把它沿法线推离
 * 曲线，于是同枝的叶子不会叠在一起。`pointOnBranch` 是把 (t, spread) 映射到
 * (x, y) 的**唯一**一处；树组件画的是同一条曲线，所以节点在构造上就一定在
 * 它自己的枝上。
 *
 * 这个文件从 `eco/data/tree.ts` 搬出来，只搬几何与成长刻度——那个文件剩下的
 * 三百多行是原型的 mock 关键词，**一个都没跟过来**。真关键词从
 * `GET /api/v1/interest/tree` 来（见 `useInterestTree.ts`），位置由
 * `liveTree.ts` 算出来。
 */

export const FIELDS: Field[] = [
  // 树冠。数学与形式从树干最顶上出来（树干自身的路径止于 y=292），所以它读起来
  // 是这棵树的主梢，而不是硬塞进一个为六根枝设计的布局里的第七根。
  { id: "formal", label: "数学与形式", en: "Mathematics & Formal", hue: "var(--mk-gold)", angle: -90 },
  { id: "science", label: "科学与自然", en: "Science & Nature", hue: "var(--mk-lake)", angle: -62 },
  { id: "humanities", label: "人文与写作", en: "Humanities & Writing", hue: "var(--mk-peach)", angle: -118 },
  { id: "making", label: "技术与创造", en: "Making & Tech", hue: "var(--mk-mist)", angle: -28 },
  { id: "society", label: "社会与世界", en: "Society & World", hue: "var(--mk-matcha)", angle: -152 },
  { id: "arts", label: "艺术与表达", en: "Arts & Expression", hue: "var(--mk-taro)", angle: -8 },
  { id: "self", label: "自我与成长", en: "Self & Growth", hue: "var(--mk-berry)", angle: -172 },
];

/** 七根主枝是一个固定的字面量列表，所以下标 0 一定存在；这个断言把这件事
 *  写下来，好过在每个调用点上都摆一个不可能发生的 undefined 分支。 */
const DEFAULT_FIELD = FIELDS[0]!;

export function fieldById(id: FieldId): Field {
  return FIELDS.find((f) => f.id === id) ?? DEFAULT_FIELD;
}

type Pt = [number, number];

/** The four cubic control points of each field's branch, trunk → tip. Typed as
 *  a fixed 4-tuple (not `Pt[]`) so destructuring gives four DEFINED points —
 *  the geometry helpers below would otherwise be littered with impossible
 *  undefined checks. */
export const BRANCH_CURVES: Record<FieldId, [Pt, Pt, Pt, Pt]> = {
  // 树冠：从树干顶端起，几乎笔直向上，带一点点右倾——完全垂直会读成桅杆。
  formal: [
    [500, 306],
    [506, 248],
    [524, 186],
    [548, 102],
  ],
  humanities: [
    [500, 352],
    [400, 300],
    [268, 268],
    [176, 168],
  ],
  science: [
    [500, 330],
    [606, 288],
    [738, 250],
    [836, 148],
  ],
  society: [
    [500, 470],
    [382, 452],
    [246, 424],
    [136, 352],
  ],
  making: [
    [500, 448],
    [626, 434],
    [760, 402],
    [880, 330],
  ],
  self: [
    [500, 588],
    [400, 592],
    [286, 578],
    [196, 528],
  ],
  arts: [
    [500, 574],
    [610, 580],
    [726, 568],
    [828, 516],
  ],
};

/** Point on a field's branch at parameter `t`, pushed `spread` px along the
 *  curve normal. One implementation, used by both the node layout and the
 *  connector stubs, so a node can never drift off its own branch. */
export function pointOnBranch(field: FieldId, t: number, spread = 0): { x: number; y: number } {
  const [p0, p1, p2, p3] = BRANCH_CURVES[field];
  const u = 1 - t;
  const bx = u * u * u * p0[0] + 3 * u * u * t * p1[0] + 3 * u * t * t * p2[0] + t * t * t * p3[0];
  const by = u * u * u * p0[1] + 3 * u * u * t * p1[1] + 3 * u * t * t * p2[1] + t * t * t * p3[1];
  // derivative → tangent → normal
  const dx =
    3 * u * u * (p1[0] - p0[0]) + 6 * u * t * (p2[0] - p1[0]) + 3 * t * t * (p3[0] - p2[0]);
  const dy =
    3 * u * u * (p1[1] - p0[1]) + 6 * u * t * (p2[1] - p1[1]) + 3 * t * t * (p3[1] - p2[1]);
  const len = Math.hypot(dx, dy) || 1;
  return { x: bx + (-dy / len) * spread, y: by + (dx / len) * spread };
}

export function branchPath(field: FieldId): string {
  const [p0, p1, p2, p3] = BRANCH_CURVES[field];
  return `M ${p0[0]} ${p0[1]} C ${p1[0]} ${p1[1]}, ${p2[0]} ${p2[1]}, ${p3[0]} ${p3[1]}`;
}

/**
 * The growth replay.
 *
 * 🚨 **The stops are RELATIVE to when she started, never absolute months.**
 * The first version read 三月 / 五月 / 七月 / 现在, which is fine for a student
 * who happens to have started in March and meaningless — worse, quietly false
 * — for everyone else. A student on her second day would be shown a five-month
 * growth story that is not hers.
 *
 * Relative buckets survive both ends of the range: someone three weeks in has
 * 起点 and 现在 and nothing between (see `stopsFor`, which drops the buckets
 * that hold nothing), and someone two years in still has five labels rather
 * than twenty-four months of dots.
 *
 * Index 3 is 现在; a keyword with `bornAt: 2` appears at the third stop and
 * stays.
 */
export const GROWTH_STOPS = [
  { label: "起点", sub: "刚开始的时候" },
  { label: "半年前", sub: "读得多起来" },
  { label: "近两个月", sub: "开始写" },
  { label: "现在", sub: "今天" },
];

/**
 * The stops worth showing, given what actually exists.
 *
 * A stop that holds no keywords is a dot she can press to see the same tree
 * she is already looking at. 现在 is always kept — there is always a present —
 * so a brand-new student gets a single label rather than a scrubber with
 * nothing to scrub.
 */
export function stopsFor(keywords: Keyword[]): number[] {
  const last = GROWTH_STOPS.length - 1;
  const live = GROWTH_STOPS.map((_, i) => i).filter(
    (i) => i === last || keywords.some((k) => k.bornAt <= i),
  );
  return live;
}

