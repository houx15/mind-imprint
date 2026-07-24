import { describe, it, expect } from "vitest";
import { DualAxisReport } from "../src/dualAxisReport";

const axiom = "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定";

const depthAxis = [
  { code: "D1", name: "任务理解与问题表述", level: "L3", evidence: "把绝对命题改成有限定判断", promptEvidence: "R4「我想把 thesis 改成…」" },
  { code: "D2", name: "证据与信源", level: "L3", evidence: "溯到 NASA/Nature", promptEvidence: "" },
  { code: "D3", name: "论证结构", level: "L3", evidence: "识别缺失 warrant", promptEvidence: "" },
  { code: "D4", name: "视角与偏见", level: "L3", evidence: "邀请反方", promptEvidence: "" },
  { code: "D5", name: "反馈处理与修订", level: "L3", evidence: "说明采纳与拒绝理由", promptEvidence: "" },
  { code: "D6", name: "反思与元认知", level: "L3", evidence: "原假设如何随证据改变", promptEvidence: "" },
];

const autonomyAxis = [
  { code: "A1", name: "方向自主", level: 3, opportunity: "given_taken", evidence: "改题收窄变量", promptEvidence: "" },
  { code: "A2", name: "发起自主", level: 3, opportunity: "given_taken", evidence: "自发补查", promptEvidence: "" },
  { code: "A3", name: "边界主权", level: 3, opportunity: "given_taken", evidence: "不要代写", promptEvidence: "" },
  { code: "A4", name: "对抗与检验", level: 3, opportunity: "given_taken", evidence: "邀请质疑 gap", promptEvidence: "" },
  { code: "A5", name: "判断署名", level: 3, opportunity: "given_taken", evidence: "自评档位", promptEvidence: "" },
  { code: "A6", name: "求真优先", level: 3, opportunity: "given_taken", evidence: "收窄结论", promptEvidence: "" },
];

const promptLens = {
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
};

const interactionEvidence = [
  { round: 4, student: "我想把 thesis 改成…", aiSummary: "帮你把绝对命题改成有限定判断", signal: "D1→L3" },
];

const guidance = {
  nextSteps: [{ title: "下一步强化 D2", task: "跑一张 SIFT 记录，追一手出处" }],
};

const officialProjection = {
  standard: { id: "ap-research", name: "AP Research" },
  components: [
    { name: "Academic Paper", judgement: "Paper 3", reason: "topic focus 贯穿方法与论证，但证据解释仍不充分" },
    { name: "POD", judgement: "18/24", reason: "Argument 与 Reflect 较强，Engage Audience 待加强" },
    { name: "训练用折算", judgement: "72/100", reason: "仅作作品就绪度参考，不与 D/A 双轴合成" },
    { name: "诚信/真实性", judgement: "高", reason: "PREP 记录完整，checkpoints 齐备" },
  ],
  alignment: [
    { item: "Through-course inquiry", standard: "围绕自选 RQ 设计并反思一个长期 inquiry", performance: "已完成收窄与初稿", impact: "支持 Paper 3 档位" },
  ],
  readiness: { score: 72, note: "仅作作品就绪度参考，不与 D/A 双轴合成" },
};

const workAndProcess = {
  workSamples: [{ title: "Thesis 修订", text: "从「中国让地球更可持续」改为限定判断版本" }],
  processMaterials: [{ name: "SIFT 记录", status: "完成", diagnosis: "溯到 NASA/Nature 一手来源" }],
};

const sample = {
  depthAxis,
  autonomyAxis,
  promptLens,
  interactionEvidence,
  narrative: "深度侧 L3 结构稳定复现，自主侧多数事件为学生自发。",
  guidance,
  axiom,
  generatedAt: "2026-07-24T00:00:00Z",
  officialProjection,
  workAndProcess,
};

describe("DualAxisReport", () => {
  it("parses a valid canonical sample with the optional project superset", () => {
    const r = DualAxisReport.parse(sample);
    expect(r.depthAxis).toHaveLength(6);
    expect(r.autonomyAxis).toHaveLength(6);
    expect(r.promptLens.stats).toHaveLength(3);
    expect(r.promptLens.lenses).toHaveLength(6);
    expect(r.officialProjection?.readiness.score).toBe(72);
    expect(r.workAndProcess?.workSamples).toHaveLength(1);
  });

  it("parses when the optional project superset (officialProjection/workAndProcess) is omitted", () => {
    const { officialProjection: _op, workAndProcess: _wp, ...rest } = sample;
    const r = DualAxisReport.parse(rest);
    expect(r.officialProjection).toBeUndefined();
    expect(r.workAndProcess).toBeUndefined();
  });

  it("rejects an out-of-range depth level", () => {
    const bad = { ...sample, depthAxis: [{ ...depthAxis[0], level: "L9" }, ...depthAxis.slice(1)] };
    expect(() => DualAxisReport.parse(bad)).toThrow();
  });

  it("rejects an out-of-range autonomy level", () => {
    const bad = { ...sample, autonomyAxis: [{ ...autonomyAxis[0], level: 6 }, ...autonomyAxis.slice(1)] };
    expect(() => DualAxisReport.parse(bad)).toThrow();
  });

  it("rejects an unknown autonomy opportunity", () => {
    const bad = { ...sample, autonomyAxis: [{ ...autonomyAxis[0], opportunity: "foo" }, ...autonomyAxis.slice(1)] };
    expect(() => DualAxisReport.parse(bad)).toThrow();
  });

  it("rejects an unknown top-level key (strict)", () => {
    const bad = { ...sample, subtotal: 11 };
    expect(() => DualAxisReport.parse(bad)).toThrow();
  });

  it("rejects an unknown key nested in a depth dim (strict)", () => {
    const bad = { ...sample, depthAxis: [{ ...depthAxis[0], score: 3 }, ...depthAxis.slice(1)] };
    expect(() => DualAxisReport.parse(bad)).toThrow();
  });
});
