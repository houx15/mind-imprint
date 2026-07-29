import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// S5 · the review room's AI-use retrospective panel + defense-readiness coach
// thread. Mock the thin api modules ReviewBlock calls directly.
vi.mock("@/workspace/api/workspace", () => ({
  getReflection: vi.fn(async () => ({ answers: [], done: false })),
  putReflection: vi.fn(async () => ({ answers: [], done: false })),
  getMirror: vi.fn(async () => null),
  postMirror: vi.fn(async () => null),
  getAIUseDraft: vi.fn(),
  postAIUse: vi.fn(async (_id: string, s: unknown) => s),
  coach: vi.fn(),
  getCoachHistory: vi.fn(async () => []),
}));
vi.mock("../../api/projects", () => ({ finishProject: vi.fn(async () => ({})) }));

import { getAIUseDraft, postAIUse, coach } from "@/workspace/api/workspace";
import { ReviewBlock } from "@/workspace/blocks/ReviewBlock";

const mockDraft = vi.mocked(getAIUseDraft);
const mockPostAIUse = vi.mocked(postAIUse);
const mockCoach = vi.mocked(coach);

const PROPOSAL = { objective: "论证中国是否让地球更可持续", reason: "关心气候", activities: "读 NASA/Nature", resources: "Zotero" };

beforeEach(() => {
  vi.clearAllMocks();
  mockDraft.mockResolvedValue({
    record: {
      coachTurns: 12,
      cardsProposed: 2,
      cardsAccepted: 1,
      cardsDismissed: 1,
      sourcesOpened: 5,
      llmCallsByPurpose: { coach: 12 },
      ghostwroteEssay: false,
      predictedScore: false,
    },
    draft: { usedFor: "溯源提问", notUsedFor: "代写正文" },
  });
  mockCoach.mockResolvedValue({ reply: "你先自己答——你的结论回答了原题吗？", proposal: null });
});

describe("ReviewBlock · AI-use retrospective (S5)", () => {
  it("renders the objective record + seeded statement and saves the student's edit", async () => {
    render(<ReviewBlock projectId="p1" proposal={PROPOSAL} />);

    // objective record line (the absences included)
    expect(await screen.findByText(/12 轮对话/)).toBeInTheDocument();
    expect(screen.getByText(/没有替你写正文/)).toBeInTheDocument();

    // seeded fields
    const used = screen.getByPlaceholderText(/澄清检索词/) as HTMLTextAreaElement;
    expect(used.value).toBe("溯源提问");

    // student edits + saves → postAIUse with the edited text
    fireEvent.change(used, { target: { value: "澄清检索词、核对来源功能" } });
    fireEvent.click(screen.getByRole("button", { name: "保存声明" }));
    await waitFor(() => {
      expect(mockPostAIUse).toHaveBeenCalledWith("p1", { usedFor: "澄清检索词、核对来源功能", notUsedFor: "代写正文" });
    });
  });

  it("defense-readiness thread sends a reflection-scope coach turn", async () => {
    render(<ReviewBlock projectId="p1" proposal={PROPOSAL} />);
    await screen.findByText(/12 轮对话/); // wait for mount loads

    await userEvent.type(screen.getByPlaceholderText(/想让印记追问哪一处/), "我的结论是不是太弱了");
    fireEvent.click(screen.getByRole("button", { name: "问印记" }));

    await waitFor(() => {
      expect(mockCoach).toHaveBeenCalledWith("p1", "reflection", "我的结论是不是太弱了");
    });
    expect(await screen.findByText(/你先自己答/)).toBeInTheDocument();
  });
});
