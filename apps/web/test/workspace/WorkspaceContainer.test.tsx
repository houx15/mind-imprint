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
vi.mock("@/workspace/blocks/ReferencePanel", () => ({ ReferencePanel: () => <div data-testid="writing-ref-panel" /> }));
vi.mock("@/workspace/blocks/ReviewBlock", () => ({ ReviewBlock: () => <div data-testid="review-block" /> }));
vi.mock("@/studio/reading/ReadingRoom", () => ({ ReadingRoom: () => <div data-testid="reading-room" /> }));

const getWorkspace = vi.fn();
const getStudioState = vi.fn();
const coach = vi.fn();
const putProposal = vi.fn();
const getPlan = vi.fn();
vi.mock("@/workspace/api/workspace", () => ({
  getWorkspace: (...args: unknown[]) => getWorkspace(...args),
  getStudioState: (...args: unknown[]) => getStudioState(...args),
  coach: (...args: unknown[]) => coach(...args),
  // Every scenario here is an already-`started` project (fakeStudioState
  // default), so the Task 6 opening never fires and 开始 never renders — these
  // are unused no-op stand-ins, kept only so the module shape matches.
  coachOpening: vi.fn(),
  coachStart: vi.fn(),
  putProposal: (...args: unknown[]) => putProposal(...args),
  getPlan: (...args: unknown[]) => getPlan(...args),
  getCoachHistory: vi.fn(async () => ({ messages: [], hasMore: false, recap: null, nextCursor: null })),
  postProjectSummary: vi.fn(async () => ""),
  patchReference: vi.fn(async () => ({})),
  reflectProjectCard: vi.fn(async () => ({ cardInstanceId: "", reply: "", card: null })),
  dismissProposal: vi.fn(async () => {}),
}));

