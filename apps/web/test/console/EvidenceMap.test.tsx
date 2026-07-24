import { describe, it, expect, beforeAll } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EvidenceMap } from "@/console/EvidenceMap";
import { DualAxisReport as DualAxisReportSchema } from "@mind-imprint/contracts";
import type { DualAxisReport as DualAxisReportT } from "@mind-imprint/contracts";

// Same fixture shape as TeacherReportView.test.tsx (Task 9/10) — the evidence
// map is a pure projection of this same canonical object, no new pipeline.
const coreReport: DualAxisReportT = {
  depthAxis: [
    { code: "D1", name: "任务理解与问题表述", level: "L3", evidence: "把研究问题收窄成对治理政策的具体追问。", promptEvidence: "" },
    { code: "D2", name: "证据与信源", level: "L2", evidence: "引用了 NASA 与 Nature Sustainability。", promptEvidence: "" },
    { code: "D3", name: "论证结构", level: "L3", evidence: "gap–method–evidence 链条基本稳定。", promptEvidence: "" },
    { code: "D4", name: "视角与偏见", level: "L2", levelRange: "L2–L3", evidence: "承认样本局限。", promptEvidence: "" },
    { code: "D5", name: "反馈处理与修订", level: "L3", evidence: "说明了采纳与拒绝审稿意见的理由。", promptEvidence: "" },
    { code: "D6", name: "反思与元认知", level: "NA", evidence: "本次会话未见亲手写的反思。", promptEvidence: "" },
  ],
  autonomyAxis: [
    { code: "A1", name: "方向自主", level: 3, opportunity: "given_taken", evidence: "自己把方向收窄到治理政策。", promptEvidence: "" },
    { code: "A2", name: "发起自主", level: 2, opportunity: "given_taken", evidence: "主动核查了一份未被要求核查的数据源。", promptEvidence: "" },
    { code: "A3", name: "边界主权", level: 1, opportunity: "given_not_taken", evidence: "只用了一次设边界的机会。", promptEvidence: "" },
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
    note: "提示词透镜只读 AI 互动痕迹，为双轴补过程证据；不是第三根评分轴。",
  },
  interactionEvidence: [
    { round: 4, student: "我想把问题收窄到治理政策层面，行吗？", aiSummary: "帮她比较了两种收窄路径的可行性。", signal: "D1 主动收窄" },
    { round: 6, student: "反方会怎么反驳这个数据？", aiSummary: "邀请她自己列反方证据。", signal: "A4 对抗检验萌芽" },
  ],
  narrative: "深度侧 L3 结构稳定复现，自主侧在方向与发起上主动。",
  guidance: {
    nextSteps: [{ title: "下一步强化 D3", task: "跑一张 SIFT 记录，练习识别反方证据。" }],
  },
  axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  generatedAt: "2026-07-19T00:00:00Z",
};

const projectReport: DualAxisReportT = {
  ...coreReport,
  officialProjection: {
    standard: { id: "ap-research", name: "AP Research" },
    components: [{ name: "Academic Paper", judgement: "Paper 3", reason: "topic focus 贯穿 method 与 line of reasoning。" }],
    alignment: [
      { item: "Through-course inquiry", standard: "围绕自选 RQ 设计、实施并反思一个长期 inquiry。", performance: "已收窄 RQ，尚未走完反思环节。", impact: "反思环节未完成会拉低 POD Reflect 项。" },
    ],
    readiness: { score: 62, note: "只作作品就绪度参考，不与 D/A 双轴合成。" },
  },
};

beforeAll(() => {
  DualAxisReportSchema.parse(coreReport);
  DualAxisReportSchema.parse(projectReport);
});

const projectContext = { projectTitle: "中国是否让地球变得更可持续？", researchQuestion: "中国的碳治理政策是否让地球更可持续？" };

describe("EvidenceMap", () => {
  it("project report: renders all 7 node labels; clicking the ai node shows a detail with the interaction count/signals", async () => {
    render(<EvidenceMap report={projectReport} context={projectContext} studentName="Phoebe" />);

    // Each node's canvas card carries a stable testid — assert its label
    // there (rather than screen.getByText globally, since the selected
    // node's label is echoed a second time in the detail panel below).
    expect(within(screen.getByTestId("evidence-map-node-center")).getByText("Phoebe · 能力报告")).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-rq")).getByText("Research Question")).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-official")).getByText("官方作品投影")).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-dAxis")).getByText("D 轴 · 认知深度")).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-aAxis")).getByText("A 轴 · 智识自主")).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-prompt")).getByText("提示词透镜")).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-ai")).getByText("AI 互动证据")).toBeInTheDocument();

    // default selection is `center` → detail panel shows the narrative
    const detail = screen.getByTestId("evidence-map-detail");
    expect(within(detail).getByText(projectReport.narrative)).toBeInTheDocument();

    await userEvent.click(screen.getByTestId("evidence-map-node-ai"));
    expect(within(detail).getByText(/D1 主动收窄/)).toBeInTheDocument();
    expect(within(detail).getByText(/A4 对抗检验萌芽/)).toBeInTheDocument();
    expect(screen.getByText("2 轮交互")).toBeInTheDocument(); // the ai node's own note
  });

  it("core-only report (no officialProjection): renders 6 nodes, 官方作品投影 absent", () => {
    render(<EvidenceMap report={coreReport} context={{}} />);

    expect(within(screen.getByTestId("evidence-map-node-center")).getByText("学生 · 能力报告")).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-rq")).getByText("Research Question")).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-dAxis")).getByText("D 轴 · 认知深度")).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-aAxis")).getByText("A 轴 · 智识自主")).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-prompt")).getByText("提示词透镜")).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-ai")).getByText("AI 互动证据")).toBeInTheDocument();

    expect(screen.queryByText("官方作品投影")).toBeNull();
    expect(screen.queryByTestId("evidence-map-node-official")).toBeNull();
  });

  it("defaults to the `center` node selected (narrative in the detail panel, not e.g. the RQ)", () => {
    render(<EvidenceMap report={projectReport} context={projectContext} />);
    const detail = screen.getByTestId("evidence-map-detail");
    expect(within(detail).getByText(projectReport.narrative)).toBeInTheDocument();
    expect(within(detail).queryByText(projectContext.researchQuestion)).toBeNull();
  });

  it("aAxis node excludes the not_supplied (A4) signal from both the count and the mean", async () => {
    render(<EvidenceMap report={coreReport} context={{}} />);

    await userEvent.click(screen.getByTestId("evidence-map-node-aAxis"));

    // Fixture: A1=3, A2=2, A3=1, A4=0 (opportunity: not_supplied — excluded),
    // A5=2, A6=1. Mean computed independently of the component's own
    // arithmetic: only the 5 supplied signals count, A4's 0 must NOT drag it
    // down (5, not 6, in the denominator).
    const suppliedLevels = [3, 2, 1, 2, 1];
    const expectedMean = suppliedLevels.reduce((sum, l) => sum + l, 0) / suppliedLevels.length;
    expect(expectedMean).toBeCloseTo(1.8);

    const detail = screen.getByTestId("evidence-map-detail");
    expect(within(detail).getByText(`5 个已提供机会的 A 轴维度中，智识自主均值为 ${expectedMean.toFixed(1)}（满分 5）。`)).toBeInTheDocument();
    expect(within(screen.getByTestId("evidence-map-node-aAxis")).getByText(`自主均值 ${expectedMean.toFixed(1)}`)).toBeInTheDocument();
  });
});
