import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createPortal } from "react-dom";
import { useStudioAiSlot } from "@/studio/ai/StudioAiSlot";

/**
 * Studio shell (Task 4, spec §17): the top bar (主页 capsule + room switcher)
 * + the constant, flippable, collapsible `AiPanel` that replaced the old
 * left `Rail`. `WorkspaceContainer.test.tsx` covers the pre-existing
 * open/deep-link seams (fully mocking every room) — this file is
 * shell-focused: it verifies the chrome itself, AND the room→panel PORTAL
 * contract by standing in a fake `PlanBlock` that follows the exact same
 * contract the real one now does (`useStudioAiSlot()` + `createPortal` —
 * see `workspace/blocks/PlanBlock.tsx`'s `FormingPhase`), so the shell's
 * plumbing is exercised without dragging in PlanBlock's own network/API
 * dependencies.
 */

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
  PlanBlock: ({ projectId }: { projectId: string }) => {
    const slot = useStudioAiSlot();
    return (
      <div data-testid="plan-block">
        work:{projectId}
        {slot && createPortal(<div data-testid="plan-coach">coach:{projectId}</div>, slot)}
      </div>
    );
  },
}));
vi.mock("@/workspace/blocks/ReadingBlock", () => ({ ReadingBlock: () => <div data-testid="reading-block" /> }));
vi.mock("@/workspace/blocks/WritingBlock", () => ({ WritingBlock: () => <div data-testid="writing-block" /> }));
vi.mock("@/workspace/blocks/ReviewBlock", () => ({ ReviewBlock: () => <div data-testid="review-block" /> }));
vi.mock("@/studio/reading/ReadingRoom", () => ({ ReadingRoom: () => <div data-testid="reading-room" /> }));

const getWorkspace = vi.fn();
vi.mock("@/workspace/api/workspace", () => ({
  getWorkspace: (...args: unknown[]) => getWorkspace(...args),
  // 印记's resume directive (Task 8): the shell lands on the room it names.
  // These shell tests exercise the plan room, so return openTool "plan".
  getStudioState: vi.fn(async () => ({
    stage: "plan_generation" as const,
    openTool: "plan" as const,
    widthTier: "half" as const,
    reference: [] as never[],
    updatedAtTurn: 0,
    // Task 6 (start gate): this shell suite exercises an already-resumed
    // project (the switcher/room chrome itself), not the gate — started:true
    // keeps the pre-existing tabs-render-immediately behavior.
    started: true,
  })),
  getPlan: vi.fn(async () => []),
  getCoachHistory: vi.fn(async () => ({ messages: [], hasMore: false, recap: null, nextCursor: null })),
  postProjectSummary: vi.fn(async () => ""),
  patchReference: vi.fn(async () => ({})),
  // ReferencePanel (writing room's left sub-pane) fetches these eagerly on
  // mount — stubbed empty since these shell tests drive the room switcher,
  // not the writing reference content.
  getLibrary: vi.fn(async () => ({ collections: [], references: [] })),
  getSnippets: vi.fn(async () => []),
  getAnnotations: vi.fn(async () => []),
}));

import { WorkspaceContainer } from "@/workspace/WorkspaceContainer";

function fakeWorkspace(id: string) {
  return {
    id,
    title: `项目 ${id}`,
    qualification: "拓展论文 EE",
    status: "working" as const,
    proposal: { objective: "", reason: "", activities: "", resources: "", counterpoints: "" },
    createdAt: "2026-08-01T00:00:00Z",
    writingFinished: false,
  };
}

async function openProject() {
  render(<WorkspaceContainer />);
  await userEvent.click(screen.getByText("open p1"));
  await screen.findByTestId("plan-block");
}

describe("Studio shell (top bar + constant AiPanel)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    getWorkspace.mockImplementation(async (id: string) => fakeWorkspace(id));
  });

  it("renders the top bar (主页 capsule + 4-room switcher) and a constant AiPanel once a project opens", async () => {
    await openProject();

    expect(screen.getByRole("button", { name: /主页/ })).toBeInTheDocument();
    for (const label of ["立题", "管理", "阅读", "写作", "回顾"]) {
      expect(screen.getByRole("button", { name: label })).toBeInTheDocument();
    }
    // The AiPanel chrome (Task 2) is always present alongside the room, not
    // gated behind any per-room condition.
    expect(screen.getByRole("button", { name: "切换 AI 面板左右" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "折叠 AI 面板" })).toBeInTheDocument();
  });

  it("switches rooms via the top-bar room switcher", async () => {
    await openProject();
    expect(screen.getByTestId("plan-block")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "写作" }));
    expect(await screen.findByTestId("writing-block")).toBeInTheDocument();
    expect(screen.queryByTestId("plan-block")).not.toBeInTheDocument();
  });

  it("portals the active room's coach content into the AiPanel (the plan room's reference integration)", async () => {
    await openProject();

    const coach = await screen.findByTestId("plan-coach");
    // It really landed inside the AiPanel body, not just anywhere in the DOM:
    // the panel header carries the flip/collapse controls, so walk up to a
    // shared ancestor and confirm both live under it.
    const flipButton = screen.getByRole("button", { name: "切换 AI 面板左右" });
    const panel = flipButton.closest("div")?.parentElement;
    expect(panel).toBeTruthy();
    expect(within(panel as HTMLElement).getByTestId("plan-coach")).toBe(coach);
  });

  it("flip swaps which side of <main> the AiPanel renders on", async () => {
    await openProject();

    const main = screen.getByRole("main");
    const row = main.parentElement as HTMLElement;
    // Default side is "left" (agentic studio: 印记 is the constant left
    // companion) → the AiPanel comes first, <main> last in DOM order.
    expect(row.lastElementChild).toBe(main);

    await userEvent.click(screen.getByRole("button", { name: "切换 AI 面板左右" }));

    // Flipped to "right" → <main> now comes first, panel last.
    expect(row.firstElementChild).toBe(main);
    expect(row.lastElementChild).not.toBe(main);
  });

  it("collapse hides the panel body (coach content unmounts) and shows an expand control", async () => {
    await openProject();
    await screen.findByTestId("plan-coach");

    await userEvent.click(screen.getByRole("button", { name: "折叠 AI 面板" }));

    expect(screen.queryByTestId("plan-coach")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "展开 AI 面板" })).toBeInTheDocument();
  });

  it("the 主页 capsule returns to the Directory", async () => {
    await openProject();

    await userEvent.click(screen.getByRole("button", { name: /主页/ }));

    expect(await screen.findByTestId("directory")).toBeInTheDocument();
    expect(screen.queryByTestId("plan-block")).not.toBeInTheDocument();
  });
});
