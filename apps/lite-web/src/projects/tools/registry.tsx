import type { ComponentType } from "react";
import type { ToolInstance } from "../../api/tools";
import { Board } from "./surfaces/Board";
import { Decide } from "./surfaces/Decide";
import { Ideas } from "./surfaces/Ideas";
import { Keep } from "./surfaces/Keep";
import { Lookback } from "./surfaces/Lookback";
import { Observe } from "./surfaces/Observe";
import { Reframe } from "./surfaces/Reframe";
import { Review } from "./surfaces/Review";
import { Split } from "./surfaces/Split";
import { Structure } from "./surfaces/Structure";

/**
 * registry.tsx —— 工具名 → 界面。
 *
 * 和后端 internal/pbl/tools.go 是同一张表的两半：那边管"属于哪一类"，这边管
 * "长什么样"。加一件工具 = 这里加一行 + 一个界面文件。
 *
 * 🚨 表里没有的工具也能被召出来（工具箱是开放的）。这时候界面退回到一张朴素
 * 的卡片：印记说了它想让她做什么，她做完点一下。功能少，但不会白屏。
 */

/**
 * 邀请卡上的那一行：这件工具**是干什么的**。
 *
 * 🚨 和界面里 ToolFrame 的 task 是两句不同的话，故意的：这里回答"要不要打开"，
 * 那里回答"打开了现在做什么"。同一句话抄两份一定会漂移，所以两边都不抄——
 * 每件工具的 task 写在它自己的界面文件里，只有这一行在这里。
 */
export const TOOL_TASKS: Record<string, string> = {
  observe: "带着一个问题出去看看，回来说说看到了什么",
  board: "把看到的、听到的、猜的、想问的都摊到板上",
  reframe: "说清楚谁需要什么，为什么",
  ideas: "多想几个办法，再挑一个先试",
  review: "AI 可能出错，需要对它产出的内容做一次深度审核",
  decide: "深入思考 AI 提供的多个选项，做出合理的决策",
  structure: "审查整体结构是否合理",
  split: "你和 AI 的分工",
  lookback: "一起复盘项目过程吧！",
  keep: "持续观察成果落地后的反馈，进一步迭代你的成果！",
};

export interface ToolSurfaceProps {
  projectId: string;
  /** 她给这个项目选的类别；空 = 还没定。见 api/projects.ts · PROJECT_KINDS。
   *  长期迭代用它决定该请她看哪几种数据——走廊上的项目问不出点击率。 */
  projectKind: string;
  /** 便签板的坐标视图开着没有，以及怎么切换。见 migration 0122。 */
  boardAxes: boolean;
  onSetBoardAxes: (on: boolean) => Promise<void>;
  tool: ToolInstance;
  /** 收工：把结果写回对话，面板回到计划。 */
  onFinish: (result: unknown, summary: string) => void;
  /**
   * 把她送进一条已经开好的支线，印记在那儿先开口。
   *
   * 🚨 有两个地方的服务端**特意**为她开了一条支线并把 id 返回了，而前端把 id
   * 扔了：审核里点「问问这一句」，和长期迭代里点「深入讨论」。结果是她向 AI
   * 提了一个问题，产品把这个问题原样变成一个要她自己填的输入框——那是教她
   * 「这里的 AI 是假的」最快的办法。
   */
  onOpenSession: (sessionId: string) => void;
  onClose: () => void;
}

/**
 * 已经做好界面的工具。
 *
 * 每一刀往这里加一行，改的是自己的界面文件，不动别人的。
 */
export const TOOL_SURFACES: Record<string, ComponentType<ToolSurfaceProps>> = {
  observe: Observe,
  board: Board,
  reframe: Reframe,
  ideas: Ideas,
  review: Review,
  decide: Decide,
  structure: Structure,
  split: Split,
  lookback: Lookback,
  keep: Keep,
};

export function surfaceFor(tool: string): ComponentType<ToolSurfaceProps> | null {
  return TOOL_SURFACES[tool] ?? null;
}
