import type { WritingOutlineItem } from "../api/writingRoom";

/**
 * outlineKind —— `apps/api/internal/api/writing_kind.go` 的 TS 孪生。
 *
 * 图上一个节点「是什么」的闭表。服务端是单一真相源：它决定深度和父节点，
 * 这一侧只负责把同一套规则画到屏幕上、并在她拖动之后算出新的 kind。
 *
 * 🚨 **两边必须逐条一致**：取值、深度、标题、老行的兜底顺序。
 * 改一边就要改另一边；两处各有一条测试钉着。
 *
 * # 为什么会有这张表（2026-09-20）
 *
 * 在这之前，模型自己挑 parentId、自己用散文写 role，这一侧再拿关键词把意思
 * 猜回来。`buildSlots` 里有三张词表，而它只在 `depth === 0` 时认结尾 ——
 * 于是一个被挂到深度 1 的结尾掉进最后那个 else，印成「分论点 3」。
 * 同事的意见 3：「这个是总结，不是分论点」。标题错是摆放错的下游。
 */

export type OutlineKind =
  | "opening"
  | "thesis"
  | "point"
  | "evidence"
  | "reference"
  | "reasoning"
  | "counter"
  | "rebuttal"
  | "gap"
  | "closing";

/** 每种块在图上的深度。**强制**，和 Go 侧的 writingKindDepths 一致。 */
export const OUTLINE_KIND_DEPTH: Record<OutlineKind, number> = {
  opening: 0,
  thesis: 0,
  closing: 0,
  point: 1,
  counter: 1,
  evidence: 2,
  reference: 2,
  reasoning: 2,
  rebuttal: 2,
  gap: 2,
};

const ALL_KINDS = Object.keys(OUTLINE_KIND_DEPTH) as OutlineKind[];

export function isOutlineKind(k: string): k is OutlineKind {
  return (ALL_KINDS as string[]).includes(k);
}

/**
 * 印在她屏幕上的小标题。
 *
 * 用的是语文课上的正式词（AGENTS.md 文案规则 6），而且是名词（规则 1）。
 * 论据分两种，各自是一个 kind —— 一个司马迁的例子没有链接可填，
 * 但它不是「她见过的事」。
 */
export function outlineKindLabel(kind: string): string {
  switch (kind) {
    case "opening":
      return "开篇";
    case "thesis":
      return "中心论点";
    case "point":
      return "分论点";
    case "counter":
      return "反方观点";
    case "rebuttal":
      return "对反方的回应";
    case "gap":
      return "待补的材料";
    case "closing":
      return "结尾";
    case "evidence":
      return "论据 · 你见过的事";
    case "reference":
      return "论据 · 你找来的材料";
    case "reasoning":
      return "道理";
    default:
      return "";
  }
}

// —— 以下只服务 0182 之前存下的老行 ——

const ROLE_WORDS_OPENING = ["开头", "引言", "开篇", "钩子", "导入", "opening", "hook", "introduction", "intro"];
const ROLE_WORDS_CLOSING = ["结尾", "结论", "总结", "收尾", "落点", "结语", "closing", "conclusion", "ending"];
const ROLE_WORDS_COUNTER = ["反方", "对方", "反对", "质疑", "counter", "objection"];
const ROLE_WORDS_REBUTTAL = ["回应", "反驳", "rebuttal", "response"];
const ROLE_WORDS_GAP = ["还没找到", "没找到", "待补", "暂时没有", "缺一份"];
// 只在深度 ≥ 2 上用：「一条理由」在深度 1 是一条分论点，在深度 2 才是
// 撑着它的一条道理。
const ROLE_WORDS_REASONING = ["道理", "解释", "推理", "分析", "原因", "理由", "reasoning", "explanation", "analysis", "reason"];
// 🚨 先判它是不是一条材料，再判它是谁的 —— 和 Go 侧同一个分工。
// 只有 role 明说是她的才算她见过的事；其余一律算她找来的。那个方向是故意的
//（老的 writingRoleIsPersonal）：这个数只用来提醒「还缺一条更有说服力的例子」，
// 少提醒一次比冤枉她强。
const ROLE_WORDS_PERSONAL = [
  "你", "自己", "亲身", "个人", "身边", "经历过", "见过",
  "your own", "personal", "my own", "you saw", "you did",
];
const ROLE_WORDS_MATERIAL = [
  "例", "经历", "的事", "事件", "故事", "案例", "人物", "证据", "场景", "现象",
  "材料", "数据", "研究", "报道", "访谈", "调查", "引用", "名言", "史实", "素材",
  "新闻", "实验", "统计", "文献", "论文",
  "example", "experience", "evidence", "story", "case",
  "data", "study", "research", "report", "quote", "survey", "statistic", "source", "paper",
];

