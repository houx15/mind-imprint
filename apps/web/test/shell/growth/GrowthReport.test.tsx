import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import type { DualAxisReport as DualAxisReportT } from "@mind-imprint/contracts";
import { GrowthReport } from "@/shell/growth/GrowthReport";
import { api } from "@/api";

// Minimal-but-schema-complete report fixture generator (same shape as the
// big fixture below, just parameterized on `narrative` so the deep-link test
// can tell two entries' expanded content apart without duplicating the
// entire six-dim/six-signal object twice).
function makeReport(narrative: string): DualAxisReportT {
  return {
    depthAxis: [
      { code: "D1", name: "任务理解与问题表述", level: "L3", evidence: "限定判断", promptEvidence: "" },
      { code: "D2", name: "证据与信源", level: "L2", evidence: "NASA", promptEvidence: "" },
      { code: "D3", name: "论证结构", level: "L3", evidence: "warrant", promptEvidence: "" },
      { code: "D4", name: "视角与偏见", level: "L2", evidence: "样本局限", promptEvidence: "" },
      { code: "D5", name: "反馈处理与修订", level: "L3", evidence: "理由", promptEvidence: "" },
      { code: "D6", name: "反思与元认知", level: "NA", evidence: "未见自写反思", promptEvidence: "" },
    ],
    autonomyAxis: [
      { code: "A1", name: "方向自主", level: 3, opportunity: "given_taken", evidence: "入场即设边界", promptEvidence: "" },
      { code: "A2", name: "发起自主", level: 2, opportunity: "given_taken", evidence: "主动补查", promptEvidence: "" },
      { code: "A3", name: "边界主权", level: 1, opportunity: "given_not_taken", evidence: "偶有边界句", promptEvidence: "" },
      { code: "A4", name: "对抗与检验", level: 0, opportunity: "not_supplied", evidence: "未出现对手邀请", promptEvidence: "" },
      { code: "A5", name: "判断署名", level: 2, opportunity: "given_taken", evidence: "自评了档位", promptEvidence: "" },
      { code: "A6", name: "求真优先", level: 1, opportunity: "given_not_taken", evidence: "未主动收窄结论", promptEvidence: "" },
    ],
    promptLens: {
      stats: [
        { label: "主动指令轮", value: "3 / 10" },
        { label: "边界设定", value: "3 次" },
        { label: "对手邀请", value: "0 次" },
      ],
      lenses: [
        { code: "L_decisions", name: "五个决定完整度", level: 2, evidence: "…" },
        { code: "L_maturity", name: "提示成熟度", level: 3, evidence: "…" },
        { code: "L_boundary", name: "边界意识", level: 2, evidence: "…" },
        { code: "L_adversary", name: "对手邀请", level: 0, evidence: "…" },
        { code: "L_directive", name: "主动指令率", level: 3, evidence: "…" },
        { code: "L_acceptance", name: "验收标准自给", level: 2, evidence: "…" },
      ],
      note: "提示词透镜只读 AI 互动痕迹，为双轴补过程证据；不是第三根评分轴，不并入任何总分。",
    },
    interactionEvidence: [{ round: 4, student: "检查是否回扣 thesis", aiSummary: "齐备", signal: "D1 主动限定" }],
    narrative,
    guidance: { nextSteps: [{ title: "下一步强化 D3", task: "跑一张 SIFT 记录" }] },
    axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
    generatedAt: "2026-07-18T00:00:00Z",
  };
}

