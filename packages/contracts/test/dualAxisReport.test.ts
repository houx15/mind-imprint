import { describe, it, expect } from "vitest";
import { DualAxisReport } from "../src/dualAxisReport";

const sample = {
  depthAxis: {
    dims: [
      { code: "D1", name: "任务理解与问题表述", score: 3, evidence: "把绝对命题改成有限定判断", promptEvidence: "R4「我想把 thesis 改成…」" },
      { code: "D3", name: "证据与信源意识", score: 2, evidence: "溯到 NASA/Nature", promptEvidence: "" },
      { code: "D4", name: "论证结构意识", score: 3, evidence: "识别缺失 warrant", promptEvidence: "" },
      { code: "D5", name: "反馈理解与修改理由", score: 3, evidence: "说明采纳与拒绝理由", promptEvidence: "" },
    ],
    subtotal: 11,
  },
  autonomyAxis: { code: "D2", name: "学生主体性 / AI 依赖度", observation: "入场即设边界", anchoredSignals: ["R1 不要直接重写"], promptedSignals: ["R3 SIFT"], adversaryInvites: 0, promptEvidence: "" },
  crossAxis: { code: "D6", name: "元认知与反思", depthLevel: "L3", initiative: "引导后", prose: "能反思，尚未自发反思", promptEvidence: "" },
  solo: [{ round: 4, excerpt: "限定判断", level: "L3", rationale: "范围限定作为概念性组织者", initiative: "自发" }],
  promptLens: {
    directiveRounds: 3, totalRounds: 10, boundarySettings: 3, adversaryInvites: 0,
    questions: [{ title: "一问 · 任务说清了吗", body: "…" }],
    bestPrompt: { round: 8, quote: "请检查是否回扣 thesis，不要帮我润色", annotation: "材料✓ 任务✓ 边界✓ 验收✓" },
    takeaway: { round: 0, quote: "请扮演一个苛刻的审稿人…", annotation: "P4 模板" },
    perRound: [{ round: 1, tier: "P3", label: "要过程·设边界" }],
  },
  timeline: [{ round: 1, task: "上传草稿", prompt: "不要直接重写", pTag: "P3", dimTags: ["D1=2", "D2=3"] }],
  keyEvidence: [{ label: "任务理解", quote: "我想把 thesis 改成…" }],
  guidance: { anchored: "主动限定 thesis", prompted: "SIFT 核查", risk: "D3 仍停留在来源等级", nextSteps: [{ title: "下一步强化 D3", body: "跑一张 SIFT 记录" }] },
  narrative: "深度侧 L3 结构稳定复现，自主侧未主动召唤对手。",
  axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  generatedAt: "2026-07-19T00:00:00Z",
};

describe("DualAxisReport", () => {
  it("parses a valid axis-structured report", () => {
    const r = DualAxisReport.parse(sample);
    expect(r.depthAxis.subtotal).toBe(11);
    expect(r.autonomyAxis.adversaryInvites).toBe(0);
    expect(r.crossAxis.depthLevel).toBe("L3");
  });

  it("rejects an unknown prompt tier", () => {
    const bad = { ...sample, promptLens: { ...sample.promptLens, perRound: [{ round: 1, tier: "P9", label: "x" }] } };
    expect(() => DualAxisReport.parse(bad)).toThrow();
  });

  it("rejects an autonomy axis carrying a score field", () => {
    const bad = { ...sample, autonomyAxis: { ...sample.autonomyAxis, score: 3 } };
    // strict schema: extra keys rejected — autonomy must never carry a score
    expect(() => DualAxisReport.parse(bad)).toThrow();
  });
});
