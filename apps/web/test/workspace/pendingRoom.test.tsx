import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

// Light, seam-focused test of the `pendingRoom` deep-link (P3 Task 1) —
// mirrors the mock harness in WorkspaceContainer.test.tsx so the studio's
// data-fetching/room internals stay mocked out and this test only asserts
// the pendingRoom → handleManualRoom plumbing.
vi.mock("@/workspace/Directory", () => ({
  Directory: ({ onOpen }: { onOpen: (id: string) => void }) => (
    <div data-testid="directory">
      <button type="button" onClick={() => onOpen("clicked-id")}>
        open clicked-id
      </button>
    </div>
  ),
}));

vi.mock("@/workspace/blocks/PlanBlock", () => ({
  PlanBlock: ({ projectId, title }: { projectId: string; title: string }) => (
    <div data-testid="plan-block">
      {projectId}:{title}
    </div>
  ),
}));
vi.mock("@/workspace/blocks/ReadingBlock", () => ({ ReadingBlock: () => <div data-testid="reading-block" /> }));
vi.mock("@/workspace/blocks/WritingBlock", () => ({ WritingBlock: () => <div data-testid="writing-block" /> }));
vi.mock("@/workspace/blocks/ReferencePanel", () => ({ ReferencePanel: () => <div data-testid="writing-ref-panel" /> }));
vi.mock("@/workspace/blocks/ReviewBlock", () => ({ ReviewBlock: () => <div data-testid="review-block" /> }));
vi.mock("@/studio/reading/ReadingRoom", () => ({ ReadingRoom: () => <div data-testid="reading-room" /> }));

const getWorkspace = vi.fn();
const getStudioState = vi.fn();
const coach = vi.fn();
const putProposal = vi.fn();
const getPlan = vi.fn();
vi.mock("@/workspace/api/workspace", () => ({
  getWorkspace: (...args: unknown[]) => getWorkspace(...args),
  getStudioState: (...args: unknown[]) => getStudioState(...args),
  coach: (...args: unknown[]) => coach(...args),
  coachOpening: vi.fn(),
  coachStart: vi.fn(),
  putProposal: (...args: unknown[]) => putProposal(...args),
  getPlan: (...args: unknown[]) => getPlan(...args),
  getCoachHistory: vi.fn(async () => ({ messages: [], hasMore: false, recap: null, nextCursor: null })),
  postProjectSummary: vi.fn(async () => ""),
  patchReference: vi.fn(async () => ({})),
  reflectProjectCard: vi.fn(async () => ({ cardInstanceId: "", reply: "", card: null })),
  dismissProposal: vi.fn(async () => {}),
}));

const createLead = vi.fn();
vi.mock("@/api/exploration", () => ({
  createLead: (...args: unknown[]) => createLead(...args),
}));

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

function fakeStudioState() {
  return {
    stage: "plan_generation" as const,
    openTool: "plan" as const,
    widthTier: "half" as const,
    reference: [] as never[],
    updatedAtTurn: 0,
    started: true,
  };
}

describe("WorkspaceContainer · pendingRoom", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getWorkspace.mockImplementation(async (id: string) => fakeWorkspace(id));
    getStudioState.mockImplementation(async () => fakeStudioState());
    coach.mockResolvedValue({
      narrate: "好的。",
      directive: fakeStudioState(),
      note: null,
      card: null,
      question: null,
      reviewRequested: false,
      planGenerated: false,
      compacted: false,
    });
    putProposal.mockImplementation(async (_id: string, p: unknown) => p);
    getPlan.mockResolvedValue([]);
    createLead.mockResolvedValue({ id: "lead-1" });
  });

  // The tour opens the demo project in one step (onEnter: openDemoProject())
  // and drives the room in a LATER step (onEnter: setStudioRoom(...)) — so in
  // practice `pendingRoom` is always set well after the project has already
  // opened and settled, never on the very same render as `initialProjectId`.
  // This test mirrors that real timing with a rerender, rather than mounting
  // both at once (which races the project-open reset-on-load logic that
  // legitimately owns `room`/`tookOver` during the initial resume-at-stage).
  it("mounts the reading room when pendingRoom is set on an already-open project", async () => {
    const { rerender } = render(<WorkspaceContainer initialProjectId="p1" />);
    // 印记 resumes into the plan room by default (studioState.openTool "plan").
    await screen.findByTestId("plan-block");

    rerender(<WorkspaceContainer initialProjectId="p1" pendingRoom="reading" />);

    expect(await screen.findByTestId("reading-block")).toBeInTheDocument();
    expect(screen.queryByTestId("plan-block")).not.toBeInTheDocument();
  });

  it("calls onPendingRoomConsumed once the pendingRoom has been acted on", async () => {
    const onConsumed = vi.fn();
    const { rerender } = render(<WorkspaceContainer initialProjectId="p2" onPendingRoomConsumed={onConsumed} />);
    await screen.findByTestId("plan-block");

    rerender(<WorkspaceContainer initialProjectId="p2" pendingRoom="writing" onPendingRoomConsumed={onConsumed} />);

    await screen.findByTestId("writing-block");
    expect(onConsumed).toHaveBeenCalledTimes(1);
  });

  it("leaves the default resume-at-stage room alone when pendingRoom is absent", async () => {
    render(<WorkspaceContainer initialProjectId="p3" />);

    // No pendingRoom → 印记's own status (openTool "plan") decides the room.
    expect(await screen.findByTestId("plan-block")).toBeInTheDocument();
    expect(screen.queryByTestId("reading-block")).not.toBeInTheDocument();
  });
});
