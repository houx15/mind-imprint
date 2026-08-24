import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const listEvaluationReports = vi.fn();
vi.mock("@/api/evaluationReport", () => ({
  listEvaluationReports: (...args: unknown[]) => listEvaluationReports(...args),
}));

const listProjects = vi.fn();
vi.mock("@/api", () => ({
  api: { listProjects: (...args: unknown[]) => listProjects(...args) },
}));

vi.mock("@/shell/report/EvaluationReportPage", () => ({
  EvaluationReportPage: ({ projectId, onBack }: { projectId: string; onBack: () => void }) => (
    <div data-testid="report-page">
      {projectId}
      <button type="button" onClick={onBack}>
        back
      </button>
    </div>
  ),
}));

import { ReportsView } from "@/shell/report/ReportsView";

const REAL = { projectId: "p1", title: "中国是否让地球更可持续", type: "拓展论文 EE", createdAt: new Date().toISOString() };
const DEMO = { projectId: "demo", title: "示例：知识与确定性", type: "TOK 论文", createdAt: new Date().toISOString(), isDemo: true };

describe("ReportsView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listProjects.mockResolvedValue([]);
  });

  it("renders a 示例 pill on the demo report entry only", async () => {
    listEvaluationReports.mockResolvedValue([DEMO, REAL]);

    render(<ReportsView />);
    await screen.findByText(REAL.title);

    expect(screen.getByText("示例")).toBeInTheDocument();
    expect(screen.getAllByText("示例")).toHaveLength(1);
  });

  it("sorts the demo report entry to the bottom even when the backend returns it first", async () => {
    listEvaluationReports.mockResolvedValue([DEMO, REAL]);

    const { container } = render(<ReportsView />);
    await screen.findByText(REAL.title);

    const titles = Array.from(container.querySelectorAll(".text-mk-h3")).map((el) => el.textContent);
    expect(titles).toEqual([REAL.title, DEMO.title]);
  });

  it("opens the read-only report directly on click — no guard modal for reports", async () => {
    listEvaluationReports.mockResolvedValue([REAL, DEMO]);

    render(<ReportsView />);
    const row = await screen.findByText(DEMO.title);
    await userEvent.click(row);

    expect(await screen.findByTestId("report-page")).toHaveTextContent("demo");
    expect(screen.queryByRole("heading", { name: "示例项目" })).not.toBeInTheDocument();
  });
});
