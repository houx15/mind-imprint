import { describe, it, expect, vi, beforeEach } from "vitest";
import { useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StudioAiSlotContext } from "@/studio/ai/StudioAiSlot";
import { StudioChatContext, type StudioChatMsg } from "@/studio/ai/StudioChatContext";

/**
 * PlanBlock (studio agentic rebuild, Task 5): the forming coach now portals
 * its chat log + composer through the SHARED `ChatLog`/`Composer`
 * (`@/studio/ai/`) instead of a bespoke inline chat implementation —
 * `FormingPhase` still calls `useStudioAiSlot()` and `createPortal`s into
 * whatever DOM node the context hands it (Task 4's contract), so tests
 * stand in a real DOM node via `StudioAiSlotContext.Provider` the same way
 * the constant AiPanel body does in the real shell.
 */

vi.mock("@/workspace/api/workspace", () => ({
  putProposal: vi.fn(async (_id: string, p: unknown) => p),
  getPlan: vi.fn(async () => []),
  createPlanItem: vi.fn(async () => ({})),
  patchPlanItem: vi.fn(async () => ({})),
  deletePlanItem: vi.fn(async () => {}),
  getLog: vi.fn(async () => []),
  addLog: vi.fn(async () => ({})),
  getCoachHistory: vi.fn(async () => ({ messages: [], hasMore: false, recap: null, nextCursor: null })),
  reflectProjectCard: vi.fn(async () => ({ cardInstanceId: "", reply: "", card: null })),
}));

import { getCoachHistory, getPlan } from "@/workspace/api/workspace";
import { PlanBlock } from "@/workspace/blocks/PlanBlock";

const mockGetCoachHistory = vi.mocked(getCoachHistory);
const mockGetPlan = vi.mocked(getPlan);

// The coach send is now the ONE container-owned loop, provided to the room via
// StudioChatContext (`sendStudioTurn`). The room's onSend/onGuideMe/onReview
// call it — so tests assert on this spy (the shared-send shape), not on a
// direct `coach()` call the room no longer makes. The stub mirrors the
// container: it appends the student turn to the store so the block still shows
// what the student sent (the reply itself is the container's job, covered in
// WorkspaceContainer.test).
const mockSend = vi.fn();

const EMPTY_PROPOSAL = { objective: "", reason: "", activities: "", resources: "", counterpoints: "" };
const FILLED_PROPOSAL = {
  objective: "论证中国是否让地球更可持续",
  reason: "关心气候变化",
  activities: "读 NASA/Nature Sustainability",
  resources: "Zotero、图书馆数据库",
  counterpoints: "",
};

// The coach thread now lives in the hoisted StudioChatContext store (owned by
// WorkspaceContainer in the real shell); FormingPhase reads/appends it via
// `useStudioChat()`. Tests stand in a stateful provider the same way the shell
// does, seeded with any initial thread the test needs.
function ChatProvider({ initial = [], children }: { initial?: StudioChatMsg[]; children: React.ReactNode }) {
  const [messages, setMessages] = useState<StudioChatMsg[]>(initial);
  const [sending, setSending] = useState(false);
  const sendStudioTurn = async (userInput: string, opts?: { quotedPart?: string }) => {
    mockSend(userInput, opts);
    setMessages((c) => [...c, { role: "student", text: userInput, quotedPart: opts?.quotedPart }]);
    return true;
  };
  return (
    <StudioChatContext.Provider
      value={{
        messages,
        setMessages,
        sending,
        setSending,
        activeProjectIdRef: { current: "p1" },
        sendStudioTurn,
        projectId: "p1",
        pendingNote: null,
        confirmedNote: null,
        pendingCard: null,
        confirmNote: () => {},
        dismissNote: () => {},
        openCard: () => {},
        dismissCard: () => {},
        questionCardAvailable: false,
        pendingQuestion: null,
        confirmQuestion: () => {},
        dismissQuestion: () => {},
        pendingLinkOffer: null,
        readLinkOffer: () => {},
        addLinkOffer: () => {},
        dismissLinkOffer: () => {},
        pendingNextStep: null,
        advanceToNextStep: () => {},
        advanceStatusTo: async () => {},
        historyHasMore: false,
        loadEarlier: () => {},
        loadingEarlier: false,
        // Task 6 (start gate): PlanBlock only ever mounts once the journey has
        // actually started (WorkspaceContainer gates the whole switcher/room
        // area on `started`), so these tests stand in a started project —
        // mirrors the shell's real invariant.
        started: true,
        startJourney: async () => {},
        starting: false,
      }}
    >
      {children}
    </StudioChatContext.Provider>
  );
}

