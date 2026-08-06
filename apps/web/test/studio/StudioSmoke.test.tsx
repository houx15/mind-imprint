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
 *     collapsed (阅读 has no coach portal at all: it's a materials-list room,
 *     the coach lives in the separate full-screen ReadingRoom surface);
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
// ReadingBlock deliberately does NOT read useStudioAiSlot — matches the real
// component (阅读 is a materials list; its coach only exists once a specific
// source is opened into the separate full-screen ReadingRoom).
vi.mock("@/workspace/blocks/ReadingBlock", () => ({
  ReadingBlock: ({ projectId }: { projectId: string }) => <div data-testid="reading-work">work:reading:{projectId}</div>,
}));
vi.mock("@/studio/reading/ReadingRoom", () => ({ ReadingRoom: () => <div data-testid="reading-room" /> }));

const getWorkspace = vi.fn();
vi.mock("@/workspace/api/workspace", () => ({
  getWorkspace: (...args: unknown[]) => getWorkspace(...args),
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
    proposal: { objective: "", reason: "", activities: "", resources: "" },
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

    // 阅读: work swaps, and — unlike the other three rooms — no coach is
    // portaled at all (materials-list room; its coach lives in the separate
    // full-screen ReadingRoom once a source is opened).
    await switchRoom("阅读");
    expect(await screen.findByTestId("reading-work")).toBeInTheDocument();
    expect(screen.queryByTestId("plan-work")).not.toBeInTheDocument();
    expect(screen.queryByTestId("plan-coach")).not.toBeInTheDocument();
    expect(screen.queryByTestId("reading-coach")).not.toBeInTheDocument();

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
    // Default: side "right" (<main> first), expanded (no persisted keys yet).
    expect(row.firstElementChild).toBe(main);
    expect(localStorage.getItem("mk-studio-ai-side")).toBeNull();
    expect(localStorage.getItem("mk-studio-ai-collapsed")).toBeNull();

    // Flip while on the plan room.
    await userEvent.click(screen.getByRole("button", { name: "切换 AI 面板左右" }));
    expect(row.firstElementChild).not.toBe(main);
    expect(localStorage.getItem("mk-studio-ai-side")).toBe("left");

    // Switching rooms must not reset the flip — it lives in the container,
    // not in any one room.
    await switchRoom("写作");
    await screen.findByTestId("writing-work");
    expect(row.firstElementChild).not.toBe(main);
    expect(localStorage.getItem("mk-studio-ai-side")).toBe("left");

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
