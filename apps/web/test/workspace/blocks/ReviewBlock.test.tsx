import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StudioAiSlotContext } from "@/studio/ai/StudioAiSlot";

// S5 · the review room's AI-use retrospective panel + defense-readiness coach
// thread + (S5·#20/#21) the two-stage 写作→回顾 flow: view-only lock until writing
// is finished, and the mirror only after the student finishes her own reflection.
//
// Task 6 (studio agentic rebuild): the coach thread + mirror now portal into
// the constant AiPanel via `useStudioAiSlot`/`createPortal` (same contract
// PlanBlock exercises in Task 5), on the shared `ChatLog`/`Composer` instead
// of a bespoke inline chat + textarea — so every render below stands up a
// real DOM node via `StudioAiSlotContext.Provider`, matching the AiPanel
// body in the real shell.
vi.mock("@/workspace/api/workspace", () => ({
  getReflection: vi.fn(async () => ({ answers: [], done: false })),
  putReflection: vi.fn(async () => ({ answers: [], done: false })),
  getMirror: vi.fn(async () => null),
  postMirror: vi.fn(async () => null),
  getAIUseDraft: vi.fn(),
  postAIUse: vi.fn(async (_id: string, s: unknown) => s),
  coach: vi.fn(),
  getCoachHistory: vi.fn(async () => []),
  // used by the #21 reflection card shelf (CoachCardPanel)
  reflectProjectCard: vi.fn(async () => ({ cardInstanceId: "ci1", reply: "" })),
  persistProjectCard: vi.fn(async () => ({ cardInstanceId: "ci1" })),
  dismissProposal: vi.fn(async () => {}),
}));
vi.mock("@/api/projects", () => ({ finishProject: vi.fn(async () => ({ status: "evaluating" })) }));

import { getAIUseDraft, postAIUse, coach, getMirror, postMirror, putReflection } from "@/workspace/api/workspace";
import { finishProject } from "@/api/projects";
import { ReviewBlock } from "@/workspace/blocks/ReviewBlock";

const mockDraft = vi.mocked(getAIUseDraft);
const mockPostAIUse = vi.mocked(postAIUse);
const mockCoach = vi.mocked(coach);
const mockGetMirror = vi.mocked(getMirror);
const mockPostMirror = vi.mocked(postMirror);
const mockPutReflection = vi.mocked(putReflection);
const mockFinishProject = vi.mocked(finishProject);

const PROPOSAL = { objective: "论证中国是否让地球更可持续", reason: "关心气候", activities: "读 NASA/Nature", resources: "Zotero", counterpoints: "" };

const MIRROR = {
  sections: [{ title: "你怎么想的", body: "你从溯源开始，撞上反例后做了让步。" }],
  carryForwards: ["下次先找反例", "把结论接回原题"],
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
  mockDraft.mockResolvedValue({
    record: {
      coachTurns: 12,
      cardsProposed: 2,
      cardsAccepted: 1,
      cardsDismissed: 1,
      sourcesOpened: 5,
      llmCallsByPurpose: { coach: 12 },
      ghostwroteEssay: false,
      predictedScore: false,
    },
    draft: { usedFor: "溯源提问", notUsedFor: "代写正文" },
  });
  // Task 9a: reflection is a context-isolated sub-agent coach (retained legacy
  // path) — the reply is shaped as OrchestratorReply (narrate/directive/note/
  // card/reviewRequested), never note/card/reviewRequested on this path.
  mockCoach.mockResolvedValue({
    narrate: "你先自己答——你的结论回答了原题吗？",
    directive: { stage: "topic_discussion", openTool: "chat", widthTier: "chat", reference: [], updatedAtTurn: 0 },
    note: null,
    card: null,
    reviewRequested: false,
  });
});

describe("ReviewBlock · AI-use retrospective (S5)", () => {
  it("renders the objective record + seeded statement and saves the student's edit", async () => {
    renderWithAiSlot(<ReviewBlock projectId="p1" proposal={PROPOSAL} status="working" writingFinished={true} />);

    // objective record line (the absences included)
    expect(await screen.findByText(/12 轮对话/)).toBeInTheDocument();
    expect(screen.getByText(/没有替你写正文/)).toBeInTheDocument();

    // seeded fields
    const used = screen.getByPlaceholderText(/澄清检索词/) as HTMLTextAreaElement;
    expect(used.value).toBe("溯源提问");

    // student edits + saves → postAIUse with the edited text
    fireEvent.change(used, { target: { value: "澄清检索词、核对来源功能" } });
    fireEvent.click(screen.getByRole("button", { name: "保存声明" }));
    await waitFor(() => {
      expect(mockPostAIUse).toHaveBeenCalledWith("p1", { usedFor: "澄清检索词、核对来源功能", notUsedFor: "代写正文" });
    });
  });

  it("supportive reflection-completion thread sends a reflection-scope coach turn through the shared Composer", async () => {
    renderWithAiSlot(<ReviewBlock projectId="p1" proposal={PROPOSAL} status="working" writingFinished={true} />);
    await screen.findByText(/12 轮对话/); // wait for mount loads

    // The coach column is portaled into the AiPanel slot, rendered on the
    // shared Composer (Task 5's pattern) — send is the aria-labelled "发送"
    // button, not the old bespoke "问印记" button.
    await userEvent.type(screen.getByPlaceholderText(/卡在哪一题/), "我的结论是不是太弱了");
    await userEvent.click(screen.getByRole("button", { name: "发送" }));

    await waitFor(() => {
      expect(mockCoach).toHaveBeenCalledWith("p1", "我的结论是不是太弱了", "reflection");
    });
    // Both the student's own turn and the AI reply land via the shared ChatLog.
    expect(screen.getByText("我的结论是不是太弱了")).toBeInTheDocument();
    expect(await screen.findByText(/你先自己答/)).toBeInTheDocument();
  });
});

