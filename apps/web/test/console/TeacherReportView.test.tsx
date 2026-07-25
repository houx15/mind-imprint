import { describe, it, expect, vi, beforeAll } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeacherReportView } from "@/console/TeacherReportView";
import type { TeacherReport } from "@/api";
import { api } from "@/api";
import { DualAxisReport as DualAxisReportSchema } from "@mind-imprint/contracts";
import type { DualAxisReport as DualAxisReportT } from "@mind-imprint/contracts";

// Canonical (non-project) 6-D + 6-A fixture, shared shape with the student
// report tests (apps/web/test/shell/report/DualAxisReport.test.tsx). One
// autonomy signal is not_supplied (platform-debt, must render distinctly).
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
  ],
  narrative: "深度侧 L3 结构稳定复现，自主侧在方向与发起上主动。",
  guidance: {
    nextSteps: [
      { title: "下一步强化 D3", task: "跑一张 SIFT 记录，练习识别反方证据。" },
    ],
  },
  axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  generatedAt: "2026-07-19T00:00:00Z",
};

const projectReport: DualAxisReportT = {
  ...coreReport,
  officialProjection: {
    standard: { id: "ap-research", name: "AP Research" },
    components: [
      { name: "Academic Paper", judgement: "Paper 3", reason: "topic focus 贯穿 method 与 line of reasoning。" },
    ],
    alignment: [
      { item: "Through-course inquiry", standard: "围绕自选 RQ 设计、实施并反思一个长期 inquiry。", performance: "已收窄 RQ，尚未走完反思环节。", impact: "反思环节未完成会拉低 POD Reflect 项。" },
    ],
    readiness: { score: 62, note: "只作作品就绪度参考，不与 D/A 双轴合成。" },
  },
  workAndProcess: {
    workSamples: [
      { title: "R4 收窄后的问题陈述", text: "碳治理政策收紧后，能源结构转型速度是否跟得上排放缺口。" },
    ],
    processMaterials: [
      { name: "SIFT 溯源记录", status: "完成", diagnosis: "已溯源到 NASA 与 Nature Sustainability 的一手数据。" },
    ],
  },
};

beforeAll(() => {
  DualAxisReportSchema.parse(coreReport);
  DualAxisReportSchema.parse(projectReport);
});

function makeClient(report: TeacherReport) {
  return { getStudentReport: vi.fn(async () => report) };
}

const projectData: TeacherReport = {
  report: projectReport,
  context: { projectTitle: "中国是否让地球变得更可持续？", researchQuestion: "中国的碳治理政策是否让地球更可持续？" },
};

const coreData: TeacherReport = {
  report: coreReport,
  context: {},
};

const DEPTH_NAMES = ["任务理解与问题表述", "证据与信源", "论证结构", "视角与偏见", "反馈处理与修订", "反思与元认知"];
const AUTONOMY_NAMES = ["方向自主", "发起自主", "边界主权", "对抗与检验", "判断署名", "求真优先"];

