import disciplinesJSON from "@mind-imprint/contracts/disciplines";
import { BRANCH_CURVES, FIELDS } from "./geometry";
import type { FieldId } from "./types";

/**
 * roots.ts —— 地面以下的那一半。
 *
 * 树冠是**她的**：每片叶子是她真做过一件事才长出来的一个领域。根是**世界的**：
 * 42 门学科，固定的、完整的，不依赖她做过多少事。
 *
 * 这个分工是根系比原来那棵树强的地方。原来没有词就是七根空枝 —— 一个刚注册的
 * 学生打开自己的树，看到的是七根什么都没有的线。现在她看到的是一棵还没长叶子
 * 的树，底下是已经铺好的土。
 *
 * 几何上，根是七根主枝在地下的镜像：起点在树干 y=760 到 830，浅的铺得宽，
 * formal 是扎得最深的主根。viewBox 因此从 1000×780 撑到 1000×1100。
 */

type Pt = [number, number];

/** 地面。树干在 TreeSvg 里画到 y=748。 */
export const GROUND_Y = 748;

/** 加了根之后这张图有多高。树冠那 780 一个像素没动。 */
export const STAGE_H = 1100;

export const ROOT_CURVES: Record<FieldId, [Pt, Pt, Pt, Pt]> = {
  // 🚨 浅的那一对（science / humanities）和深的那一对（making / society）之间
  // 要拉开：第一版四根挤在同一个深度带里，靠近树干的那几个学科标签直接叠在
  // 一起（实测截图里「文学批评」压着「政治经济与制度」）。浅的走得更平、更早
  // 抬起来，深的扎得更陡。
  science: [
    [505, 770],
    [658, 792],
    [800, 816],
    [902, 862],
  ],
  humanities: [
    [495, 770],
    [342, 792],
    [200, 816],
    [98, 862],
  ],
  making: [
    [508, 800],
    [656, 848],
    [806, 910],
    [890, 1000],
  ],
  society: [
    [492, 800],
    [344, 848],
    [194, 910],
    [110, 1000],
  ],
  // arts / self 是**内圈**：第一版它们和 making / society 交叉，交叉处的两个
  // 标签叠在一起（实测截图里「视觉设计」压着「材料科学」）。收窄之后整个根系
  // 是一个同心的扇形 —— 浅宽的 science/humanities、深宽的 making/society、
  // 内圈的 arts/self，正中是 formal 那根主根，四对之间不再有交点。
  arts: [
    [506, 832],
    [590, 892],
    [656, 958],
    [694, 1046],
  ],
  self: [
    [494, 832],
    [410, 892],
    [344, 958],
    [306, 1046],
  ],
  // 主根，扎得最深。和树冠上 formal 从树干顶端笔直向上是同一个位置关系。
  formal: [
    [500, 800],
    [502, 876],
    [506, 962],
    [508, 1042],
  ],
};

function bez(p: [Pt, Pt, Pt, Pt], t: number): Pt {
  const u = 1 - t;
  return [
    u * u * u * p[0][0] + 3 * u * u * t * p[1][0] + 3 * u * t * t * p[2][0] + t * t * t * p[3][0],
    u * u * u * p[0][1] + 3 * u * u * t * p[1][1] + 3 * u * t * t * p[2][1] + t * t * t * p[3][1],
  ];
}

function tangent(p: [Pt, Pt, Pt, Pt], t: number): Pt {
  const u = 1 - t;
  const dx =
    3 * u * u * (p[1][0] - p[0][0]) + 6 * u * t * (p[2][0] - p[1][0]) + 3 * t * t * (p[3][0] - p[2][0]);
  const dy =
    3 * u * u * (p[1][1] - p[0][1]) + 6 * u * t * (p[2][1] - p[1][1]) + 3 * t * t * (p[3][1] - p[2][1]);
  const len = Math.hypot(dx, dy) || 1;
  return [dx / len, dy / len];
}

/** 根上某一点，沿法向推 `spread` 像素。和 pointOnBranch 是同一套数学。 */
export function pointOnRoot(field: FieldId, t: number, spread = 0): { x: number; y: number } {
  const p = ROOT_CURVES[field];
  const [x, y] = bez(p, t);
  const [tx, ty] = tangent(p, t);
  return { x: x + -ty * spread, y: y + tx * spread };
}

export function rootPath(field: FieldId): string {
  const p = ROOT_CURVES[field];
  return `M ${p[0][0]} ${p[0][1]} C ${p[1][0]} ${p[1][1]}, ${p[2][0]} ${p[2][1]}, ${p[3][0]} ${p[3][1]}`;
}

