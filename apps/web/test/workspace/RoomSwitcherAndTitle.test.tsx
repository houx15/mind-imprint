import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

/**
 * Task 8: the 5 room tabs must read as real navigation (icon + ≥14px label +
 * legible inactive contrast, not `Segmented`'s 12px hint styling), and the
 * project title must be reachable in full wherever it's clamped/truncated
 * (top-bar tap-to-reveal + tooltip, home/directory card `title` attr).
 */

import { RoomSwitcher } from "@/workspace/RoomSwitcher";
import { BLOCK_META } from "@/workspace/Icon";

describe("RoomSwitcher", () => {
  it("renders each of the 5 tabs with an icon (svg) and a label at text-mk-body (not text-mk-small)", () => {
    render(<RoomSwitcher value="plan" onChange={vi.fn()} />);

    for (const b of BLOCK_META) {
      const button = screen.getByRole("button", { name: b.label });
      // Icon.
      expect(button.querySelector("svg")).not.toBeNull();
      // Label element carries the body-weight class, not the small/hint one.
      const label = screen.getByText(b.label);
      expect(label.className).toContain("text-mk-body");
      expect(label.className).not.toContain("text-mk-small");
    }
  });

  it("marks the active tab with the white thumb and keeps inactive tabs legible (not text-mk-muted)", () => {
    render(<RoomSwitcher value="writing" onChange={vi.fn()} />);

    const active = screen.getByRole("button", { name: "写作" });
    expect(active).toHaveAttribute("aria-pressed", "true");
    expect(active.className).toContain("text-mk-ink");
    expect(active.className).toContain("bg-mk-surface");

    const inactive = screen.getByRole("button", { name: "提案" });
    expect(inactive).toHaveAttribute("aria-pressed", "false");
    expect(inactive.className).toContain("text-mk-ink/70");
    expect(inactive.className).not.toContain("text-mk-muted");
  });

  it("calls onChange with the clicked tab's key", async () => {
    const onChange = vi.fn();
    render(<RoomSwitcher value="plan" onChange={onChange} />);

    await userEvent.click(screen.getByRole("button", { name: "阅读" }));
    expect(onChange).toHaveBeenCalledWith("reading");
  });

  it("highlights no tab when value is empty (chat-only state)", () => {
    render(<RoomSwitcher value="" onChange={vi.fn()} />);
    for (const b of BLOCK_META) {
      expect(screen.getByRole("button", { name: b.label })).toHaveAttribute("aria-pressed", "false");
    }
  });
});

// ---------------------------------------------------------------------------
// Top-bar title (WorkspaceContainer's TopBar): full title via tooltip +
// tap-to-reveal. Reuses the StudioShell test harness's mocks (same shell
// mount path) with a title long enough to truncate.
// ---------------------------------------------------------------------------

vi.mock("@/workspace/Directory", () => ({
  Directory: ({ onOpen }: { onOpen: (id: string) => void }) => (
    <div data-testid="directory">
      <button type="button" onClick={() => onOpen("p1")}>
        open p1
      </button>
    </div>
  ),
}));

vi.mock("@/workspace/blocks/PlanBlock", () => ({
  PlanBlock: () => <div data-testid="plan-block" />,
}));
vi.mock("@/workspace/blocks/ReadingBlock", () => ({ ReadingBlock: () => <div data-testid="reading-block" /> }));
vi.mock("@/workspace/blocks/WritingBlock", () => ({ WritingBlock: () => <div data-testid="writing-block" /> }));
vi.mock("@/workspace/blocks/ReviewBlock", () => ({ ReviewBlock: () => <div data-testid="review-block" /> }));
vi.mock("@/studio/reading/ReadingRoom", () => ({ ReadingRoom: () => <div data-testid="reading-room" /> }));

const LONG_TITLE =
  "中国是否让地球变得更可持续？一个关于碳排放、可再生能源投资与全球气候治理格局的深入探究与论证分析";

const getWorkspace = vi.fn();
vi.mock("@/workspace/api/workspace", () => ({
  getWorkspace: (...args: unknown[]) => getWorkspace(...args),
  getStudioState: vi.fn(async () => ({
    stage: "plan_generation" as const,
    openTool: "plan" as const,
    widthTier: "half" as const,
    reference: [] as never[],
    updatedAtTurn: 0,
    started: true,
  })),
  getPlan: vi.fn(async () => []),
  getCoachHistory: vi.fn(async () => ({ messages: [], hasMore: false, recap: null, nextCursor: null })),
  postProjectSummary: vi.fn(async () => ""),
  patchReference: vi.fn(async () => ({})),
  getLibrary: vi.fn(async () => ({ collections: [], references: [] })),
  getSnippets: vi.fn(async () => []),
  getAnnotations: vi.fn(async () => []),
}));

import { WorkspaceContainer } from "@/workspace/WorkspaceContainer";

describe("WorkspaceContainer TopBar title", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    getWorkspace.mockImplementation(async (id: string) => ({
      id,
      title: LONG_TITLE,
      qualification: "拓展论文 EE",
      status: "working" as const,
      proposal: { objective: "", reason: "", activities: "", resources: "", counterpoints: "" },
      createdAt: "2026-08-01T00:00:00Z",
      writingFinished: false,
    }));
  });

  it("renders the full title as a native title attr, truncated by default, and toggles wrap on click", async () => {
    render(<WorkspaceContainer />);
    await userEvent.click(screen.getByText("open p1"));

    const titleEl = await screen.findByRole("button", { name: "展开完整标题" });
    expect(titleEl).toHaveAttribute("title", LONG_TITLE);
    expect(titleEl).toHaveTextContent(LONG_TITLE);
    expect(titleEl.className).toContain("truncate");
    expect(titleEl.className).not.toContain("whitespace-normal");

    await userEvent.click(titleEl);
    expect(titleEl.className).not.toContain("truncate");
    expect(titleEl.className).toContain("whitespace-normal");

    await userEvent.click(titleEl);
    expect(titleEl.className).toContain("truncate");
  });
});
