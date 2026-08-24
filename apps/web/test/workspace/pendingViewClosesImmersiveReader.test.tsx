import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type { MaterialSource } from "@mind-imprint/contracts";

// Regression test for the guided tour's projects journey (P6): once the
// reading-room segment opens the real immersive `ReadingRoom` via
// `pendingDemoReading` (openDemoReadingRoom, Task 5), `readingSource != null`
// renders it OVER every room. The tour's later `setReadingView`/`setWritingView`
// deep-links (reading-library, writing steps) must close that immersive reader
// first, or the underlying room they're trying to land the tour on (and the
// spotlit elements inside it) never mounts.
//
// Mirrors the mock harness in pendingDemoReading.test.tsx / pendingRoom.test.tsx.
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

function fakeMaterialSource(id: string): MaterialSource {
  return {
    id,
    title: "Chen et al. (2019), Nature Sustainability",
    sourceUrl: "https://doi.org/10.1038/s41893-019-0220-7",
    kind: "article",
    origin: "fetched",
    blocks: [{ id: "b1", text: "Satellite data …" }],
    locked: false,
    role: "",
    tier: "",
    takeaway: "",
    anchors: [],
    timeSpentS: 0,
    lateralRead: false,
    isLateralInstrument: false,
    siftSkipped: false,
    lateralRelation: "",
    lateralJudgment: "",
  };
}

describe("WorkspaceContainer · pendingReadingView / pendingWritingView close the immersive reader", () => {
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

  it("closes the immersive reader and mounts reading-library's ReadingBlock when pendingReadingView arrives", async () => {
    const onReadingViewConsumed = vi.fn();
    const { rerender } = render(<WorkspaceContainer initialProjectId="p1" />);
    await screen.findByTestId("plan-block");

    // reading-room-0: openDemoReadingRoom() → pendingRoom("reading") +
    // pendingDemoReading (the real immersive reader opens over everything).
    const source = fakeMaterialSource("mat-1");
    rerender(
      <WorkspaceContainer
        initialProjectId="p1"
        pendingRoom="reading"
        pendingDemoReading={{ source, referenceId: "ref-1" }}
        onPendingReadingViewConsumed={onReadingViewConsumed}
      />,
    );
    expect(await screen.findByTestId("reading-room")).toBeInTheDocument();
    expect(screen.queryByTestId("reading-block")).not.toBeInTheDocument();

    // rr-* spotlight steps issue neither setReadingView nor setWritingView —
    // re-rendering with the same pendingDemoReading (already consumed, so the
    // caller would have cleared it in real use) and no new deep-link must
    // leave the immersive reader open.
    rerender(
      <WorkspaceContainer initialProjectId="p1" pendingRoom="reading" onPendingReadingViewConsumed={onReadingViewConsumed} />,
    );
    expect(screen.getByTestId("reading-room")).toBeInTheDocument();

    // reading-library-0: setStudioRoom("reading") + setReadingView("list").
    rerender(
      <WorkspaceContainer
        initialProjectId="p1"
        pendingRoom="reading"
        pendingReadingView="list"
        onPendingReadingViewConsumed={onReadingViewConsumed}
      />,
    );

    expect(await screen.findByTestId("reading-block")).toBeInTheDocument();
    expect(screen.queryByTestId("reading-room")).not.toBeInTheDocument();
    expect(onReadingViewConsumed).toHaveBeenCalledTimes(1);
  });

  it("closes the immersive reader and mounts the writing room when pendingWritingView arrives", async () => {
    const onWritingViewConsumed = vi.fn();
    const { rerender } = render(<WorkspaceContainer initialProjectId="p2" />);
    await screen.findByTestId("plan-block");

    const source = fakeMaterialSource("mat-2");
    rerender(
      <WorkspaceContainer
        initialProjectId="p2"
        pendingRoom="reading"
        pendingDemoReading={{ source, referenceId: "ref-2" }}
        onPendingWritingViewConsumed={onWritingViewConsumed}
      />,
    );
    expect(await screen.findByTestId("reading-room")).toBeInTheDocument();

    // writing-2: setWritingView({ doc: "proposal", tab: "snippets" }) — this
    // is the exact step from the bug report; the reader must close so
    // `writing-aicard`/`writing-tabs` can mount underneath.
    rerender(
      <WorkspaceContainer
        initialProjectId="p2"
        pendingRoom="writing"
        pendingWritingView={{ doc: "proposal", tab: "snippets" }}
        onPendingWritingViewConsumed={onWritingViewConsumed}
      />,
    );

    expect(await screen.findByTestId("writing-block")).toBeInTheDocument();
    expect(screen.queryByTestId("reading-room")).not.toBeInTheDocument();
    expect(onWritingViewConsumed).toHaveBeenCalledTimes(1);
  });

  it("does not touch the immersive reader when pendingReadingView is absent", async () => {
    const { rerender } = render(<WorkspaceContainer initialProjectId="p3" />);
    await screen.findByTestId("plan-block");

    const source = fakeMaterialSource("mat-3");
    rerender(
      <WorkspaceContainer initialProjectId="p3" pendingRoom="reading" pendingDemoReading={{ source, referenceId: "ref-3" }} />,
    );
    expect(await screen.findByTestId("reading-room")).toBeInTheDocument();

    rerender(<WorkspaceContainer initialProjectId="p3" pendingRoom="reading" />);
    expect(screen.getByTestId("reading-room")).toBeInTheDocument();
    expect(screen.queryByTestId("reading-block")).not.toBeInTheDocument();
  });
});
