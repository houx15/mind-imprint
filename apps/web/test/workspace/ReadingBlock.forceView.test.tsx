import { describe, it, expect, vi, beforeEach } from "vitest";
import { useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import type { Reference } from "@mind-imprint/contracts";
import { StudioAiSlotContext } from "@/studio/ai/StudioAiSlot";
import { StudioChatContext, type StudioChatMsg } from "@/studio/ai/StudioChatContext";

// A single already-read source so the 列表 (library) table has a row — the
// table only renders when viewMode==="list" AND refs.length>0.
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
  readingReason: "",
  readingFocus: "",
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
vi.mock("@/workspace/blocks/exploration/ExplorationView", () => ({
  ExplorationView: ({ coach }: { coach?: ReactNode }) => (
    <div>
      graph-stub
      {coach}
    </div>
  ),
}));
vi.mock("@/workspace/export", () => ({ exportAnnotatedBib: vi.fn() }));

import { ReadingBlock } from "@/workspace/blocks/ReadingBlock";

// The room portals its coach into a StudioAiSlot and reads a StudioChat — stand
// in the same provider pair the real shell provides (mirrors ReadingBlock.test).
function ChatProvider({ children }: { children: ReactNode }) {
  const [messages, setMessages] = useState<StudioChatMsg[]>([]);
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

// A stable slot node the portal contract needs; created once per test in
// beforeEach so it survives a rerender (RTL keeps the same container).
let slot: HTMLDivElement;
beforeEach(() => {
  vi.clearAllMocks();
  slot = document.createElement("div");
  document.body.appendChild(slot);
});

// Harness keeps ChatProvider + slot stable so a rerender with a new forceView
// reconciles (never remounts ReadingBlock) — the ref guard is per-mount, so the
// "doesn't re-force after a manual toggle" assertion needs the SAME mount.
function Harness({ projectId, forceView }: { projectId: string; forceView?: "list" | "graph" }) {
  return (
    <ChatProvider>
      <StudioAiSlotContext.Provider value={slot}>
        <ReadingBlock projectId={projectId} title="T" setReadingSource={() => {}} forceView={forceView} />
      </StudioAiSlotContext.Provider>
    </ChatProvider>
  );
}

const libraryTable = () => document.querySelector('[data-tour="library-table"]');

describe("ReadingBlock · forceView (tour deep-link)", () => {
  it("forceView='list' overrides the mount-time graph default and shows the library table", async () => {
    render(<Harness projectId="fv-list" forceView="list" />);
    // The library table (list view) is forced open, not the default graph.
    await waitFor(() => expect(libraryTable()).toBeInTheDocument());
    expect(screen.queryByText("graph-stub")).toBeNull();
  });

  it("does NOT re-force after the student manually toggles away (ref guard)", async () => {
    const { rerender } = render(<Harness projectId="fv-guard" forceView="list" />);
    // Forced into 列表 first.
    await waitFor(() => expect(libraryTable()).toBeInTheDocument());

    // Student manually toggles to 探索 (graph).
    await userEvent.click(screen.getByRole("button", { name: "探索" }));
    expect(await screen.findByText("graph-stub")).toBeInTheDocument();

    // A rerender with the SAME forceView must not snap back to 列表 — the ref
    // guard applies each distinct forced value at most once, so it never fights
    // the student's later manual choice.
    rerender(<Harness projectId="fv-guard" forceView="list" />);
    expect(await screen.findByText("graph-stub")).toBeInTheDocument();
    expect(libraryTable()).toBeNull();
  });
});
