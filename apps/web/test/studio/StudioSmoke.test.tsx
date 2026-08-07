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
 *     collapsed. 阅读 is special: the shell's constant panel is HIDDEN for it
 *     (reading is a distinct full-screen surface with its own coach column),
 *     so there's no AiPanel chrome at all while 阅读 is active;
 *  3. switching rooms swaps the <main> work content independently of the
 *     panel's own side/collapsed state.
 *
 * Each mocked room block follows the exact portal contract the real ones use
 * (`useStudioAiSlot()` + `createPortal`, see PlanBlock/WritingBlock/
 * ReviewBlock) so the shell's plumbing is exercised without dragging in any
 * room's own network/API dependencies. ReadingBlock is mocked WITHOUT a
 * portal, matching the real component (spec: 阅读's coach only exists once a
 * source is opened into the separate ReadingRoom surface, not in this list
 * room).
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
// ReadingBlock does NOT portal into the shell panel — the real component owns
// its OWN inline coach column, and the shell hides its constant AiPanel while
// 阅读 is active (see WorkspaceContainer `showAiPanel`). The mock renders just
// the work marker; the test asserts the shell panel chrome is absent for 阅读.
vi.mock("@/workspace/blocks/ReadingBlock", () => ({
  ReadingBlock: ({ projectId }: { projectId: string }) => <div data-testid="reading-work">work:reading:{projectId}</div>,
}));
vi.mock("@/studio/reading/ReadingRoom", () => ({ ReadingRoom: () => <div data-testid="reading-room" /> }));

const getWorkspace = vi.fn();
vi.mock("@/workspace/api/workspace", () => ({
  getWorkspace: (...args: unknown[]) => getWorkspace(...args),
  getPlan: vi.fn(async () => []),
  getCoachHistory: vi.fn(async () => []),
  postProjectSummary: vi.fn(async () => ""),
  patchReference: vi.fn(async () => ({})),
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

    // 阅读: work swaps, and — unlike the other three rooms — the shell's
    // CONSTANT AiPanel is HIDDEN entirely. Reading is a distinct full-screen
    // surface that owns its own coach column (印记 · 找资料); showing the shell
    // panel too would leave it empty (list mode) or duplicate it as a second
    // 印记 column (graph mode). So while 阅读 is active there is no AiPanel
    // chrome at all (no flip/collapse controls).
    await switchRoom("阅读");
    expect(await screen.findByTestId("reading-work")).toBeInTheDocument();
    expect(screen.queryByTestId("plan-work")).not.toBeInTheDocument();
    expect(screen.queryByTestId("plan-coach")).not.toBeInTheDocument();
    expect(screen.queryByTestId("reading-coach")).not.toBeInTheDocument();
    // The constant panel is gone for reading — its flip/collapse chrome absent.
    expect(screen.queryByRole("button", { name: "切换 AI 面板左右" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "折叠 AI 面板" })).not.toBeInTheDocument();
    // Switching AWAY from reading brings the panel back (to 写作 next).

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
