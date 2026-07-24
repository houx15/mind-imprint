import { describe, expect, it } from "vitest";
import { GrowthHistory } from "../src/growthHistory";

// Full DualAxisReport shape (report is .strict() — checked against
// src/dualAxisReport.ts). Reuses the canonical fixture shape from
// test/dualAxisReport.test.ts so all consumers agree on one representative report.
const report = {
  depthAxis: [
    { code: "D1", name: "任务理解与问题表述", level: "L3", evidence: "限定判断", promptEvidence: "R4" },
    { code: "D2", name: "证据与信源", level: "L3", evidence: "NASA", promptEvidence: "" },
    { code: "D3", name: "论证结构", level: "L3", evidence: "warrant", promptEvidence: "" },
    { code: "D4", name: "视角与偏见", level: "L3", evidence: "反方", promptEvidence: "" },
    { code: "D5", name: "反馈处理与修订", level: "L3", evidence: "理由", promptEvidence: "" },
    { code: "D6", name: "反思与元认知", level: "NA", evidence: "", promptEvidence: "" },
  ],
  autonomyAxis: [
    { code: "A1", name: "方向自主", level: 3, opportunity: "given_taken", evidence: "改题", promptEvidence: "" },
    { code: "A2", name: "发起自主", level: 3, opportunity: "given_taken", evidence: "自发补查", promptEvidence: "" },
    { code: "A3", name: "边界主权", level: 2, opportunity: "given_taken", evidence: "不要代写", promptEvidence: "" },
    { code: "A4", name: "对抗与检验", level: 0, opportunity: "not_supplied", evidence: "", promptEvidence: "" },
    { code: "A5", name: "判断署名", level: 3, opportunity: "given_taken", evidence: "自评档位", promptEvidence: "" },
    { code: "A6", name: "求真优先", level: 2, opportunity: "given_taken", evidence: "收窄结论", promptEvidence: "" },
  ],
  promptLens: {
    stats: [
      { label: "对话轮次", value: "10" },
      { label: "边界句", value: "3" },
      { label: "对手邀请", value: "0" },
    ],
    lenses: [
      { code: "L_decisions", name: "五个决定完整度", level: 3, evidence: "多数提示词含任务+材料+边界" },
      { code: "L_maturity", name: "提示成熟度", level: 3, evidence: "多要过程" },
      { code: "L_boundary", name: "边界意识", level: 3, evidence: "3 条边界句" },
      { code: "L_adversary", name: "对手邀请", level: 0, evidence: "0 次" },
      { code: "L_directive", name: "主动指令率", level: 3, evidence: "多轮主动改路线" },
      { code: "L_acceptance", name: "验收标准自给", level: 3, evidence: "给出可回扣 RQ 的验收标准" },
    ],
    note: "提示词透镜只读 AI 互动痕迹，为双轴补过程证据；不是第三根评分轴，不并入任何总分。",
  },
  interactionEvidence: [{ round: 4, student: "我想把 thesis 改成…", aiSummary: "帮你把绝对命题改成有限定判断", signal: "D1→L3" }],
  narrative: "深度侧 L3 结构稳定复现，自主侧多数事件为学生自发。",
  guidance: { nextSteps: [{ title: "下一步强化 D2", task: "跑一张 SIFT 记录，追一手出处" }] },
  axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  generatedAt: "2026-07-18T00:00:00Z",
};

describe("GrowthHistory", () => {
  it("parses a representative payload with a nullable sublabel", () => {
    const parsed = GrowthHistory.parse({
      entries: [
        {
          surface: "course",
          scopeId: "00000000-0000-0000-0000-0000000000c1",
          label: "信息素养",
          sublabel: "回看",
          createdAt: "2026-07-18T00:00:00Z",
          report: { ...report, generatedAt: "2026-07-18T00:00:00Z" },
        },
        {
          surface: "project",
          scopeId: "11111111-1111-1111-1111-111111111111",
          label: "中国可持续",
          sublabel: null,
          createdAt: "2026-07-17T00:00:00Z",
          report: { ...report, generatedAt: "2026-07-17T00:00:00Z" },
        },
      ],
    });
    expect(parsed.entries).toHaveLength(2);
    expect(parsed.entries[1]!.sublabel).toBeNull();
  });

  it("rejects an unknown surface", () => {
    expect(() =>
      GrowthHistory.parse({ entries: [{ surface: "email", scopeId: "x", label: "x", sublabel: null, createdAt: "x", report }] }),
    ).toThrow();
  });
});
