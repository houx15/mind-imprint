import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

/**
 * Task 6 (start gate): a brand-new project shows a pure full-width 印记 chat
 * with NO tabs at all — 印记's opening turn ends 准备好开始了吗 with a real
 * 开始 button in place of the composer. Clicking 开始 flips
 * `studio_state.started` → true (via `coach/start`), which reveals the
 * interactive area + the 5-segment switcher and opens the first room.
 *
 * An already-started project (the normal case every other WorkspaceContainer
 * test exercises) must be unaffected: tabs render immediately, no 开始
 * button, and the one-shot `coach/opening` call never fires (the thread
 * already has turns — firing it again would be a wasted/duplicate spend on
 * a real LLM call).
 *
 * Mirrors WorkspaceContainer.test.tsx's mocking shape (rooms + API module),
 * kept in its own file since it exercises a materially different seam (the
 * gate itself) from that file's resume-at-stage / morphing-width coverage.
 */

vi.mock("@/workspace/Directory", () => ({
  Directory: () => <div data-testid="directory" />,
}));

vi.mock("@/workspace/blocks/PlanBlock", () => ({
  PlanBlock: ({ projectId }: { projectId: string }) => <div data-testid="plan-block">{projectId}</div>,
}));
vi.mock("@/workspace/blocks/ReadingBlock", () => ({ ReadingBlock: () => <div data-testid="reading-block" /> }));
vi.mock("@/workspace/blocks/WritingBlock", () => ({ WritingBlock: () => <div data-testid="writing-block" /> }));
vi.mock("@/workspace/blocks/ReferencePanel", () => ({ ReferencePanel: () => <div data-testid="writing-ref-panel" /> }));
vi.mock("@/workspace/blocks/ReviewBlock", () => ({ ReviewBlock: () => <div data-testid="review-block" /> }));
vi.mock("@/studio/reading/ReadingRoom", () => ({ ReadingRoom: () => <div data-testid="reading-room" /> }));

const getWorkspace = vi.fn();
const getStudioState = vi.fn();
const getCoachHistory = vi.fn();
const coachOpening = vi.fn();
const coachStart = vi.fn();
const getPlan = vi.fn();
vi.mock("@/workspace/api/workspace", () => ({
  getWorkspace: (...args: unknown[]) => getWorkspace(...args),
  getStudioState: (...args: unknown[]) => getStudioState(...args),
  getCoachHistory: (...args: unknown[]) => getCoachHistory(...args),
  coach: vi.fn(),
  coachOpening: (...args: unknown[]) => coachOpening(...args),
  coachStart: (...args: unknown[]) => coachStart(...args),
  putProposal: vi.fn(async (_id: string, p: unknown) => p),
  getPlan: (...args: unknown[]) => getPlan(...args),
  postProjectSummary: vi.fn(async () => ""),
  patchReference: vi.fn(async () => ({})),
  reflectProjectCard: vi.fn(async () => ({ cardInstanceId: "", reply: "", card: null })),
  dismissProposal: vi.fn(async () => {}),
}));

vi.mock("@/api/exploration", () => ({ createLead: vi.fn() }));

import { WorkspaceContainer } from "@/workspace/WorkspaceContainer";

function fakeWorkspace(id: string) {
  return {
    id,
    title: `项目 ${id}`,
    qualification: "拓展论文 EE",
    status: "working" as const,
    proposal: { objective: "", reason: "", activities: "", resources: "" },
    createdAt: "2026-08-01T00:00:00Z",
    writingFinished: false,
  };
}

const OPENING_NARRATE =
  "先聊聊：你想做什么、为什么想做——想清楚这些，项目会更扎实。准备好开始了吗？";

function notStartedState() {
  return {
    stage: "topic_discussion" as const,
    openTool: "chat" as const,
    widthTier: "chat" as const,
    reference: [] as never[],
    updatedAtTurn: 0,
    started: false,
  };
}

function startedState() {
  return {
    stage: "proposal_forming" as const,
    openTool: "forming" as const,
    widthTier: "half" as const,
    reference: [] as never[],
    updatedAtTurn: 3,
    started: true,
  };
}

const BLOCK_LABELS = ["提案", "管理", "阅读", "写作", "回顾"];

beforeEach(() => {
  vi.clearAllMocks();
  getWorkspace.mockImplementation(async (id: string) => fakeWorkspace(id));
  getPlan.mockResolvedValue([]);
});

