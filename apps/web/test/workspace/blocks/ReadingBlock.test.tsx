import { describe, it, expect, vi, beforeEach } from "vitest";
import { useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import type { Reference } from "@mind-imprint/contracts";
import { StudioAiSlotContext } from "@/studio/ai/StudioAiSlot";
import { StudioChatContext, type StudioChatMsg } from "@/studio/ai/StudioChatContext";

// A single already-read source with no stage tag yet, carrying a saved brief
// (readingReason/readingFocus) so we can assert the inline stage picker persists
// via the full-replace reading-brief without wiping those fields.
const ref: Reference = {
  id: "r1",
  title: "Chen et al. (2019), Nature Sustainability",
  classification: "期刊论文",
  author: "Chen, C. et al.",
  credentials: "同行评议",
  year: "2019",
  url: "https://doi.org/x",
  tags: [],
  collectionId: null,
  credibility: "strong",
  evaluation: "",
  readingNote: "",
  decision: "use",
  pending: false,
  searchHints: [],
  materialId: "m1",
  notes: [],
  phaseTag: null,
  readingReason: "看它是否支持我的主张",
  readingFocus: "证据强度",
  takeaway: null,
  readingStatus: "to_read",
};

vi.mock("@/workspace/api/workspace", () => ({
  getLibrary: vi.fn(async () => ({ collections: [], references: [ref] })),
  createCollection: vi.fn(),
  createReference: vi.fn(),
  patchReference: vi.fn(async () => {}),
  enterReading: vi.fn(),
  pasteContent: vi.fn(),
  NoReadableContentError: class extends Error {},
}));
vi.mock("@/api/reading", () => ({ putReadingBrief: vi.fn(async () => {}) }));
vi.mock("@/api/exploration", () => ({ getExploration: vi.fn(async () => ({ leads: [], danglingSourceIds: [] })) }));
// Task 3 (P2a): ExplorationView no longer receives a `coach` slot at all (the
// room's own docked coach is gone) — the stub renders whatever it's handed so
// a test can assert nothing coach-shaped comes through. The stub also exposes
// a button to simulate a rabbit-hole card's `onCardReflected` callback, so a
// test can assert the reflect result now bridges into the shared studio
// thread instead of a local FloatingCoach buffer.
vi.mock("@/workspace/blocks/exploration/ExplorationView", () => ({
  ExplorationView: ({
    coach,
    onCardReflected,
  }: {
    coach?: ReactNode;
    onCardReflected?: (studentText: string, reply: string, card?: unknown) => void;
  }) => (
    <div>
      graph-stub
      {onCardReflected && (
        <button type="button" onClick={() => onCardReflected("挖了一层", "不错的发现", undefined)}>
          simulate reflect
        </button>
      )}
      {coach}
    </div>
  ),
}));
vi.mock("@/workspace/export", () => ({ exportAnnotatedBib: vi.fn() }));

import { ReadingBlock } from "@/workspace/blocks/ReadingBlock";
import { putReadingBrief } from "@/api/reading";

const mockBrief = vi.mocked(putReadingBrief);

// Task 3 (P2a): ReadingBlock now portals its coach into the constant AiPanel
// via `useStudioAiSlot()` / reads the shared thread via `useStudioChat()` —
// same room→panel contract as PlanBlock/WritingBlock/ReviewBlock. Tests stand
// in the same provider pair the real shell provides.
function ChatProvider({ initial = [], children }: { initial?: StudioChatMsg[]; children: React.ReactNode }) {
  const [messages, setMessages] = useState<StudioChatMsg[]>(initial);
  const [sending, setSending] = useState(false);
  const sendStudioTurn = async (userInput: string) => {
    setMessages((c) => [...c, { role: "student", text: userInput }]);
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
        pendingQuestion: null,
        confirmQuestion: () => {},
        dismissQuestion: () => {},
        pendingNextStep: null,
        advanceToNextStep: () => {},
        advanceStatusTo: async () => {},
        historyHasMore: false,
        loadEarlier: () => {},
        loadingEarlier: false,
        started: true,
        startJourney: async () => {},
        starting: false,
      }}
    >
      {children}
    </StudioChatContext.Provider>
  );
}

// A real DOM node the portal contract needs (jsdom createPortal requires it);
// RTL's `screen` queries document.body — where this node lives — so portaled
// content is found the same as any other.
function renderReadingBlock(ui: React.ReactElement, initialMessages: StudioChatMsg[] = []) {
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
});

