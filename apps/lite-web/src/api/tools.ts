import { apiFetch } from "./client";

// api/tools.ts —— 工具箱的客户端。
// 形状读自 apps/api/internal/api/pbl_artifacts.go 和 internal/pbl/tools.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

/** thinking = 当场和印记一起做完。world = 她要离开屏幕，过几天才回来。 */
export type ToolKind = "thinking" | "world";

export type ToolStatus = "summoned" | "accepted" | "declined" | "done";

export interface ToolInstance {
  id: string;
  /** 工具名。自由字符串——工具箱是开放的。 */
  tool: string;
  kind: ToolKind;
  /** 界面上显示的名字。后端查过工具箱了，前端不用自己再猜一遍。 */
  label: string;
  /** 印记为什么这时候递它。没有理由的工具是伏击。 */
  reason: string;
  status: ToolStatus;
  result?: unknown;
  note: string;
  acceptedAt: string | null;
  resolvedAt: string | null;
  createdAt: string;
}

export function listTools(projectId: string): Promise<ToolInstance[]> {
  return apiFetch<ToolInstance[]>(`${base(projectId)}/tools`);
}

/**
 * 她自己从材料清单点开的工具，理由固定这一句。
 *
 * 🚨 服务端拒绝没有理由的工具（对的：一件说不出为什么的工具对她是一次打断），
 * 所以浏览也得给个理由。但这一句同时是个标记：**这不是一件待办**。
 * 她回头看一眼便签板，不该在"进行中"里多出一条任务——看东西不是任务。
 */
export const SELF_OPENED = "由你打开";

export function summonTool(
  projectId: string,
  body: { tool: string; reason: string; kind?: ToolKind; sessionId?: string },
): Promise<ToolInstance> {
  return apiFetch<ToolInstance>(`${base(projectId)}/tools`, { method: "POST", body: JSON.stringify(body) });
}

/** 她打开了。对 world 工具，这一步之后她就出门了。 */
export function acceptTool(projectId: string, toolId: string): Promise<ToolInstance> {
  return apiFetch<ToolInstance>(`${base(projectId)}/tools/${toolId}/accept`, { method: "POST" });
}

/**
 * 了结一件工具。
 *
 * note 是可选的：拒绝不需要理由。如果说「不用」比说「好」还费事，那就不是
 * 一个真的选择。
 */
export function resolveTool(
  projectId: string,
  toolId: string,
  body: { status: "done" | "declined"; result?: unknown; note?: string },
): Promise<ToolInstance> {
  return apiFetch<ToolInstance>(`${base(projectId)}/tools/${toolId}/resolve`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

/** 还没了结的：刚递出来的，和她答应了正在做的。 */
export function openTools(tools: ToolInstance[]): ToolInstance[] {
  return tools.filter((t) => t.status === "summoned" || t.status === "accepted");
}

/**
 * 她出门在做的事。
 *
 * 单独挑出来，因为界面对它的态度不一样：一件 world 工具挂在那里是正常的，
 * 不该显示成待办、更不该被催。
 */
export function awayTools(tools: ToolInstance[]): ToolInstance[] {
  return tools.filter((t) => t.kind === "world" && t.status === "accepted");
}
