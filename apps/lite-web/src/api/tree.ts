import { apiFetch } from "./client";

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
  patch: { title?: string; body?: string },
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

/** 上一条同层或更浅的节点——「缩进」就是挂到它下面去。 */
export function indentTarget(ordered: TreeNode[], nodeId: string): TreeNode | null {
  const i = ordered.findIndex((n) => n.id === nodeId);
  if (i <= 0) return null;
  const me = ordered[i]!;
  for (let j = i - 1; j >= 0; j--) {
    const candidate = ordered[j]!;
    if (candidate.depth === me.depth) return candidate;
    if (candidate.depth < me.depth) return null;
  }
  return null;
}

export function treeTodo(state: TreeState): string {
  if (state.nodes.length < 3) return "至少先分出三块";
  const unanswered = CHECK_QUESTIONS.filter(
    (q) => !state.checks.find((c) => c.question === q.question && c.answer.trim()),
  ).length;
  return unanswered ? `还有 ${unanswered} 个问题没想` : "";
}
