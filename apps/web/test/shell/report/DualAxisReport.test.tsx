import { describe, it, expect, beforeAll } from "vitest";
import { render, screen } from "@testing-library/react";
import { DualAxisReport } from "@/shell/report/DualAxisReport";
import { DualAxisReport as DualAxisReportSchema } from "@mind-imprint/contracts";
import type { DualAxisReport as DualAxisReportT } from "@mind-imprint/contracts";

// Canonical (non-project) fixture — the shape every surface (growth/chat/
// course) shares. Six depth dims (D1–D6, one NA + one with levelRange), six
// autonomy signals (A1–A6, one not_supplied), 3 prompt-lens stats + 6 lenses,
// interactionEvidence, guidance.nextSteps, axiom, generatedAt. NO
// officialProjection/workAndProcess — those are project-surface-only.
const report: DualAxisReportT = {
  depthAxis: [
    { code: "D1", name: "任务理解与问题表述", level: "L3", evidence: "把「中国是否让地球更可持续」收窄成对碳排放与治理政策的具体追问。", promptEvidence: "R4：我想把问题收窄到治理政策层面，行吗？" },
    { code: "D2", name: "证据与信源", level: "L2", evidence: "引用了 NASA 与 Nature Sustainability，但未比较两者立场差异。", promptEvidence: "" },
    { code: "D3", name: "论证结构", level: "L3", evidence: "gap–method–evidence 链条基本稳定，结论限定在治理层面。", promptEvidence: "" },
    { code: "D4", name: "视角与偏见", level: "L2", levelRange: "L2–L3", evidence: "承认样本局限，尚未系统邀请反方证据。", promptEvidence: "" },
    { code: "D5", name: "反馈处理与修订", level: "L3", evidence: "分别说明了采纳与拒绝审稿意见的理由。", promptEvidence: "" },
    { code: "D6", name: "反思与元认知", level: "NA", evidence: "本次会话未见学生亲手写的反思。", promptEvidence: "" },
  ],
  autonomyAxis: [
    { code: "A1", name: "方向自主", level: 3, opportunity: "given_taken", evidence: "自己把方向从「地球是否可持续」收窄到治理政策。", promptEvidence: "" },
    { code: "A2", name: "发起自主", level: 2, opportunity: "given_taken", evidence: "主动去核查了一份未被要求核查的数据源。", promptEvidence: "" },
    { code: "A3", name: "边界主权", level: 1, opportunity: "given_not_taken", evidence: "平台提供了设边界的机会，学生只用了一次。", promptEvidence: "" },
    { code: "A4", name: "对抗与检验", level: 0, opportunity: "not_supplied", evidence: "本次会话未出现对手邀请环节。", promptEvidence: "" },
    { code: "A5", name: "判断署名", level: 2, opportunity: "given_taken", evidence: "为自己的结论范围做了解释。", promptEvidence: "" },
    { code: "A6", name: "求真优先", level: 1, opportunity: "given_not_taken", evidence: "面对不利证据未收窄结论。", promptEvidence: "" },
  ],
  promptLens: {
    stats: [
      { label: "主动指令轮", value: "3 / 10" },
      { label: "边界设定", value: "3 次" },
      { label: "对手邀请", value: "0 次" },
    ],
    lenses: [
      { code: "L_decisions", name: "五个决定完整度", level: 2, evidence: "近几条提示词平均含 2 项决定。" },
      { code: "L_maturity", name: "提示成熟度", level: 3, evidence: "多数提示要过程而非直接要成品。" },
      { code: "L_boundary", name: "边界意识", level: 2, evidence: "出现过「不要替我下结论」。" },
      { code: "L_adversary", name: "对手邀请", level: 0, evidence: "未出现反方审稿请求。" },
      { code: "L_directive", name: "主动指令率", level: 3, evidence: "约三成轮次由学生主动变更任务。" },
      { code: "L_acceptance", name: "验收标准自给", level: 2, evidence: "给出过可回到研究问题的验收标准。" },
    ],
    note: "提示词透镜只读 AI 互动痕迹，为双轴补过程证据；不是第三根评分轴，不并入任何总分。",
  },
  interactionEvidence: [
    { round: 4, student: "我想把问题收窄到治理政策层面，行吗？", aiSummary: "帮她比较了两种收窄路径的可行性，未替她下结论。", signal: "D1 主动收窄" },
    { round: 8, student: "检查一下这段是否回扣了我的 thesis。", aiSummary: "指出了一处结论与证据不匹配。", signal: "D5 主动求反馈" },
  ],
  narrative: "深度侧 L3 结构稳定复现，自主侧在方向与发起上主动，边界与对抗仍待发展。",
  guidance: {
    nextSteps: [
      { title: "下一步强化 D3", task: "跑一张 SIFT 记录，练习识别反方证据。" },
      { title: "多用一次边界句", task: "下次对话主动说明「不要替我下结论」。" },
    ],
  },
  axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  generatedAt: "2026-07-19T00:00:00Z",
};

