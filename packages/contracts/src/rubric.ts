import { z } from "zod";

export const SoloLevel = z.enum(["L1", "L2", "L3", "L4", "NA"]);
export type SoloLevel = z.infer<typeof SoloLevel>;

// A scored level excludes N/A (N/A = insufficient evidence, rendered neutrally, not on the bar).
export type ScoredLevel = Exclude<SoloLevel, "NA">;
export const SOLO_LABELS: Record<ScoredLevel, string> = { L1: "萌芽", L2: "发展中", L3: "熟练", L4: "卓越" };

export interface RubricDimension {
  id: string; name: string; framework: string;
  anchors: { L1: string; L2: string; L3: string; L4: string };
}

// Rubric container: wraps a set of dimensions with metadata and validation.
export interface Rubric {
  id: string;
  name: string;
  dimensions: RubricDimension[];
}

export const FULL_RUBRIC: RubricDimension[] = [
  { id: "D1", name: "提问清晰度", framework: "ATL 思维 · QUEST-Q（输入）",
    anchors: { L1: "直接抛一句话问题，不给 AI 任何背景或目标", L2: "给一点背景，但目标/约束模糊，常需 AI 反问澄清", L3: "主动提供任务背景、目标与约束，问题具体可执行", L4: "结构化拆解需求，分步追问并根据回答迭代提问" } },
  { id: "D2", name: "信源辨识", framework: "媒介/信息素养 · CRAAP",
    anchors: { L1: "完全信任 AI / 来源，从不追问出处", L2: "偶尔问「真的吗？」但不深入", L3: "主动要求论据，能识别来源等级", L4: "主动交叉验证，识别信源之间的利益关系与冲突" } },
  { id: "D3", name: "横向验证", framework: "ATL 研究 · 横向阅读 SHEG",
    anchors: { L1: "只看单一来源，不另开查证", L2: "想到要多看，但没真去找", L3: "主动多源对照，找到 2+ 独立来源", L4: "溯到原始出处，比较各源权威性与一致性" } },
  { id: "D4", name: "多视角与让步", framework: "QUEST-E · 论证评估",
    anchors: { L1: "只站自己一方，无视反方", L2: "提到反方但轻描淡写 / 稻草人", L3: "主动找反方并正面回应", L4: "构建反方最强论证(steelman)后再让步反驳" } },
  { id: "D5", name: "论证拆解", framework: "QUEST-U · 论证分析",
    anchors: { L1: "把观点当事实，不分论点论据", L2: "能复述但不辨结构", L3: "能识别论点-论据-假设结构", L4: "识别隐藏前提与论证谬误" } },
  { id: "D6", name: "反思与元认知", framework: "ATL 反思 · TOK 认知者与知识",
    anchors: { L1: "不觉察自己被 AI 影响", L2: "事后偶尔回顾", L3: "主动校准信心，觉察思维盲点", L4: "觉察自己作为认知者的位置，迁移方法" } },
  { id: "D7", name: "论证质量", framework: "QUEST-S · ATL 沟通（输出）",
    anchors: { L1: "只堆观点 / 复制 AI 原话，无论点-论据结构", L2: "有结论但论据零散，结构不完整", L3: "论点-论据-解释结构完整，引用有出处", L4: "结构严谨且回应反方，论证链条经得起追问" } },
  { id: "D8", name: "信息再生产", framework: "ATL 媒介伦理 · 学术诚信（输出）",
    anchors: { L1: "整段照搬 AI 输出，不标注、不改写", L2: "偶尔改写，但分不清哪些是 AI、哪些是自己的", L3: "明确区分 AI 贡献与个人加工，主动声明 AI 使用", L4: "在 AI 基础上有独立判断与增量，诚信声明清晰可核" } },
  { id: "D9", name: "AI 边界与伦理", framework: "TOK 知识与技术 · 伦理使用（输出）",
    anchors: { L1: "把 AI 当全知，不质疑其可能出错或编造", L2: "知道 AI 会错，但不主动核查", L3: "主动核查 AI 可能幻觉处，识别其知识边界", L4: "系统性评估 AI 局限与伦理风险，按场景决定是否/如何用" } },
  { id: "D10", name: "协作编排", framework: "意图与编排 · 跨轮驱动与贡献",
    anchors: { L1: "把 AI 当答案机器：直接要成品，不带入自己的材料，不追问不调整", L2: "被 AI 追问后才补充自己的材料，不主动规划协作步骤", L3: "未经提示就带入自己的草稿/链接/提纲，并跨轮驱动改进", L4: "跨步骤编排 AI 角色、管理上下文、沉淀可复用结构" } },
];

// CT_RUBRIC: the Critical Thinking rubric used by the platform.
// OPCVL (HS-D1…D12) is a separate rubric, deferred to its module (assessment §10 Q3).
export const CT_RUBRIC: Rubric = {
  id: "ct",
  name: "AI 批判思维（9+1 维）",
  dimensions: FULL_RUBRIC,
};

// assertRubricComplete: validates that a rubric has all required anchors (no blanks).
export function assertRubricComplete(r: Rubric): void {
  for (const d of r.dimensions) {
    for (const lvl of ["L1", "L2", "L3", "L4"] as const) {
      if (!d.anchors[lvl] || d.anchors[lvl].trim() === "") {
        throw new Error(`rubric ${r.id} dim ${d.id} missing ${lvl} anchor`);
      }
    }
  }
}
