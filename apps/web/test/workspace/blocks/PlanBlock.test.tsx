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
  getCoachHistory: vi.fn(async () => []),
  generatePlan: vi.fn(async () => []),
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
        pendingCard: null,
        confirmNote: () => {},
        dismissNote: () => {},
        openCard: () => {},
        dismissCard: () => {},
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
  mockGetCoachHistory.mockResolvedValue([]);
  mockGetPlan.mockResolvedValue([]);
});

describe("PlanBlock · forming coach on the shared AiPanel (Task 5)", () => {
  it("renders the scripted intro in the shared ChatLog and the summon shelf, portaled into the AI slot", async () => {
    renderWithAiSlot(
      <PlanBlock projectId="p1" title="T" qualification="拓展论文 EE" proposal={EMPTY_PROPOSAL} phase="forming" onPlanGenerated={() => {}} refreshWorkspace={() => {}} />,
    );

    // The scripted intro (not an LLM call) renders via the shared ChatLog.
    expect(await screen.findByText(/先想清楚四件事/)).toBeInTheDocument();
    // The summon shelf (CoachCardPanel, FORMING_DECK) is always visible.
    expect(await screen.findByRole("button", { name: "提问卡" })).toBeInTheDocument();
  });

  it("sends a message through the shared Composer via the container-owned send loop", async () => {
    renderWithAiSlot(
      <PlanBlock projectId="p1" title="T" qualification="拓展论文 EE" proposal={EMPTY_PROPOSAL} phase="forming" onPlanGenerated={() => {}} refreshWorkspace={() => {}} />,
    );
    await screen.findByText(/先想清楚四件事/);

    const textarea = screen.getByPlaceholderText("说说你的想法……（Shift+Enter 换行）");
    await userEvent.type(textarea, "我想研究中国的碳排放");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    // The room no longer calls coach() directly — it drives the ONE shared send.
    expect(mockSend).toHaveBeenCalledWith("我想研究中国的碳排放", undefined);
    // The student's own turn lands in the shared store (mirrors the container).
    expect(await screen.findByText("我想研究中国的碳排放")).toBeInTheDocument();
  });

  it("生成项目计划 unlocks only once all FOUR required dims are filled — 反例/张力 stays optional (spec §5 gate)", async () => {
    renderWithAiSlot(
      <PlanBlock projectId="p1" title="T" qualification="拓展论文 EE" proposal={EMPTY_PROPOSAL} phase="forming" onPlanGenerated={() => {}} refreshWorkspace={() => {}} />,
    );
    await screen.findByText(/先想清楚四件事/);

    const gen = screen.getByRole("button", { name: /生成项目计划/ });
    expect(gen).toBeDisabled();

    // Fill three of the four required — still gated.
    await userEvent.type(screen.getByRole("textbox", { name: /^目标/ }), "以中国为例的研究问题");
    await userEvent.type(screen.getByRole("textbox", { name: /^缘由/ }), "关心气候矛盾");
    await userEvent.type(screen.getByRole("textbox", { name: /^活动与时间/ }), "溯源→读→写");
    expect(gen).toBeDisabled();

    // The 4th REQUIRED dim opens the gate — even though 反例/张力 is left empty.
    await userEvent.type(screen.getByRole("textbox", { name: /^资源/ }), "NASA、学校数据库");
    expect(gen).toBeEnabled();
    expect(screen.getByRole("textbox", { name: /^可能的反例/ })).toHaveValue("");
  });

  it("P2a Tier-1 strip: keeps 生成项目计划/让印记看看我的开题, drops the redundant 聊聊计划/写开题报告/中EN chrome", async () => {
    renderWithAiSlot(
      <PlanBlock projectId="p1" title="T" qualification="拓展论文 EE" proposal={EMPTY_PROPOSAL} phase="forming" onPlanGenerated={() => {}} refreshWorkspace={() => {}} />,
    );
    await screen.findByText(/先想清楚四件事/);

    // Kept (Tier-2, or a direct review request — both stay working).
    expect(screen.getByRole("button", { name: /生成项目计划/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "让印记看看我的开题" })).toBeInTheDocument();

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
        onPlanGenerated={() => {}}
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
        onPlanGenerated={() => {}}
        refreshWorkspace={() => {}}
      />,
    );

    expect(await screen.findByText("读：找反例")).toBeInTheDocument();
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
        onPlanGenerated={() => {}}
        refreshWorkspace={() => {}}
      />,
    );

    expect(await screen.findByText("读：找反例")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "查看我的题目" })).toBeNull();
    expect(screen.queryByRole("button", { name: "收起" })).toBeNull();
    expect(screen.queryByRole("button", { name: "进入 →" })).toBeNull();
  });
});