// Project-surface superset fixture — adds officialProjection + workAndProcess.
const projectReport: DualAxisReportT = {
  ...report,
  officialProjection: {
    standard: { id: "ap-research", name: "AP Research" },
    components: [
      { name: "Academic Paper", judgement: "Paper 3", reason: "topic focus 贯穿 method 与 line of reasoning，但证据解释仍不充分。" },
    ],
    alignment: [
      { item: "Through-course inquiry", standard: "围绕自选 RQ 设计、实施并反思一个长期 inquiry。", performance: "已收窄 RQ，尚未走完反思环节。", impact: "反思环节未完成会拉低 POD Reflect 项。" },
    ],
    readiness: { score: 62, note: "只作作品就绪度参考，不与 D/A 双轴合成。" },
  },
  workAndProcess: {
    workSamples: [
      { title: "R4 收窄后的问题陈述", text: "中国的碳治理政策是否让地球更可持续？" },
    ],
    processMaterials: [
      { name: "SIFT 溯源记录", status: "完成", diagnosis: "已溯源到 NASA 与 Nature Sustainability 的一手数据。" },
    ],
  },
};

beforeAll(() => {
  // Three-shape alignment: the fixtures used to drive this test must
  // themselves parse .strict() against the Zod contract.
  DualAxisReportSchema.parse(report);
  DualAxisReportSchema.parse(projectReport);
});

describe("DualAxisReport", () => {
  it("renders the axiom verbatim", () => {
    render(<DualAxisReport report={report} />);
    expect(screen.getByText(/两轴永不合成总分/)).toBeTruthy();
  });

  it("renders the narrative", () => {
    render(<DualAxisReport report={report} />);
    expect(screen.getByText(/深度侧 L3 结构稳定复现/)).toBeTruthy();
  });

  it("renders six depth-axis level badges L1–L4/NA, with NO /12 subtotal anywhere", () => {
    const { container } = render(<DualAxisReport report={report} />);
    const d1 = screen.getByText("任务理解与问题表述").closest("article")!;
    expect(d1.textContent).toMatch(/L3/);
    expect(screen.getByText("暂无可计入的证据")).toBeTruthy(); // D6 NA
    expect(screen.getByText(/L2–L3/)).toBeTruthy(); // D4 levelRange
    expect(container.textContent).not.toMatch(/\/\s*12/);
  });

  it("shows depth promptEvidence only when non-empty", () => {
    render(<DualAxisReport report={report} />);
    expect(screen.getByText(/R4：我想把问题收窄到治理政策层面/)).toBeTruthy();
  });

  it("renders six autonomy-axis signals with level 0–5", () => {
    render(<DualAxisReport report={report} />);
    const nameEl = screen.getByText("方向自主");
    expect(nameEl).toBeTruthy();
    const card = nameEl.closest("article")!;
    expect(card.textContent).toMatch(/Lv\.?\s*3/);
  });

  it("shows not_supplied as 暂无·机会未提供 with no numeric level", () => {
    const { container } = render(<DualAxisReport report={report} />);
    expect(screen.getByText(/暂无·机会未提供/)).toBeTruthy();
    const card = screen.getByText("对抗与检验").closest("article") ?? container;
    expect(card.textContent).not.toMatch(/Lv\.?\s*0/);
  });

  it("renders 3 prompt-lens stat cards and 6 lens cards", () => {
    const { container } = render(<DualAxisReport report={report} />);
    expect(container.querySelectorAll('[data-testid="lens-stat-card"]').length).toBe(3);
    expect(container.querySelectorAll('[data-testid="lens-card"]').length).toBe(6);
    expect(screen.getByText(/提示词透镜只读 AI 互动痕迹/)).toBeTruthy();
  });

  it("renders interactionEvidence rows per round", () => {
    render(<DualAxisReport report={report} />);
    expect(screen.getByText(/检查一下这段是否回扣了我的 thesis/)).toBeTruthy();
    expect(screen.getByText(/D5 主动求反馈/)).toBeTruthy();
  });

  it("renders guidance.nextSteps", () => {
    render(<DualAxisReport report={report} />);
    expect(screen.getByText("下一步强化 D3")).toBeTruthy();
    expect(screen.getByText(/跑一张 SIFT 记录/)).toBeTruthy();
  });

  it("does NOT render an official-projection or work-and-process section when absent", () => {
    render(<DualAxisReport report={report} />);
    expect(screen.queryByText("官方投影")).toBeNull();
    expect(screen.queryByText("作品与过程")).toBeNull();
  });

  it("renders officialProjection readiness /100 with a note disclaiming D/A composition, when present", () => {
    render(<DualAxisReport report={projectReport} />);
    expect(screen.getByText(/62\s*\/\s*100/)).toBeTruthy();
    expect(screen.getByText(/不与 D\/A/)).toBeTruthy();
    expect(screen.getByText("Academic Paper")).toBeTruthy();
    expect(screen.getByText("Paper 3")).toBeTruthy();
    expect(screen.getByText(/Through-course inquiry/)).toBeTruthy();
  });

  it("renders workAndProcess work samples and process materials, when present", () => {
    render(<DualAxisReport report={projectReport} />);
    expect(screen.getByText(/R4 收窄后的问题陈述/)).toBeTruthy();
    expect(screen.getByText("SIFT 溯源记录")).toBeTruthy();
    expect(screen.getByText("完成")).toBeTruthy();
  });

  it("never renders a 证据地图 (evidence map) section — deferred to Spec D", () => {
    render(<DualAxisReport report={projectReport} />);
    expect(screen.queryByText(/证据地图/)).toBeNull();
  });
});
