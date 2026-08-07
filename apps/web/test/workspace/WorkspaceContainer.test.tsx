import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// Keep this a light, seam-focused test of the `initialProjectId` deep-link
// (Task 6) — everything below the shell's own room-swap logic is mocked out
// so the test doesn't drag in every room's own data-fetching.
vi.mock("@/workspace/Directory", () => ({
  Directory: ({
    onOpen,
    autoOpenCreate,
    onAutoOpenCreateHandled,
  }: {
    onOpen: (id: string) => void;
    autoOpenCreate?: boolean;
    onAutoOpenCreateHandled?: () => void;
  }) => (
    <div data-testid="directory">
      <button type="button" onClick={() => onOpen("clicked-id")}>
        open clicked-id
      </button>
      {/* Surfaces exactly what Directory itself does on mount, so this test
          can verify WorkspaceContainer forwards the create-drawer deep-link
          (autoOpenCreate + its consumed-callback) down correctly, without
          re-testing Directory's own drawer-opening behavior (covered in
          Directory.test.tsx). */}
      {autoOpenCreate && <div data-testid="auto-open-create" />}
      <button type="button" onClick={onAutoOpenCreateHandled}>
        consume auto-open-create
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
vi.mock("@/workspace/blocks/WritingReferencePanel", () => ({ WritingReferencePanel: () => <div data-testid="writing-ref-panel" /> }));
vi.mock("@/workspace/blocks/ReviewBlock", () => ({ ReviewBlock: () => <div data-testid="review-block" /> }));
vi.mock("@/studio/reading/ReadingRoom", () => ({ ReadingRoom: () => <div data-testid="reading-room" /> }));

const getWorkspace = vi.fn();
const getStudioState = vi.fn();
const coach = vi.fn();
const putProposal = vi.fn();
vi.mock("@/workspace/api/workspace", () => ({
  getWorkspace: (...args: unknown[]) => getWorkspace(...args),
  getStudioState: (...args: unknown[]) => getStudioState(...args),
  coach: (...args: unknown[]) => coach(...args),
  putProposal: (...args: unknown[]) => putProposal(...args),
  getPlan: vi.fn(async () => []),
  getCoachHistory: vi.fn(async () => []),
  postProjectSummary: vi.fn(async () => ""),
  patchReference: vi.fn(async () => ({})),
  reflectProjectCard: vi.fn(async () => ({ cardInstanceId: "", reply: "", card: null })),
  dismissProposal: vi.fn(async () => {}),
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

type OpenTool = "chat" | "plan" | "reading" | "writing" | "reflection";
function fakeStudioState(openTool: OpenTool) {
  return {
    stage: "plan_generation" as const,
    openTool,
    widthTier: "half" as const,
    reference: [] as never[],
    updatedAtTurn: 0,
  };
}

// The full OrchestratorReply the container's send loop applies each turn.
function fakeReply(
  narrate: string,
  openTool: OpenTool,
  extra: { note?: unknown; card?: unknown } = {},
) {
  return {
    narrate,
    directive: fakeStudioState(openTool),
    note: extra.note ?? null,
    card: extra.card ?? null,
    reviewRequested: false,
  };
}

describe("WorkspaceContainer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getWorkspace.mockImplementation(async (id: string) => fakeWorkspace(id));
    // Default: 印记 has already opened the plan board — keeps the deep-link
    // tests below asserting the plan room (now via resume-at-stage, not a
    // forced landing). Individual cases override for chat/writing.
    getStudioState.mockImplementation(async () => fakeStudioState("plan"));
    coach.mockResolvedValue(fakeReply("好的。", "chat"));
    putProposal.mockImplementation(async (_id: string, p: unknown) => p);
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

  it("forwards autoOpenCreate to the directory, and onAutoOpenCreateConsumed back up once it fires", async () => {
    const onAutoOpenCreateConsumed = vi.fn();
    render(<WorkspaceContainer autoOpenCreate onAutoOpenCreateConsumed={onAutoOpenCreateConsumed} />);

    expect(screen.getByTestId("auto-open-create")).toBeInTheDocument();
    expect(onAutoOpenCreateConsumed).not.toHaveBeenCalled();

    await userEvent.click(screen.getByText("consume auto-open-create"));
    expect(onAutoOpenCreateConsumed).toHaveBeenCalledTimes(1);
  });

  it("does not flag autoOpenCreate on the directory when it isn't set", () => {
    render(<WorkspaceContainer />);
    expect(screen.queryByTestId("auto-open-create")).not.toBeInTheDocument();
  });

  it("lands on the chat-first landing (not the plan board) when studio_state.openTool is chat", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("chat"));
    render(<WorkspaceContainer initialProjectId="pc" />);

    expect(await screen.findByTestId("chat-first")).toBeInTheDocument();
    expect(screen.queryByTestId("plan-block")).not.toBeInTheDocument();
  });

  it("resumes at the writing room when studio_state.openTool is writing", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("writing"));
    render(<WorkspaceContainer initialProjectId="pw" />);

    expect(await screen.findByTestId("writing-block")).toBeInTheDocument();
    expect(screen.queryByTestId("plan-block")).not.toBeInTheDocument();
    expect(screen.queryByTestId("chat-first")).not.toBeInTheDocument();
  });

  it("lets the manual switcher take over from the chat-first landing (spec §6)", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("chat"));
    render(<WorkspaceContainer initialProjectId="pchat" />);

    // Lands chat-first (印记 keeps chat primary), switcher available.
    expect(await screen.findByTestId("chat-first")).toBeInTheDocument();

    // The student manually navigates to 写作 — the room must mount even though
    // 印记's status is still chat.
    await userEvent.click(screen.getByRole("button", { name: "写作" }));

    expect(await screen.findByTestId("writing-block")).toBeInTheDocument();
    expect(screen.queryByTestId("chat-first")).not.toBeInTheDocument();
  });

  it("keeps the switcher live even when getStudioState fails (no stuck-forever chat-first)", async () => {
    getStudioState.mockRejectedValue(new Error("boom"));
    render(<WorkspaceContainer initialProjectId="perr" />);

    // Fetch failed → studioState stays null → chat-first (never a plan flash).
    expect(await screen.findByTestId("chat-first")).toBeInTheDocument();
    expect(screen.queryByTestId("plan-block")).not.toBeInTheDocument();

    // The switcher is not a dead escape hatch: a click still mounts the room.
    await userEvent.click(screen.getByRole("button", { name: "立项" }));

    expect(await screen.findByTestId("plan-block")).toBeInTheDocument();
    expect(screen.queryByTestId("chat-first")).not.toBeInTheDocument();
  });

  // Task 9b · the container-owned 印记 chat: in chat-first the constant AiPanel
  // shows the REAL chat, a turn calls `coach`, and the returned directive is
  // APPLIED (auto-configure the view — here it opens the writing room).
  it("chat-first renders the 印记 Composer; a turn calls coach and applies the reply's directive (opens 写作)", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("chat"));
    // 印记 replies AND, via the directive, decides to open the writing room.
    coach.mockResolvedValue(fakeReply("我们去写作台看看。", "writing"));
    render(<WorkspaceContainer initialProjectId="pc" />);

    // The panel shows the real chat composer (not just the calm landing).
    const composer = await screen.findByPlaceholderText(/和印记说说你的项目/);
    await userEvent.type(composer, "我想研究中国的可持续");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    // The turn hit the orchestrator with just (id, input) — no scope on studio path.
    await waitFor(() => expect(coach).toHaveBeenCalledWith("pc", "我想研究中国的可持续"));
    // Directive applied: the writing room mounts, chat-first is gone.
    expect(await screen.findByTestId("writing-block")).toBeInTheDocument();
    expect(screen.queryByTestId("chat-first")).not.toBeInTheDocument();
  });

  // Whole-branch review Fix 1: `room` used to be left stale across a project
  // switch — a project resumed into the reading room, then a NEW project
  // resolving chat-first, left `room==="reading"` stuck (the load effect reset
  // studioState/tookOver/etc but never `room`). `showAiPanel` used to be
  // `room !== "reading"`, so the panel stayed hidden even though chat-first
  // has nowhere else to render its portal — an empty 印记 panel with no
  // switcher-visible escape. Guards BOTH halves of the fix: the reset-on-load
  // AND the chat-first-always-shows-the-panel derivation.
  it("does not leave an empty 印记 panel when switching from a reading-room project to a chat-first project", async () => {
    getStudioState.mockImplementation(async (id: string) =>
      id === "pa" ? fakeStudioState("reading") : fakeStudioState("chat"),
    );
    const { rerender } = render(<WorkspaceContainer initialProjectId="pa" />);

    // Project A resumes into the reading room.
    expect(await screen.findByTestId("reading-block")).toBeInTheDocument();

    // Switch to project B, which resolves chat-first.
    rerender(<WorkspaceContainer initialProjectId="pb" />);

    expect(await screen.findByTestId("chat-first")).toBeInTheDocument();
    expect(screen.queryByTestId("reading-block")).not.toBeInTheDocument();
    // The panel is not empty: the real 印记 chat composer is portaled in —
    // this is what a stale `room==="reading"` used to hide.
    expect(await screen.findByPlaceholderText(/和印记说说你的项目/)).toBeInTheDocument();
  });

  // Task 9b · a reply carrying a note OFFER renders a confirm chip; confirming
  // read-modify-writes the proposal board (getWorkspace → putProposal merged).
  it("a reply with a note renders a confirm chip; confirming persists the merged proposal", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("chat"));
    coach.mockResolvedValue(
      fakeReply("记下来吧。", "chat", { note: { section: "objective", value: "以中国为例回答可持续问题" } }),
    );
    render(<WorkspaceContainer initialProjectId="pn" />);

    const composer = await screen.findByPlaceholderText(/和印记说说你的项目/);
    await userEvent.type(composer, "帮我把目标记下来");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    // The 克制 confirm chip surfaces the offer (打开由学生确认).
    const confirm = await screen.findByRole("button", { name: "记进「目标」" });
    await userEvent.click(confirm);

    // Read-modify-write: re-read the proposal, append into the section, persist.
    await waitFor(() =>
      expect(putProposal).toHaveBeenCalledWith(
        "pn",
        expect.objectContaining({ objective: "以中国为例回答可持续问题" }),
      ),
    );
  });

  // Task 9b · when the target section ALREADY has content, confirming appends
  // with a newline join (never clobbers her own words) — the other branch of
  // confirmNote's read-modify-write merge.
  it("confirming a note appends with a newline when the section already has content", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("chat"));
    // This project's objective is already written — the merge must preserve it.
    getWorkspace.mockImplementation(async (id: string) => ({
      ...fakeWorkspace(id),
      proposal: { objective: "先前写好的目标。", reason: "", activities: "", resources: "" },
    }));
    coach.mockResolvedValue(
      fakeReply("记下来吧。", "chat", { note: { section: "objective", value: "再补一句想法" } }),
    );
    render(<WorkspaceContainer initialProjectId="pm" />);

    const composer = await screen.findByPlaceholderText(/和印记说说你的项目/);
    await userEvent.type(composer, "帮我补充目标");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    const confirm = await screen.findByRole("button", { name: "记进「目标」" });
    await userEvent.click(confirm);

    // Existing content + "\n" + the note's value — not a clobber.
    await waitFor(() =>
      expect(putProposal).toHaveBeenCalledWith(
        "pm",
        expect.objectContaining({ objective: "先前写好的目标。\n再补一句想法" }),
      ),
    );
  });
});
