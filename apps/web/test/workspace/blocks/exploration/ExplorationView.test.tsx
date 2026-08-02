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
// Followup fix (2026-08): the rabbit-hole finalize path now runs through the
// shared reflect turn (card_persist.go's explorationDeckCards allowlist) so it
// gets AI feedback like every other deck card — mocked so submitting the real
// StudioCardSheet in tests never hits the network, and so the reflect call
// itself (and the feedback it hands back to the parent) is assertable.
vi.mock("@/workspace/api/workspace", () => ({
  enterReading: vi.fn(),
  NoReadableContentError: class extends Error {},
  reflectProjectCard: vi.fn(async () => ({
    cardInstanceId: "card-1",
    reply: "AI 反馈：这个方向能查一查沙漠光伏的生态影响。",
    card: null,
  })),
}));

import { getExploration, createLead, patchLead, digDeeper } from "@/api/exploration";
import { reflectProjectCard } from "@/workspace/api/workspace";

const mockGetExploration = vi.mocked(getExploration);
const mockCreateLead = vi.mocked(createLead);
const mockPatchLead = vi.mocked(patchLead);
const mockDigDeeper = vi.mocked(digDeeper);
const mockReflectProjectCard = vi.mocked(reflectProjectCard);

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

  // Batch5 Item A #1/#2: attach a source under an OPEN lead — modeled as a
  // CHILD branch (createLead+parentLeadId) that's immediately connected, so
  // the parent lead itself stays open for more sources.
  it("＋来源: picking an existing reference creates a connected child under the lead, parent stays open", async () => {
    mockCreateLead.mockResolvedValueOnce({
      id: "child1",
      text: DANGLING_REF.title,
      status: "open",
      origin: "manual",
      sourceReferenceId: null,
      connectedReferenceId: null,
      position: 3,
      parentLeadId: LEAD_OPEN.id,
    });
    const user = userEvent.setup();
    render(<ExplorationView projectId="p1" references={[NASA_REF, DANGLING_REF]} />);
    await screen.findByText(LEAD_OPEN.text);

    await user.click(screen.getByRole("button", { name: "＋来源" }));
    await user.click(await screen.findByRole("button", { name: DANGLING_REF.title }));

    await waitFor(() => {
      expect(mockCreateLead).toHaveBeenCalledWith("p1", DANGLING_REF.title, { parentLeadId: LEAD_OPEN.id });
      expect(mockPatchLead).toHaveBeenCalledWith("p1", "child1", {
        status: "connected",
        connectedReferenceId: DANGLING_REF.id,
      });
    });
    // the parent lead itself was never patched to "connected" — it stays open
    expect(mockPatchLead).not.toHaveBeenCalledWith("p1", LEAD_OPEN.id, expect.anything());
  });

  // Item A #2: registering a brand-new untracked source from the graph, with
  // no lead selected, just calls the lifted onCreateReference (no attach).
  it("＋添加来源: registers an untracked source via the lifted onCreateReference, unattached", async () => {
    const onCreateReference = vi.fn().mockResolvedValue({ ...NASA_REF, id: "r9", title: "新论文" });
    const user = userEvent.setup();
    render(<ExplorationView projectId="p1" references={[NASA_REF, DANGLING_REF]} onCreateReference={onCreateReference} />);
    await screen.findByText(LEAD_OPEN.text);

    await user.click(screen.getByRole("button", { name: "＋ 添加来源" }));
    await user.type(screen.getByPlaceholderText("来源标题"), "新论文");
    // "添加" also labels the unrelated manual-lead-add button further down the
    // page — the modal's submit renders first in the tree.
    await user.click(screen.getAllByRole("button", { name: "添加" })[0]!);

    await waitFor(() => expect(onCreateReference).toHaveBeenCalledWith({ title: "新论文", url: undefined }));
    // unattached: no lead was chosen, so no createLead/patchLead follow-up
    expect(mockCreateLead).not.toHaveBeenCalled();
  });

  // Batch5 Item B, updated by the followup fix (2026-08): selecting an open
  // 线索 to dig from turns finalize into: reflect the card (FEEDBACK, exactly
  // once), THEN AUTOMATICALLY reuse digDeeper (SUGGESTIONS, no manual "让印记
  //给方向" click), showing suggestions in the SAME shared focus panel.
  it("rabbit-hole finalize: selecting a 线索 reflects the card (feedback), auto-runs digDeeper (suggestions), and shows both", async () => {
    const user = userEvent.setup();
    const onCardReflected = vi.fn();
    render(
      <ExplorationView
        projectId="p1"
        references={[NASA_REF, DANGLING_REF]}
        onCardReflected={onCardReflected}
      />,
    );
    await screen.findByText(LEAD_OPEN.text);

    await user.click(screen.getByRole("button", { name: "＋ 兔子洞" }));
    await screen.findByText("兔子洞·兴趣雷达卡");

    await user.selectOptions(screen.getByLabelText("从哪条线索挖"), LEAD_OPEN.id);
    await user.click(screen.getByRole("button", { name: /提交并钉到过程树/ }));

    // FEEDBACK: the reflect turn ran on the rabbit-hole card, exactly once —
    // not the old coach-silent persist path.
    await waitFor(() => expect(mockReflectProjectCard).toHaveBeenCalledTimes(1));
    expect(mockReflectProjectCard).toHaveBeenCalledWith(
      "p1",
      "rabbit-hole",
      expect.any(Object),
      expect.any(Array),
      "find_sources",
    );
    // ...and the reply was handed up to the parent room (no manual click).
    await waitFor(() => {
      expect(onCardReflected).toHaveBeenCalledWith(
        expect.any(String),
        "AI 反馈：这个方向能查一查沙漠光伏的生态影响。",
        undefined,
      );
    });
    // SUGGESTIONS: digDeeper ran automatically — no manual "让印记给方向" click.
    await waitFor(() => {
      expect(mockDigDeeper).toHaveBeenCalledWith("p1", { leadId: LEAD_OPEN.id, thought: "" });
    });
    // the card closed and the suggestion surfaced in the shared 深挖 panel —
    // not a separate, easy-to-miss spot.
    expect(screen.queryByText("兔子洞·兴趣雷达卡")).not.toBeInTheDocument();
    expect(await screen.findByText("深挖这条线索")).toBeInTheDocument();
    expect(await screen.findByText("查一下中国的可再生能源装机增速")).toBeInTheDocument();
    // no loose lead was created — the selected-lead path never double-creates
    expect(mockCreateLead).not.toHaveBeenCalled();
  });

  // Item B: leaving no lead selected and writing nothing must never be a
  // silent no-op (the S3 invisibility bug) — it tells the student so. The
  // card still reflects (feedback always runs), it's only the dig/lead
  // follow-up that's skipped when there's nothing to go on.
  it("rabbit-hole finalize: no lead selected and nothing written shows a gentle notice, not silence", async () => {
    mockGetExploration.mockResolvedValue({ leads: [], danglingSourceIds: [] });
    const user = userEvent.setup();
    render(<ExplorationView projectId="p1" references={[]} />);
    await screen.findByText("这里还是空的");

    await user.click(screen.getByRole("button", { name: "＋ 兔子洞" }));
    await screen.findByText("兔子洞·兴趣雷达卡");
    await user.click(screen.getByRole("button", { name: /提交并钉到过程树/ }));

    await waitFor(() => expect(mockReflectProjectCard).toHaveBeenCalledTimes(1));
    expect(await screen.findByText(/这次没记下新的方向或线索/)).toBeInTheDocument();
    expect(mockCreateLead).not.toHaveBeenCalled();
    expect(mockDigDeeper).not.toHaveBeenCalled();
  });
});
