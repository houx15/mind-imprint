import { describe, it, expect, vi, beforeAll } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeacherReportView } from "@/console/TeacherReportView";
import type { TeacherReport } from "@/api";
import { api, ApiError } from "@/api";
import { DualAxisReport as DualAxisReportSchema } from "@mind-imprint/contracts";
import type { DualAxisReport as DualAxisReportT, EvaluationReport } from "@mind-imprint/contracts";
import type { EvalReportEnvelope } from "@/api";
import { MOCK_EVALUATION_REPORT } from "@/shell/report/EvaluationReport/__fixtures__/mock";

// Task 13: the teacher end now renders the SAME EvaluationReport the student
// sees (EvaluationReportView) as the main report body, fed by the new
// getStudentEvaluationReport client fn — this test file was rewritten to
// match (the old hand-rolled dual-axis body + scroll-spy catalog it used to
// exercise are gone). The auxiliary surfaces (EvidenceMap, ParentReport, the
// identity/context header) are still fed by the ORIGINAL getStudentReport
// fetch, so this fixture stays around for those assertions.
const coreReport: DualAxisReportT = {
  depthAxis: [
    { code: "D1", name: "任务理解与问题表述", level: "L3", evidence: "把研究问题收窄成对治理政策的具体追问。", promptEvidence: "" },
  ],
  autonomyAxis: [
    { code: "A1", name: "方向自主", level: 3, opportunity: "given_taken", evidence: "自己把方向收窄到治理政策。", promptEvidence: "" },
    { code: "A4", name: "对抗与检验", level: 0, opportunity: "not_supplied", evidence: "本次会话未出现对手邀请环节。", promptEvidence: "" },
  ],
  promptLens: {
    stats: [{ label: "主动指令轮", value: "3 / 10" }],
    lenses: [{ code: "L_decisions", name: "五个决定完整度", level: 2, evidence: "近几条提示词平均含 2 项决定。" }],
    note: "提示词透镜只读 AI 互动痕迹，为双轴补过程证据；不是第三根评分轴。",
  },
  interactionEvidence: [
    { round: 4, student: "我想把问题收窄到治理政策层面，行吗？", aiSummary: "帮她比较了两种收窄路径的可行性。", signal: "D1 主动收窄" },
  ],
  narrative: "深度侧 L3 结构稳定复现，自主侧在方向与发起上主动。",
  guidance: { nextSteps: [{ title: "下一步强化 D3", task: "跑一张 SIFT 记录，练习识别反方证据。" }] },
  axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  generatedAt: "2026-07-19T00:00:00Z",
};

const projectReport: DualAxisReportT = {
  ...coreReport,
  officialProjection: {
    standard: { id: "ap-research", name: "AP Research" },
    components: [{ name: "Academic Paper", judgement: "Paper 3", reason: "topic focus 贯穿 method 与 line of reasoning。" }],
    alignment: [],
    readiness: { score: 62, note: "只作作品就绪度参考，不与 D/A 双轴合成。" },
  },
  workAndProcess: {
    workSamples: [{ title: "R4 收窄后的问题陈述", text: "碳治理政策收紧后，能源结构转型速度是否跟得上排放缺口。" }],
    processMaterials: [{ name: "SIFT 溯源记录", status: "完成", diagnosis: "已溯源到 NASA 与 Nature Sustainability 的一手数据。" }],
  },
};

beforeAll(() => {
  DualAxisReportSchema.parse(coreReport);
  DualAxisReportSchema.parse(projectReport);
});

// Task 5 (frontend envelope, 2026-08-14): the client now returns the
// three-state envelope; this file's mocks wrap the plain `EvaluationReport`
// fixture as `{status:"ready", report}` (or `null` for "no report yet") to
// match, without changing what this view is exercised for.
function makeClient(report: TeacherReport, evalReport: EvaluationReport | null = MOCK_EVALUATION_REPORT) {
  return {
    getStudentReport: vi.fn(async () => report),
    getStudentEvaluationReport: vi.fn(async (): Promise<EvalReportEnvelope | null> =>
      evalReport ? { status: "ready" as const, report: evalReport } : null,
    ),
  };
}

