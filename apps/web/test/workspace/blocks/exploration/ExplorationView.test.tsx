import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { DigCandidate, ExplorationLead, QuestionEdge, Reference } from "@mind-imprint/contracts";
import { ExplorationView } from "@/workspace/blocks/exploration/ExplorationView";

// GVa · the Level-1 map is now a React Flow graph. React Flow can't render in
// jsdom (needs ResizeObserver + a measured container), so we mock @xyflow/react
// with a lightweight shim that renders the passed nodes/edges as inspectable
// DOM: each node is a <div data-testid="rf-node"> whose click delegates to
// onNodeClick (the zoom-into-a-question path), and each edge is rendered through
// the real custom edge component (its label chip + 确认/忽略/relabel controls).
// The heavy layout/theme/count logic is unit-tested separately in
// warrenLayout.test.ts; here we assert the wired behavior end-to-end.
vi.mock("@xyflow/react", async () => {
  const React = await import("react");
  const Position = { Left: "left", Right: "right", Top: "top", Bottom: "bottom" };
  type AnyProps = Record<string, any>;
  return {
    __esModule: true,
    Position,
    ReactFlow: ({ nodes = [], edges = [], nodeTypes = {}, edgeTypes = {}, onNodeClick }: AnyProps) =>
      React.createElement(
        "div",
        { "data-testid": "rf-mock" },
        ...nodes.map((n: AnyProps) => {
          const C = nodeTypes[n.type];
          return React.createElement(
            "div",
            { key: n.id, "data-testid": "rf-node", onClick: (e: any) => onNodeClick && onNodeClick(e, n) },
            C ? React.createElement(C, { id: n.id, data: n.data }) : null,
          );
        }),
        ...edges.map((e: AnyProps) => {
          const C = edgeTypes[e.type];
          return C
            ? React.createElement(C, {
                key: e.id,
                id: e.id,
                data: e.data,
                source: e.source,
                target: e.target,
                sourceX: 0,
                sourceY: 0,
                targetX: 0,
                targetY: 0,
                sourcePosition: Position.Right,
                targetPosition: Position.Left,
              })
            : null;
        }),
      ),
    ReactFlowProvider: ({ children }: AnyProps) => React.createElement(React.Fragment, null, children),
    Background: () => null,
    Controls: () => null,
    Handle: () => null,
    BaseEdge: () => null,
    EdgeLabelRenderer: ({ children }: AnyProps) => React.createElement(React.Fragment, null, children),
    getBezierPath: () => ["", 0, 0],
    applyNodeChanges: (_changes: any, nodes: any) => nodes,
  };
});

// The view calls a thin exploration client directly. Mock the exact functions
// the component imports (createEdge/deleteLead are new to GVa's map).
vi.mock("@/api/exploration", () => ({
  getExploration: vi.fn(),
  createLead: vi.fn(),
  deleteLead: vi.fn(),
  digExploration: vi.fn(),
  adoptCandidate: vi.fn(),
  patchLead: vi.fn(),
  proposeEdges: vi.fn(),
  createEdge: vi.fn(),
  patchEdge: vi.fn(),
  deleteEdge: vi.fn(),
}));
// The 进入阅读室 path (a paper node's ⋯ menu) goes through workspace.enterReading;
// mocked so the component's import resolves and never hits the network in tests.
vi.mock("@/workspace/api/workspace", () => ({
  enterReading: vi.fn(),
  NoReadableContentError: class extends Error {},
}));

import {
  getExploration,
  createLead,
  deleteLead,
  digExploration,
  adoptCandidate,
  patchLead,
  proposeEdges,
  patchEdge,
  deleteEdge,
} from "@/api/exploration";

