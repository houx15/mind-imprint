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