// The AiPanel body is a real DOM node the panel hands down via context; the
// portal contract (Task 4) needs a genuine element to portal into (jsdom
// createPortal requires it), and RTL's `screen` queries document.body — where
// this node lives — so portaled content is found the same as any other.
function renderWithAiSlot(ui: React.ReactElement, initialMessages: StudioChatMsg[] = []) {
  const slot = document.createElement("div");
  document.body.appendChild(slot);
  return render(
    <ChatProvider initial={initialMessages}>
      <StudioAiSlotContext.Provider value={slot}>{ui}</StudioAiSlotContext.Provider>
    </ChatProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockSend.mockClear();
  mockGetCoachHistory.mockResolvedValue({ messages: [], hasMore: false, recap: null, nextCursor: null });
  mockGetPlan.mockResolvedValue([]);
});

describe("PlanBlock · forming coach on the shared AiPanel (Task 5)", () => {
  // Task 6 (start gate): the hardcoded 提案 intro is GONE — the framing now
  // comes from the live agent's `coach/start` narrate (seeded into the store
  // BEFORE this room ever mounts), not a local scripted fallback. An empty
  // thread renders empty; the 提问卡 entry shows while 目标 is empty.
  it("renders NO scripted intro — an empty thread stays empty, and the 提问卡 entry shows while 目标 empty", async () => {
    renderWithAiSlot(
      <PlanBlock projectId="p1" title="T" qualification="拓展论文 EE" proposal={EMPTY_PROPOSAL} phase="forming" refreshWorkspace={() => {}} />,
    );

    // The 提问卡 entry (replaces the old self-summon shelf) shows while 目标 empty.
    expect(await screen.findByRole("button", { name: /还没头绪/ })).toBeInTheDocument();
    // The old hardcoded intro never renders.
    expect(screen.queryByText(/先想清楚四件事/)).toBeNull();
  });

  it("sends a message through the shared Composer via the container-owned send loop", async () => {
    renderWithAiSlot(
      <PlanBlock projectId="p1" title="T" qualification="拓展论文 EE" proposal={EMPTY_PROPOSAL} phase="forming" refreshWorkspace={() => {}} />,
    );

    const textarea = await screen.findByPlaceholderText("说说你的想法……（Shift+Enter 换行）");
    await userEvent.type(textarea, "我想研究中国的碳排放");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    // The room no longer calls coach() directly — it drives the ONE shared send.
    expect(mockSend).toHaveBeenCalledWith("我想研究中国的碳排放", undefined);
    // The student's own turn lands in the shared store (mirrors the container).
    expect(await screen.findByText("我想研究中国的碳排放")).toBeInTheDocument();
  });

  it("Round 3: reframed as research-framing — no 让印记看看我的开题/导出开题报告 buttons, DimFields still work", async () => {
    renderWithAiSlot(
      <PlanBlock projectId="p1" title="T" qualification="拓展论文 EE" proposal={FILLED_PROPOSAL} phase="forming" refreshWorkspace={() => {}} />,
    );
    await screen.findByText("先搭好研究的大框架");

    // The old two-button footer (印记's 开题 review + a .docx export) is gone —
    // 提案/forming is framing chat now, not a proposal draft with an export.
    expect(screen.queryByRole("button", { name: /让印记看看我的开题/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /导出开题报告/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /生成项目计划/ })).toBeNull();
    // DimFields still work — now view-by-default (5 dims, each with an 编辑
    // affordance); clicking 编辑 reveals the edit box.
    const editBtns = screen.getAllByRole("button", { name: "编辑" });
    expect(editBtns).toHaveLength(5);
    await userEvent.click(editBtns[0]!);
    expect(screen.getByPlaceholderText("跟印记聊几句，这里会慢慢填上")).toBeInTheDocument();
  });

  it("Round 3: retitled eyebrow/h1/subtitle read as research-framing, and the old Tier-1 chrome stays gone", async () => {
    renderWithAiSlot(
      <PlanBlock projectId="p1" title="T" qualification="拓展论文 EE" proposal={EMPTY_PROPOSAL} phase="forming" refreshWorkspace={() => {}} />,
    );
    await screen.findByText("先搭好研究的大框架");

    expect(screen.getByText("立项 · 先想清楚再动手")).toBeInTheDocument();
    expect(screen.getByText("把这几件事聊清楚，计划会据此长出来。")).toBeInTheDocument();

    // Killed Tier-1 chrome — 印记 cues these instead.
    expect(screen.queryByRole("button", { name: "聊聊计划" })).toBeNull();
    expect(screen.queryByRole("button", { name: /写开题报告/ })).toBeNull();
    expect(screen.queryByRole("button", { name: "中" })).toBeNull();
    expect(screen.queryByRole("button", { name: "EN" })).toBeNull();
  });

  it("renders the CONTINUOUS thread from the hoisted store (not just this room's slice), and shows the recap in-chat", async () => {
    // The thread is loaded ONCE by WorkspaceContainer into the hoisted store and
    // handed down; this room reads it via the provider (no per-room re-fetch).
    // Seeding the provider stands in for that container load.
    renderWithAiSlot(
      <PlanBlock
        projectId="p1"
        title="T"
        qualification="拓展论文 EE"
        proposal={EMPTY_PROPOSAL}
        phase="forming"
        refreshWorkspace={() => {}}
        recap="欢迎回来——你上次聊到了判断尺度。"
      />,
      [{ role: "ai", text: "我们上次聊到判断尺度。", card: null }],
    );
    // The one continuous 印记 conversation across 立项/写作 renders from the store.
    expect(await screen.findByText("我们上次聊到判断尺度。")).toBeInTheDocument();
    // The recap is 印记's opening line inside the chat (not a separate banner).
    expect(screen.getByText("欢迎回来——你上次聊到了判断尺度。")).toBeInTheDocument();
  });
});