describe("GrowthReport", () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it("shows the empty state with no reports", async () => {
    vi.spyOn(api, "getGrowthHistory").mockResolvedValue([]);
    render(<GrowthReport />);
    expect(await screen.findByText("还没有报告")).toBeTruthy();
    expect(screen.queryByText(/生成/)).toBeNull(); // no generate / 重新生成
  });

  it("renders a history row and expands it to the DualAxisReport", async () => {
    vi.spyOn(api, "getGrowthHistory").mockResolvedValue([
      { surface: "project", scopeId: "p1", label: "中国可持续", sublabel: null, createdAt: "2026-07-18T00:00:00Z",
        report: {
          depthAxis: [
            { code: "D1", name: "任务理解与问题表述", level: "L3", evidence: "限定判断", promptEvidence: "R4" },
            { code: "D2", name: "证据与信源", level: "L2", evidence: "NASA", promptEvidence: "" },
            { code: "D3", name: "论证结构", level: "L3", evidence: "warrant", promptEvidence: "" },
            { code: "D4", name: "视角与偏见", level: "L2", evidence: "样本局限", promptEvidence: "" },
            { code: "D5", name: "反馈处理与修订", level: "L3", evidence: "理由", promptEvidence: "" },
            { code: "D6", name: "反思与元认知", level: "NA", evidence: "未见自写反思", promptEvidence: "" },
          ],
          autonomyAxis: [
            { code: "A1", name: "方向自主", level: 3, opportunity: "given_taken", evidence: "入场即设边界", promptEvidence: "" },
            { code: "A2", name: "发起自主", level: 2, opportunity: "given_taken", evidence: "主动补查", promptEvidence: "" },
            { code: "A3", name: "边界主权", level: 1, opportunity: "given_not_taken", evidence: "偶有边界句", promptEvidence: "" },
            { code: "A4", name: "对抗与检验", level: 0, opportunity: "not_supplied", evidence: "未出现对手邀请", promptEvidence: "" },
            { code: "A5", name: "判断署名", level: 2, opportunity: "given_taken", evidence: "自评了档位", promptEvidence: "" },
            { code: "A6", name: "求真优先", level: 1, opportunity: "given_not_taken", evidence: "未主动收窄结论", promptEvidence: "" },
          ],
          promptLens: {
            stats: [
              { label: "主动指令轮", value: "3 / 10" },
              { label: "边界设定", value: "3 次" },
              { label: "对手邀请", value: "0 次" },
            ],
            lenses: [
              { code: "L_decisions", name: "五个决定完整度", level: 2, evidence: "…" },
              { code: "L_maturity", name: "提示成熟度", level: 3, evidence: "…" },
              { code: "L_boundary", name: "边界意识", level: 2, evidence: "…" },
              { code: "L_adversary", name: "对手邀请", level: 0, evidence: "…" },
              { code: "L_directive", name: "主动指令率", level: 3, evidence: "…" },
              { code: "L_acceptance", name: "验收标准自给", level: 2, evidence: "…" },
            ],
            note: "提示词透镜只读 AI 互动痕迹，为双轴补过程证据；不是第三根评分轴，不并入任何总分。",
          },
          interactionEvidence: [
            { round: 4, student: "检查是否回扣 thesis", aiSummary: "齐备", signal: "D1 主动限定" },
          ],
          narrative: "深度侧 L3 结构稳定复现，自主侧未主动召唤对手。",
          guidance: { nextSteps: [{ title: "下一步强化 D3", task: "跑一张 SIFT 记录" }] },
          axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
          generatedAt: "2026-07-18T00:00:00Z",
        } },
    ]);
    render(<GrowthReport />);
    expect(await screen.findByText("中国可持续")).toBeTruthy();
    // newest is auto-expanded → the DualAxisReport body is visible (axiom + a depth dim)
    expect(screen.getByText("任务理解与问题表述")).toBeTruthy();
    expect(screen.getByText(/两轴永不合成总分/)).toBeTruthy();
    expect(screen.queryByText(/生成/)).toBeNull();
  });

  it("expands the initialScopeId entry instead of the newest, when provided (deep-link from a project's 查看评估报告)", async () => {
    vi.spyOn(api, "getGrowthHistory").mockResolvedValue([
      { surface: "project", scopeId: "p-newest", label: "最新项目", sublabel: null, createdAt: "2026-07-20T00:00:00Z", report: makeReport("最新项目的叙述") },
      { surface: "project", scopeId: "p-target", label: "目标项目", sublabel: null, createdAt: "2026-07-01T00:00:00Z", report: makeReport("目标项目的叙述") },
    ]);
    render(<GrowthReport initialScopeId="p-target" />);
    await screen.findByText("目标项目");
    // the deep-linked entry (older, not newest) is the one auto-expanded
    expect(screen.getByText("目标项目的叙述")).toBeTruthy();
    expect(screen.queryByText("最新项目的叙述")).toBeNull();
  });
});

describe("GrowthReport (no internal tab strip)", () => {
  afterEach(() => { vi.restoreAllMocks(); });

  // 工具卡 (ToolkitCards) moved to the top-level 图鉴 gallery tab (Task 7) —
  // GrowthReport now renders only 学习记录 directly, with no internal
  // Tabs/Segmented control of its own (StudentApp's 我 hub already has the
  // 成长报告/设置 segmented control one level up).
  it("renders 学习记录 directly with no internal tab strip", async () => {
    vi.spyOn(api, "getGrowthHistory").mockResolvedValue([]);
    render(<GrowthReport />);
    await waitFor(() => expect(screen.getByText("还没有报告")).toBeTruthy());
    expect(screen.queryByRole("button", { name: /工具卡/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /学习记录/ })).toBeNull();
    expect(screen.queryByRole("tablist")).toBeNull();
  });

  it("accepts onOpenCourse without invoking it (kept for StudentApp call-site compatibility, unused now that ToolkitCards lives at the top level)", async () => {
    vi.spyOn(api, "getGrowthHistory").mockResolvedValue([]);
    const onOpenCourse = vi.fn();
    render(<GrowthReport onOpenCourse={onOpenCourse} />);
    await waitFor(() => expect(screen.getByText("还没有报告")).toBeTruthy());
    expect(onOpenCourse).not.toHaveBeenCalled();
  });
});
