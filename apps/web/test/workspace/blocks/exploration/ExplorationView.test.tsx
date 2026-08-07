import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { DigCandidate, ExplorationLead, QuestionEdge, Reference } from "@mind-imprint/contracts";
import { MACARONS } from "@/ui/tokens";
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
    ReactFlow: ({ nodes = [], edges = [], nodeTypes = {}, edgeTypes = {}, onNodeClick, onConnect }: AnyProps) => {
      // Expose the latest onConnect so a test can simulate a drag-to-connect
      // (jsdom can't perform the real handle drag). See fireConnect() below.
      (globalThis as any).__warrenOnConnect = onConnect;
      return React.createElement(
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
      );
    },
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
  createEdge,
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
const mockCreateEdge = vi.mocked(createEdge);
const mockPatchEdge = vi.mocked(patchEdge);
const mockDeleteEdge = vi.mocked(deleteEdge);

// Simulate a React Flow drag-to-connect (jsdom can't perform the real handle
// drag): the @xyflow/react mock stashes the live onConnect prop; call it with a
// {source, target} pair, wrapped in act() so the resulting state updates flush.
async function fireConnect(source: string, target: string) {
  const onConnect = (globalThis as any).__warrenOnConnect as ((c: { source: string; target: string }) => void) | undefined;
  if (!onConnect) throw new Error("onConnect not captured — ReactFlow mock not mounted");
  await act(async () => {
    onConnect({ source, target });
  });
}

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
  abstract: "这篇给出了 2001–2020 年间的全球植被覆盖变化，重点是中国的绿化趋势。",
});

// GVd · a recognizable stand-in for the 印记·找资料 coach (the sidebar's DEFAULT
// 'ai' state). The real coach lives in ReadingBlock; ExplorationView just docks
// whatever node it's handed, so a marker is enough to assert the state machine.
const COACH_SLOT = <div data-testid="coach-slot">印记 · 找资料</div>;

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
  createdAt: "2026-08-01T00:00:00Z",
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
  createdAt: "2026-08-01T00:05:00Z",
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
  createdAt: "2026-08-01T00:10:00Z",
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
  mockCreateEdge.mockResolvedValue({ ...CONFIRMED_EDGE });
  mockPatchEdge.mockResolvedValue({ ...CONFIRMED_EDGE });
  mockDeleteEdge.mockResolvedValue(undefined);
  (globalThis as any).__warrenOnConnect = undefined;
});

// Each test uses a distinct projectId: the component persists map⇄hole zoom in
// a module-level Map keyed by projectId, so a fresh id guarantees a clean start
// (default = map overview) regardless of test order.
let pid = 0;
const nextPid = () => `p-${++pid}`;

// From the map, zoom into a root question node → its "inside" (the Level-2
// mindmap + sidebar). A map node is a <div> whose click delegates to onNodeClick;
// click its text.
async function zoomInto(user: ReturnType<typeof userEvent.setup>, text: string) {
  await user.click(await screen.findByText(text));
  await screen.findByRole("button", { name: "← 返回兔子洞地图" });
}

// Render with the coach slot wired (as ReadingBlock does), so the sidebar's
// DEFAULT 'ai' state is assertable.
function renderView(projectId: string, references: Reference[], onLibraryChanged?: () => void) {
  return render(
    <ExplorationView
      projectId={projectId}
      references={references}
      coach={COACH_SLOT}
      onLibraryChanged={onLibraryChanged}
    />,
  );
}

// Click a node INSIDE the Level-2 mindmap by its text. The focused question's
// text also appears in the hole header, so scope to the rf-node wrappers (the
// mock renders each node as data-testid="rf-node") to click the real node.
async function clickNode(user: ReturnType<typeof userEvent.setup>, text: string) {
  const nodes = await screen.findAllByTestId("rf-node");
  const target = nodes.find((n) => within(n).queryByText(text));
  if (!target) throw new Error(`no mindmap node with text: ${text}`);
  await user.click(target);
}