const projectData: TeacherReport = {
  report: projectReport,
  context: { projectTitle: "中国是否让地球变得更可持续？", researchQuestion: "中国的碳治理政策是否让地球更可持续？" },
};

const coreData: TeacherReport = {
  report: coreReport,
  context: {},
};

describe("TeacherReportView", () => {
  it("project surface: fetches the new evaluation report and renders it as the main body, alongside the evidence-map slot", async () => {
    const client = makeClient(projectData);
    render(
      <TeacherReportView client={client} classId="c1" userId="u1" surface="project" scopeId="p1" onBack={() => {}} />,
    );

    expect(await screen.findByText("全部学生")).toBeInTheDocument();
    const evalBody = await screen.findByTestId("evaluation-report");
    expect(evalBody).toBeInTheDocument();
    // the shared report's own title (from its `basics.title`) renders inside it —
    // scoped to the report body since the teacher chrome's own H1 shows the
    // same project title text.
    expect(within(evalBody).getByText(MOCK_EVALUATION_REPORT.basics.title)).toBeInTheDocument();

    expect(screen.getByTestId("evidence-map-slot")).not.toBeNull();

    expect(client.getStudentReport).toHaveBeenCalledWith("c1", "u1", "project", "p1");
    expect(client.getStudentEvaluationReport).toHaveBeenCalledWith("c1", "u1", "p1");
  });

  it("fix-wave: when the retired getStudentReport 404s/errors but getStudentEvaluationReport succeeds, the new report still renders with no error banner", async () => {
    const client = {
      getStudentReport: vi.fn(async () => {
        throw new ApiError("not_found", "not found", 404);
      }),
      getStudentEvaluationReport: vi.fn(async (): Promise<EvalReportEnvelope | null> => ({ status: "ready", report: MOCK_EVALUATION_REPORT })),
    };
    render(
      <TeacherReportView client={client} classId="c1" userId="u1" surface="project" scopeId="p1" onBack={() => {}} />,
    );

    const evalBody = await screen.findByTestId("evaluation-report");
    expect(within(evalBody).getByText(MOCK_EVALUATION_REPORT.basics.title)).toBeInTheDocument();

    // No error banner from the retired fetch's failure — it degrades silently.
    expect(screen.queryByText("加载失败")).toBeNull();
    expect(screen.queryByText("重试")).toBeNull();

    // Old-shape auxiliary surfaces (证据地图, live 导出家长版 PDF) omit
    // themselves gracefully rather than crashing on the missing `data`.
    expect(screen.queryByTestId("evidence-map-slot")).toBeNull();
    expect(screen.queryByText("证据地图")).toBeNull();
    const exportBtn = await screen.findByText("导出家长版 PDF");
    expect(exportBtn.closest("[title]")).toHaveAttribute("title", "家长版报告即将上线");
  });

  it("project surface with no generated report yet: shows an empty-state placeholder instead of crashing", async () => {
    const client = makeClient(projectData, null);
    render(
      <TeacherReportView client={client} classId="c1" userId="u1" surface="project" scopeId="p1" onBack={() => {}} />,
    );
    expect(await screen.findByText("这个项目还没有生成过程评估报告。")).toBeInTheDocument();
    expect(screen.queryByTestId("evaluation-report")).toBeNull();
  });

  it("non-project surface (chat): does not call the project-scoped evaluation-report fetch, shows a placeholder body, but still renders the evidence map", async () => {
    const client = makeClient(coreData);
    render(<TeacherReportView client={client} classId="c1" userId="u1" surface="chat" scopeId="ch1" onBack={() => {}} />);

    expect(await screen.findByText("该类型报告的统一格式暂未上线，敬请期待。")).toBeInTheDocument();
    expect(screen.queryByTestId("evaluation-report")).toBeNull();
    expect(client.getStudentEvaluationReport).not.toHaveBeenCalled();
    expect(screen.getByTestId("evidence-map-slot")).not.toBeNull();
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
});
