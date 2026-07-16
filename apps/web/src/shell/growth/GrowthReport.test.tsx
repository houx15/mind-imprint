import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type { Assessment } from "@mind-imprint/contracts";

vi.mock("../../api", async (orig) => {
  const real = await orig<typeof import("../../api")>();
  return {
    ...real,
    api: {
      ...real.api,
      listProjects: vi.fn(),
      getAssessment: vi.fn(),
      generateAssessment: vi.fn(),
    },
  };
});

import { api } from "../../api";
import { GrowthReport } from "./GrowthReport";

const project = { id: "p1", title: "中国是否让地球变得更可持续？", qualLabel: "论证写作", activeStation: "writing" };

const assessment: Assessment = {
  dimensions: [
    { code: "D2", name: "信源辨识", level: "L4", evidence: "学生溯源到 NASA 与 Nature Sustainability 的原始数据，而非转述文章。" },
    { code: "D7", name: "让步与反驳", level: "NA", evidence: "" },
  ],
  narrative: "你在这次任务中主动溯源、并正面处理了反例，而不是回避它。",
  generatedAt: "2026-07-13T12:34:56Z",
};

describe("GrowthReport", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.listProjects as any).mockResolvedValue([project]);
  });

  it("renders a dimension row: name, level label (L4 → 卓越), and evidence", async () => {
    (api.getAssessment as any).mockResolvedValue(assessment);
    render(<GrowthReport />);

    expect(await screen.findByText("信源辨识")).toBeInTheDocument();
    expect(screen.getByText("卓越")).toBeInTheDocument();
    expect(screen.getByText(/溯源到 NASA/)).toBeInTheDocument();
  });

  it("renders an NA dimension muted, with no fake level label", async () => {
    (api.getAssessment as any).mockResolvedValue(assessment);
    render(<GrowthReport />);

    expect(await screen.findByText("让步与反驳")).toBeInTheDocument();
    expect(screen.getByText(/证据不足/)).toBeInTheDocument();
    // never fabricate a scored label for the NA dim
    expect(screen.queryByText("萌芽")).not.toBeInTheDocument();
  });

  it("renders the growth narrative", async () => {
    (api.getAssessment as any).mockResolvedValue(assessment);
    render(<GrowthReport />);

    expect(await screen.findByText(/你在这次任务中主动溯源/)).toBeInTheDocument();
    expect(screen.getAllByText(/你的思维印记/).length).toBeGreaterThan(0);
  });

  it("shows an honest empty state (no dimension rows) when there is no assessment yet", async () => {
    (api.getAssessment as any).mockResolvedValue(null);
    render(<GrowthReport />);

    expect(await screen.findByText(/还没有成长报告/)).toBeInTheDocument();
    expect(screen.getByText("生成成长报告")).toBeInTheDocument();
    expect(screen.queryByText("信源辨识")).not.toBeInTheDocument();
    expect(screen.queryByText("卓越")).not.toBeInTheDocument();
  });

  it("the generate button calls generateAssessment and renders the result", async () => {
    (api.getAssessment as any).mockResolvedValue(null);
    (api.generateAssessment as any).mockResolvedValue(assessment);
    render(<GrowthReport />);

    const btn = await screen.findByText("生成成长报告");
    fireEvent.click(btn);

    await waitFor(() => expect(api.generateAssessment).toHaveBeenCalledWith("p1"));
    expect(await screen.findByText("信源辨识")).toBeInTheDocument();
    expect(screen.getByText("卓越")).toBeInTheDocument();
  });
});
