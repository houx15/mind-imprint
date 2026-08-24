import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type { MaterialSource } from "@mind-imprint/contracts";

// Light, seam-focused test of the `pendingDemoReading` deep-link (P6, Task 5)
// — mirrors `pendingRoom.test.tsx`'s mock harness so the studio's
// data-fetching/room internals stay mocked out and this test only asserts the
// pendingDemoReading → openReadingSource(..., { demoMode, initialMessages })
// plumbing into the real immersive `ReadingRoom`.
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

const readingRoomProps: {
  demoMode?: boolean;
  initialMessages?: unknown;
  sourceId?: string;
  referenceId?: string;
  readingNote?: string | null;
} = {};
vi.mock("@/studio/reading/ReadingRoom", () => ({
  ReadingRoom: (props: {
    demoMode?: boolean;
    initialMessages?: unknown;
    source: MaterialSource;
    referenceId: string;
    readingNote?: string | null;
  }) => {
    readingRoomProps.demoMode = props.demoMode;
    readingRoomProps.initialMessages = props.initialMessages;
    readingRoomProps.sourceId = props.source.id;
    readingRoomProps.referenceId = props.referenceId;
    readingRoomProps.readingNote = props.readingNote;
    return <div data-testid="reading-room" />;
  },
}));

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

describe("WorkspaceContainer · pendingDemoReading", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    readingRoomProps.demoMode = undefined;
    readingRoomProps.initialMessages = undefined;
    readingRoomProps.sourceId = undefined;
    readingRoomProps.referenceId = undefined;
    readingRoomProps.readingNote = undefined;
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

  // Mirrors pendingRoom.test.tsx's real timing: the tour opens the demo
  // project first (initialProjectId), and only later drives pendingDemoReading
  // once that project has settled — never on the same render.
  it("opens the real immersive ReadingRoom with demoMode + the seeded transcript once pendingDemoReading is set", async () => {
    const onConsumed = vi.fn();
    const { rerender } = render(<WorkspaceContainer initialProjectId="demo1" onPendingDemoReadingConsumed={onConsumed} />);
    await screen.findByTestId("plan-block");

    const source = fakeMaterialSource("mat-271");
    // P8 Task 7: StudentApp's openDemoReadingRoom now carries the seeded
    // reference note (0091) alongside the source/referenceId — this must
    // reach ReadingRoom's readingNote prop (its 7th openReadingSource arg),
    // not stay undefined, so 我的笔记 shows real content on the demo.
    const seededNote = "读到这里先记一笔：论文用 NASA MODIS 2000–2017 的数据……";
    rerender(
      <WorkspaceContainer
        initialProjectId="demo1"
        pendingDemoReading={{ source, referenceId: "ref-260", readingNote: seededNote }}
        onPendingDemoReadingConsumed={onConsumed}
      />,
    );

    expect(await screen.findByTestId("reading-room")).toBeInTheDocument();
    expect(readingRoomProps.demoMode).toBe(true);
    expect(readingRoomProps.sourceId).toBe("mat-271");
    expect(readingRoomProps.referenceId).toBe("ref-260");
    expect(readingRoomProps.readingNote).toBe(seededNote);
    expect(Array.isArray(readingRoomProps.initialMessages)).toBe(true);
    expect((readingRoomProps.initialMessages as unknown[]).length).toBeGreaterThan(0);
    expect(onConsumed).toHaveBeenCalledTimes(1);
  });

  it("leaves the default resume-at-stage room alone when pendingDemoReading is absent", async () => {
    render(<WorkspaceContainer initialProjectId="demo2" />);

    expect(await screen.findByTestId("plan-block")).toBeInTheDocument();
    expect(screen.queryByTestId("reading-room")).not.toBeInTheDocument();
  });
});
