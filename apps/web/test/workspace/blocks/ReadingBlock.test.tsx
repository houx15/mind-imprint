import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Reference } from "@mind-imprint/contracts";

// A single already-read source with no stage tag yet, carrying a saved brief
// (readingReason/readingFocus) so we can assert the inline stage picker persists
// via the full-replace reading-brief without wiping those fields.
const ref: Reference = {
  id: "r1",
  title: "Chen et al. (2019), Nature Sustainability",
  classification: "期刊论文",
  author: "Chen, C. et al.",
  credentials: "同行评议",
  year: "2019",
  url: "https://doi.org/x",
  tags: [],
  collectionId: null,
  credibility: "strong",
  evaluation: "",
  readingNote: "",
  decision: "use",
  pending: false,
  searchHints: [],
  materialId: "m1",
  notes: [],
  phaseTag: null,
  readingReason: "看它是否支持我的主张",
  readingFocus: "证据强度",
  takeaway: null,
};

vi.mock("@/workspace/api/workspace", () => ({
  getLibrary: vi.fn(async () => ({ collections: [], references: [ref] })),
  createCollection: vi.fn(),
  createReference: vi.fn(),
  patchReference: vi.fn(async () => {}),
  enterReading: vi.fn(),
  pasteContent: vi.fn(),
  coach: vi.fn(async () => ({ reply: "", proposal: null, linkOffer: null, dimSuggestion: null })),
  getCoachHistory: vi.fn(async () => []),
  NoReadableContentError: class extends Error {},
}));
vi.mock("@/api/reading", () => ({ putReadingBrief: vi.fn(async () => {}) }));
vi.mock("@/api/exploration", () => ({ getExploration: vi.fn(async () => ({ leads: [], danglingSourceIds: [] })) }));
vi.mock("@/workspace/blocks/exploration/ExplorationView", () => ({ ExplorationView: () => <div>graph-stub</div> }));
vi.mock("@/workspace/export", () => ({ exportAnnotatedBib: vi.fn() }));

import { ReadingBlock } from "@/workspace/blocks/ReadingBlock";
import { putReadingBrief } from "@/api/reading";

const mockBrief = vi.mocked(putReadingBrief);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("ReadingBlock · #4 graph-default + #2 stage tag", () => {
  it("opens on the 探索图谱 when the project already has sources (#4)", async () => {
    render(<ReadingBlock projectId="p1" title="T" setReadingSource={() => {}} />);
    // graph is the default view once there are references
    expect(await screen.findByText("graph-stub")).toBeInTheDocument();
    // the list grid header is not shown until the student switches to 列表
    expect(screen.queryByText("标题")).toBeNull();
  });

  // #18: 探索图谱 is now the default even for a brand-new EMPTY library — the
  // graph shows its own empty/线索 state (with ＋添加来源 right there), so
  // there's no reason to force 列表 first just because nothing exists yet.
  it("opens on the 探索图谱 even when the library is empty (#18)", async () => {
    const { getLibrary } = await import("@/workspace/api/workspace");
    vi.mocked(getLibrary).mockResolvedValueOnce({ collections: [], references: [] });
    render(<ReadingBlock projectId="p2" title="T" setReadingSource={() => {}} />);
    expect(await screen.findByText("graph-stub")).toBeInTheDocument();
    // 列表 is still one click away, with its own empty-state affordance
    await userEvent.click(screen.getByRole("button", { name: "列表" }));
    expect(await screen.findByText(/先加一篇来源/)).toBeInTheDocument();
  });

  it("stage picker persists via the reading brief, carrying reason/focus (#2)", async () => {
    render(<ReadingBlock projectId="p1" title="T" setReadingSource={() => {}} />);
    await screen.findByText("graph-stub");
    // switch to the list where the inline stage picker lives
    await userEvent.click(screen.getByRole("button", { name: "列表" }));
    const picker = await screen.findByLabelText("用于哪个阶段");
    await userEvent.selectOptions(picker, "支持论点");
    await waitFor(() =>
      expect(mockBrief).toHaveBeenCalledWith("p1", "r1", {
        readingReason: "看它是否支持我的主张",
        readingFocus: "证据强度",
        phaseTag: "支持论点",
      }),
    );
  });
});

// Q3 followup (2026-08): 探索图谱 has no reference-preview sidebar (unlike
// 列表), so the "找资料" coach docks as a permanent, always-visible right
// column there instead of hiding behind a floating chip. 列表 keeps the old
// floating-chip behavior since its Preview column already occupies that space.
describe("ReadingBlock · Q3 docked coach in 探索图谱", () => {
  it("docks the 找资料 coach as a visible right column in 探索图谱 (not a floating chip)", async () => {
    render(<ReadingBlock projectId="q3-graph" title="T" setReadingSource={() => {}} />);
    await screen.findByText("graph-stub");
    // docked: visible by default — no floating chip is needed to open it
    expect(await screen.findByText("印记 · 找资料")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /问印记 · 找资料/ })).toBeNull();
  });

  it("keeps the coach as a floating chip in 列表 (reference-preview sidebar intact)", async () => {
    render(<ReadingBlock projectId="q3-list" title="T" setReadingSource={() => {}} />);
    await screen.findByText("graph-stub");
    await userEvent.click(screen.getByRole("button", { name: "列表" }));
    // list view: the floating chip is the only way to open the coach
    expect(await screen.findByRole("button", { name: /问印记 · 找资料/ })).toBeInTheDocument();
  });
});

// #3 · when the library is empty, the coach greeting ACKNOWLEDGES the known
// project topic (a static interpolated string — no model call) instead of asking
// for it, and the empty-state copy names the topic too.
describe("ReadingBlock · #3 topic-aware empty state", () => {
  it("interpolates the project topic into the coach opener and empty copy", async () => {
    const { getLibrary } = await import("@/workspace/api/workspace");
    vi.mocked(getLibrary).mockResolvedValueOnce({ collections: [], references: [] });
    render(
      <ReadingBlock
        projectId="p1"
        title="中国是否让地球变得更可持续？"
        setReadingSource={() => {}}
      />,
    );
    // the coach greeting names the topic and keeps the 不替你搜 restraint — this
    // holds regardless of which view (#18: 探索图谱 is now the default even
    // for an empty library) is showing, since FloatingCoach's opener is keyed
    // off refs.length, not viewMode.
    expect(
      await screen.findByText(/你的题目是「中国是否让地球变得更可持续？」/),
    ).toBeInTheDocument();
    expect(screen.getByText(/但我不替你搜/)).toBeInTheDocument();
    // the 列表 empty-state copy references the topic too — switch there since
    // 探索图谱 (its own empty state) is the default now.
    await userEvent.click(screen.getByRole("button", { name: "列表" }));
    expect(await screen.findByText(/围绕「中国是否让地球变得更可持续？」/)).toBeInTheDocument();
  });

  it("falls back to the generic ask when no topic is set", async () => {
    const { getLibrary } = await import("@/workspace/api/workspace");
    vi.mocked(getLibrary).mockResolvedValueOnce({ collections: [], references: [] });
    render(<ReadingBlock projectId="p1" title="" setReadingSource={() => {}} />);
    expect(await screen.findByText(/跟我说说你的题目/)).toBeInTheDocument();
  });
});
