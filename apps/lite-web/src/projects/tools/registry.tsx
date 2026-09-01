import type { ComponentType } from "react";
import type { ToolInstance } from "../../api/tools";
import { Board } from "./surfaces/Board";
import { Ideas } from "./surfaces/Ideas";
import { Observe } from "./surfaces/Observe";
import { Reframe } from "./surfaces/Reframe";

/**
 * registry.tsx —— 工具名 → 界面。
 *
 * 和后端 internal/pbl/tools.go 是同一张表的两半：那边管"属于哪一类"，这边管
 * "长什么样"。加一件工具 = 这里加一行 + 一个界面文件。
 *
 * 🚨 表里没有的工具也能被召出来（工具箱是开放的）。这时候界面退回到一张朴素
 * 的卡片：印记说了它想让她做什么，她做完点一下。功能少，但不会白屏。
 */

/** 一句话：打开这件工具，她要做的是什么。给她看的，所以说人话。 */
export const TOOL_TASKS: Record<string, string> = {
  observe: "带着一个问题出去看看，回来说说看到了什么",
  board: "把看到的、听到的、猜的、想问的都摊到板上",
  reframe: "说清楚谁需要什么，为什么",
  ideas: "多想几个办法，再挑一个先试",
  review: "一段一段看过去，不同意就说出来",
  decide: "摆开选项，说清这里什么最重要",
  structure: "先看整体骨架，再动手写",
  split: "这件事哪几步你做，哪几步印记做",
  lookback: "回头看这个项目，你是怎么走到这儿的",
  keep: "上线之后拿回来的数据，说明了什么",
};

export interface ToolSurfaceProps {
  projectId: string;
  tool: ToolInstance;
  /** 收工：把结果写回对话，面板回到计划。 */
  onFinish: (result: unknown, summary: string) => void;
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
};

export function surfaceFor(tool: string): ComponentType<ToolSurfaceProps> | null {
  return TOOL_SURFACES[tool] ?? null;
}