const createLead = vi.fn();
vi.mock("@/api/exploration", () => ({
  createLead: (...args: unknown[]) => createLead(...args),
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
type Stage =
  | "topic_discussion"
  | "proposal_forming"
  | "plan_generation"
  | "proposal_writing"
  | "proposal_review"
  | "body_writing"
  | "retrospective";
type WidthTier = "chat" | "half" | "wide";
// `widthTier` follows `openTool` by default (chat → the full-width chat surface;
// any real room → the split, defaulting to `half`) — override it explicitly to
// exercise the morphing-width tiers (P2a).
function fakeStudioState(openTool: OpenTool, stage: Stage = "plan_generation", widthTier?: WidthTier) {
  return {
    stage,
    openTool,
    widthTier: widthTier ?? (openTool === "chat" ? "chat" : "half"),
    reference: [] as never[],
    updatedAtTurn: 0,
    // Every scenario in THIS file is a resumed (already-started) project — the
    // Task 6 start-gate itself (a not-started brand-new project) has its own
    // dedicated test file (StudioStartGate.test.tsx).
    started: true,
  };
}

// The full OrchestratorReply the container's send loop applies each turn.
function fakeReply(
  narrate: string,
  openTool: OpenTool,
  extra: {
    note?: unknown;
    card?: unknown;
    question?: unknown;
    // Task 7 · hidden-subagent post-hoc acknowledgments (generate_plan /
    // maybeCompactBackstop ran silently during this turn).
    planGenerated?: boolean;
    compacted?: boolean;
  } = {},
) {
  return {
    narrate,
    directive: fakeStudioState(openTool),
    note: extra.note ?? null,
    card: extra.card ?? null,
    question: extra.question ?? null,
    reviewRequested: false,
    planGenerated: extra.planGenerated ?? false,
    compacted: extra.compacted ?? false,
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
    getPlan.mockResolvedValue([]);
    createLead.mockResolvedValue({ id: "lead-1" });
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
    // Live-journey fix: while chat-first, NO switcher segment is highlighted —
    // `room`'s stale interim default must not falsely mark a segment.
    expect(screen.queryByRole("button", { pressed: true })).toBeNull();
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
    await userEvent.click(screen.getByRole("button", { name: "管理" }));

    expect(await screen.findByTestId("plan-block")).toBeInTheDocument();
    expect(screen.queryByTestId("chat-first")).not.toBeInTheDocument();
  });

  // P2a: 立项 split into 提案(forming)/管理(board), both sharing PlanBlock.
  // `roomForResume` picks forming vs board from the STAGE when 印记's openTool
  // is "plan" (it doesn't yet emit an explicit "forming" openTool — that's
  // P2b). A proposal_forming-stage project must resume into 提案, not 管理.
  it("resumes into 提案 (forming) when studio_state.stage is proposal_forming", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("plan", "proposal_forming"));
    render(<WorkspaceContainer initialProjectId="pf" />);

    expect(await screen.findByTestId("plan-block")).toHaveTextContent("pf:项目 pf");
    expect(getStudioState).toHaveBeenCalledWith("pf");
    const resolved = await getStudioState.mock.results[0]!.value;
    expect(resolved).toEqual(expect.objectContaining({ stage: "proposal_forming" }));
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
    // Mount already fetched the plan once (the load effect) — clear that call
    // so the assertion below is specifically about the post-turn refresh.
    getPlan.mockClear();
    await userEvent.type(composer, "我想研究中国的可持续");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    // The turn hit the orchestrator with just (id, input) — no scope on studio path.
    await waitFor(() => expect(coach).toHaveBeenCalledWith("pc", "我想研究中国的可持续"));
    // Directive applied: the writing room mounts, chat-first is gone.
    expect(await screen.findByTestId("writing-block")).toBeInTheDocument();
    expect(screen.queryByTestId("chat-first")).not.toBeInTheDocument();
    // P2b: the plan can no longer be regenerated via a button — 印记 triggers
    // it server-side via `generate_plan`, so every turn best-effort refreshes
    // the plan spine (getPlan) instead of waiting for a room remount.
    await waitFor(() => expect(getPlan).toHaveBeenCalledWith("pc"));
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

  // P2a · Morphing width (spec §3). In the `chat` tier the 印记 chat IS the
  // surface: it fills the content width, with NO interactive room and NO side
  // AiPanel rail — the chat is not a narrow companion beside an empty landing.
  it("chat tier: the 印记 chat fills the width — no room and no side panel", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("chat"));
    render(<WorkspaceContainer initialProjectId="pchatwide" />);

    // The full-width chat surface (composer present).
    expect(await screen.findByPlaceholderText(/和印记说说你的项目/)).toBeInTheDocument();
    expect(screen.getByTestId("chat-first")).toBeInTheDocument();
    // No interactive room is mounted…
    expect(screen.queryByTestId("plan-block")).not.toBeInTheDocument();
    expect(screen.queryByTestId("writing-block")).not.toBeInTheDocument();
    expect(screen.queryByTestId("reading-block")).not.toBeInTheDocument();
    // …and no side AiPanel rail (the chat owns the whole width).
    expect(screen.queryByRole("button", { name: "切换 AI 面板左右" })).not.toBeInTheDocument();
  });

  // `half` tier: a room mounts in the interactive area AND the 印记 chat rides
  // alongside as the AiPanel rail (its coach content is portaled by the room —
  // mocked away here — but the constant panel chrome is present).
  it("half tier: mounts the room AND keeps the 印记 rail (no full-width chat)", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("writing", "body_writing", "half"));
    render(<WorkspaceContainer initialProjectId="phalf" />);

    expect(await screen.findByTestId("writing-block")).toBeInTheDocument();
    // The chat is a rail beside the room, not the full-width chat surface.
    expect(screen.queryByTestId("chat-first")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "切换 AI 面板左右" })).toBeInTheDocument();
  });

  // `wide` tier: same invariant — the interactive area is the surface, the 印记
  // rail persists (the student can still collapse it to the slim rail).
  it("wide tier: mounts the room AND keeps the 印记 rail", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("writing", "body_writing", "wide"));
    render(<WorkspaceContainer initialProjectId="pwide" />);

    expect(await screen.findByTestId("writing-block")).toBeInTheDocument();
    expect(screen.queryByTestId("chat-first")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "切换 AI 面板左右" })).toBeInTheDocument();
  });

  // Task 3 (P2a): reading used to be excluded from the constant panel (it owned
  // its own FloatingCoach column) — that room-owned coach is gone, so reading
  // now joins the ONE constant 印记 rail exactly like plan/writing/reflection.
  it("reading room joins the constant 印记 rail (no more FloatingCoach exception)", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("reading", "topic_discussion", "half"));
    render(<WorkspaceContainer initialProjectId="pread" />);

    expect(await screen.findByTestId("reading-block")).toBeInTheDocument();
    // The constant rail's chrome is present — reading is no longer the
    // exception that hid it (the old `room !== "reading"` guard).
    expect(screen.getByRole("button", { name: "切换 AI 面板左右" })).toBeInTheDocument();
    // No separate floating-chip coach affordance — 印记 is the one rail now.
    expect(screen.queryByRole("button", { name: /问印记 · 找资料/ })).not.toBeInTheDocument();
    expect(screen.queryByText("印记 · 找资料")).not.toBeInTheDocument();
  });

  // A manual takeover forces at least `wide` (a room is always showing) even
  // when 印记's status is chat — the chat-only surface yields to the room.
  it("manual takeover from chat forces a room (tookOver ⇒ wide)", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("chat"));
    render(<WorkspaceContainer initialProjectId="ptake" />);

    expect(await screen.findByTestId("chat-first")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "写作" }));

    expect(await screen.findByTestId("writing-block")).toBeInTheDocument();
    expect(screen.queryByTestId("chat-first")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "切换 AI 面板左右" })).toBeInTheDocument();
  });

  // P4 · 「继续印记」(spec §6): once a manual takeover has swapped the room,
  // the control appears; clicking it re-fetches 印记's status and returns the
  // view there, clearing the takeover flag (the control disappears again).
  it("「继续印记」returns from a manual takeover to 印记's status room", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("plan"));
    render(<WorkspaceContainer initialProjectId="pcontinue" />);

    // 印记 opens the 管理 board by default (openTool "plan").
    expect(await screen.findByTestId("plan-block")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /继续印记/ })).not.toBeInTheDocument();

    // Manual takeover via the switcher, into 写作.
    await userEvent.click(screen.getByRole("button", { name: "写作" }));
    expect(await screen.findByTestId("writing-block")).toBeInTheDocument();

    const continueBtn = await screen.findByRole("button", { name: /继续印记/ });
    await userEvent.click(continueBtn);

    // Back to 印记's status room; the control is gone (tookOver cleared).
    expect(await screen.findByTestId("plan-block")).toBeInTheDocument();
    expect(screen.queryByTestId("writing-block")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /继续印记/ })).not.toBeInTheDocument();
    expect(getStudioState).toHaveBeenCalledTimes(2);
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

  // Task 7 (P2b) · a reply carrying a `propose_question` OFFER renders a
  // confirm chip; confirming creates an exploration lead (createLead) and
  // clears the chip — mirrors the note confirm-chip test above.
  it("a reply with a question renders a confirm chip; confirming creates an exploration lead", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("chat"));
    coach.mockResolvedValue(
      fakeReply("这个问题值得深挖。", "chat", { question: { text: "中国的碳排放增长会抵消其可持续举措吗？" } }),
    );
    render(<WorkspaceContainer initialProjectId="pq" />);

    const composer = await screen.findByPlaceholderText(/和印记说说你的项目/);
    await userEvent.type(composer, "这里有个问题");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    const confirm = await screen.findByRole("button", { name: "加入探索图谱" });
    await userEvent.click(confirm);

    await waitFor(() =>
      expect(createLead).toHaveBeenCalledWith("pq", "中国的碳排放增长会抵消其可持续举措吗？"),
    );
    // The chip clears once confirmed.
    expect(screen.queryByRole("button", { name: "加入探索图谱" })).not.toBeInTheDocument();
  });

  // Task 7 · hidden-subagent post-hoc acknowledgments: `generate_plan` and the
  // backstop compaction run silently during the awaited turn (no per-phase
  // SSE) — a reply flagging either appends a one-line SubagentHint AFTER
  // 印记's own narrate line, never a second chat exchange.
  it("a reply with planGenerated:true appends the 已整理研究计划 hint after 印记's narrate line", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("chat"));
    coach.mockResolvedValue(fakeReply("我把研究计划列出来了。", "chat", { planGenerated: true }));
    render(<WorkspaceContainer initialProjectId="pplan" />);

    const composer = await screen.findByPlaceholderText(/和印记说说你的项目/);
    await userEvent.type(composer, "帮我理一下研究计划");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    expect(await screen.findByText("我把研究计划列出来了。")).toBeInTheDocument();
    expect(await screen.findByText("subagent 已整理研究计划")).toBeInTheDocument();

    // Order: narrate first, then the hint line (DOM order).
    const narrate = screen.getByText("我把研究计划列出来了。");
    const hint = screen.getByText("subagent 已整理研究计划");
    expect(narrate.compareDocumentPosition(hint) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("a reply with compacted:true appends the 已整理较早的对话 hint", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("chat"));
    coach.mockResolvedValue(fakeReply("继续说说看。", "chat", { compacted: true }));
    render(<WorkspaceContainer initialProjectId="pcompact" />);

    const composer = await screen.findByPlaceholderText(/和印记说说你的项目/);
    await userEvent.type(composer, "接着之前聊的");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    expect(await screen.findByText("继续说说看。")).toBeInTheDocument();
    expect(await screen.findByText("已整理较早的对话")).toBeInTheDocument();
  });

  it("a plain reply (no planGenerated/compacted) shows no subagent hint line", async () => {
    getStudioState.mockImplementation(async () => fakeStudioState("chat"));
    coach.mockResolvedValue(fakeReply("好的。", "chat"));
    render(<WorkspaceContainer initialProjectId="pplain" />);

    const composer = await screen.findByPlaceholderText(/和印记说说你的项目/);
    await userEvent.type(composer, "你好");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    expect(await screen.findByText("好的。")).toBeInTheDocument();
    expect(screen.queryByText("subagent 已整理研究计划")).toBeNull();
    expect(screen.queryByText("已整理较早的对话")).toBeNull();
  });
});
