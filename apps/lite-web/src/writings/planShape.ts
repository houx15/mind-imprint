import type { WritingOutlineItem } from "../api/writingRoom";
import { outlineKindOf } from "./outlineKind";

/**
 * 这份计划现在有几块 —— 按 kind 数，和服务端 writingPlanShapeOf 数的是同一批东西。
 *
 * 🚨 2026-09-20：原来这里按**深度**数，外加一张关键词表去认「这是不是一个
 * 例子」。深度是模型定的，于是一条挂错层的分论点会被数成一个例子。
 * 现在节点自己带着 kind（outlineKind.ts 的闭表），深度不参与。
 *
 * 🚨 **这里只数数，不判断「够不够」。**
 * 「够不够」那条判据在服务端（writingPlanShape.ready），这块面板只在服务端说
 * ready 的时候才出现。要是这里再写一遍那条判据，两份实现迟早分岔 —— 而分岔的
 * 那一天，屏幕上会出现「够了」和它自己列的数对不上的情况，比不说还糟。
 * 所以这个函数交出来的是事实（几条），不是结论（够了）。
 */
export type PlanShape = { top: number; points: number; material: number };

export function planShapeOf(items: WritingOutlineItem[]): PlanShape {
  const s: PlanShape = { top: 0, points: 0, material: 0 };
  for (const it of items) {
    if (it.text.trim() === "") continue;
    // 这个 switch 和服务端 writingPlanShape.count 逐条一致。
    switch (outlineKindOf(it)) {
      case "opening":
      case "thesis":
      case "closing":
        s.top++;
        break;
      case "point":
      case "counter":
        s.points++;
        break;
      case "evidence":
      case "reference":
        s.material++;
        break;
      // 道理和对反方的回应是推理，不是材料；待补的材料是一个洞，也不算。
      case "reasoning":
      case "rebuttal":
      case "gap":
        break;
    }
  }
  return s;
}

/**
 * 把形状写成她读得懂的一行 —— 这一行要回答的是「你凭什么说够了」。
 *
 * 产品负责人 2026-09-12 报的原话：「AI 就判断已足以支撑一篇文章，**判断依据
 * 不清晰**。」原来那块绿框只有一句「这份思路已经够撑起一篇」，没有任何一个字
 * 说它数了什么。她因此没法判断这句话该不该信，也不知道自己还差什么。
 *
 * 名词，不写成句子（AGENTS.md §界面文案怎么写 第 1 条）。
 */
export function planShapeLine(s: PlanShape): string {
  return `中心论点 ${s.top} · 分论点 ${s.points} · 例子 ${s.material}`;
}
