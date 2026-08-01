import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Reference } from "@mind-imprint/contracts";
import { ExplorationView } from "@/workspace/blocks/exploration/ExplorationView";

// S3 Task 10: mock the thin exploration client module the view calls
// directly (no injected-api prop here, unlike ReadingRoom) — same pattern
// AskPanel.test.tsx uses for @/api/voice.
vi.mock("@/api/exploration", () => ({
  getExploration: vi.fn(),
  createLead: vi.fn(),
  patchLead: vi.fn(),
  deleteLead: vi.fn(),
  digDeeper: vi.fn(),
}));

import { getExploration, createLead, patchLead, digDeeper } from "@/api/exploration";

const mockGetExploration = vi.mocked(getExploration);
const mockCreateLead = vi.mocked(createLead);
const mockPatchLead = vi.mocked(patchLead);
const mockDigDeeper = vi.mocked(digDeeper);

// Reference fixture shape matches apps/web/test/api/workspaceLibrary.test.ts.
function makeRef(overrides: Partial<Reference>): Reference {
  return {
    id: "r-base",
    title: "来源",
    classification: "数据集",
    author: "作者",
    credentials: "",
    year: "2023",
    url: "https://x.test/",
    tags: [],
    collectionId: null,
    credibility: null,
    evaluation: "",
    decision: null,
    pending: false,
    searchHints: [],
    materialId: null,
    notes: [],
    ...overrides,
  };
}

const NASA_REF = makeRef({
  id: "r1",
  title: "NASA 卫星植被覆盖数据",
  materialId: "m1",
});
const DANGLING_REF = makeRef({
  id: "r2",
  title: "《自然·可持续发展》论文",
  materialId: "m2",
});

const LEAD_OPEN = {
  id: "lead1",
  text: "中国碳排放全球第一，这跟可持续矛盾吗？",
  status: "open" as const,
  origin: "takeaway" as const,
  sourceReferenceId: NASA_REF.id,
  connectedReferenceId: null,
  position: 0,
  parentLeadId: null,
};

// The Important fix from the S2/S3 review: a lead whose source reference was
// later deleted (DB nulls sourceReferenceId on delete) must still render,
// not silently vanish from the whole graph.
const LOOSE_CONNECTED_LEAD = {
  id: "lead2",
  text: "已经追完的旧线索",
  status: "connected" as const,
  origin: "manual" as const,
  sourceReferenceId: null,
  connectedReferenceId: NASA_REF.id,
  position: 1,
  parentLeadId: null,
};

beforeEach(() => {
  vi.clearAllMocks();
  mockGetExploration.mockResolvedValue({
    leads: [LEAD_OPEN, LOOSE_CONNECTED_LEAD],
    danglingSourceIds: [DANGLING_REF.id],
  });
  mockPatchLead.mockResolvedValue({ ...LEAD_OPEN, status: "pruned" });
  mockCreateLead.mockResolvedValue({
    id: "lead3",
    text: "new",
    status: "open",
    origin: "guide",
    sourceReferenceId: null,
    connectedReferenceId: null,
    position: 2,
    parentLeadId: null,
  });
  mockDigDeeper.mockResolvedValue({
    directions: [{ direction: "查一下中国的可再生能源装机增速", why: "能直接检验反例是否站得住" }],
  });
});

