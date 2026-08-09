import { describe, it, expect, vi, beforeEach } from "vitest";
import { useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StudioChatContext, type StudioChatMsg, type StudioChatValue } from "@/studio/ai/StudioChatContext";

/**
 * StudioCoachChat (Task 5 · history pagination): the studio thread now loads
 * only its RECENT page (`getCoachHistory`'s paginated `CoachHistoryPage`) —
 * this test exercises the piece StudioCoachChat itself owns: rendering the
 * digest recap card, showing 载入更早的对话 when `historyHasMore`, and
 * prepending an older page above the currently-loaded messages once clicked.
 *
 * The harness below mirrors WorkspaceContainer's real `loadEarlier` (fetch →
 * prepend → update cursor/hasMore) so the wiring through `useStudioChat()` is
 * exercised the same way the real container drives it, without dragging in
 * the whole shell.
 */

vi.mock("@/workspace/api/workspace", () => ({
  getCoachHistory: vi.fn(),
}));

import { getCoachHistory } from "@/workspace/api/workspace";
import { StudioCoachChat, StudioTurnChips, toChatMessages } from "@/studio/ai/StudioCoachChat";

const mockGetCoachHistory = vi.mocked(getCoachHistory);

function turnRole(i: number): "student" | "ai" {
  return i % 2 === 0 ? "student" : "ai";
}

const RECENT_PAGE = {
  messages: Array.from({ length: 20 }, (_, i) => ({
    role: turnRole(i),
    text: `第 ${i + 1} 轮`,
    card: null,
  })),
  hasMore: true,
  recap: "我们之前聊到X",
  nextCursor: "c1",
};

const OLDER_PAGE = {
  messages: [{ role: "ai" as const, text: "更早的第一句", card: null }],
  hasMore: false,
  recap: null,
  nextCursor: null,
};

