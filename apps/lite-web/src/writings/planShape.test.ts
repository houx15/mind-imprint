import { describe, expect, it } from "vitest";
import { planShapeLine, planShapeOf } from "./planShape";
import type { WritingOutlineItem } from "../api/writingRoom";

function node(depth: number, text: string): WritingOutlineItem {
  return { id: `${depth}-${text}`, text, role: "", depth, position: 0 };
}

describe("planShapeOf", () => {
  /**
   * 产品负责人 2026-09-12 那张截图上的图，照着摆一遍。
   *
   * 两个最上层的块（一个还标着「待确认」）、两条理由、一条她自己的经历。
   * 这就是当时绿框弹出来的那一刻，服务端数到的形状。
   */
  it("数出截图上那张图的形状", () => {
    const s = planShapeOf([
      node(0, "人口多的城市，好高中多，大家读的高中不会那么统一"),
      node(0, "人多了，学校多了，自然就分散了"),
      node(1, "孩子多了学位不够，所以要多开设学校"),
      node(2, "家乡考生从五千涨到一万多，学校多开了好几所"),
      node(1, "各地都能开设很好的学校"),
    ]);
    expect(s).toEqual({ top: 2, points: 2, material: 1 });
    expect(planShapeLine(s)).toBe("中心论点 2 · 分论点 2 · 例子 1");
  });

  /** 深度 3 及以下都算她的材料，别漏掉。 */
  it("更深的节点也算材料", () => {
    expect(planShapeOf([node(3, "再深一层"), node(4, "还要深")]).material).toBe(2);
  });

  /** 空白节点不算 —— 服务端 writingPlanShapeOf 也是这么跳过的。 */
  it("空白的节点不数", () => {
    expect(planShapeOf([node(0, "   "), node(1, "")])).toEqual({ top: 0, points: 0, material: 0 });
  });
});
