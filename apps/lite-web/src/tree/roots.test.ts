import { describe, expect, it } from "vitest";
import { BRANCH_CURVES } from "./geometry";
import { GROUND_Y, ROOT_NODES, STAGE_H, TRUNK_X, leafShape, pointOnRoot, threadPath } from "./roots";
import type { FieldId } from "./types";

/**
 * roots.test.ts —— 根系那几条**人眼盯不住**的不变量。
 *
 * 按 AGENTS.md 的「只写逻辑测试」：这里没有一条断言是「某个元素渲染了」。测的
 * 是四件读代码和看截图都不容易发现的事：42 个节点两两不重叠、每个节点都在地面
 * 以下、叶柄真的落在枝上、整张图不出画框。
 *
 * 前三件在一张 1500px 宽的截图上都长得「差不多对」—— 两个标签差六个像素，人眼
 * 看是挨着，看不出是压着；而一旦压上，学生看到的是一坨糊在一起的字。
 */

describe("42 个学科节点在根上的位置", () => {
  it("每根主枝六个，共 42 个，id 不重复", () => {
    expect(ROOT_NODES).toHaveLength(42);
    expect(new Set(ROOT_NODES.map((n) => n.id)).size).toBe(42);
  });

  it("全部在地面以下", () => {
    for (const n of ROOT_NODES) {
      expect(n.y, `${n.zh} 跑到地面上去了`).toBeGreaterThan(GROUND_Y);
    }
  });

  it("全部在画框里，标签也是", () => {
    for (const n of ROOT_NODES) {
      expect(n.x, `${n.zh} 顶出左边`).toBeGreaterThan(40);
      expect(n.x, `${n.zh} 顶出右边`).toBeLessThan(960);
      // 标签在节点上下 17px 处，再留一点行高。
      expect(n.y + 30, `${n.zh} 的标签掉出画框底`).toBeLessThan(STAGE_H);
    }
  });

  // 🚨 这一条是这个文件存在的主要理由。实测截图里出现过两次标签重叠
  // （文学批评 / 政治经济与制度，视觉设计 / 材料科学），两次都是根与根交叉
  // 的地方。42 个节点有 861 对，靠看截图是查不完的。
  it("两两之间不会近到标签叠在一起", () => {
    const tooClose: string[] = [];
    for (let i = 0; i < ROOT_NODES.length; i += 1) {
      for (let j = i + 1; j < ROOT_NODES.length; j += 1) {
        const a = ROOT_NODES[i]!;
        const b = ROOT_NODES[j]!;
        const dx = Math.abs(a.x - b.x);
        const dy = Math.abs(a.y - b.y);
        // 只有**横竖都近**才会压上：同一列错开三十像素是两行字，不是重叠。
        // 中文标签按 11px 字号估，四个字约 44px 宽。
        if (dx < 46 && dy < 24) tooClose.push(`${a.zh} × ${b.zh}`);
      }
    }
    expect(tooClose, `这些学科的标签会叠在一起：${tooClose.join("、")}`).toEqual([]);
  });
});

describe("叶子", () => {
  const fields: FieldId[] = [
    "formal",
    "science",
    "making",
    "society",
    "humanities",
    "arts",
    "self",
  ];

  // 叶柄必须**正好**落在枝那条贝塞尔上。差几个像素在截图上看不出来（叶子看着
  // 还是挨着枝的），但那正是「这张图就是你的模型」悄悄退化成装饰的那一步 ——
  // 一片飘在枝旁边的叶子，和一张画出来的树没有区别。
  it("叶柄落在它那根枝上", () => {
    for (const f of fields) {
      for (const t of [0.3, 0.55, 0.92]) {
        for (const spread of [52, -52]) {
          const leaf = leafShape(f, t, spread);
          const p = BRANCH_CURVES[f];
          const u = 1 - t;
          const bx =
            u * u * u * p[0][0] +
            3 * u * u * t * p[1][0] +
            3 * u * t * t * p[2][0] +
            t * t * t * p[3][0];
          const by =
            u * u * u * p[0][1] +
            3 * u * u * t * p[1][1] +
            3 * u * t * t * p[2][1] +
            t * t * t * p[3][1];
          // blade 的第一个点就是叶柄。
          const [sx, sy] = leaf.blade
            .slice(2, leaf.blade.indexOf(" C"))
            .split(",")
            .map(Number) as [number, number];
          expect(Math.hypot(sx - bx, sy - by), `${f} t=${t} 的叶柄离枝太远`).toBeLessThan(0.6);
        }
      }
    }
  });

  // spread 的正负决定叶子朝哪边。原来的光珠也是这么摆的，所以这条保证「换成
  // 叶子」没有偷偷改掉树冠的布局语义。
  it("朝向跟着 spread 的正负走", () => {
    const right = leafShape("making", 0.6, 52);
    const left = leafShape("making", 0.6, -52);
    const mid = pointOnRoot("making", 0); // 只是拿来确认两边真的分开
    expect(mid).toBeDefined();
    expect(right.mx).not.toBeCloseTo(left.mx, 0);
    // 一左一右，中点必须落在叶柄的两侧。
    const stemX = (right.mx + left.mx) / 2;
    expect((right.mx - stemX) * (left.mx - stemX)).toBeLessThan(0);
  });
});