// A slim stand-in for WorkspaceContainer's hoisted store — seeded from the
// "recent page" the way the container's load effect does, with `loadEarlier`
// wired to the (mocked) API the same way the container's does.
function Harness({ projectId = "p1" }: { projectId?: string }) {
  const [messages, setMessages] = useState<StudioChatMsg[]>(
    RECENT_PAGE.messages.map((m) => ({ role: m.role, text: m.text, card: m.card })),
  );
  const [sending, setSending] = useState(false);
  const [historyCursor, setHistoryCursor] = useState<string | null>(RECENT_PAGE.nextCursor);
  const [historyHasMore, setHistoryHasMore] = useState(RECENT_PAGE.hasMore);
  const [loadingEarlier, setLoadingEarlier] = useState(false);

  function loadEarlier() {
    if (!historyCursor || loadingEarlier) return;
    setLoadingEarlier(true);
    getCoachHistory(projectId, "studio", { before: historyCursor })
      .then((page) => {
        setMessages((prev) => [
          ...page.messages.map((m) => ({ role: m.role, text: m.text, card: m.card ?? null })),
          ...prev,
        ]);
        setHistoryCursor(page.nextCursor);
        setHistoryHasMore(page.hasMore);
      })
      .finally(() => setLoadingEarlier(false));
  }

  return (
    <StudioChatContext.Provider
      value={{
        messages,
        setMessages,
        sending,
        setSending,
        activeProjectIdRef: { current: projectId },
        sendStudioTurn: async () => true,
        projectId,
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
        historyHasMore,
        loadEarlier,
        loadingEarlier,
        started: true,
        startJourney: async () => {},
        starting: false,
      }}
    >
      <StudioCoachChat recap={RECENT_PAGE.recap} />
    </StudioChatContext.Provider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("StudioCoachChat · history pagination (Task 5)", () => {
  it("renders the digest recap card and a 载入更早的对话 control when the first page has more", async () => {
    render(<Harness />);

    expect(await screen.findByText("我们之前聊到X")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /载入更早的对话/ })).toBeInTheDocument();
  });

  it("never renders the old scripted CHAT_INTRO string", () => {
    render(<Harness />);
    expect(screen.queryByText(/把你手上的真实任务丢给我/)).toBeNull();
  });

  it("clicking 载入更早的对话 prepends the older page above the newer messages, then the control disappears", async () => {
    mockGetCoachHistory.mockResolvedValueOnce(OLDER_PAGE);
    render(<Harness />);

    const loadMore = screen.getByRole("button", { name: /载入更早的对话/ });
    await userEvent.click(loadMore);

    expect(mockGetCoachHistory).toHaveBeenCalledWith("p1", "studio", { before: "c1" });

    // The older turn lands…
    await waitFor(() => expect(screen.getByText("更早的第一句")).toBeInTheDocument());
    // …ABOVE the newer messages (DOM order, since the render prepends it).
    const older = screen.getByText("更早的第一句");
    const newer = screen.getByText("第 1 轮");
    expect(older.compareDocumentPosition(newer) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    // hasMore is now false → the control is gone.
    expect(screen.queryByRole("button", { name: /载入更早的对话/ })).toBeNull();
  });

  it("shows no 载入更早 control and no recap card when the thread has no earlier page", () => {
    mockGetCoachHistory.mockResolvedValue({ messages: [], hasMore: false, recap: null, nextCursor: null });
    render(
      <StudioChatContext.Provider
        value={{
          messages: [{ role: "ai", text: "你好", card: null }],
          setMessages: () => {},
          sending: false,
          setSending: () => {},
          activeProjectIdRef: { current: "p1" },
          sendStudioTurn: async () => true,
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
        <StudioCoachChat recap={null} />
      </StudioChatContext.Provider>,
    );
    expect(screen.queryByRole("button", { name: /载入更早的对话/ })).toBeNull();
    expect(screen.queryByText("我们之前聊到X")).toBeNull();
  });
});

// Round 3, Part 1: an AI turn's `text` now renders through the shared
// `ChatMarkdown` (bold/lists/links), not the old `**bold**`-only `renderRich`
// — a student turn stays plain text (she types prose, not markup).
describe("toChatMessages · markdown mapping (Round 3)", () => {
  it("renders an AI turn's markdown as real elements (list item + link + strong)", () => {
    const [msg] = toChatMessages([
      { role: "ai", text: "- a\n- b\n\n访问[这里](https://e.com)看看\n\n**重点**别忘了" },
    ]);
    expect(msg!.role).toBe("assistant");
    render(<>{msg!.node}</>);
    expect(screen.getByText("a").tagName).toBe("LI");
    expect(screen.getByRole("link", { name: "这里" })).toHaveAttribute("href", "https://e.com");
    expect(screen.getByText("重点").tagName).toBe("STRONG");
  });

  it("keeps a student turn as plain text — markdown syntax is never mis-rendered", () => {
    const [msg] = toChatMessages([{ role: "student", text: "我用了 **不是加粗** 和 - 不是列表" }]);
    expect(msg!.role).toBe("student");
    render(<>{msg!.node}</>);
    expect(screen.getByText("我用了 **不是加粗** 和 - 不是列表")).toBeInTheDocument();
    expect(screen.queryByRole("listitem")).toBeNull();
  });
});

// Task 7 (hidden-subagent hints, 2026-08-08): a `StudioChatMsg` with `hint` set
// maps to a system-role node rendering a `SubagentHint` (done) instead of a
// chat bubble — this is how `sendStudioTurn`'s plan/compaction acknowledgments
// (appended after 印记's narrate line) actually reach the thread.
describe("toChatMessages · hint mapping (Task 7)", () => {
  it("maps a message with `hint` to a system node rendering the hint text", () => {
    const [msg] = toChatMessages([{ role: "ai", text: "", hint: "subagent 已整理研究计划" }]);
    expect(msg!.role).toBe("system");
    render(<>{msg!.node}</>);
    expect(screen.getByText("subagent 已整理研究计划")).toBeInTheDocument();
  });

  it("a hint message renders inside the live thread (via StudioCoachChat) alongside a normal turn", async () => {
    render(
      <StudioChatContext.Provider
        value={{
          messages: [
            { role: "student", text: "帮我整理一下研究计划" },
            { role: "ai", text: "好的，我已经把计划列出来了。" },
            { role: "ai", text: "", hint: "subagent 已整理研究计划" },
          ],
          setMessages: () => {},
          sending: false,
          setSending: () => {},
          activeProjectIdRef: { current: "p1" },
          sendStudioTurn: async () => true,
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
        <StudioCoachChat recap={null} />
      </StudioChatContext.Provider>,
    );

    expect(await screen.findByText("好的，我已经把计划列出来了。")).toBeInTheDocument();
    expect(screen.getByText("subagent 已整理研究计划")).toBeInTheDocument();
  });
});

// Bug fix (2026-08-08): tapping 记进 must swap the actionable chip for a lasting
// "已记进" acknowledgment instead of making it vanish. StudioTurnChips renders
// the actionable NoteConfirmChip while `pendingNote` is set, and the quiet
// recorded chip once only `confirmedNote` is set.
describe("StudioTurnChips · note confirm → 已记进 acknowledgment", () => {
  function chipsValue(over: Partial<StudioChatValue>): StudioChatValue {
    return {
      messages: [] as StudioChatMsg[],
      setMessages: () => {},
      sending: false,
      setSending: () => {},
      activeProjectIdRef: { current: "p1" },
      sendStudioTurn: async () => true,
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
      ...over,
    };
  }

  it("shows the actionable 记进 chip while a note is pending", () => {
    render(
      <StudioChatContext.Provider value={chipsValue({ pendingNote: { section: "objective", value: "研究北京绿地与心理健康" } })}>
        <StudioTurnChips />
      </StudioChatContext.Provider>,
    );
    expect(screen.getByRole("button", { name: /记进「目标」/ })).toBeInTheDocument();
    expect(screen.queryByText(/已记进/)).toBeNull();
  });

  it("shows the 已记进 acknowledgment (no buttons) once confirmed", () => {
    render(
      <StudioChatContext.Provider value={chipsValue({ pendingNote: null, confirmedNote: { section: "objective", value: "研究北京绿地与心理健康" } })}>
        <StudioTurnChips />
      </StudioChatContext.Provider>,
    );
    expect(screen.getByText(/已记进「目标」/)).toBeInTheDocument();
    expect(screen.getByText("研究北京绿地与心理健康")).toBeInTheDocument();
    // The acknowledgment is quiet — no actionable buttons.
    expect(screen.queryByRole("button", { name: /记进/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /跳过/ })).toBeNull();
  });

  it("a fresh pending note takes precedence over a stale confirmed one", () => {
    render(
      <StudioChatContext.Provider
        value={chipsValue({
          pendingNote: { section: "reason", value: "新的一条" },
          confirmedNote: { section: "objective", value: "旧的一条" },
        })}
      >
        <StudioTurnChips />
      </StudioChatContext.Provider>,
    );
    expect(screen.getByRole("button", { name: /记进「缘由」/ })).toBeInTheDocument();
    expect(screen.queryByText(/已记进/)).toBeNull();
  });
});