describe("Studio start gate (Task 6)", () => {
  it("a brand-new project (started:false, empty thread) shows NO tabs and a real 开始 button; clicking it reveals the switcher", async () => {
    getStudioState.mockResolvedValue(notStartedState());
    getCoachHistory.mockResolvedValue({ messages: [], hasMore: false, recap: null, nextCursor: null });
    coachOpening.mockResolvedValue({
      narrate: OPENING_NARRATE,
      directive: notStartedState(),
      note: null,
      card: null,
      question: null,
      reviewRequested: false,
      planGenerated: false,
      compacted: false,
    });

    render(<WorkspaceContainer initialProjectId="pnew" />);

    // The opening fired once (empty thread + not started) and its narrate
    // landed as the sole AI turn.
    expect(await screen.findByText(OPENING_NARRATE)).toBeInTheDocument();
    expect(coachOpening).toHaveBeenCalledWith("pnew");
    expect(coachOpening).toHaveBeenCalledTimes(1);

    // No tabs at all — stronger than the old chat-only (which still rendered
    // an unhighlighted switcher row).
    for (const label of BLOCK_LABELS) {
      expect(screen.queryByRole("button", { name: label })).not.toBeInTheDocument();
    }
    // No room is mounted either.
    expect(screen.queryByTestId("plan-block")).not.toBeInTheDocument();

    // A real, visible 开始 button stands where the composer would be — not a
    // text composer (no textbox to type into pre-start).
    const startBtn = screen.getByRole("button", { name: "开始" });
    expect(startBtn).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/和印记说说你的项目/)).not.toBeInTheDocument();

    coachStart.mockResolvedValue({
      narrate: "好，那我们先想清楚题目——你对什么问题好奇？",
      directive: startedState(),
      note: null,
      card: null,
      question: null,
      reviewRequested: false,
      planGenerated: false,
      compacted: false,
    });

    await userEvent.click(startBtn);

    expect(coachStart).toHaveBeenCalledWith("pnew");
    // The switcher/tabs now render — the container re-rendered on the
    // directive's started:true + openTool:"forming".
    expect(await screen.findByRole("button", { name: "提案" })).toBeInTheDocument();
    for (const label of BLOCK_LABELS) {
      expect(screen.getByRole("button", { name: label })).toBeInTheDocument();
    }
    // 印记 opened the first room (forming ⇒ PlanBlock).
    expect(await screen.findByTestId("plan-block")).toBeInTheDocument();
    // The 开始 button is gone.
    expect(screen.queryByRole("button", { name: "开始" })).not.toBeInTheDocument();
  });

  it("an already-started project with a non-empty thread renders tabs immediately, no 开始 button, and never calls coach/opening", async () => {
    getStudioState.mockResolvedValue(startedState());
    getCoachHistory.mockResolvedValue({
      messages: [
        { role: "student", text: "我想研究可持续问题", card: null },
        { role: "ai", text: "好，我们先聊聊你的问题。", card: null },
      ],
      hasMore: false,
      recap: null,
      nextCursor: null,
    });

    render(<WorkspaceContainer initialProjectId="pexisting" />);

    // Tabs render right away.
    expect(await screen.findByRole("button", { name: "提案" })).toBeInTheDocument();
    for (const label of BLOCK_LABELS) {
      expect(screen.getByRole("button", { name: label })).toBeInTheDocument();
    }
    expect(await screen.findByTestId("plan-block")).toBeInTheDocument();

    // No 开始 button anywhere.
    expect(screen.queryByRole("button", { name: "开始" })).not.toBeInTheDocument();
    // The one-shot opening never fires for a resumed thread.
    expect(coachOpening).not.toHaveBeenCalled();
  });

  // The old hardcoded 提案 intro (INTRO_ZH, PlanBlock's former scripted
  // "要做好一个研究项目，先想清楚四件事…") is gone — 印记's live opening/start
  // narrate is what frames the project now. `PlanBlock` is mocked at the top
  // of this file (mirrors the sibling WorkspaceContainer.test.tsx), so the
  // meaningful, non-trivial version of this assertion — a REAL PlanBlock
  // never rendering INTRO_ZH — lives in PlanBlock.test.tsx (Task 6 update);
  // this integration-level check just confirms neither of THIS file's own
  // fixtures (the opening/start narrate copy above) happens to reintroduce it.
  it("never renders the old hardcoded 提案 intro copy anywhere in the gate flow", async () => {
    getStudioState.mockResolvedValue(notStartedState());
    getCoachHistory.mockResolvedValue({ messages: [], hasMore: false, recap: null, nextCursor: null });
    coachOpening.mockResolvedValue({
      narrate: OPENING_NARRATE,
      directive: notStartedState(),
      note: null,
      card: null,
      question: null,
      reviewRequested: false,
      planGenerated: false,
      compacted: false,
    });

    render(<WorkspaceContainer initialProjectId="pintro" />);

    await screen.findByText(OPENING_NARRATE);
    expect(screen.queryByText(/要做好一个研究项目，先想清楚四件事/)).not.toBeInTheDocument();
  });
});
