import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

// Minimal mock setup mirrors WorkspaceContainer.test.tsx: mock out every room
// so this test stays focused on the shell's isDemo read-only treatment (the
// banner + the studio composer gating), not any one room's own data-fetching.
vi.mock("@/workspace/Directory", () => ({
  Directory: () => <div data-testid="directory" />,
}));
vi.mock("@/workspace/blocks/PlanBlock", () => ({ PlanBlock: () => <div data-testid="plan-block" /> }));
vi.mock("@/workspace/blocks/ReadingBlock", () => ({ ReadingBlock: () => <div data-testid="reading-block" /> }));
vi.mock("@/workspace/blocks/WritingBlock", () => ({ WritingBlock: () => <div data-testid="writing-block" /> }));
vi.mock("@/workspace/blocks/ReferencePanel", () => ({ ReferencePanel: () => <div data-testid="writing-ref-panel" /> }));
vi.mock("@/workspace/blocks/ReviewBlock", () => ({ ReviewBlock: () => <div data-testid="review-block" /> }));
vi.mock("@/studio/reading/ReadingRoom", () => ({ ReadingRoom: () => <div data-testid="reading-room" /> }));

const getWorkspace = vi.fn();
const getStudioState = vi.fn();
vi.mock("@/workspace/api/workspace", () => ({
  getWorkspace: (...args: unknown[]) => getWorkspace(...args),
  getStudioState: (...args: unknown[]) => getStudioState(...args),
  coach: vi.fn(),
  coachOpening: vi.fn(),
  coachStart: vi.fn(),
  putProposal: vi.fn(),
  getPlan: vi.fn(async () => []),
  getCoachHistory: vi.fn(async () => ({ messages: [], hasMore: false, recap: null, nextCursor: null })),
  postProjectSummary: vi.fn(async () => ""),
  patchReference: vi.fn(async () => ({})),
  reflectProjectCard: vi.fn(async () => ({ cardInstanceId: "", reply: "", card: null })),
  dismissProposal: vi.fn(async () => {}),
}));

vi.mock("@/api/exploration", () => ({
  createLead: vi.fn(),
}));

import { WorkspaceContainer } from "@/workspace/WorkspaceContainer";

function fakeWorkspace(id: string, isDemo: boolean) {
  return {
    id,
    title: `项目 ${id}`,
    qualification: "拓展论文 EE",
    status: "working" as const,
    proposal: { objective: "", reason: "", activities: "", resources: "" },
    createdAt: "2026-08-01T00:00:00Z",
    writingFinished: false,
    isDemo,
  };
}

// chat-first (openTool "chat") is the surface where the studio Composer
// (the send box) renders directly in WorkspaceContainer's own tree — the
// simplest, most reliable write control to assert gating on.
function fakeStudioState() {
  return {
    stage: "topic_discussion" as const,
    openTool: "chat" as const,
    widthTier: "chat" as const,
    reference: [] as never[],
    updatedAtTurn: 0,
    started: true,
  };
}

describe("WorkspaceContainer demo read-only treatment", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getStudioState.mockImplementation(async () => fakeStudioState());
  });

  it("shows a read-only banner and disables the studio composer when isDemo is true", async () => {
    getWorkspace.mockImplementation(async (id: string) => fakeWorkspace(id, true));
    render(<WorkspaceContainer initialProjectId="pdemo" />);

    expect(await screen.findByText(/演示项目 · 只读/)).toBeInTheDocument();

    const composer = await screen.findByPlaceholderText(/和印记说说你的项目/);
    expect(composer).toBeDisabled();
  });

  it("shows no banner and an enabled composer for a normal (non-demo) project", async () => {
    getWorkspace.mockImplementation(async (id: string) => fakeWorkspace(id, false));
    render(<WorkspaceContainer initialProjectId="preal" />);

    const composer = await screen.findByPlaceholderText(/和印记说说你的项目/);
    expect(composer).not.toBeDisabled();
    expect(screen.queryByText(/演示项目 · 只读/)).not.toBeInTheDocument();
  });
});