/**
 * 副根。每根主根上分两条短的 —— 没有它们，根系读起来是七条线，不是一团根。
 */
export function subRootPaths(field: FieldId): string[] {
  const p = ROOT_CURVES[field];
  return [
    [0.38, 0.9],
    [0.62, -0.8],
  ].map(([t, side]) => {
    const a = bez(p, t!);
    const tg = tangent(p, t!);
    const nx = -tg[1];
    const ny = tg[0];
    const L = 118;
    const ex = a[0] + tg[0] * L * 0.62 + nx * L * side! * 0.5;
    const ey = a[1] + tg[1] * L * 0.62 + ny * L * side! * 0.5 + 46;
    return (
      `M ${a[0].toFixed(1)} ${a[1].toFixed(1)} ` +
      `C ${(a[0] + tg[0] * 34).toFixed(1)} ${(a[1] + tg[1] * 34 + 16).toFixed(1)}, ` +
      `${((a[0] + ex) / 2).toFixed(1)} ${((a[1] + ey) / 2 + 18).toFixed(1)}, ` +
      `${ex.toFixed(1)} ${ey.toFixed(1)}`
    );
  });
}

/* ── 42 门学科在根上的位置 ──────────────────────────────────────────────── */

type RawDiscipline = { id: string; field: string; zh: string; en: string; asks: string };

export type RootNode = {
  id: string;
  field: FieldId;
  zh: string;
  asks: string;
  x: number;
  y: number;
  /** 标签挂在节点下面还是上面。上下交错，相邻两个节点的字才不会咬在一起。 */
  labelBelow: boolean;
};

/**
 * 学科节点的位置。
 *
 * 🚨 **由学科表里的顺序决定，不由哈希决定** —— 和树冠上 placeOnBranch 同一个
 * 办法，同一个理由：哈希会撞，撞了就是两个标签压在同一个像素上。
 *
 * 由此还得到一件对的事：**每个学生看到的根系都一样**。学科表是世界，不是她的
 * 数据；一棵每个人都不同的「世界」会让这张图说不出自己在讲什么。
 */
/**
 * 每根根上节点的相位，单位是**一个节点间距**（0 到 1）。
 *
 * 它挡的是标签重叠：相邻两根的节点因此落在不同深度上。写成「一个间距的几分
 * 之几」而不是一个绝对的 t 偏移，是因为后者会**压缩**那根根的可用长度 ——
 * 试过一次，formal 的六个节点被挤进 0.54..0.95，自己跟自己叠上了。
 *
 * 🚨 **这七个数是搜出来的，不是调出来的。** 手动一个个试是打地鼠：把右边那对
 * 分开，左边那对立刻叠上（前后叠过五对）。42 个节点有 861 对，人眼查不完，
 * 所以判据在 roots.test.ts 里，值由一次穷举得到 —— 只有 science 和 society
 * 需要错开，其余五根同相。改根的曲线之后要重跑那条测试，它会告诉你哪一对叠了。
 */
const FIELD_PHASE: Record<FieldId, number> = {
  science: 0.75,
  society: 0.45,
  formal: 0,
  making: 0,
  humanities: 0,
  arts: 0,
  self: 0,
};

export const ROOT_NODES: RootNode[] = (() => {
  const all = disciplinesJSON as RawDiscipline[];
  const out: RootNode[] = [];
  for (const field of FIELDS.map((f) => f.id)) {
    const rows = all.filter((d) => d.field === field);
    rows.forEach((d, i) => {
      // 从 0.40 起步而不是贴着树干：靠近树干的那一段是七根根汇聚的地方，把
      // 节点摆在那里，相邻两根上的学科会挤成一团。
      //
      // 🚨 每根根还带一个**相位**。没有它就是打地鼠：把右边那对分开，左边那对
      // 立刻叠上（实测「遗传与进化 × 人工智能」修好之后，「媒介素养 × 政治经济
      // 与制度」接着叠）。相位让相邻两根上的节点落在**不同的深度**上，交叉点
      // 附近因此不会同时有两个节点。
      const span = 0.52;
      const step = span / Math.max(1, rows.length - 1);
      const t = 0.36 + FIELD_PHASE[field] * step + (i / Math.max(1, rows.length - 1)) * span;
      // 🚨 偏移量**随深度放大**。浅的那几根几乎是横着走的，法向因此几乎是竖直
      // 的 —— 固定偏移会把靠近树干的节点直接推到地面以上（实测「生态学」跑到
      // y=735，地面在 748）。近处收窄同时也解决了汇聚处的拥挤。
      const grow = 0.45 + 0.55 * t;
      const spread = (i % 2 === 0 ? 1 : -1) * (20 + (i % 3) * 16) * grow;
      const { x, y } = pointOnRoot(field, t, spread);
      // 标签上下交错。同一根根上六个节点，全部标在下面时相邻两个的字会咬在
      // 一起 —— 交错之后它们各占一层。
      out.push({ id: d.id, field, zh: d.zh, asks: d.asks, x, y, labelBelow: i % 2 === 0 });
    });
  }
  return out;
})();