describe("ExplorationView", () => {
  it("renders a source branch with its lead and a 剪枝 control", async () => {
    render(<ExplorationView projectId="p1" references={[NASA_REF, DANGLING_REF]} />);

    expect(await screen.findByText(LEAD_OPEN.text)).toBeInTheDocument();
    expect(screen.getByText(NASA_REF.title)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "剪枝" })).toBeInTheDocument();
  });

  it("prune flow: clicking 剪枝 calls patchLead with status pruned", async () => {
    const user = userEvent.setup();
    render(<ExplorationView projectId="p1" references={[NASA_REF, DANGLING_REF]} />);

    await screen.findByText(LEAD_OPEN.text);
    await user.click(screen.getByRole("button", { name: "剪枝" }));

    await waitFor(() => {
      expect(mockPatchLead).toHaveBeenCalledWith("p1", LEAD_OPEN.id, { status: "pruned" });
    });
  });

  it("dig-deeper: renders directions and 记为线索 calls createLead, never automatically", async () => {
    const user = userEvent.setup();
    render(<ExplorationView projectId="p1" references={[NASA_REF, DANGLING_REF]} />);

    await screen.findByText(LEAD_OPEN.text);
    // 克制: nothing was auto-added just from mounting or fetching.
    expect(mockCreateLead).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "深挖一层" }));

    const direction = await screen.findByText("查一下中国的可再生能源装机增速");
    expect(direction).toBeInTheDocument();
    expect(screen.getByText("能直接检验反例是否站得住")).toBeInTheDocument();

    // Still nothing created — the direction is only a suggestion until adopted.
    expect(mockCreateLead).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "记为线索" }));

    await waitFor(() => {
      // whole-graph dig → adopt as a top-level lead (no parent)
      expect(mockCreateLead).toHaveBeenCalledWith("p1", "查一下中国的可再生能源装机增速", undefined);
    });
  });

  it("per-lead dig: 深挖这条 focuses the lead and passes leadId + thought (#12/#13)", async () => {
    const user = userEvent.setup();
    render(<ExplorationView projectId="p1" references={[NASA_REF, DANGLING_REF]} />);
    await screen.findByText(LEAD_OPEN.text);

    await user.click(screen.getByRole("button", { name: "深挖这条" }));
    // the dig section now names this lead and offers a thought input
    expect(await screen.findByText("深挖这条线索")).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText(/说说你现在的想法/), "我想比人均和总量");
    await user.click(screen.getByRole("button", { name: "让印记给方向" }));

    await waitFor(() => {
      expect(mockDigDeeper).toHaveBeenCalledWith("p1", { leadId: "lead1", thought: "我想比人均和总量" });
    });
  });

  it("分支: a child lead renders nested and ＋分支 creates a lead under its parent (#12)", async () => {
    const CHILD = {
      id: "child1", text: "分支：核算口径", status: "open" as const, origin: "manual" as const,
      sourceReferenceId: null, connectedReferenceId: null, position: 3, parentLeadId: "lead1",
    };
    mockGetExploration.mockResolvedValue({ leads: [LEAD_OPEN, CHILD], danglingSourceIds: [] });
    const user = userEvent.setup();
    render(<ExplorationView projectId="p1" references={[NASA_REF, DANGLING_REF]} />);
    // the child renders (nested under its parent), not in a flat list
    expect(await screen.findByText("分支：核算口径")).toBeInTheDocument();
    // open the parent's add-branch input, type, submit
    await user.click(screen.getAllByRole("button", { name: "＋分支" })[0]!);
    await user.type(screen.getByPlaceholderText(/这条线索下的一个分支/), "新分支");
    await user.click(screen.getByRole("button", { name: "加" }));
    await waitFor(() => expect(mockCreateLead).toHaveBeenCalledWith("p1", "新分支", { parentLeadId: "lead1" }));
  });

  it("dangling: a reference in danglingSourceIds renders in the 悬空来源 tray", async () => {
    render(<ExplorationView projectId="p1" references={[NASA_REF, DANGLING_REF]} />);

    await screen.findByText(LEAD_OPEN.text);
    expect(screen.getByText("悬空来源 · 读完了但还没牵出线索")).toBeInTheDocument();
    expect(screen.getByText(DANGLING_REF.title)).toBeInTheDocument();
  });

  it("renders an orphaned loose lead (sourceReferenceId null) instead of vanishing it", async () => {
    render(<ExplorationView projectId="p1" references={[NASA_REF, DANGLING_REF]} />);

    expect(await screen.findByText(LOOSE_CONNECTED_LEAD.text)).toBeInTheDocument();
  });

  // S4: the 兔子洞 card entry (S3's TODO). Opening is the student's tap — the
  // card sheet is NOT mounted on render, only after the button click (克制).
  it("rabbit-hole entry: the card sheet mounts only after tapping ＋ 兔子洞", async () => {
    render(<ExplorationView projectId="p1" references={[NASA_REF, DANGLING_REF]} />);
    await screen.findByText(LEAD_OPEN.text);

    const entry = screen.getByRole("button", { name: "＋ 兔子洞" });
    expect(screen.queryByText("兔子洞·兴趣雷达卡")).not.toBeInTheDocument();
    await userEvent.click(entry);
    expect(await screen.findByText("兔子洞·兴趣雷达卡")).toBeInTheDocument();
  });
});
