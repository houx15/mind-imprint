import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ChatReport } from "@/shell/chat/ChatReport";
import { api } from "@/api";

vi.mock("@/api", () => ({ api: { getChatAssessment: vi.fn(), generateChatAssessment: vi.fn() } }));

// Real DTO shape (checked against packages/contracts/src/dualAxisReport.ts):
// a full DualAxisReport, reusing the exact fixture shape from Task 7's
// apps/web/test/shell/report/DualAxisReport.test.tsx so it parses .strict()
// and renders via the shared <DualAxisReport> component. No officialProjection/
// workAndProcess — chat is not a project surface.
const sample = {
  depthAxis: [
    { code: "D1", name: "问题意识", level: "L3", evidence: "你追问了三次", promptEvidence: "R4" },
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
  narrative: "你的问题越来越锋利。",
  guidance: { nextSteps: [{ title: "下一步强化 D3", task: "跑一张 SIFT 记录" }] },
  axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  generatedAt: "2026-07-18T00:00:00Z",
};

beforeEach(() => { vi.clearAllMocks(); });

describe("ChatReport", () => {
  it("shows the opt-in CTA and never auto-generates when no report exists", async () => {
    (api.getChatAssessment as any).mockResolvedValue(null);
    render(<ChatReport threadId="t1" onClose={() => {}} />);
    await screen.findByText("生成本次对话的思维印记");
    expect(api.generateChatAssessment).not.toHaveBeenCalled(); // 铁律 2: opt-in only
  });

  it("generates on click and renders the dimensions + narrative", async () => {
    (api.getChatAssessment as any).mockResolvedValue(null);
    (api.generateChatAssessment as any).mockResolvedValue(sample);
    render(<ChatReport threadId="t1" onClose={() => {}} />);
    fireEvent.click(await screen.findByText("生成本次对话的思维印记"));
    await waitFor(() => expect(screen.getByText("问题意识")).toBeInTheDocument());
    expect(screen.getByText("你的问题越来越锋利。")).toBeInTheDocument();
  });

  it("rehydrates a stored report on mount without generating", async () => {
    (api.getChatAssessment as any).mockResolvedValue(sample);
    render(<ChatReport threadId="t1" onClose={() => {}} />);
    await waitFor(() => expect(screen.getByText("问题意识")).toBeInTheDocument());
    expect(api.generateChatAssessment).not.toHaveBeenCalled();
  });

  // The chat fixture carries no officialProjection — that projection is a
  // project-surface-only superset (Task 7).
  it("never renders an official-projection section — a chat thread has no officialProjection", async () => {
    (api.getChatAssessment as any).mockResolvedValue(sample);
    render(<ChatReport threadId="t1" onClose={() => {}} />);
    await waitFor(() => expect(screen.getByText("问题意识")).toBeInTheDocument());
    expect(screen.queryByText("官方投影")).toBeNull();
  });
});