export const ROOT_NODE_BY_ID = new Map(ROOT_NODES.map((n) => [n.id, n]));

/* ── 叶子 ───────────────────────────────────────────────────────────────── */

export type Leaf = {
  /** 叶片轮廓，两段三次贝塞尔围成的梭形。 */
  blade: string;
  /** 叶脉。 */
  rib: string;
  /** 叶片中点，用来挂发光与命中区。 */
  mx: number;
  my: number;
  /** 叶尖再往外一点，标签落在这里。 */
  lx: number;
  ly: number;
};

/**
 * 一片叶子。
 *
 * 叶柄落在枝上（`bez(branch, t)`），叶尖沿法向伸出去。方向沿用原来 `spread`
 * 的正负，所以左右交替的布局语义一个字没变 —— 这片叶子长在原来那颗光珠所在的
 * 位置上，只是现在它有形状了。
 */
export function leafShape(field: FieldId, t: number, spread: number): Leaf {
  const p = BRANCH_CURVES[field];
  const sgn = spread >= 0 ? 1 : -1;
  const L = Math.max(56, Math.abs(spread) * 1.35);
  const u = 1 - t;
  const bx =
    u * u * u * p[0][0] + 3 * u * u * t * p[1][0] + 3 * u * t * t * p[2][0] + t * t * t * p[3][0];
  const by =
    u * u * u * p[0][1] + 3 * u * u * t * p[1][1] + 3 * u * t * t * p[2][1] + t * t * t * p[3][1];
  const dx =
    3 * u * u * (p[1][0] - p[0][0]) + 6 * u * t * (p[2][0] - p[1][0]) + 3 * t * t * (p[3][0] - p[2][0]);
  const dy =
    3 * u * u * (p[1][1] - p[0][1]) + 6 * u * t * (p[2][1] - p[1][1]) + 3 * t * t * (p[3][1] - p[2][1]);
  const len = Math.hypot(dx, dy) || 1;
  // 叶轴 = 枝的法向乘 spread 的符号。
  const ux = (-dy / len) * sgn;
  const uy = (dx / len) * sgn;
  const nx = -uy;
  const ny = ux;
  const W = L * 0.36;
  const P = (a: number, b: number) =>
    `${(bx + ux * a + nx * b).toFixed(1)},${(by + uy * a + ny * b).toFixed(1)}`;
  return {
    blade:
      `M ${P(0, 0)} C ${P(L * 0.24, -W)} ${P(L * 0.74, -W * 0.8)} ${P(L, 0)} ` +
      `C ${P(L * 0.74, W * 0.8)} ${P(L * 0.24, W)} ${P(0, 0)} Z`,
    rib: `M ${P(L * 0.06, 0)} L ${P(L * 0.9, 0)}`,
    mx: bx + ux * L * 0.52,
    my: by + uy * L * 0.52,
    lx: bx + ux * (L + 26),
    ly: by + uy * (L + 26),
  };
}

/**
 * 一片叶子到一个学科节点之间的那条线。
 *
 * **顺着树干往下走**，而不是斜穿过整张图：这条线要读起来像「这个词的根扎在
 * 那里」，而一条从树梢直接拉到根尖的直线读起来只是一条连线。
 */
export function threadPath(
  from: { x: number; y: number },
  to: { x: number; y: number },
): string {
  return (
    `M ${from.x.toFixed(1)} ${from.y.toFixed(1)} ` +
    `C ${from.x.toFixed(1)} ${(from.y + 110).toFixed(1)}, 500 ${GROUND_Y - 88}, 500 ${GROUND_Y} ` +
    `C 500 ${GROUND_Y + 52}, ${to.x.toFixed(1)} ${(to.y - 100).toFixed(1)}, ` +
    `${to.x.toFixed(1)} ${to.y.toFixed(1)}`
  );
}
