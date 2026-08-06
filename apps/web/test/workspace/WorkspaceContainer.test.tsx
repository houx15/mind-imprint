import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// Keep this a light, seam-focused test of the `initialProjectId` deep-link
// (Task 6) — everything below the shell's own room-swap logic is mocked out
// so the test doesn't drag in every room's own data-fetching.
vi.mock("@/workspace/Directory", () => ({
  Directory: ({ onOpen }: { onOpen: (id: string) => void }) => (
    <div data-testid="directory">
      <button type="button" onClick={() => onOpen("clicked-id")}>
        open clicked-id
      </button>
    </div>
  ),
}));

vi.mock("@/workspace/blocks/PlanBlock", () => ({
  PlanBlock: ({ projectId, title }: { projectId: string; title: string }) => (
    <div data-testid="plan-block">
      {projectId}:{title}
    </div>
  ),
}));
vi.mock("@/workspace/blocks/ReadingBlock", () => ({ ReadingBlock: () => <div data-testid="reading-block" /> }));
vi.mock("@/workspace/blocks/WritingBlock", () => ({ WritingBlock: () => <div data-testid="writing-block" /> }));
vi.mock("@/workspace/blocks/ReviewBlock", () => ({ ReviewBlock: () => <div data-testid="review-block" /> }));
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

describe("WorkspaceContainer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getWorkspace.mockImplementation(async (id: string) => fakeWorkspace(id));
  });

  it("shows the directory when no project is open", () => {
    render(<WorkspaceContainer />);
    expect(screen.getByTestId("directory")).toBeInTheDocument();
  });

  it("opens the given project on mount when initialProjectId is provided", async () => {
    render(<WorkspaceContainer initialProjectId="p42" />);

    expect(await screen.findByTestId("plan-block")).toHaveTextContent("p42:项目 p42");
    expect(screen.queryByTestId("directory")).not.toBeInTheDocument();
    expect(getWorkspace).toHaveBeenCalledWith("p42");
  });

  it("calls onInitialProjectIdConsumed once the deep-linked project has been opened", async () => {
    const onConsumed = vi.fn();
    render(<WorkspaceContainer initialProjectId="p7" onInitialProjectIdConsumed={onConsumed} />);

    await screen.findByTestId("plan-block");
    expect(onConsumed).toHaveBeenCalledTimes(1);
  });

  it("still opens a project via a normal directory click when initialProjectId is absent", async () => {
    render(<WorkspaceContainer />);

    await userEvent.click(screen.getByText("open clicked-id"));
    expect(await screen.findByTestId("plan-block")).toHaveTextContent("clicked-id:");
  });
});
