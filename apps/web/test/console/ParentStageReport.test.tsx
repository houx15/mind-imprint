import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ParentStageReport } from "@/console/ParentStageReport";
import { api } from "@/api";

const deterministic = {
  cover: { name: "林知远", subject: "第 30 周（7.20–7.26）", klass: "IBDP 一年级 · 研究组", typeLabel: "阶段报告", dateStr: "2026年7月25日", warmLine: "" },
  stats: [
    { value: "6 天", label: "本周活跃" }, { value: "78", label: "对话轮次" },
    { value: "3 份", label: "生成报告" }, { value: "5 节", label: "完成课程" },
  ],
  stageGrowth: "", stageHighlight: "", stageForward: "", advice: [], prose: null as null,
};
const composed = {
  ...deterministic,
  cover: { ...deterministic.cover, warmLine: "这一阶段，孩子表现突出。" },
  stageGrowth: "这段时间他更愿意先自己想。", stageHighlight: "本周主动请 AI 当反方。",
  stageForward: "给他更高的目标。", advice: [{ title: "请他讲给你听", text: "让他讲清楚。" }],
  prose: "present" as const,
};

describe("ParentStageReport", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders deterministic stats + 生成 button pre-prose", async () => {
    vi.spyOn(api, "getParentStageReport").mockResolvedValue(deterministic as never);
    render(<ParentStageReport classId="c1" studentId="s1" studentName="林知远" onClose={() => {}} />);
    expect(await screen.findByText("6 天")).toBeInTheDocument();
    expect(screen.getByText("本周活跃")).toBeInTheDocument();
    expect(screen.getByText("阶段报告")).toBeInTheDocument();
    expect(screen.getByText("生成家长版正文")).toBeInTheDocument();
    // No stage sections render before prose exists (敢于空白: nothing to show yet).
    expect(screen.queryByText("这段时间的变化")).not.toBeInTheDocument();
    expect(screen.queryByText("本阶段亮点")).not.toBeInTheDocument();
  });

  it("composes on click and fills the growth prose", async () => {
    vi.spyOn(api, "getParentStageReport").mockResolvedValue(deterministic as never);
    const gen = vi.spyOn(api, "generateParentStageProse").mockResolvedValue(composed as never);
    render(<ParentStageReport classId="c1" studentId="s1" onClose={() => {}} />);
    fireEvent.click(await screen.findByText("生成家长版正文"));
    await waitFor(() => expect(gen).toHaveBeenCalledWith("c1", "s1"));
    expect(await screen.findByText("这段时间他更愿意先自己想。")).toBeInTheDocument();
    expect(screen.getByText("本周主动请 AI 当反方。")).toBeInTheDocument();
    expect(screen.getByText(/给他更高的目标。/)).toBeInTheDocument();
    // No A-axis number anywhere in the stage report (RL-5).
    expect(screen.queryByText(/\b[0-5]\s*级/)).toBeNull();
  });
});
