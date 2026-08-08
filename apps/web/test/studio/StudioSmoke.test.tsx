import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createPortal } from "react-dom";
import { useStudioAiSlot } from "@/studio/ai/StudioAiSlot";

/**
 * Task 11 (studio integration smoke test): a full walk across the shared
 * studio shell — open a project, switch across all 4 rooms via the top-bar
 * switcher, and flip + collapse the constant AiPanel — verifying the three
 * things `StudioShell.test.tsx` (Task 4) only checks in isolation for the
 * plan room:
 *
 *  1. the panel's side/collapsed state PERSISTS to localStorage
 *     (`mk-studio-ai-side` / `mk-studio-ai-collapsed`) and survives a room
 *     switch (it lives in WorkspaceContainer, not any one room);
 *  2. the ACTIVE room's coach — and only the active room's — is what's
 *     portaled into the panel when expanded, and nothing is portaled when
 *     collapsed. ALL FOUR rooms (Task 3, P2a: 阅读 joined the others) portal
 *     into the ONE constant panel the same way — no room owns its own coach
 *     column any more;
 *  3. switching rooms swaps the <main> work content independently of the
 *     panel's own side/collapsed state.
 *
 * Each mocked room block follows the exact portal contract the real ones use
 * (`useStudioAiSlot()` + `createPortal`, see PlanBlock/WritingBlock/
 * ReviewBlock/ReadingBlock) so the shell's plumbing is exercised without
 * dragging in any room's own network/API dependencies.
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

function makeRoomBlock(key: string) {
  return ({ projectId }: { projectId: string }) => {
    const slot = useStudioAiSlot();
    return (
      <div data-testid={`${key}-work`}>
        work:{key}:{projectId}
        {slot && createPortal(<div data-testid={`${key}-coach`}>coach:{key}:{projectId}</div>, slot)}
      </div>
    );
  };
}

vi.mock("@/workspace/blocks/PlanBlock", () => ({ PlanBlock: makeRoomBlock("plan") }));
vi.mock("@/workspace/blocks/WritingBlock", () => ({ WritingBlock: makeRoomBlock("writing") }));
vi.mock("@/workspace/blocks/ReviewBlock", () => ({ ReviewBlock: makeRoomBlock("reflection") }));
// Task 3 (P2a): ReadingBlock now portals into the shell panel exactly like
// every other room — its own FloatingCoach (find_sources) column is gone.
vi.mock("@/workspace/blocks/ReadingBlock", () => ({ ReadingBlock: makeRoomBlock("reading") }));
vi.mock("@/studio/reading/ReadingRoom", () => ({ ReadingRoom: () => <div data-testid="reading-room" /> }));

const getWorkspace = vi.fn();
vi.mock("@/workspace/api/workspace", () => ({
  getWorkspace: (...args: unknown[]) => getWorkspace(...args),
  getPlan: vi.fn(async () => []),
  getCoachHistory: vi.fn(async () => ({ messages: [], hasMore: false, recap: null, nextCursor: null })),
  postProjectSummary: vi.fn(async () => ""),
  patchReference: vi.fn(async () => ({})),
  // 印记's AI-managed status (Task 8). Resolve to the plan room so this smoke
  // test lands on the plan board (its walk then uses the switcher).
  getStudioState: vi.fn(async () => ({
    stage: "plan_generation",
    openTool: "plan",
    widthTier: "half",
    reference: [],
    updatedAtTurn: 0,
  })),
  // The container-owned send loop + note/card handlers (Task 9b) — stubbed so
  // the module resolves; this smoke test drives the switcher, not the chat.
  coach: vi.fn(async () => ({
    narrate: "",
    directive: { stage: "plan_generation", openTool: "plan", widthTier: "half", reference: [], updatedAtTurn: 0 },
    note: null,
    card: null,
    question: null,
    reviewRequested: false,
  })),
  putProposal: vi.fn(async (_id: string, p: unknown) => p),
  reflectProjectCard: vi.fn(async () => ({ cardInstanceId: "", reply: "", card: null })),
  dismissProposal: vi.fn(async () => {}),
  // ReferencePanel (writing room's left sub-pane) fetches these eagerly on
  // mount — stubbed empty since this smoke test drives the room switcher,
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
  await screen.findByTestId("plan-work");
}

async function switchRoom(label: string) {
  await userEvent.click(screen.getByRole("button", { name: label }));
}

describe("Studio integration smoke test (Task 11)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    getWorkspace.mockImplementation(async (id: string) => fakeWorkspace(id));
  });

  it("swaps both work content and the panel's coach content across all 4 rooms via the top-bar switcher", async () => {
    await openProject();

    // Lands in 立项 (plan): its work AND coach are both present.
    expect(screen.getByTestId("plan-work")).toBeInTheDocument();
    expect(await screen.findByTestId("plan-coach")).toBeInTheDocument();

    // 阅读: work swaps, and — like every other room now (Task 3, P2a) — its
    // coach is portaled into the SAME constant AiPanel. No more exception.
    await switchRoom("阅读");
    expect(await screen.findByTestId("reading-work")).toBeInTheDocument();
    expect(await screen.findByTestId("reading-coach")).toBeInTheDocument();
    expect(screen.queryByTestId("plan-work")).not.toBeInTheDocument();
    expect(screen.queryByTestId("plan-coach")).not.toBeInTheDocument();
    // The constant panel's flip/collapse chrome stays present for 阅读 too.
    expect(screen.getByRole("button", { name: "切换 AI 面板左右" })).toBeInTheDocument();

    // 写作: both work and coach swap to the writing room's.
    await switchRoom("写作");
    expect(await screen.findByTestId("writing-work")).toBeInTheDocument();
    expect(await screen.findByTestId("writing-coach")).toBeInTheDocument();
    expect(screen.queryByTestId("reading-work")).not.toBeInTheDocument();

    // 回顾: both work and coach swap to the reflection room's.
    await switchRoom("回顾");
    expect(await screen.findByTestId("reflection-work")).toBeInTheDocument();
    expect(await screen.findByTestId("reflection-coach")).toBeInTheDocument();
    expect(screen.queryByTestId("writing-work")).not.toBeInTheDocument();
    expect(screen.queryByTestId("writing-coach")).not.toBeInTheDocument();
  });

  it("persists the panel's flip + collapse state to localStorage and across room switches", async () => {
    await openProject();

    const main = screen.getByRole("main");
    const row = main.parentElement as HTMLElement;
    // Default: side "left" (<main> last), expanded (no persisted keys yet).
    expect(row.lastElementChild).toBe(main);
    expect(localStorage.getItem("mk-studio-ai-side")).toBeNull();
    expect(localStorage.getItem("mk-studio-ai-collapsed")).toBeNull();

    // Flip while on the plan room → side "right" (<main> first).
    await userEvent.click(screen.getByRole("button", { name: "切换 AI 面板左右" }));
    expect(row.firstElementChild).toBe(main);
    expect(localStorage.getItem("mk-studio-ai-side")).toBe("right");

    // Switching rooms must not reset the flip — it lives in the container,
    // not in any one room.
    await switchRoom("写作");
    await screen.findByTestId("writing-work");
    expect(row.firstElementChild).toBe(main);
    expect(localStorage.getItem("mk-studio-ai-side")).toBe("right");

    // Collapse while on the writing room.
    await userEvent.click(screen.getByRole("button", { name: "折叠 AI 面板" }));
    expect(localStorage.getItem("mk-studio-ai-collapsed")).toBe("1");
    expect(screen.queryByTestId("writing-coach")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "展开 AI 面板" })).toBeInTheDocument();

    // Collapse persists across another room switch too, and the swapped-to
    // room's work still renders even though the panel is collapsed.
    await switchRoom("回顾");
    expect(await screen.findByTestId("reflection-work")).toBeInTheDocument();
    expect(screen.queryByTestId("reflection-coach")).not.toBeInTheDocument();
    expect(localStorage.getItem("mk-studio-ai-collapsed")).toBe("1");

    // Expanding again reconnects the slot to whichever room is now active
    // (reflection), not a stale one from before the collapse.
    await userEvent.click(screen.getByRole("button", { name: "展开 AI 面板" }));
    expect(await screen.findByTestId("reflection-coach")).toBeInTheDocument();
    expect(localStorage.getItem("mk-studio-ai-collapsed")).toBe("0");
  });
});