const mockGetExploration = vi.mocked(getExploration);
const mockCreateLead = vi.mocked(createLead);
const mockDeleteLead = vi.mocked(deleteLead);
const mockDigExploration = vi.mocked(digExploration);
const mockAdoptCandidate = vi.mocked(adoptCandidate);
const mockPatchLead = vi.mocked(patchLead);
const mockProposeEdges = vi.mocked(proposeEdges);
const mockPatchEdge = vi.mocked(patchEdge);
const mockDeleteEdge = vi.mocked(deleteEdge);

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
    readingStatus: "to_read",
    ...overrides,
  };
}

const NASA_REF = makeRef({
  id: "r1",
  title: "NASA 卫星植被覆盖数据",
  author: "NASA",
  year: "2023",
  journal: "Nature Sustainability",
});

// A top-level question node — parentLeadId null makes it a root of the tree.
const ROOT_LEAD: ExplorationLead = {
  id: "lead1",
  text: "中国碳排放全球第一，这跟可持续矛盾吗？",
  status: "open",
  origin: "manual",
  sourceReferenceId: null,
  connectedReferenceId: null,
  position: 0,
  parentLeadId: null,
};

// A paper node nested UNDER the question node (parentLeadId = ROOT_LEAD.id),
// connected to a real reference so its title badge renders.
const CHILD_PAPER: ExplorationLead = {
  id: "lead2",
  text: "顺着这条挖到的论文",
  status: "connected",
  origin: "manual",
  sourceReferenceId: null,
  connectedReferenceId: NASA_REF.id,
  position: 1,
  parentLeadId: ROOT_LEAD.id,
};

// A SECOND top-level question node — two roots make the map graph non-trivial
// (a real edge can span them).
const SECOND_ROOT: ExplorationLead = {
  id: "lead3",
  text: "可再生能源装机增速能抵消碳排放吗？",
  status: "open",
  origin: "manual",
  sourceReferenceId: null,
  connectedReferenceId: null,
  position: 2,
  parentLeadId: null,
};

// A confirmed labeled edge between the two roots — the map must render its label.
const CONFIRMED_EDGE: QuestionEdge = {
  id: "edge1",
  fromLeadId: ROOT_LEAD.id,
  toLeadId: SECOND_ROOT.id,
  label: "反驳/张力",
  status: "confirmed",
};

// A 印记-proposed (dashed) edge between the two roots — status:"proposed" carries
// the 确认/忽略 controls until the student decides (铁律②).
const PROPOSED_EDGE: QuestionEdge = {
  id: "edge2",
  fromLeadId: ROOT_LEAD.id,
  toLeadId: SECOND_ROOT.id,
  label: "支持",
  status: "proposed",
};

const CANDIDATE: DigCandidate = {
  doi: "10.1234/renew",
  title: "中国可再生能源装机增速研究",
  authors: "Li 等",
  year: "2023",
  journal: "Nature Sustainability",
  abstract: "本文分析了中国可再生能源装机的增长趋势。",
  url: "https://x.test/renew",
};

beforeEach(() => {
  vi.clearAllMocks();
  mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD], danglingSourceIds: [], edges: [] });
  mockCreateLead.mockResolvedValue({ ...ROOT_LEAD, id: "leadN", text: "new" });
  mockDeleteLead.mockResolvedValue(undefined);
  mockPatchLead.mockResolvedValue({ ...ROOT_LEAD, status: "pruned" });
  mockDigExploration.mockResolvedValue({ candidates: [CANDIDATE] });
  mockAdoptCandidate.mockResolvedValue({
    lead: { ...CHILD_PAPER, id: "lead9", connectedReferenceId: "r9" },
    reference: { ...NASA_REF, id: "r9" },
  });
  mockProposeEdges.mockResolvedValue([]);
  mockPatchEdge.mockResolvedValue({ ...CONFIRMED_EDGE });
  mockDeleteEdge.mockResolvedValue(undefined);
});

// Each test uses a distinct projectId: the component persists map⇄hole zoom in
// a module-level Map keyed by projectId, so a fresh id guarantees a clean start
// (default = map overview) regardless of test order.
let pid = 0;
const nextPid = () => `p-${++pid}`;