const hasAny = (role: string, words: string[]) => {
  const r = role.toLowerCase();
  return words.some((w) => r.includes(w));
};

/**
 * 把老行的自由散文 role 映射成一个 kind。顺序和 Go 侧的
 * `writingKindFromRole` 逐条一致：先认最具体的，再认开篇 / 结尾，
 * 再认论据，最后按深度兜底。
 *
 * 🚨 兜底**一行都不丢**：任何 role 都会得到一个 kind。
 */
export function outlineKindFromRole(role: string, depth: number): OutlineKind {
  if (hasAny(role, ROLE_WORDS_GAP)) return "gap";
  if (hasAny(role, ROLE_WORDS_REBUTTAL)) return "rebuttal";
  if (hasAny(role, ROLE_WORDS_COUNTER)) return "counter";
  if (hasAny(role, ROLE_WORDS_OPENING)) return "opening";
  if (hasAny(role, ROLE_WORDS_CLOSING)) return "closing";
  if (depth >= 2 && hasAny(role, ROLE_WORDS_REASONING)) return "reasoning";
  if (hasAny(role, ROLE_WORDS_MATERIAL)) {
    return hasAny(role, ROLE_WORDS_PERSONAL) ? "evidence" : "reference";
  }
  if (depth <= 0) return "thesis";
  if (depth === 1) return "point";
  // 深度 2 而 role 说不出它是什么：当成她找来的。见上面那段注释。
  return "reference";
}

/**
 * 读一个节点的 kind。服务端从 0182 起每一行都带着它；老行现算一个，
 * 别让空字符串一路掉进 switch 的 default。
 */
export function outlineKindOf(item: Pick<WritingOutlineItem, "role" | "depth"> & { kind?: string; source?: string }): OutlineKind {
  if (item.kind && isOutlineKind(item.kind)) return item.kind;
  // 有出处的一定是她找来的 —— 和 0182 的回填同一条规则。
  if ((item.source ?? "").trim() !== "" && item.depth > 0) return "reference";
  return outlineKindFromRole(item.role ?? "", item.depth);
}

/** 这个节点是材料（写进某一段），不是一段。 */
export function outlineKindIsMaterial(kind: OutlineKind): boolean {
  return kind === "evidence" || kind === "reference";
}

/**
 * 这个节点写进某一段里，而不是自己成为一段。
 * 材料、道理、对反方的回应、待补，都是这一类。
 */
export function outlineKindIsInlineContent(kind: OutlineKind): boolean {
  return outlineKindIsMaterial(kind) || kind === "reasoning" || kind === "rebuttal" || kind === "gap";
}

/**
 * 她把一个节点拖到了新的深度之后，它的 kind 该是什么。
 *
 * 深度对得上就不动（把一条论据从一个分论点挪到另一个，它还是论据）。
 * 对不上就换成那一层的默认种类 —— 她做的那个动作的意思正是
 *「这其实是一条理由」，标题当场跟着变，她才看得见自己刚才做成了什么。
 *
 * `hasThesis` 决定深度 0 的默认：图上已经有中心论点了，再拖一个上来就是结尾
 *（一篇只有一个中心论点）。
 */
export function rekindForDepth(kind: OutlineKind, depth: number, hasThesis: boolean): OutlineKind {
  if (OUTLINE_KIND_DEPTH[kind] === depth) return kind;
  if (depth <= 0) return hasThesis ? "closing" : "thesis";
  if (depth === 1) return "point";
  return "evidence";
}
