import { describe, it, expect, vi } from "vitest";
import { useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import { StudioAiSlotContext } from "@/studio/ai/StudioAiSlot";
import { StudioChatContext, type StudioChatMsg } from "@/studio/ai/StudioChatContext";

// P3 Task 2 (guided projects tour) · sanity check that the mechanical
// `data-tour` anchor sweep landed on real, rendered DOM nodes for the
// TourRunner (Task 3) to spotlight — not a full behavioral re-test of these
// rooms (their own suites already cover that). Mirrors the render harnesses
// ReviewBlock.test.tsx / WritingBlock.test.tsx already use for the shared
// AiPanel portal contract.

vi.mock("@/workspace/api/workspace", () => ({
  getReflection: vi.fn(async () => ({ answers: [], done: false })),
  putReflection: vi.fn(async () => ({ answers: [], done: false })),
  getAIUseDraft: vi.fn(async () => ({
    record: { coachTurns: 0, cardsProposed: 0, cardsAccepted: 0, cardsDismissed: 0, sourcesOpened: 0, llmCallsByPurpose: {}, ghostwroteEssay: false, predictedScore: false },
    draft: { usedFor: "", notUsedFor: "" },
  })),
  postAIUse: vi.fn(async () => {}),
  coach: vi.fn(),
  getCoachHistory: vi.fn(async () => ({ messages: [], hasMore: false, recap: null, nextCursor: null })),
  reflectProjectCard: vi.fn(async () => ({ cardInstanceId: "ci1", reply: "" })),
  persistProjectCard: vi.fn(async () => ({ cardInstanceId: "ci1" })),
  dismissProposal: vi.fn(async () => {}),
  getLog: vi.fn(async () => []),
  getDraft: vi.fn(async () => "我的草稿第一段。"),
  getOutline: vi.fn(async () => []),
  putOutline: vi.fn(async () => []),
  getSnippets: vi.fn(async () => []),
  putSnippets: vi.fn(async () => []),
  getLibrary: vi.fn(async () => ({ collections: [], references: [] })),
}));
vi.mock("@/api/projects", () => ({
  finishProject: vi.fn(async () => ({ status: "evaluating" })),
  finishWriting: vi.fn(async () => ({ writingFinished: true })),
  reopenWriting: vi.fn(async () => ({ writingFinished: false })),
}));
vi.mock("@/api/writing", () => ({
  putBuffer: vi.fn(async () => {}),
  flushBufferKeepalive: vi.fn(),
  runDraftReview: vi.fn(),
}));
vi.mock("@/api/exploration", () => ({ getExploration: vi.fn(async () => ({ leads: [], danglingSourceIds: [] })) }));
vi.mock("@/workspace/export", () => ({ exportDraftDocx: vi.fn(async () => new Blob()) }));
vi.mock("@/api/proposalAnnotations", () => ({
  getProposalAnnotations: vi.fn(async () => []),
  reviewProposalAnnotations: vi.fn(async () => []),
}));

import { ReviewBlock } from "@/workspace/blocks/ReviewBlock";
import { WritingBlock } from "@/workspace/blocks/WritingBlock";

const PROPOSAL = { objective: "论证中国是否让地球更可持续", reason: "关心气候", activities: "读 NASA/Nature", resources: "Zotero", counterpoints: "" };

// The AiPanel body is a real DOM node the panel hands down via context; the
// portal contract needs a genuine element to portal into (jsdom's
// createPortal requires it) — mirrors ReviewBlock.test.tsx/WritingBlock.test.tsx.
function renderWithAiSlot(ui: React.ReactElement) {
  const slot = document.createElement("div");
  document.body.appendChild(slot);
  return render(<StudioAiSlotContext.Provider value={slot}>{ui}</StudioAiSlotContext.Provider>);
}

function ChatProvider({ children }: { children: React.ReactNode }) {
  const [messages, setMessages] = useState<StudioChatMsg[]>([]);
  return (
    <StudioChatContext.Provider
      value={{
        messages,
        setMessages,
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
        started: true,
        startJourney: async () => {},
        starting: false,
      }}
    >
      {children}
    </StudioChatContext.Provider>
  );
}

function renderWritingWithSlot(ui: React.ReactElement) {
  const slot = document.createElement("div");
  document.body.appendChild(slot);
  return render(
    <ChatProvider>
      <StudioAiSlotContext.Provider value={slot}>{ui}</StudioAiSlotContext.Provider>
    </ChatProvider>,
  );
}

describe("data-tour anchors · studio rooms (P3 Task 2)", () => {
  it("ReviewBlock renders the reflection-prompts anchor", async () => {
    renderWithAiSlot(<ReviewBlock projectId="p1" proposal={PROPOSAL} status="working" writingFinished={true} />);
    expect(await screen.findByText(/回过头看看这一程/)).toBeInTheDocument();
    // eslint-disable-next-line testing-library/no-node-access
    expect(document.querySelector('[data-tour="reflection-prompts"]')).toBeInTheDocument();
  });

  it("WritingBlock renders the writing-tabs anchor", async () => {
    renderWritingWithSlot(
      <WritingBlock projectId="p1" title="T" proposal={PROPOSAL} status="working" writingFinished={false} refreshWorkspace={() => {}} />,
    );
    await waitFor(() => {
      // eslint-disable-next-line testing-library/no-node-access
      expect(document.querySelector('[data-tour="writing-tabs"]')).toBeInTheDocument();
    });
    // eslint-disable-next-line testing-library/no-node-access
    expect(document.querySelector('[data-tour="writing-tabs"]')?.textContent).toContain("大纲");
  });
});