describe("ReviewBlock · view-only lock before 完成写作 (#20)", () => {
  it("locks the room until writing is finished — no AI-use seed, no mirror, no finish", async () => {
    renderWithAiSlot(<ReviewBlock projectId="p1" proposal={PROPOSAL} status="working" writingFinished={false} />);

    // a calm lock notice — no nav button (印记 cues 完成写作 in the chat)
    expect(await screen.findByText(/写完初稿后，这里会解锁回顾/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /去写作房间/ })).toBeNull();

    // reflection textareas are read-only outline
    const areas = screen.getAllByPlaceholderText(/完成写作后在这里回顾/) as HTMLTextAreaElement[];
    expect(areas.length).toBeGreaterThan(0);
    areas.forEach((a) => expect(a).toBeDisabled());

    // nothing composes / no finish path
    expect(mockGetMirror).not.toHaveBeenCalled();
    expect(mockPostMirror).not.toHaveBeenCalled();
    expect(mockDraft).not.toHaveBeenCalled(); // AI-use draft not seeded
    expect(screen.queryByRole("button", { name: /我写完了我的反思/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /定稿并开始评估/ })).toBeNull();
    // the mirror pane (portaled into the AI slot) shows the "you first, AI
    // after" note (no compose)
    expect(screen.getByText(/你先说，AI 后照/)).toBeInTheDocument();
  });
});

describe("ReviewBlock · mutual reflection ordering (#21)", () => {
  it("composes the mirror only AFTER the student finishes her reflection, then gates 定稿", async () => {
    mockGetMirror.mockResolvedValue(null);
    mockPostMirror.mockResolvedValue(MIRROR);

    renderWithAiSlot(<ReviewBlock projectId="p1" proposal={PROPOSAL} status="working" writingFinished={true} />);
    await screen.findByText(/12 轮对话/);

    // BEFORE marking her reflection done: no mirror compose, no 定稿 button
    expect(mockPostMirror).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: /定稿并开始评估/ })).toBeNull();
    expect(screen.getByRole("button", { name: /我写完了我的反思/ })).toBeInTheDocument();

    // step (a): student marks HER reflection done
    await userEvent.click(screen.getByRole("button", { name: /我写完了我的反思/ }));
    await waitFor(() => expect(mockPutReflection).toHaveBeenCalledWith("p1", { answers: expect.any(Array), done: true }));

    // NOW the mirror composes (AI reflects second)
    await waitFor(() => expect(mockPostMirror).toHaveBeenCalledWith("p1"));
    expect(await screen.findByText(/你从溯源开始/)).toBeInTheDocument();

    // step (b): 定稿并评估 now available → finishProject
    const finalize = await screen.findByRole("button", { name: /定稿并开始评估/ });
    await userEvent.click(finalize);
    await userEvent.click(screen.getByRole("button", { name: "定稿并评估" })); // confirm
    await waitFor(() => expect(mockFinishProject).toHaveBeenCalledWith("p1"));
  });

  it("renders the reflection card shelf once unlocked (#21)", async () => {
    renderWithAiSlot(<ReviewBlock projectId="p1" proposal={PROPOSAL} status="working" writingFinished={true} />);
    await screen.findByText(/12 轮对话/);
    // REFLECTION_DECK cards are summonable by the student
    expect(await screen.findByRole("button", { name: /学习报告/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /元认知/ })).toBeInTheDocument();
  });
});

// Q4 followup (2026-08): the AI panel is the ACTIVE coach while she's
// editing — "印记陪你把回顾写完" + the reflection cards + 问印记 chat all live
// there (not buried at the bottom of the left column) — and it becomes the
// mirror ("你的思维印记") only once she's marked her own reflection done.
describe("ReviewBlock · Q4 coach-then-mirror AI panel", () => {
  it("shows the coach (not the mirror) in the AI panel while editing, then swaps to the mirror after 我写完了我的反思", async () => {
    mockGetMirror.mockResolvedValue(null);
    mockPostMirror.mockResolvedValue(MIRROR);

    renderWithAiSlot(<ReviewBlock projectId="p1" proposal={PROPOSAL} status="working" writingFinished={true} />);
    await screen.findByText(/12 轮对话/);

    // during editing: the coach heading + chat input are visible, the mirror
    // heading/compose is NOT
    expect(screen.getByText("印记陪你把回顾写完")).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/卡在哪一题/)).toBeInTheDocument();
    expect(screen.queryByText("你的思维印记")).toBeNull();
    expect(mockPostMirror).not.toHaveBeenCalled();

    // step (a): mark her own reflection done
    await userEvent.click(screen.getByRole("button", { name: /我写完了我的反思/ }));
    await waitFor(() => expect(mockPutReflection).toHaveBeenCalledWith("p1", { answers: expect.any(Array), done: true }));

    // now the AI panel swaps: mirror shows, coach thread is gone
    expect(await screen.findByText("你的思维印记")).toBeInTheDocument();
    expect(screen.queryByText("印记陪你把回顾写完")).toBeNull();
    await waitFor(() => expect(mockPostMirror).toHaveBeenCalledWith("p1"));
  });
});