describe("ReadingBlock · #4 graph-default + #2 stage tag", () => {
  it("opens on the 探索图谱 when the project already has sources (#4)", async () => {
    renderReadingBlock(<ReadingBlock projectId="p1" title="T" setReadingSource={() => {}} />);
    // graph is the default view once there are references
    expect(await screen.findByText("graph-stub")).toBeInTheDocument();
    // the list grid header is not shown until the student switches to 列表
    expect(screen.queryByText("标题")).toBeNull();
  });

  // #18: 探索图谱 is now the default even for a brand-new EMPTY library — the
  // graph shows its own empty/线索 state (with ＋添加来源 right there), so
  // there's no reason to force 列表 first just because nothing exists yet.
  it("opens on the 探索图谱 even when the library is empty (#18)", async () => {
    const { getLibrary } = await import("@/workspace/api/workspace");
    vi.mocked(getLibrary).mockResolvedValueOnce({ collections: [], references: [] });
    renderReadingBlock(<ReadingBlock projectId="p2" title="T" setReadingSource={() => {}} />);
    expect(await screen.findByText("graph-stub")).toBeInTheDocument();
    // 列表 is still one click away, with its own empty-state affordance
    await userEvent.click(screen.getByRole("button", { name: "图书馆" }));
    expect(await screen.findByText(/先加一篇来源/)).toBeInTheDocument();
  });

  it("stage picker persists via the reading brief, carrying reason/focus (#2)", async () => {
    renderReadingBlock(<ReadingBlock projectId="p1" title="T" setReadingSource={() => {}} />);
    await screen.findByText("graph-stub");
    // switch to the list where the inline stage picker lives
    await userEvent.click(screen.getByRole("button", { name: "图书馆" }));
    const picker = await screen.findByLabelText("用于哪个阶段");
    await userEvent.selectOptions(picker, "支持论点");
    await waitFor(() =>
      expect(mockBrief).toHaveBeenCalledWith("p1", "r1", {
        readingReason: "看它是否支持我的主张",
        readingFocus: "证据强度",
        phaseTag: "支持论点",
      }),
    );
  });
});

// Task 3 (P2a): the room's own coach (docked in 探索图谱, a floating chip in
// 列表 — both a context-isolated "find_sources" thread) is deleted. 印记 is
// now the ONE constant rail, portaled via `useStudioAiSlot()` — present in
// BOTH view modes, reading the SAME shared studio thread every other room
// does. No 探索图谱-vs-列表 asymmetry any more.
describe("ReadingBlock · joins the constant 印记 rail (Task 3, P2a)", () => {
  it("portals the shared StudioCoachChat into the constant AiPanel slot, in both view modes", async () => {
    renderReadingBlock(
      <ReadingBlock projectId="q3-graph" title="T" setReadingSource={() => {}} />,
      [{ role: "ai", text: "一段既有的对话" }],
    );
    await screen.findByText("graph-stub");
    // The shared thread's content shows via the portaled StudioCoachChat.
    expect(await screen.findByText("一段既有的对话")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "图书馆" }));
    // Same portaled coach persists across the view-mode toggle — ReadingBlock
    // no longer mounts a per-view coach of its own.
    expect(screen.getByText("一段既有的对话")).toBeInTheDocument();
  });

  it("no longer renders the deleted FloatingCoach (no floating chip, no docked header, no ExplorationView coach slot)", async () => {
    renderReadingBlock(<ReadingBlock projectId="q3-nofloat" title="T" setReadingSource={() => {}} />);
    await screen.findByText("graph-stub");
    expect(screen.queryByRole("button", { name: /问印记 · 找资料/ })).toBeNull();
    expect(screen.queryByText("印记 · 找资料")).toBeNull();

    await userEvent.click(screen.getByRole("button", { name: "图书馆" }));
    expect(screen.queryByRole("button", { name: /问印记 · 找资料/ })).toBeNull();
    expect(screen.queryByText("印记 · 找资料")).toBeNull();
  });

  it("bridges a rabbit-hole reflect result from ExplorationView into the shared studio thread", async () => {
    renderReadingBlock(<ReadingBlock projectId="q3-bridge" title="T" setReadingSource={() => {}} />);
    await screen.findByText("graph-stub");
    await userEvent.click(screen.getByRole("button", { name: "simulate reflect" }));
    // Both the student turn and 印记's reply land in the ONE shared thread —
    // rendered by the portaled StudioCoachChat, no local FloatingCoach buffer.
    expect(await screen.findByText("挖了一层")).toBeInTheDocument();
    expect(await screen.findByText("不错的发现")).toBeInTheDocument();
  });
});

// #3 · when the library is empty, the 列表 empty-state copy names the known
// project topic (a static interpolated string — no model call) instead of a
// generic prompt. Each test uses its own fresh projectId — ReadingBlock's
// viewModeMemo is a module-level Map keyed by projectId that outlives any
// single test, so reusing an id another test already toggled to 列表 would
// leave the next test starting on the wrong view.
describe("ReadingBlock · #3 topic-aware empty state", () => {
  it("interpolates the project topic into the 列表 empty-state copy", async () => {
    const { getLibrary } = await import("@/workspace/api/workspace");
    vi.mocked(getLibrary).mockResolvedValueOnce({ collections: [], references: [] });
    renderReadingBlock(
      <ReadingBlock
        projectId="p3-topic"
        title="中国是否让地球变得更可持续？"
        setReadingSource={() => {}}
      />,
    );
    // 探索图谱 (its own empty state) is the default now — switch to 列表.
    await screen.findByText("graph-stub");
    await userEvent.click(screen.getByRole("button", { name: "图书馆" }));
    expect(await screen.findByText(/围绕「中国是否让地球变得更可持续？」/)).toBeInTheDocument();
  });

  it("falls back to the generic empty-state copy when no topic is set", async () => {
    const { getLibrary } = await import("@/workspace/api/workspace");
    vi.mocked(getLibrary).mockResolvedValueOnce({ collections: [], references: [] });
    renderReadingBlock(<ReadingBlock projectId="p3-empty" title="" setReadingSource={() => {}} />);
    await screen.findByText("graph-stub");
    await userEvent.click(screen.getByRole("button", { name: "图书馆" }));
    expect(await screen.findByText(/先加一篇来源/)).toBeInTheDocument();
  });
});
