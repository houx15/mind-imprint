import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { currentTask, ResearchPanel } from "@/workspace/blocks/exploration/ResearchPanel";
import type { EvidenceMapSubQuestion } from "@mind-imprint/contracts";

vi.mock("@/api/evidenceMap", () => ({
  getEvidenceMap: vi.fn(),
  reviewSubQuestion: vi.fn(),
  advanceEssayStage: vi.fn(async () => ({ stage: "statement", surface: "writing" })),
}));
import { getEvidenceMap, reviewSubQuestion, advanceEssayStage } from "@/api/evidenceMap";

const sq = (id: string, text: string): EvidenceMapSubQuestion => ({ id, text, papers: [] });

describe("currentTask", () => {
  it("picks the first sub-question without a saturated verdict", () => {
    const list = [sq("a", "A"), sq("b", "B")];
    expect(currentTask(list, {})?.id).toBe("a");
    expect(currentTask(list, { a: { subQuestionId: "a", saturated: true, why: "", gaps: [] } })?.id).toBe("b");
  });
  it("falls back to the first when all saturated", () => {
    const list = [sq("a", "A")];
    expect(currentTask(list, { a: { subQuestionId: "a", saturated: true, why: "", gaps: [] } })?.id).toBe("a");
  });
});

describe("ResearchPanel", () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it("shows the current task, reviews a sub-question, and advances on saturated", async () => {
    vi.mocked(getEvidenceMap).mockResolvedValue({
      mainQuestion: "主问题",
      subQuestions: [{ id: "a", text: "子问题一", papers: [{ id: "p1", title: "P", nature: "support", triage: "", hasNote: true }] }],
    });
    vi.mocked(reviewSubQuestion).mockResolvedValue({ subQuestionId: "a", saturated: true, why: "支持与反例齐了", gaps: [] });
    const onAdvanced = vi.fn();

    render(<ResearchPanel projectId="p1" onAdvanced={onAdvanced} />);

    // the panel renders (the sub-question appears in both the task line + list)
    const reviewBtn = await screen.findByText("证据够了吗？");
    expect(screen.getAllByText("子问题一").length).toBeGreaterThan(0);

    // review → saturated verdict + 去写这条论点
    fireEvent.click(reviewBtn);
    await waitFor(() => expect(reviewSubQuestion).toHaveBeenCalledWith("p1", "a"));
    const writeBtn = await screen.findByText("去写这条论点");
    fireEvent.click(writeBtn);
    // deep-link: lands on THIS sub-question's claim step (sq id "a").
    await waitFor(() => expect(advanceEssayStage).toHaveBeenCalledWith("p1", "statement", "a"));
    expect(onAdvanced).toHaveBeenCalled();
  });

  it("renders nothing when the map has no sub-questions", async () => {
    vi.mocked(getEvidenceMap).mockResolvedValue({ mainQuestion: "", subQuestions: [] });
    const { container } = render(<ResearchPanel projectId="p1" onAdvanced={vi.fn()} />);
    await waitFor(() => expect(getEvidenceMap).toHaveBeenCalled());
    expect(container.textContent).toBe("");
  });
});
