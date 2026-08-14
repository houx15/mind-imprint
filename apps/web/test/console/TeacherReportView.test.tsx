import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeacherReportView } from "@/console/TeacherReportView";
import type { EvalReportEnvelope } from "@/api";
import { MOCK_EVALUATION_REPORT } from "@/shell/report/EvaluationReport/__fixtures__/mock";

// Task 6 (2026-08-14 retirement pass): the teacher end now renders ONLY the
// new EvaluationReport (EvaluationReportView), fed by
// getStudentEvaluationReport's three-state envelope. The retired
// getStudentReport fetch, the old DualAxisReport-shaped fixtures, and the
// EvidenceMap/ParentReport auxiliary surfaces this file used to exercise are
// gone along with the code they tested.

function makeClient(envelope: EvalReportEnvelope | null) {
  return {
    getStudentEvaluationReport: vi.fn(async (): Promise<EvalReportEnvelope | null> => envelope),
  };
}

describe("TeacherReportView", () => {
  it("project surface: fetches the evaluation report and renders it as the main body", async () => {
    const client = makeClient({ status: "ready", report: MOCK_EVALUATION_REPORT });
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

    expect(client.getStudentEvaluationReport).toHaveBeenCalledWith("c1", "u1", "p1");
  });

  it("project surface, envelope generating: shows a generating placeholder, not the report body", async () => {
    const client = makeClient({ status: "generating" });
    render(
      <TeacherReportView client={client} classId="c1" userId="u1" surface="project" scopeId="p1" onBack={() => {}} />,
    );
    expect(await screen.findByText("报告生成中……")).toBeInTheDocument();
    expect(screen.queryByTestId("evaluation-report")).toBeNull();
  });

  it("project surface, envelope failed: shows an error state with 重试 that retries the fetch", async () => {
    const client = {
      getStudentEvaluationReport: vi
        .fn<[], Promise<EvalReportEnvelope | null>>()
        .mockResolvedValueOnce({ status: "failed" })
        .mockResolvedValueOnce({ status: "ready", report: MOCK_EVALUATION_REPORT }),
    };
    render(
      <TeacherReportView client={client} classId="c1" userId="u1" surface="project" scopeId="p1" onBack={() => {}} />,
    );
    expect(await screen.findByText("重试")).toBeInTheDocument();
    await userEvent.click(screen.getByText("重试"));
    expect(await screen.findByTestId("evaluation-report")).toBeInTheDocument();
    expect(client.getStudentEvaluationReport).toHaveBeenCalledTimes(2);
  });

  it("project surface with no generated report yet (null envelope): shows an empty-state placeholder instead of crashing", async () => {
    const client = makeClient(null);
    render(
      <TeacherReportView client={client} classId="c1" userId="u1" surface="project" scopeId="p1" onBack={() => {}} />,
    );
    expect(await screen.findByText("这个项目还没有生成过程评估报告。")).toBeInTheDocument();
    expect(screen.queryByTestId("evaluation-report")).toBeNull();
  });

  it("non-project surface (chat): does not call the project-scoped evaluation-report fetch, shows a placeholder body", async () => {
    const client = makeClient({ status: "ready", report: MOCK_EVALUATION_REPORT });
    render(<TeacherReportView client={client} classId="c1" userId="u1" surface="chat" scopeId="ch1" onBack={() => {}} />);

    expect(await screen.findByText("该类型报告的统一格式暂未上线，敬请期待。")).toBeInTheDocument();
    expect(screen.queryByTestId("evaluation-report")).toBeNull();
    expect(client.getStudentEvaluationReport).not.toHaveBeenCalled();
  });

  it("calls onBack when the breadcrumb is clicked", async () => {
    const onBack = vi.fn();
    render(<TeacherReportView client={makeClient(null)} classId="c1" userId="u1" surface="chat" scopeId="ch1" onBack={onBack} />);
    await userEvent.click(await screen.findByText("全部学生"));
    expect(onBack).toHaveBeenCalled();
  });

  it("when studentName is supplied, renders it in the breadcrumb's second segment and as `{name} · {title}` in the H1", async () => {
    render(
      <TeacherReportView
        client={makeClient({ status: "ready", report: MOCK_EVALUATION_REPORT })}
        classId="c1"
        userId="u1"
        surface="project"
        scopeId="p1"
        studentName="Phoebe"
        onBack={() => {}}
      />,
    );
    expect(await screen.findByText("Phoebe")).toBeInTheDocument();
    expect(screen.getByText(`Phoebe · ${MOCK_EVALUATION_REPORT.basics.title}`)).toBeInTheDocument();
  });

  it("without studentName, falls back to the report title for both the breadcrumb and the H1", async () => {
    render(
      <TeacherReportView
        client={makeClient({ status: "ready", report: MOCK_EVALUATION_REPORT })}
        classId="c1"
        userId="u1"
        surface="project"
        scopeId="p1"
        onBack={() => {}}
      />,
    );
    expect(await screen.findAllByText(MOCK_EVALUATION_REPORT.basics.title)).not.toHaveLength(0);
  });
});
