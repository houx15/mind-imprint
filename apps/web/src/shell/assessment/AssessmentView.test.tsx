import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MOCK_EVALUATION_REPORT } from "@/shell/report/EvaluationReport/__fixtures__/mock";
import { AssessmentView } from "./AssessmentView";

// jsdom doesn't implement IntersectionObserver; EvaluationReportView's Ruler
// guards its absence but stub it here too so the real observe()/disconnect()
// wiring exercises when a report actually renders (mirrors
// shell/report/EvaluationReport/index.test.tsx).
class IntersectionObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).IntersectionObserver = IntersectionObserverStub;

// 图鉴 (ToolkitCards) has its own dedicated suite
// (test/shell/growth/ToolkitCards.test.tsx) covering its card-catalog fetch
// — stub it here so this suite stays focused on AssessmentView's own wiring
// (the Segmented + the 成长报告 timeline + report page hand-off).
vi.mock("@/shell/growth/ToolkitCards", () => ({
  ToolkitCards: () => <div data-testid="gallery-cards" />,
}));

vi.mock("@/api/evaluationReport", () => ({
  listEvaluationReports: vi.fn(),
  getEvaluationReport: vi.fn(),
  generateEvaluationReport: vi.fn(),
}));

import { listEvaluationReports, getEvaluationReport, generateEvaluationReport } from "@/api/evaluationReport";

const ENTRY = {
  projectId: MOCK_EVALUATION_REPORT.projectId,
  title: MOCK_EVALUATION_REPORT.basics.title,
  type: MOCK_EVALUATION_REPORT.basics.type,
  createdAt: MOCK_EVALUATION_REPORT.generatedAt,
};

describe("AssessmentView", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows a Segmented with 成长报告 / 图鉴", async () => {
    vi.mocked(listEvaluationReports).mockResolvedValue([]);
    render(<AssessmentView />);
    expect(screen.getByText("成长报告")).toBeInTheDocument();
    expect(screen.getByText("图鉴")).toBeInTheDocument();
    await waitFor(() => expect(listEvaluationReports).toHaveBeenCalled());
  });

  it("lists a seeded report entry in the 成长报告 timeline", async () => {
    vi.mocked(listEvaluationReports).mockResolvedValue([ENTRY]);
    render(<AssessmentView />);
    await waitFor(() => expect(screen.getByText(ENTRY.title)).toBeInTheDocument());
  });

  it("switches to 图鉴 and renders the tool-card catalog", async () => {
    vi.mocked(listEvaluationReports).mockResolvedValue([]);
    render(<AssessmentView />);
    await userEvent.click(screen.getByText("图鉴"));
    expect(screen.getByTestId("gallery-cards")).toBeTruthy();
  });

  it("clicking a timeline row opens the report page (fixture title visible)", async () => {
    vi.mocked(listEvaluationReports).mockResolvedValue([ENTRY]);
    vi.mocked(getEvaluationReport).mockResolvedValue({ status: "ready", report: MOCK_EVALUATION_REPORT });
    render(<AssessmentView />);
    await waitFor(() => expect(screen.getByText(ENTRY.title)).toBeInTheDocument());
    await userEvent.click(screen.getByText(ENTRY.title));
    await waitFor(() => expect(screen.getByTestId("evaluation-report")).toBeInTheDocument());
    expect(generateEvaluationReport).not.toHaveBeenCalled();
  });

  it("deep-links directly into a report page when initialProjectId is set", async () => {
    vi.mocked(listEvaluationReports).mockResolvedValue([ENTRY]);
    vi.mocked(getEvaluationReport).mockResolvedValue({ status: "ready", report: MOCK_EVALUATION_REPORT });
    render(<AssessmentView initialProjectId={ENTRY.projectId} />);
    await waitFor(() => expect(screen.getByTestId("evaluation-report")).toBeInTheDocument());
    // listEvaluationReports still isn't needed to reach the deep-linked page.
    expect(getEvaluationReport).toHaveBeenCalledWith(ENTRY.projectId);
  });

  it("falls back to generateEvaluationReport when no report exists yet (first-open-wins)", async () => {
    vi.mocked(listEvaluationReports).mockResolvedValue([ENTRY]);
    vi.mocked(getEvaluationReport).mockResolvedValue(null);
    vi.mocked(generateEvaluationReport).mockResolvedValue({ status: "ready", report: MOCK_EVALUATION_REPORT });
    render(<AssessmentView />);
    await waitFor(() => expect(screen.getByText(ENTRY.title)).toBeInTheDocument());
    await userEvent.click(screen.getByText(ENTRY.title));
    await waitFor(() => expect(screen.getByTestId("evaluation-report")).toBeInTheDocument());
    expect(generateEvaluationReport).toHaveBeenCalledWith(ENTRY.projectId);
  });
});
