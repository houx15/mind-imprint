import { apiFetch } from "./client";
import { tone, type Tone, type ToneName } from "../shared/tone";

// api/tree.ts —— 结构图。形状读自 apps/api/internal/api/pbl_tree.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

export interface TreeNode {
  id: string;
  tree: string;
  parentId: string | null;
  depth: number;
  ordinal: number;
  title: string;
  /** 这一块要放什么。 */
  body: string;
  author: "student" | "yinji";
  edited: boolean;
  /** 她把这一块摆在哪。0,0 = 还没摆过，界面按层级自动铺。 */
  x: number;
  y: number;
}

export type CheckQuestion = "covers" | "coherent" | "better";

export interface TreeCheck {
  question: CheckQuestion;
  answer: string;
}

export interface TreeState {
  tree: string;
  nodes: TreeNode[];
  checks: TreeCheck[];
}

/** 产品负责人 2026-09-01 的三个问题，原样。 */
export const CHECK_QUESTIONS: { question: CheckQuestion; ask: string; hint: string }[] = [
  {
    question: "covers",
    ask: "这个框架是否覆盖了所有应当呈现的内容？",
    hint: "思考是否所有主要内容都会被这个框架涵盖",
  },
  {
    question: "coherent",
    ask: "这个框架的逻辑顺序是否合理？",
    hint: "检查各部分之间的逻辑关系，想象如果是你来介绍这个框架，是否连贯、舒服",
  },
  {
    question: "better",
    ask: "你有更好的建议吗",
    hint: "",
  },
];

export function getTree(projectId: string, tree = "main"): Promise<TreeState> {
  return apiFetch<TreeState>(`${base(projectId)}/tree?tree=${encodeURIComponent(tree)}`);
}

export function createNode(
  projectId: string,
  body: { title: string; parentId?: string; body?: string; tree?: string; ordinal?: number },
): Promise<TreeNode> {
  return apiFetch<TreeNode>(`${base(projectId)}/tree`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function updateNode(
  projectId: string,
  nodeId: string,
  patch: { title?: string; body?: string; x?: number; y?: number },
): Promise<TreeNode> {
  return apiFetch<TreeNode>(`${base(projectId)}/tree/${nodeId}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}

export function moveNode(
  projectId: string,
  nodeId: string,
  body: { parentId?: string; ordinal?: number },
): Promise<TreeNode> {
  return apiFetch<TreeNode>(`${base(projectId)}/tree/${nodeId}/move`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function deleteNode(projectId: string, nodeId: string): Promise<void> {
  return apiFetch<void>(`${base(projectId)}/tree/${nodeId}`, { method: "DELETE" });
}

export function answerCheck(
  projectId: string,
  question: CheckQuestion,
  answer: string,
  tree = "main",
): Promise<TreeCheck> {
  return apiFetch<TreeCheck>(`${base(projectId)}/tree-checks`, {
    method: "POST",
    body: JSON.stringify({ question, answer, tree }),
  });
}

/**
 * 摊平成一份可以直接画的大纲：父节点后面紧跟它的孩子。
 *
 * 服务端按 depth 排序返回（一层一层），而屏幕上要的是"这一条下面是它自己的
 * 几个小块"。两者不是同一个顺序，转换必须在一个地方做完——散在界面里，缩进
 * 迟早会和实际的父子关系对不上。
 */
export function outline(nodes: TreeNode[]): TreeNode[] {
  const byParent = new Map<string, TreeNode[]>();
  for (const n of nodes) {
    const key = n.parentId ?? "";
    const list = byParent.get(key);
    if (list) list.push(n);
    else byParent.set(key, [n]);
  }
  for (const list of byParent.values()) {
    list.sort((a, b) => a.ordinal - b.ordinal);
  }
  const out: TreeNode[] = [];
  const walk = (parent: string, guard: number) => {
    if (guard > 8) return; // 数据坏了也不该让页面转不出来
    for (const n of byParent.get(parent) ?? []) {
      out.push(n);
      walk(n.id, guard + 1);
    }
  };
  walk("", 0);
  return out;
}

/**
 * 还差什么。
 *
 * 🚨 只剩"有没有结构"这一件事。结构由印记提（所以"至少三块"不会发生），
 * 三个问题是思考框架而不是作业——产品负责人 2026-09-02：「we don'''t require
 * student to answer textual question, but only to provide a thinking frame」。
 * 把它们做成必答门槛，等于把一次审视变成一份问卷。
 */
export function treeTodo(state: TreeState): string {
  return state.nodes.length === 0 ? "暂时没有需要审查的结构" : "";
}

/**
 * 自动铺一遍：层级决定列，同层的依次往下排。
 *
 * 只用在还没摆过的节点上（x=y=0）。她一拖，位置就成了她的——一张图的形状本身
 * 就是她的思考痕迹，不该每次打开被重排。
 */
export const NODE_MIN_H = 56;
/** 两块之间留的空。 */
const ROW_GAP = 18;
/** 列距。 */
const COL_W = 190;

export function autoLayout(
  nodes: TreeNode[],
  /**
   * 每一块**量出来的**真实高度。给不出来的按 NODE_MIN_H 算。
   *
   * 🚨 原来是按固定行距 74px 往下排的，而块高是 minHeight:56——标题加说明一换行
   * 就撑到九十多、一百多，直接盖住下一块。线上那棵 15 个节点的树，有三处文字被
   * 后一块压掉了半句，还有两处叠在一起。一件让她「看结构有没有漏」的工具，自己
   * 先把内容遮住了。
   */
  heights?: Map<string, number>,
): Map<string, { x: number; y: number }> {
  const out = new Map<string, { x: number; y: number }>();
  // 每一列下一块从哪儿开始。按真实高度累加，不按行号乘固定行距。
  const nextY = new Map<number, number>();
  for (const n of outline(nodes)) {
    if (n.x !== 0 || n.y !== 0) {
      out.set(n.id, { x: n.x, y: n.y });
      continue;
    }
    const y = nextY.get(n.depth) ?? 16;
    out.set(n.id, { x: 16 + n.depth * COL_W, y });
    nextY.set(n.depth, y + Math.max(NODE_MIN_H, heights?.get(n.id) ?? NODE_MIN_H) + ROW_GAP);
  }
  return out;
}

/**
 * 每一层一个颜色。
 *
 * 🚨 产品负责人 2026-09-03：「colorful, interactive」。这张图本来就能拖、能删、
 * 能双击改字，缺的是"看得见形状"——十几个一模一样的灰盒子连成一片，她看不出
 * 哪些是同一层的、哪一支特别深。而这件工具要她判断的恰恰是形状：盖全了吗、
 * 顺得下来吗、有没有更好的分法。
 *
 * 层数不会很深（服务端最多三层），越深越淡。
 */
const DEPTH_TONES: ToneName[] = ["mist", "taro", "matcha", "peach"];

export function depthTone(depth: number): Tone {
  return tone(DEPTH_TONES[Math.min(Math.max(depth, 0), DEPTH_TONES.length - 1)] as ToneName);
}
