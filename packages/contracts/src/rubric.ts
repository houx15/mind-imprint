import { z } from "zod";

export const RUBRIC_TAGS = [
  "D1_来源意识",
  "D2_交叉验证",
  "D5_论证结构",
  "D7_对立观点处理",
] as const;

export type RubricTag = (typeof RUBRIC_TAGS)[number];

export const SoloLevel = z.enum(["L1", "L2", "L3", "L4"]);
export type SoloLevel = z.infer<typeof SoloLevel>;
export const SOLO_LABELS: Record<SoloLevel, string> = { L1: "萌芽", L2: "发展中", L3: "熟练", L4: "卓越" };

export interface RubricDimension {
  id: string; name: string; framework: string;
  anchors: { L1: string; L2: string; L3: string; L4: string };
}

export const DEMO_RUBRIC: RubricDimension[] = [
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
];
