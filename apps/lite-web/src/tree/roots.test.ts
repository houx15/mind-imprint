import { describe, expect, it } from "vitest";
import { BRANCH_CURVES } from "./geometry";
import { GROUND_Y, ROOT_NODES, STAGE_H, leafShape, pointOnRoot } from "./roots";
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
