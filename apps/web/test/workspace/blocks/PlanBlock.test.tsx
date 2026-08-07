import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StudioAiSlotContext } from "@/studio/ai/StudioAiSlot";

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
  coach: vi.fn(async () => ({ reply: "", proposal: null, linkOffer: null, dimSuggestion: null })),
  getCoachHistory: vi.fn(async () => []),
  generatePlan: vi.fn(async () => []),
  createReference: vi.fn(async () => ({})),
}));

import { coach, getCoachHistory, getPlan } from "@/workspace/api/workspace";
import { PlanBlock } from "@/workspace/blocks/PlanBlock";

const mockCoach = vi.mocked(coach);
const mockGetCoachHistory = vi.mocked(getCoachHistory);
const mockGetPlan = vi.mocked(getPlan);

const EMPTY_PROPOSAL = { objective: "", reason: "", activities: "", resources: "", counterpoints: "" };
const FILLED_PROPOSAL = {
  objective: "论证中国是否让地球更可持续",
  reason: "关心气候变化",
  activities: "读 NASA/Nature Sustainability",
  resources: "Zotero、图书馆数据库",
  counterpoints: "",
};

// The AiPanel body is a real DOM node the panel hands down via context; the
// portal contract (Task 4) needs a genuine element to portal into (jsdom
// createPortal requires it), and RTL's `screen` queries document.body — where
// this node lives — so portaled content is found the same as any other.
function renderWithAiSlot(ui: React.ReactElement) {
  const slot = document.createElement("div");
  document.body.appendChild(slot);
  return render(<StudioAiSlotContext.Provider value={slot}>{ui}</StudioAiSlotContext.Provider>);
}

beforeEach(() => {
  vi.clearAllMocks();
  mockCoach.mockResolvedValue({ reply: "", proposal: null, linkOffer: null, dimSuggestion: null });
  mockGetCoachHistory.mockResolvedValue([]);
  mockGetPlan.mockResolvedValue([]);
});

describe("PlanBlock · forming coach on the shared AiPanel (Task 5)", () => {
  it("renders the scripted intro in the shared ChatLog and the summon shelf, portaled into the AI slot", async () => {
    renderWithAiSlot(
      <PlanBlock projectId="p1" title="T" qualification="拓展论文 EE" proposal={EMPTY_PROPOSAL} onOpenRoom={() => {}} refreshWorkspace={() => {}} />,
    );

    // The scripted intro (not an LLM call) renders via the shared ChatLog.
    expect(await screen.findByText(/先想清楚四件事/)).toBeInTheDocument();
    // The summon shelf (CoachCardPanel, FORMING_DECK) is always visible.
    expect(await screen.findByRole("button", { name: "提问卡" })).toBeInTheDocument();
  });

  it("sends a message through the shared Composer and appends the coach's reply via ChatLog", async () => {
    mockCoach.mockResolvedValueOnce({
      reply: "你想回答的到底是什么问题？",
      proposal: null,
      linkOffer: null,
      dimSuggestion: null,
    });
    renderWithAiSlot(
      <PlanBlock projectId="p1" title="T" qualification="拓展论文 EE" proposal={EMPTY_PROPOSAL} onOpenRoom={() => {}} refreshWorkspace={() => {}} />,
    );
    await screen.findByText(/先想清楚四件事/);

    const textarea = screen.getByPlaceholderText("说说你的想法……（Shift+Enter 换行）");
    await userEvent.type(textarea, "我想研究中国的碳排放");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    expect(mockCoach).toHaveBeenCalledWith("p1", "forming", "我想研究中国的碳排放");
    expect(await screen.findByText("你想回答的到底是什么问题？")).toBeInTheDocument();
    // The student's own turn also lands in the shared log.
    expect(screen.getByText("我想研究中国的碳排放")).toBeInTheDocument();
  });

  it("生成项目计划 unlocks only once all FOUR required dims are filled — 反例/张力 stays optional (spec §5 gate)", async () => {
    renderWithAiSlot(
      <PlanBlock projectId="p1" title="T" qualification="拓展论文 EE" proposal={EMPTY_PROPOSAL} onOpenRoom={() => {}} refreshWorkspace={() => {}} />,
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
        onOpenRoom={() => {}}
        refreshWorkspace={() => {}}
      />,
    );

    expect(await screen.findByText("读：找反例")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "看板" })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "活动日志" }));
    await waitFor(() => expect(screen.getByText("还没有记录")).toBeInTheDocument());
  });
});