describe("PlanBlock · working phase (plan board) is unaffected by the coach restyle", () => {
  it("opens straight on the board (Segmented view switcher) when the proposal is already filled", async () => {
    mockGetPlan.mockResolvedValue([
      { id: "i1", title: "读：找反例", tag: "read", column: "todo", stage: "阶段一", refMaterialId: null, start: 0, days: 2, position: 0 },
    ]);
    renderWithAiSlot(
      <PlanBlock
        projectId="p1"
        title="中国是否让地球更可持续？"
        qualification="拓展论文 EE"
        proposal={FILLED_PROPOSAL}
        createdAt="2026-08-01T00:00:00Z"
        phase="working"
        refreshWorkspace={() => {}}
      />,
    );

    // §3 · opens on the 甘特图 by default (which strips the 读：/写：/省： prefix).
    expect(await screen.findByText("找反例")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "看板" })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "活动日志" }));
    await waitFor(() => expect(screen.getByText("还没有记录")).toBeInTheDocument());
  });

  it("P2a Tier-1 strip: the board drops the 查看我的题目/收起 toggle and the 进入 → per-card doorway", async () => {
    mockGetPlan.mockResolvedValue([
      { id: "i1", title: "读：找反例", tag: "read", column: "todo", stage: "阶段一", refMaterialId: null, start: 0, days: 2, position: 0 },
    ]);
    renderWithAiSlot(
      <PlanBlock
        projectId="p1"
        title="中国是否让地球更可持续？"
        qualification="拓展论文 EE"
        proposal={FILLED_PROPOSAL}
        createdAt="2026-08-01T00:00:00Z"
        phase="working"
        refreshWorkspace={() => {}}
      />,
    );

    // Switch to 看板 (the card view) to check the Tier-1 strip drops the doorway.
    expect(await screen.findByText("找反例")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "看板" }));
    expect(await screen.findByText("读：找反例")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "查看我的题目" })).toBeNull();
    expect(screen.queryByRole("button", { name: "收起" })).toBeNull();
    expect(screen.queryByRole("button", { name: "进入 →" })).toBeNull();
  });

  // §3 gap G4 · the recap "继续工作" continue moved OUT of the plan pane INTO the
  // AI chat (StudioTurnChips recapContinue chip) — no pane banner here anymore.

  // P7 · guided-tour deep-link: `forceView` forces the board's 看板/甘特图/活动日志
  // toggle deterministically (mirrors WritingBlock's `forceTab`/ReferencePanel's
  // `forceTab` one-shot pattern) so a tour step can land on 活动日志 without ever
  // re-fighting the student's own later Segmented click.
  it("forceView='log' switches to 活动日志 once, ref-guarded (P7 guided-tour deep-link)", async () => {
    mockGetPlan.mockResolvedValue([
      { id: "i1", title: "读：找反例", tag: "read", column: "todo", stage: "阶段一", refMaterialId: null, start: 0, days: 2, position: 0 },
    ]);
    const onConsumed = vi.fn();
    const slot = document.createElement("div");
    document.body.appendChild(slot);

    const { rerender } = render(
      <ChatProvider>
        <StudioAiSlotContext.Provider value={slot}>
          <PlanBlock
            projectId="p1"
            title="中国是否让地球更可持续？"
            qualification="拓展论文 EE"
            proposal={FILLED_PROPOSAL}
            createdAt="2026-08-01T00:00:00Z"
            phase="working"
            refreshWorkspace={() => {}}
            forceView="log"
            onForceViewConsumed={onConsumed}
          />
        </StudioAiSlotContext.Provider>
      </ChatProvider>,
    );

    // Defaults to 甘特图, but `forceView` jumps straight to 活动日志ーboth the
    // empty-state copy and its tour anchor show up.
    expect(await screen.findByText("还没有记录")).toBeInTheDocument();
    expect(document.querySelector('[data-tour="manage-activity-log"]')).toBeInTheDocument();
    expect(onConsumed).toHaveBeenCalledTimes(1);

    // The student manually switches to 看板.
    await userEvent.click(screen.getByRole("button", { name: "看板" }));
    expect(await screen.findByText("读：找反例")).toBeInTheDocument();

    // A rerender with the SAME forceView must not snap back to 活动日志 (ref
    // guard applies each distinct value at most once) or re-fire the callback.
    rerender(
      <ChatProvider>
        <StudioAiSlotContext.Provider value={slot}>
          <PlanBlock
            projectId="p1"
            title="中国是否让地球更可持续？"
            qualification="拓展论文 EE"
            proposal={FILLED_PROPOSAL}
            createdAt="2026-08-01T00:00:00Z"
            phase="working"
            refreshWorkspace={() => {}}
            forceView="log"
            onForceViewConsumed={onConsumed}
          />
        </StudioAiSlotContext.Provider>
      </ChatProvider>,
    );
    expect(await screen.findByText("读：找反例")).toBeInTheDocument();
    expect(onConsumed).toHaveBeenCalledTimes(1);
  });

  // Round 3, Part 3: 管理 used to portal NOTHING into the shared AiPanel slot
  // (a blank 印记 panel) — it now portals the SAME `StudioCoachChat` every
  // other room shows (mirrors ReadingBlock's `useStudioAiSlot`+`createPortal`
  // contract), reading the ONE hoisted thread — not a second conversation.
  it("Round 3: portals the shared StudioCoachChat (the one continuous 印记 thread) into the AiPanel slot", async () => {
    renderWithAiSlot(
      <PlanBlock
        projectId="p1"
        title="中国是否让地球更可持续？"
        qualification="拓展论文 EE"
        proposal={FILLED_PROPOSAL}
        createdAt="2026-08-01T00:00:00Z"
        phase="working"
        refreshWorkspace={() => {}}
      />,
      [{ role: "ai", text: "我们上次聊到判断尺度。", card: null }],
    );
    // The hoisted store's existing thread shows via the portaled StudioCoachChat
    // — proof it's the SAME conversation, not an empty/second panel.
    expect(await screen.findByText("我们上次聊到判断尺度。")).toBeInTheDocument();
    // The shared Composer (StudioCoachChat's own) is present too.
    expect(screen.getByPlaceholderText("和印记说说你的项目……（Shift+Enter 换行）")).toBeInTheDocument();
  });
});