// From the map, zoom into a root question node → its "inside" (the A6 tree).
// A map node is a <div> whose click delegates to onNodeClick; click its text.
async function zoomInto(user: ReturnType<typeof userEvent.setup>, text: string) {
  await user.click(await screen.findByText(text));
  await screen.findByRole("button", { name: "← 返回地图" });
}

describe("ExplorationView", () => {
  it("renders the plain empty state when there are no roots", async () => {
    mockGetExploration.mockResolvedValue({ leads: [], danglingSourceIds: [], edges: [] });
    render(<ExplorationView projectId={nextPid()} references={[]} />);

    expect(await screen.findByText("这里还是空的")).toBeInTheDocument();
    // no rabbit metaphor in the copy — plain guidance only
    expect(screen.getByText(/在上面记下一个你想弄清楚的问题/)).toBeInTheDocument();
  });

  it("the map root input CREATES a top-level question node via createLead (not a keyword dig)", async () => {
    mockGetExploration.mockResolvedValue({ leads: [], danglingSourceIds: [], edges: [] });
    const projectId = nextPid();
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[]} />);
    await screen.findByText("这里还是空的");

    await user.type(
      screen.getByPlaceholderText(/记一个你想弄清楚的问题/),
      "中国碳排放全球第一，这跟可持续矛盾吗？",
    );
    await user.click(screen.getByRole("button", { name: "记下问题" }));

    await waitFor(() => {
      expect(mockCreateLead).toHaveBeenCalledWith(projectId, "中国碳排放全球第一，这跟可持续矛盾吗？");
    });
    // creating a question is NOT a dig — no paper search fired
    expect(mockDigExploration).not.toHaveBeenCalled();
  });

  it("the MAP renders an ellipse node per root with its 文献 x 篇 paper tally", async () => {
    // ROOT_LEAD has one adopted paper (CHILD_PAPER); SECOND_ROOT has none.
    mockGetExploration.mockResolvedValue({
      leads: [ROOT_LEAD, CHILD_PAPER, SECOND_ROOT],
      danglingSourceIds: [],
      edges: [],
    });
    render(<ExplorationView projectId={nextPid()} references={[NASA_REF]} />);

    // both roots show as map nodes; the child paper is NOT a root on the map
    expect(await screen.findByText(ROOT_LEAD.text)).toBeInTheDocument();
    expect(screen.getByText(SECOND_ROOT.text)).toBeInTheDocument();
    expect(screen.queryByText(CHILD_PAPER.text)).toBeNull();
    // the paper tally: ROOT_LEAD → 文献 1 篇, SECOND_ROOT → 文献 0 篇
    expect(screen.getByText("文献 1 篇")).toBeInTheDocument();
    expect(screen.getByText("文献 0 篇")).toBeInTheDocument();
  });

  it("the graph title 兔子洞地图 has a ? that explains the metaphor in plain words", async () => {
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD], danglingSourceIds: [], edges: [] });
    const user = userEvent.setup();
    render(<ExplorationView projectId={nextPid()} references={[]} />);

    expect(await screen.findByText("兔子洞地图")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "这是什么" }));
    expect(screen.getByText(/每个问题就是一个洞口/)).toBeInTheDocument();
  });

  it("the MAP renders a confirmed edge's label between two roots", async () => {
    mockGetExploration.mockResolvedValue({
      leads: [ROOT_LEAD, SECOND_ROOT],
      danglingSourceIds: [],
      edges: [CONFIRMED_EDGE],
    });
    render(<ExplorationView projectId={nextPid()} references={[]} />);

    await screen.findByText(ROOT_LEAD.text);
    // the closed-vocabulary relation word renders as the edge's label chip
    expect(screen.getByText("反驳/张力")).toBeInTheDocument();
  });

  it("× on a map node opens a confirm modal; confirming calls deleteLead", async () => {
    const projectId = nextPid();
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD], danglingSourceIds: [], edges: [] });
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[]} />);
    await screen.findByText(ROOT_LEAD.text);

    // 克制: pressing × alone does not delete — it asks first
    await user.click(screen.getByRole("button", { name: "删除这个问题" }));
    expect(await screen.findByText("删除这个问题？")).toBeInTheDocument();
    expect(mockDeleteLead).not.toHaveBeenCalled();

    // confirming the modal removes the question via deleteLead
    await user.click(screen.getByRole("button", { name: "删除" }));
    await waitFor(() => expect(mockDeleteLead).toHaveBeenCalledWith(projectId, ROOT_LEAD.id));
  });

  it("zooming into a node shows its subtree with nested paper children; 返回地图 zooms back out", async () => {
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD, CHILD_PAPER], danglingSourceIds: [], edges: [] });
    const user = userEvent.setup();
    render(<ExplorationView projectId={nextPid()} references={[NASA_REF]} />);

    await zoomInto(user, ROOT_LEAD.text);

    // inside the hole: the root plus its nested child paper (with its title badge)
    expect(screen.getByText(CHILD_PAPER.text)).toBeInTheDocument();
    expect(screen.getByText(`论文 · ${NASA_REF.title}`)).toBeInTheDocument();

    // back to the map: the node reappears, the subtree child is gone
    await user.click(screen.getByRole("button", { name: "← 返回地图" }));
    await screen.findByText(ROOT_LEAD.text);
    expect(screen.queryByText(CHILD_PAPER.text)).toBeNull();
  });

  it("深挖 inside a hole calls digExploration({leadId}) and fills the tray", async () => {
    const projectId = nextPid();
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[NASA_REF]} />);
    await zoomInto(user, ROOT_LEAD.text);

    // 克制: nothing dug just from zooming in
    expect(mockDigExploration).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "深挖" }));

    await waitFor(() => {
      expect(mockDigExploration).toHaveBeenCalledWith(projectId, { leadId: ROOT_LEAD.id });
    });

    // the tray opens with its plain heading and the candidate's bibliographic line
    expect(await screen.findByText("刚挖到这些论文")).toBeInTheDocument();
    expect(screen.getByText(CANDIDATE.title)).toBeInTheDocument();
    expect(screen.getByText("Li 等 · 2023 · Nature Sustainability")).toBeInTheDocument();
  });

  it("采纳 adopts the candidate UNDER the dug node (parentLeadId = that node's id) — papers never roots", async () => {
    const projectId = nextPid();
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[NASA_REF]} />);
    await zoomInto(user, ROOT_LEAD.text);

    await user.click(screen.getByRole("button", { name: "深挖" }));
    await screen.findByText(CANDIDATE.title);

    // 克制: still nothing persisted until she taps 采纳
    expect(mockAdoptCandidate).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "采纳" }));

    // INVARIANT: adopting always carries a parentLeadId equal to the dug node.
    await waitFor(() => {
      expect(mockAdoptCandidate).toHaveBeenCalledWith(projectId, CANDIDATE, { parentLeadId: ROOT_LEAD.id });
    });
    // and the tree is refetched to show the freshly-nested paper
    await waitFor(() => expect(mockGetExploration).toHaveBeenCalledTimes(2));
  });

  it("丢弃 removes a candidate from the tray with no server call", async () => {
    const user = userEvent.setup();
    render(<ExplorationView projectId={nextPid()} references={[NASA_REF]} />);
    await zoomInto(user, ROOT_LEAD.text);

    await user.click(screen.getByRole("button", { name: "深挖" }));
    await screen.findByText(CANDIDATE.title);

    await user.click(screen.getByRole("button", { name: "丢弃" }));

    await waitFor(() => expect(screen.queryByText(CANDIDATE.title)).toBeNull());
    // discard is client-only — it never adopts
    expect(mockAdoptCandidate).not.toHaveBeenCalled();
  });

  it("剪枝 (in the ⋯ menu, inside a hole) prunes a node via patchLead", async () => {
    const projectId = nextPid();
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[NASA_REF]} />);
    await zoomInto(user, ROOT_LEAD.text);

    await user.click(screen.getByRole("button", { name: "更多" }));
    await user.click(screen.getByRole("button", { name: "剪枝" }));

    await waitFor(() => {
      expect(mockPatchLead).toHaveBeenCalledWith(projectId, ROOT_LEAD.id, { status: "pruned" });
    });
  });

  // ---- 印记-proposed labeled edges: propose / confirm / dismiss / relabel ----

  it("the propose button calls proposeEdges; the returned proposed edge renders dashed with 确认/忽略", async () => {
    // first load: two roots, no edges. After proposing, the refetch returns the
    // new proposed (dashed) edge.
    mockGetExploration
      .mockResolvedValueOnce({ leads: [ROOT_LEAD, SECOND_ROOT], danglingSourceIds: [], edges: [] })
      .mockResolvedValue({ leads: [ROOT_LEAD, SECOND_ROOT], danglingSourceIds: [], edges: [PROPOSED_EDGE] });
    mockProposeEdges.mockResolvedValue([PROPOSED_EDGE]);
    const projectId = nextPid();
    const user = userEvent.setup();
    const { container } = render(<ExplorationView projectId={projectId} references={[]} />);
    await screen.findByText(ROOT_LEAD.text);

    await user.click(screen.getByRole("button", { name: "让印记找找问题之间的关系" }));

    await waitFor(() => expect(mockProposeEdges).toHaveBeenCalledWith(projectId));
    // the proposed edge draws dashed and carries the confirm/dismiss controls
    expect(await screen.findByRole("button", { name: "确认" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "忽略" })).toBeInTheDocument();
    expect(container.querySelector('[data-testid="edge-proposed"]')).not.toBeNull();
  });

  it("确认 on a proposed edge calls patchEdge with {status:'confirmed'} then refetches", async () => {
    mockGetExploration.mockResolvedValue({
      leads: [ROOT_LEAD, SECOND_ROOT],
      danglingSourceIds: [],
      edges: [PROPOSED_EDGE],
    });
    const projectId = nextPid();
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[]} />);

    await user.click(await screen.findByRole("button", { name: "确认" }));

    await waitFor(() => {
      expect(mockPatchEdge).toHaveBeenCalledWith(projectId, PROPOSED_EDGE.id, { status: "confirmed" });
    });
    // refetched so the map reflects the now-confirmed edge (铁律: server truth)
    await waitFor(() => expect(mockGetExploration).toHaveBeenCalledTimes(2));
  });

  it("忽略 on a proposed edge calls deleteEdge", async () => {
    mockGetExploration.mockResolvedValue({
      leads: [ROOT_LEAD, SECOND_ROOT],
      danglingSourceIds: [],
      edges: [PROPOSED_EDGE],
    });
    const projectId = nextPid();
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[]} />);

    await user.click(await screen.findByRole("button", { name: "忽略" }));

    await waitFor(() => expect(mockDeleteEdge).toHaveBeenCalledWith(projectId, PROPOSED_EDGE.id));
    // 忽略 never confirms
    expect(mockPatchEdge).not.toHaveBeenCalled();
  });

  it("relabeling a confirmed edge calls patchEdge with the new {label} from the closed set", async () => {
    mockGetExploration.mockResolvedValue({
      leads: [ROOT_LEAD, SECOND_ROOT],
      danglingSourceIds: [],
      edges: [CONFIRMED_EDGE],
    });
    const projectId = nextPid();
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[]} />);

    // the confirmed edge's label chip is itself a button → opens the relabel picker
    await user.click(await screen.findByRole("button", { name: CONFIRMED_EDGE.label }));
    // pick a different closed-vocabulary label
    await user.click(screen.getByRole("button", { name: "支持" }));

    await waitFor(() => {
      expect(mockPatchEdge).toHaveBeenCalledWith(projectId, CONFIRMED_EDGE.id, { label: "支持" });
    });
  });
});