describe("叶子连到学科的那条线", () => {
  /** 把 `M x y C ax ay, bx by, cx cy` 拆回四个点。 */
  function ctrlPoints(d: string): [number, number][] {
    const nums = d.match(/-?\d+(\.\d+)?/g)!.map(Number);
    return [
      [nums[0]!, nums[1]!],
      [nums[2]!, nums[3]!],
      [nums[4]!, nums[5]!],
      [nums[6]!, nums[7]!],
    ];
  }

  function sample(d: string, t: number): { x: number; y: number } {
    const p = ctrlPoints(d);
    const u = 1 - t;
    const at = (i: 0 | 1) =>
      u * u * u * p[0]![i] + 3 * u * u * t * p[1]![i] + 3 * u * t * t * p[2]![i] + t * t * t * p[3]![i];
    return { x: at(0), y: at(1) };
  }

  // 六片叶子在树冠上的大致位置，两片在左、两片在右、两片靠中间。
  const leaves = [
    { x: 330, y: 400 },
    { x: 690, y: 300 },
    { x: 520, y: 250 },
    { x: 640, y: 470 },
    { x: 400, y: 520 },
    { x: 860, y: 330 },
  ];

  // 🚨 这个文件里第二条「人眼盯不住」的不变量。上一版把每条线都穿过
  // `(500, GROUND_Y)`，六条线在地面交成一个结；换成直连之后，唯一能证明
  // 「不再是一个总站」的办法就是量它们在地面那一层横向散开多少。
  it("在地面那一层不收敛到同一个点", () => {
    const groundX: number[] = [];
    for (const leaf of leaves) {
      for (const node of ROOT_NODES.slice(0, 8)) {
        const d = threadPath(leaf, node);
        // 二分找 y == GROUND_Y 的那个 t。线是单调下降的（下一条测试保证），
        // 所以二分一定收敛。
        let lo = 0;
        let hi = 1;
        for (let i = 0; i < 40; i += 1) {
          const mid = (lo + hi) / 2;
          if (sample(d, mid).y < GROUND_Y) lo = mid;
          else hi = mid;
        }
        groundX.push(sample(d, (lo + hi) / 2).x);
      }
    }
    const spread = Math.max(...groundX) - Math.min(...groundX);
    expect(spread, "所有线在地面挤到了一起，又变回一个总站了").toBeGreaterThan(200);
  });

  it("一路向下，不往回勾", () => {
    for (const leaf of leaves) {
      for (const node of ROOT_NODES) {
        const d = threadPath(leaf, node);
        let prev = -Infinity;
        for (let i = 0; i <= 24; i += 1) {
          const y = sample(d, i / 24).y;
          expect(y, `${node.zh} 那条线在 t=${i / 24} 处往回走了`).toBeGreaterThanOrEqual(prev - 0.01);
          prev = y;
        }
      }
    }
  });

  // 线要顺着树干走，不能斜穿整张图 —— 这是上一版唯一做对、必须保住的一件事。
  it("过地面时贴着树干那一带，不从画框边上横过去", () => {
    for (const leaf of leaves) {
      for (const node of ROOT_NODES) {
        const d = threadPath(leaf, node);
        let lo = 0;
        let hi = 1;
        for (let i = 0; i < 40; i += 1) {
          const mid = (lo + hi) / 2;
          if (sample(d, mid).y < GROUND_Y) lo = mid;
          else hi = mid;
        }
        const x = sample(d, (lo + hi) / 2).x;
        expect(Math.abs(x - TRUNK_X), `${node.zh} 那条线离树干太远了`).toBeLessThan(300);
      }
    }
  });

  it("端点就是叶子和学科本身", () => {
    const node = ROOT_NODES[10]!;
    const d = threadPath({ x: 330, y: 400 }, node);
    const p = ctrlPoints(d);
    expect(p[0]).toEqual([330, 400]);
    expect(p[3]![0]).toBeCloseTo(node.x, 0);
    expect(p[3]![1]).toBeCloseTo(node.y, 0);
  });
});
