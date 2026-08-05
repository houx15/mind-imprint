import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { DigCandidate, ExplorationLead, QuestionEdge, Reference } from "@mind-imprint/contracts";
import { ExplorationView } from "@/workspace/blocks/exploration/ExplorationView";

// A6 redesign: the view now calls a thin exploration client directly. Mock the
// exact functions the new component imports — getExploration, createLead,
// digExploration, adoptCandidate, patchLead (deleteLead is exported for parity
// but unused by this surface). Same module-mock pattern the old test used.
vi.mock("@/api/exploration", () => ({
  getExploration: vi.fn(),
  createLead: vi.fn(),
  digExploration: vi.fn(),
  adoptCandidate: vi.fn(),
  patchLead: vi.fn(),
  deleteLead: vi.fn(),
}));
// The 进入阅读室 path (a paper node's ⋯ menu) goes through workspace.enterReading;
// mocked so the component's import resolves and never hits the network in tests.
vi.mock("@/workspace/api/workspace", () => ({
  enterReading: vi.fn(),
  NoReadableContentError: class extends Error {},
}));

import { getExploration, createLead, digExploration, adoptCandidate, patchLead } from "@/api/exploration";

const mockGetExploration = vi.mocked(getExploration);
const mockCreateLead = vi.mocked(createLead);
const mockDigExploration = vi.mocked(digExploration);
const mockAdoptCandidate = vi.mocked(adoptCandidate);
const mockPatchLead = vi.mocked(patchLead);

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
// (a real edge can span them, and shared-paper 同源 hints can appear).
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
  mockPatchLead.mockResolvedValue({ ...ROOT_LEAD, status: "pruned" });
  mockDigExploration.mockResolvedValue({ candidates: [CANDIDATE] });
  mockAdoptCandidate.mockResolvedValue({
    lead: { ...CHILD_PAPER, id: "lead9", connectedReferenceId: "r9" },
    reference: { ...NASA_REF, id: "r9" },
  });
});

// Each test uses a distinct projectId: the component persists map⇄hole zoom in
// a module-level Map keyed by projectId, so a fresh id guarantees a clean start
// (default = map overview) regardless of test order.
let pid = 0;
const nextPid = () => `p-${++pid}`;

// From the map, zoom into a root question node → its "inside" (the A6 tree).
// The node card is a button labelled with the question text.
async function zoomInto(user: ReturnType<typeof userEvent.setup>, text: string) {
  await user.click(await screen.findByRole("button", { name: new RegExp(text.slice(0, 8)) }));
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

  it("the MAP renders a node card per root question", async () => {
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD, SECOND_ROOT], danglingSourceIds: [], edges: [] });
    render(<ExplorationView projectId={nextPid()} references={[]} />);

    // both roots show as clickable map nodes; children are NOT shown on the map
    expect(await screen.findByRole("button", { name: new RegExp(ROOT_LEAD.text.slice(0, 8)) })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: new RegExp(SECOND_ROOT.text.slice(0, 8)) })).toBeInTheDocument();
  });

  it("the MAP renders a confirmed edge's label between two roots", async () => {
    mockGetExploration.mockResolvedValue({
      leads: [ROOT_LEAD, SECOND_ROOT],
      danglingSourceIds: [],
      edges: [CONFIRMED_EDGE],
    });
    render(<ExplorationView projectId={nextPid()} references={[]} />);

    await screen.findByRole("button", { name: new RegExp(ROOT_LEAD.text.slice(0, 8)) });
    // the closed-vocabulary relation word renders as the edge's midpoint chip
    expect(screen.getByText("反驳/张力")).toBeInTheDocument();
  });

  it("draws a faint 同源 hint line when two roots share an adopted paper", async () => {
    // both roots have a child paper connected to the SAME reference → 同源 pair
    const childUnderSecond: ExplorationLead = {
      ...CHILD_PAPER,
      id: "lead4",
      parentLeadId: SECOND_ROOT.id,
      connectedReferenceId: NASA_REF.id,
    };
    mockGetExploration.mockResolvedValue({
      leads: [ROOT_LEAD, CHILD_PAPER, SECOND_ROOT, childUnderSecond],
      danglingSourceIds: [],
      edges: [],
    });
    const { container } = render(<ExplorationView projectId={nextPid()} references={[NASA_REF]} />);

    await screen.findByRole("button", { name: new RegExp(ROOT_LEAD.text.slice(0, 8)) });
    expect(container.querySelector('[data-testid="samesource-hint"]')).not.toBeNull();
  });

  it("zooming into a node shows its subtree with nested paper children; 返回地图 zooms back out", async () => {
    mockGetExploration.mockResolvedValue({ leads: [ROOT_LEAD, CHILD_PAPER], danglingSourceIds: [], edges: [] });
    const user = userEvent.setup();
    render(<ExplorationView projectId={nextPid()} references={[NASA_REF]} />);

    await zoomInto(user, ROOT_LEAD.text);

    // inside the hole: the root plus its nested child paper (with its title badge)
    expect(screen.getByText(CHILD_PAPER.text)).toBeInTheDocument();
    expect(screen.getByText(`论文 · ${NASA_REF.title}`)).toBeInTheDocument();

    // back to the map: the node card reappears, the subtree child is gone
    await user.click(screen.getByRole("button", { name: "← 返回地图" }));
    await screen.findByRole("button", { name: new RegExp(ROOT_LEAD.text.slice(0, 8)) });
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
});