describe("ExplorationView", () => {
  it("renders the plain empty state when there are no roots", async () => {
    mockGetExploration.mockResolvedValue({ leads: [], danglingSourceIds: [], edges: [] });
    render(<ExplorationView projectId={nextPid()} references={[]} />);

    expect(await screen.findByText("这里还是空的")).toBeInTheDocument();
    // no rabbit metaphor in the copy — plain guidance only
    expect(screen.getByText(/在上面记下一个你想弄清楚的问题/)).toBeInTheDocument();
    // no projectTitle passed → no driving-question seed offered
    expect(screen.queryByRole("button", { name: "用我的研究问题开始" })).toBeNull();
  });

  // ---- GVf · seed the driving question (empty state) ----

  it("the empty state offers the driving question; clicking it fills the input, then 记下问题 creates it via createLead", async () => {
    mockGetExploration.mockResolvedValue({ leads: [], danglingSourceIds: [], edges: [] });
    const projectId = nextPid();
    const question = "中国是否让地球变得更可持续？";
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[]} projectTitle={question} />);
    await screen.findByText("这里还是空的");

    const input = screen.getByPlaceholderText(/记一个你想弄清楚的问题/) as HTMLInputElement;
    expect(input.value).toBe("");

    // 铁律①: the pre-fill only fills the input — nothing is created yet
    await user.click(screen.getByRole("button", { name: "用我的研究问题开始" }));
    expect(input.value).toBe(question);
    expect(mockCreateLead).not.toHaveBeenCalled();

    // she still has to confirm/edit + click 记下问题 herself
    await user.click(screen.getByRole("button", { name: "记下问题" }));

    await waitFor(() => {
      expect(mockCreateLead).toHaveBeenCalledWith(projectId, question);
    });
  });

  it("the driving-question seed is editable before it's created", async () => {
    mockGetExploration.mockResolvedValue({ leads: [], danglingSourceIds: [], edges: [] });
    const projectId = nextPid();
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[]} projectTitle="原始研究问题" />);
    await screen.findByText("这里还是空的");

    await user.click(screen.getByRole("button", { name: "用我的研究问题开始" }));
    const input = screen.getByPlaceholderText(/记一个你想弄清楚的问题/);
    await user.clear(input);
    await user.type(input, "改过的问题");
    await user.click(screen.getByRole("button", { name: "记下问题" }));

    await waitFor(() => {
      expect(mockCreateLead).toHaveBeenCalledWith(projectId, "改过的问题");
    });
  });

  // ---- GVf · promote a reading note into a question ----

  it("从笔记新建问题 lists references with a readingNote (note text + source title); selecting one calls createLead with sourceReferenceId", async () => {
    const noteRef = makeRef({
      id: "rNote",
      title: "一篇留过笔记的来源",
      readingNote: "碳排放和可持续发展之间似乎有矛盾，值得细挖。",
    });
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD], danglingSourceIds: [], edges: [] });
    const projectId = nextPid();
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[NASA_REF, noteRef]} />);
    await screen.findByText(ROOT_LEAD.text);

    await user.click(screen.getByRole("button", { name: "从笔记新建问题" }));
    expect(await screen.findByText(noteRef.readingNote!)).toBeInTheDocument();
    expect(screen.getByText(noteRef.title)).toBeInTheDocument();
    // NASA_REF has no readingNote — it must not appear in the picker
    expect(screen.queryByText(NASA_REF.title)).toBeNull();

    await user.click(screen.getByText(noteRef.readingNote!));

    await waitFor(() => {
      expect(mockCreateLead).toHaveBeenCalledWith(projectId, noteRef.readingNote, { sourceReferenceId: noteRef.id });
    });
    await waitFor(() => expect(mockGetExploration).toHaveBeenCalledTimes(2));
  });

  it("从笔记新建问题 shows a gentle empty state when there are no reading notes", async () => {
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD], danglingSourceIds: [], edges: [] });
    const user = userEvent.setup();
    render(<ExplorationView projectId={nextPid()} references={[NASA_REF]} />);
    await screen.findByText(ROOT_LEAD.text);

    await user.click(screen.getByRole("button", { name: "从笔记新建问题" }));
    expect(await screen.findByText("还没有阅读笔记")).toBeInTheDocument();
    expect(mockCreateLead).not.toHaveBeenCalled();
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

  it("the MAP renders a card node per root with its 文献 x 篇 paper tally", async () => {
    // ROOT_LEAD has one adopted paper (CHILD_PAPER); SECOND_ROOT has none.
    mockGetExploration.mockResolvedValue({
      leads: [ROOT_LEAD, CHILD_PAPER, SECOND_ROOT],
      danglingSourceIds: [],
      edges: [],
    });
    const { container } = render(<ExplorationView projectId={nextPid()} references={[NASA_REF]} />);

    // both roots show as map nodes; the child paper is NOT a root on the map
    expect(await screen.findByText(ROOT_LEAD.text)).toBeInTheDocument();
    expect(screen.getByText(SECOND_ROOT.text)).toBeInTheDocument();
    expect(screen.queryByText(CHILD_PAPER.text)).toBeNull();
    // the paper tally: ROOT_LEAD → 文献 1 篇, SECOND_ROOT → 文献 0 篇
    expect(screen.getByText("文献 1 篇")).toBeInTheDocument();
    expect(screen.getByText("文献 0 篇")).toBeInTheDocument();

    // GVe (design rebuild, spec §18): each root node carries a macaron ordinal
    // theme — a real token from ui/tokens.ts MACARONS, and adjacent roots get
    // DIFFERENT macarons (the anti-collision guarantee, at the DOM level).
    const themedNodes = container.querySelectorAll<HTMLElement>("[data-theme]");
    expect(themedNodes.length).toBe(2);
    const themeKeys = Array.from(themedNodes).map((el) => el.dataset.theme);
    for (const key of themeKeys) expect(Object.keys(MACARONS)).toContain(key);
    expect(new Set(themeKeys).size).toBe(2);
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

  // ---- GVd · ONE unified sidebar: 'ai' (coach) ⇄ 'node' ⇄ 'results' ----

  it("the sidebar DEFAULTS to the coach ('ai') at Level-1, and stays on it at Level-2 until a node is clicked", async () => {
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD, CHILD_PAPER], danglingSourceIds: [], edges: [] });
    const user = userEvent.setup();
    renderView(nextPid(), [NASA_REF]);

    // Level-1 (map): the coach IS the right sidebar
    expect(await screen.findByTestId("coach-slot")).toBeInTheDocument();

    // Level-2 (inside a question): still the coach — nothing is auto-selected
    await zoomInto(user, ROOT_LEAD.text);
    expect(screen.getByTestId("coach-slot")).toBeInTheDocument();
    // no node metadata shown yet
    expect(screen.queryByText(NASA_REF.title)).toBeNull();
  });

  it("clicking a paper node → 'node' shows its title, abstract, reading-status + the three find-actions; ← 印记 returns to the coach", async () => {
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD, CHILD_PAPER], danglingSourceIds: [], edges: [] });
    const user = userEvent.setup();
    const { container } = renderView(nextPid(), [NASA_REF]);

    await zoomInto(user, ROOT_LEAD.text);
    // the mindmap renders the adopted paper node (a paper is NEVER a root — it
    // hangs inside the question)
    expect(screen.getByText(CHILD_PAPER.text)).toBeInTheDocument();

    // click the paper node → the sidebar becomes 'node' with ITS metadata
    await clickNode(user, CHILD_PAPER.text);
    expect(await screen.findByText(NASA_REF.title)).toBeInTheDocument();

    // GVe (design rebuild, spec §18): selecting a node rings it in ACCENT, never
    // a macaron theme color — accent is reserved for selection.
    const selectedNode = container.querySelector<HTMLElement>('[data-selected="true"]');
    expect(selectedNode).not.toBeNull();
    expect(selectedNode!.getAttribute("style") ?? "").toContain("--mk-accent");
    // metadata is shown as clear labeled fields (not one grey run-on line)
    expect(screen.getByText("作者")).toBeInTheDocument();
    expect(screen.getByText("NASA")).toBeInTheDocument();
    expect(screen.getByText("年份")).toBeInTheDocument();
    expect(screen.getByText("2023")).toBeInTheDocument();
    expect(screen.getByText("期刊")).toBeInTheDocument();
    expect(screen.getByText("Nature Sustainability")).toBeInTheDocument();
    // the full abstract is shown
    expect(screen.getByText(/全球植被覆盖变化/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "打开原文" })).toHaveAttribute("href", NASA_REF.url);
    // the three find-actions a paper offers
    expect(screen.getByRole("button", { name: "找相似文献" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "找它引用的文献" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "找引用它的文献" })).toBeInTheDocument();
    // the coach is hidden while a node is selected
    expect(screen.queryByTestId("coach-slot")).toBeNull();

    // ← 印记 returns to the coach
    await user.click(screen.getByRole("button", { name: "← 印记" }));
    expect(await screen.findByTestId("coach-slot")).toBeInTheDocument();
    expect(screen.queryByText(NASA_REF.title)).toBeNull();
  });

  it("找它引用的文献 on a paper node digs via digExploration({leadId, mode:'citation'}) and lists results in 'results'", async () => {
    const projectId = nextPid();
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD, CHILD_PAPER], danglingSourceIds: [], edges: [] });
    const user = userEvent.setup();
    renderView(projectId, [NASA_REF]);
    await zoomInto(user, ROOT_LEAD.text);
    await clickNode(user, CHILD_PAPER.text);

    // 克制: nothing dug just from selecting the node
    expect(mockDigExploration).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "找它引用的文献" }));

    await waitFor(() => {
      expect(mockDigExploration).toHaveBeenCalledWith(projectId, { leadId: CHILD_PAPER.id, mode: "citation" });
    });
    // results render with the candidate's bibliographic line
    expect(await screen.findByText(CANDIDATE.title)).toBeInTheDocument();
    expect(screen.getByText("Li 等 · 2023 · Nature Sustainability")).toBeInTheDocument();
  });

  it("a question node shows 来源 / 论文列表 / 记于; a paper in the list is clickable → selects that paper", async () => {
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD, CHILD_PAPER], danglingSourceIds: [], edges: [] });
    const user = userEvent.setup();
    renderView(nextPid(), [NASA_REF]);

    await zoomInto(user, ROOT_LEAD.text);
    // click the CENTER question node → its metadata
    await clickNode(user, ROOT_LEAD.text);

    // 来源 (manual → human-friendly), 记于 (createdAt → YYYY-MM-DD), 论文列表 header
    expect(await screen.findByText("手动记下")).toBeInTheDocument();
    expect(screen.getByText("2026-08-01")).toBeInTheDocument();
    expect(screen.getByText("论文列表")).toBeInTheDocument();

    // the descendant paper appears in the 论文列表 by its reference title, and is
    // clickable → selects that paper (sidebar becomes the paper's 'node')
    await user.click(screen.getByRole("button", { name: NASA_REF.title }));
    // the paper's metadata renders as clear labeled fields
    expect(await screen.findByText("Nature Sustainability")).toBeInTheDocument();
    expect(screen.getByText("NASA")).toBeInTheDocument();
    expect(screen.getByText("2023")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "找引用它的文献" })).toBeInTheDocument();
  });

  it("找相似文献 on a question node digs via digExploration({leadId, mode:'similar'}) and lists results", async () => {
    const projectId = nextPid();
    const user = userEvent.setup();
    renderView(projectId, [NASA_REF]);
    await zoomInto(user, ROOT_LEAD.text);
    await clickNode(user, ROOT_LEAD.text);

    // 克制: nothing dug just from selecting the question
    expect(mockDigExploration).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "找相似文献" }));

    await waitFor(() => {
      expect(mockDigExploration).toHaveBeenCalledWith(projectId, { leadId: ROOT_LEAD.id, mode: "similar" });
    });
    expect(await screen.findByText(CANDIDATE.title)).toBeInTheDocument();
  });

  it("a question node's keyword box digs by keyword via digExploration({keyword, mode:'similar'})", async () => {
    const projectId = nextPid();
    const user = userEvent.setup();
    renderView(projectId, [NASA_REF]);
    await zoomInto(user, ROOT_LEAD.text);
    await clickNode(user, ROOT_LEAD.text);

    await user.type(screen.getByPlaceholderText("换个词找…"), "可再生能源");
    await user.click(screen.getByRole("button", { name: "找" }));

    await waitFor(() => {
      expect(mockDigExploration).toHaveBeenCalledWith(projectId, { keyword: "可再生能源", mode: "similar" });
    });
  });

  it("采纳 in 'results' adopts the candidate UNDER the dug node (parentLeadId = that node's id) — papers never roots, and reloads the library so the new paper's bib is available", async () => {
    const projectId = nextPid();
    const onLibraryChanged = vi.fn();
    const user = userEvent.setup();
    renderView(projectId, [NASA_REF], onLibraryChanged);
    await zoomInto(user, ROOT_LEAD.text);
    await clickNode(user, ROOT_LEAD.text);

    await user.click(screen.getByRole("button", { name: "找相似文献" }));
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
    // the library is reloaded too — else the new reference's bib would be absent
    // and the paper node's metadata would render every field as 「—」
    await waitFor(() => expect(onLibraryChanged).toHaveBeenCalled());
  });

  // Job 3 (studio agentic restyle 2026-08): the per-question paper-dig loading
  // state renders the RabbitHoleLoader scene (from @/ui) instead of a plain
  // spinner/text — assert via its caption, which stays put under the loader.
  it("shows the rabbit-hole loader while digging for papers", async () => {
    const projectId = nextPid();
    let resolveDig!: (v: { candidates: DigCandidate[] }) => void;
    mockDigExploration.mockImplementationOnce(
      () => new Promise((resolve) => { resolveDig = resolve; }),
    );
    const user = userEvent.setup();
    renderView(projectId, [NASA_REF]);
    await zoomInto(user, ROOT_LEAD.text);
    await clickNode(user, ROOT_LEAD.text);

    await user.click(screen.getByRole("button", { name: "找相似文献" }));

    // mid-flight: the rabbit-hole scene shows, not the old plain-text spinner
    expect(await screen.findByText("印记在找相关论文……")).toBeInTheDocument();
    expect(mockAdoptCandidate).not.toHaveBeenCalled();

    await act(async () => {
      resolveDig({ candidates: [CANDIDATE] });
    });
    await screen.findByText(CANDIDATE.title);
    expect(screen.queryByText("印记在找相关论文……")).toBeNull();
  });

  it("丢弃 removes a candidate from 'results' with no server call", async () => {
    const user = userEvent.setup();
    renderView(nextPid(), [NASA_REF]);
    await zoomInto(user, ROOT_LEAD.text);
    await clickNode(user, ROOT_LEAD.text);

    await user.click(screen.getByRole("button", { name: "找相似文献" }));
    await screen.findByText(CANDIDATE.title);

    await user.click(screen.getByRole("button", { name: "丢弃" }));

    await waitFor(() => expect(screen.queryByText(CANDIDATE.title)).toBeNull());
    // discard is client-only — it never adopts
    expect(mockAdoptCandidate).not.toHaveBeenCalled();
  });

  it("'results' offers a ← 印记 back to the coach and a ← 返回 back to the node", async () => {
    const user = userEvent.setup();
    renderView(nextPid(), [NASA_REF]);
    await zoomInto(user, ROOT_LEAD.text);
    await clickNode(user, ROOT_LEAD.text);

    await user.click(screen.getByRole("button", { name: "找相似文献" }));
    await screen.findByText(CANDIDATE.title);

    // ← 返回 goes back to the node (its find-actions reappear, results gone)
    await user.click(screen.getByRole("button", { name: "← 返回" }));
    expect(await screen.findByRole("button", { name: "找相似文献" })).toBeInTheDocument();
    expect(screen.queryByText(CANDIDATE.title)).toBeNull();

    // dig again, then ← 印记 returns all the way to the coach
    await user.click(screen.getByRole("button", { name: "找相似文献" }));
    await screen.findByText(CANDIDATE.title);
    await user.click(screen.getByRole("button", { name: "← 印记" }));
    expect(await screen.findByTestId("coach-slot")).toBeInTheDocument();
  });

  it("× on a node inside a question opens a confirm modal; confirming calls deleteLead (no 剪枝)", async () => {
    const projectId = nextPid();
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD, CHILD_PAPER], danglingSourceIds: [], edges: [] });
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[NASA_REF]} />);
    await zoomInto(user, ROOT_LEAD.text);

    // nodes carry an × (root + paper); the paper node's × is the last one.
    const deletes = screen.getAllByRole("button", { name: "删除这个节点" });
    await user.click(deletes[deletes.length - 1]!);

    // 克制: pressing × alone does not delete — it asks first
    expect(await screen.findByText("删除这一项？")).toBeInTheDocument();
    expect(mockDeleteLead).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "删除" }));
    await waitFor(() => expect(mockDeleteLead).toHaveBeenCalledWith(projectId, CHILD_PAPER.id));
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

  // ---- drag-to-connect (student-drawn relation) ----

  it("drag-connecting two roots opens the label picker; picking a label calls createEdge and refetches", async () => {
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD, SECOND_ROOT], danglingSourceIds: [], edges: [] });
    const projectId = nextPid();
    const user = userEvent.setup();
    render(<ExplorationView projectId={projectId} references={[]} />);
    await screen.findByText(ROOT_LEAD.text);

    // simulate the drag from ROOT_LEAD's handle onto SECOND_ROOT
    await fireConnect(ROOT_LEAD.id, SECOND_ROOT.id);

    // the closed-vocabulary label picker appears; nothing persisted yet (克制)
    expect(await screen.findByText("这两个问题是什么关系？")).toBeInTheDocument();
    expect(mockCreateEdge).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "支持" }));

    // createEdge carries the dragged direction + chosen label, then refetch
    await waitFor(() => {
      expect(mockCreateEdge).toHaveBeenCalledWith(projectId, {
        fromLeadId: ROOT_LEAD.id,
        toLeadId: SECOND_ROOT.id,
        label: "支持",
      });
    });
    await waitFor(() => expect(mockGetExploration).toHaveBeenCalledTimes(2));
  });

  it("drag-connecting a pair that already has an edge is a no-op (never fires createEdge — would 500 on the UNIQUE constraint)", async () => {
    // an edge ROOT_LEAD → SECOND_ROOT already exists in this direction
    mockGetExploration.mockResolvedValue({
      leads: [ROOT_LEAD, SECOND_ROOT],
      danglingSourceIds: [],
      edges: [CONFIRMED_EDGE],
    });
    render(<ExplorationView projectId={nextPid()} references={[]} />);
    await screen.findByText(ROOT_LEAD.text);

    await fireConnect(ROOT_LEAD.id, SECOND_ROOT.id);

    // no picker, no request — the duplicate is guarded client-side with a note
    expect(screen.queryByText("这两个问题是什么关系？")).toBeNull();
    expect(mockCreateEdge).not.toHaveBeenCalled();
    expect(screen.getByText("这两个问题已经连过了。")).toBeInTheDocument();
  });
});
