import { describe, it, expect, vi, beforeEach } from "vitest";
import { useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StudioChatContext, type StudioChatMsg } from "@/studio/ai/StudioChatContext";

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
import { StudioCoachChat } from "@/studio/ai/StudioCoachChat";

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
        pendingCard: null,
        confirmNote: () => {},
        dismissNote: () => {},
        openCard: () => {},
        dismissCard: () => {},
        pendingQuestion: null,
        confirmQuestion: () => {},
        dismissQuestion: () => {},
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
          pendingCard: null,
          confirmNote: () => {},
          dismissNote: () => {},
          openCard: () => {},
          dismissCard: () => {},
          pendingQuestion: null,
          confirmQuestion: () => {},
          dismissQuestion: () => {},
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