describe("TeacherReportView", () => {
  it("project report: renders breadcrumb, RQ, all 6 D + 6 A names, readiness score/note, alignment, work samples, next steps, and the evidence-map slot", async () => {
    const client = makeClient(projectData);
    const { container } = render(
      <TeacherReportView client={client} classId="c1" userId="u1" surface="project" scopeId="p1" onBack={() => {}} />,
    );

    expect(await screen.findByText("全部学生")).toBeInTheDocument();
    // The evidence map's RQ node now renders the same RQ text verbatim (short
    // RQs are not truncated per the design's conditional `short()` helper —
    // see EvidenceMap.tsx), so this now appears twice page-wide. Scope the
    // assertion to the 总览 section, which is where this specific assertion
    // cares about it living.
    expect(
      within(screen.getByTestId("teacher-report-overview")).getByText("中国的碳治理政策是否让地球更可持续？"),
    ).toBeInTheDocument();

    for (const name of DEPTH_NAMES) expect(screen.getByText(name)).toBeInTheDocument();
    for (const name of AUTONOMY_NAMES) expect(screen.getByText(name)).toBeInTheDocument();

    expect(screen.getByText("62")).toBeInTheDocument();
    expect(screen.getByText(/只作作品就绪度参考，不与 D\/A 双轴合成/)).toBeInTheDocument();

    expect(screen.getByText("Academic Paper")).toBeInTheDocument();
    expect(screen.getByText(/Through-course inquiry/)).toBeInTheDocument();

    expect(screen.getByText("R4 收窄后的问题陈述")).toBeInTheDocument();
    expect(screen.getByText(/碳治理政策收紧后/)).toBeInTheDocument();

    expect(screen.getByText("下一步强化 D3")).toBeInTheDocument();

    expect(container.querySelector('[data-testid="evidence-map-slot"]')).not.toBeNull();

    expect(client.getStudentReport).toHaveBeenCalledWith("c1", "u1", "project", "p1");
  });

  it("core-only report (no officialProjection/workAndProcess): omits 官方作品投影 and 作品与过程, keeps core sections", async () => {
    const client = makeClient(coreData);
    render(<TeacherReportView client={client} classId="c1" userId="u1" surface="chat" scopeId="ch1" onBack={() => {}} />);

    await screen.findAllByText("总览");

    // Absent from BOTH the catalog nav and the section heading — zero matches.
    expect(screen.queryByText("官方作品投影")).toBeNull();
    expect(screen.queryByText("作品与过程")).toBeNull();

    // These headings are intentionally echoed in the sticky catalog nav
    // (nav item + <h2>), so assert presence via getAllByText rather than
    // getByText (which throws on >1 match).
    expect(screen.getAllByText("D / A 双轴读数").length).toBeGreaterThan(0);
    expect(screen.getAllByText("证据地图").length).toBeGreaterThan(0);
    expect(screen.getAllByText("学生 · AI 交互证据").length).toBeGreaterThan(0);
    expect(screen.getAllByText("提示词透镜").length).toBeGreaterThan(0);
    expect(screen.getAllByText("下一步脚手架").length).toBeGreaterThan(0);
    for (const name of DEPTH_NAMES) expect(screen.getByText(name)).toBeInTheDocument();
  });

  it("renders a not_supplied autonomy signal as 暂无·机会未提供", async () => {
    render(<TeacherReportView client={makeClient(coreData)} classId="c1" userId="u1" surface="chat" scopeId="ch1" onBack={() => {}} />);
    expect(await screen.findByText("暂无·机会未提供")).toBeInTheDocument();
  });

  it("for a project surface, clicking 导出家长版 PDF opens the ParentReport overlay (surface:project, this view's scopeId)", async () => {
    const getParentReport = vi.spyOn(api, "getParentReport").mockResolvedValue({
      cover: { name: "Phoebe", subject: "", klass: "", typeLabel: "项目报告", dateStr: "", warmLine: "" },
      glance: "", dOverview: "", aOverview: "", dRows: [], aRows: [], opportunity: "", advice: [], prose: null,
    });
    render(<TeacherReportView client={makeClient(projectData)} classId="c1" userId="u1" surface="project" scopeId="p1" onBack={() => {}} />);
    const exportBtn = await screen.findByText("导出家长版 PDF");
    expect(exportBtn.closest("a")).toBeNull();
    await userEvent.click(exportBtn);
    expect(getParentReport).toHaveBeenCalledWith("c1", "u1", "project", "p1");
    expect(await screen.findByText("下载 PDF")).toBeInTheDocument();
    getParentReport.mockRestore();
  });

  it("for a non-project surface (chat), 导出家长版 PDF stays an inert placeholder (E1 serves surface:project only)", async () => {
    render(<TeacherReportView client={makeClient(coreData)} classId="c1" userId="u1" surface="chat" scopeId="ch1" onBack={() => {}} />);
    const exportBtn = await screen.findByText("导出家长版 PDF");
    expect(exportBtn.closest("a")).toBeNull();
    expect(exportBtn.closest("[title]")).toHaveAttribute("title", "家长版报告即将上线");
    await userEvent.click(exportBtn);
    expect(screen.queryByText("下载 PDF")).toBeNull();
  });

  it("calls onBack when the breadcrumb is clicked", async () => {
    const onBack = vi.fn();
    render(<TeacherReportView client={makeClient(coreData)} classId="c1" userId="u1" surface="chat" scopeId="ch1" onBack={onBack} />);
    await userEvent.click(await screen.findByText("全部学生"));
    expect(onBack).toHaveBeenCalled();
  });

  it("when studentName is supplied, renders it in the breadcrumb's second segment and as `{name} · {title}` in the H1", async () => {
    render(
      <TeacherReportView
        client={makeClient(projectData)}
        classId="c1"
        userId="u1"
        surface="project"
        scopeId="p1"
        studentName="Phoebe"
        onBack={() => {}}
      />,
    );
    expect(await screen.findByText("Phoebe")).toBeInTheDocument();
    expect(screen.getByText("Phoebe · 中国是否让地球变得更可持续？")).toBeInTheDocument();
  });

  it("without studentName, falls back to the project title for both the breadcrumb and the H1", async () => {
    render(<TeacherReportView client={makeClient(projectData)} classId="c1" userId="u1" surface="project" scopeId="p1" onBack={() => {}} />);
    expect(await screen.findAllByText("中国是否让地球变得更可持续？")).not.toHaveLength(0);
  });

  it("gates 研究概况 on a non-empty narrative", async () => {
    const client = makeClient({ ...coreData, report: { ...coreReport, narrative: "" } });
    render(<TeacherReportView client={client} classId="c1" userId="u1" surface="chat" scopeId="ch1" onBack={() => {}} />);
    await screen.findAllByText("总览");
    expect(screen.queryByText("研究概况")).toBeNull();
  });

  it("restores the section captions from the binding design", async () => {
    render(<TeacherReportView client={makeClient(projectData)} classId="c1" userId="u1" surface="project" scopeId="p1" onBack={() => {}} />);
    await screen.findAllByText("总览");
    expect(screen.getByText(/先按 AP Research 官方标准/)).toBeInTheDocument();
    expect(screen.getByText("D 轴看研究理解、证据和论证能走多深；A 轴看学生是否真正拥有这些研究判断。两者不合成总分。")).toBeInTheDocument();
    expect(screen.getByText("点击节点，看 RQ、官方投影、双轴、AI 互动如何串起这个项目的证据。")).toBeInTheDocument();
    expect(screen.getByText("学生输入、AI 回应摘要与可读出的评估信号，用来支撑 A 轴与提示词透镜判断。")).toBeInTheDocument();
    expect(screen.getByText("提示词透镜只读 AI 互动痕迹，为双轴补过程证据；它不是第三根评分轴。")).toBeInTheDocument();
    expect(screen.getByText("只保留能推进官方表现和双轴弱点的动作，不做泛泛润色。")).toBeInTheDocument();
  });
});
