import { createContext, useContext } from "react";
import type { Dispatch, MutableRefObject, SetStateAction } from "react";
import type { CardTurnRef, NoteProposal, CardProposalWire, QuestionProposal, NextStep, LinkOffer } from "@mind-imprint/contracts";

/**
 * StudioChatContext (coach-persistence hoist).
 *
 * The AI coach conversation — the ONE continuous 印记 thread that spans 立项 and
 * 写作 — used to live as local `useState` inside each room (PlanBlock's
 * `FormingPhase`, WritingBlock's `CoachRail`). Because a room switch unmounts
 * one room and mounts the other, that local state was thrown away and each room
 * re-loaded the thread from the server on mount — so switching 立项↔写作 reset
 * (and re-fetched) the conversation every time.
 *
 * The fix hoists the message array + the in-flight `sending` flag up to
 * `WorkspaceContainer`, which loads the thread ONCE per opened project and
 * hands it down through this context. Both working rooms read and append to the
 * same store via `useStudioChat()`, so the conversation is continuous across
 * room swaps — no reset, no re-fetch.
 *
 * `StudioChatMsg` is the superset of the two rooms' former local `ChatMsg`
 * shapes: `card` (a card-turn chip) comes from both; `quotedPart` (就这一段) is
 * WritingBlock-only but harmless in the plan room (its mapper ignores it).
 *
 * `hint` (Task 7, hidden-subagent acknowledgments) marks a message as a
 * transient `SubagentHint` line instead of a chat bubble — appended after
 * 印记's narrate line when a turn's reply reports `planGenerated`/`compacted`.
 * `toChatMessages` renders it as a `system`-role node (no bubble chrome),
 * mirroring the `card` branch.
 */
export type StudioChatMsg = {
  role: "ai" | "student";
  text: string;
  card?: CardTurnRef | null;
  quotedPart?: string;
  hint?: string;
};

// A scripted 印记 chat message with action buttons (§ gaps G3/G5/G6): the
// deterministic in-chat interactions the doc wants — proposal mode-choice, the
// outline intro's 开始写作, the plan-introduction walkthrough — as a message +
// selectable buttons in the chat, not a pane gate. `id` dedupes/replaces.
export type ChatAction = {
  id: string;
  text: string;
  actions: { label: string; run: () => void; primary?: boolean }[];
};

export type StudioChatValue = {
  messages: StudioChatMsg[];
  setMessages: Dispatch<SetStateAction<StudioChatMsg[]>>;
  sending: boolean;
  setSending: (b: boolean) => void;
  // The LIVE opened-project id (a ref, so an async turn closure reads the
  // current value, not a stale capture). A room guards its post-await appends
  // with `activeProjectIdRef.current === <its own projectId>`: a reply that
  // resolves after the student switched PROJECTS is dropped from the display
  // (it's still persisted server-side), while a plain room switch within the
  // same project still lands its reply. See useStudioChat consumers.
  activeProjectIdRef: MutableRefObject<string | null>;
  // ── Task 9b: the container-owned 印记 send loop + note/card offers ────────
  // The ONE studio send: appends the student turn, calls the orchestrator
  // (`coach(id, input)`), appends 印记's narration, APPLIES the returned
  // directive (stage/openTool — auto-configure, always overridable), and
  // surfaces the reply's note/card OFFERS below. Owned by WorkspaceContainer so
  // chat-first AND both working rooms drive ONE loop. Resolves `true` when the
  // turn landed for the still-active project (so a caller can clear per-turn UI
  // like 写作's pinned focusPart only on success), `false` otherwise.
  sendStudioTurn: (userInput: string, opts?: { quotedPart?: string }) => Promise<boolean>;
  // The current project (null in the directory). Rooms/chat read it where they
  // need the live id without threading a prop.
  projectId: string | null;
  // 印记's per-turn OFFERS (铁律②: proposed, never auto-applied). `pendingNote`
  // is a proposal-section note the student confirms into the board; `pendingCard`
  // is a thinking-card the student may open. Both cleared at the start of the
  // next turn (and on project switch).
  pendingNote: NoteProposal | null;
  // After the student confirms `pendingNote`, the actionable chip is replaced by
  // a quiet "记下了" acknowledgment carrying this note (铁律②: her tap is visibly
  // honored, not vanished). Cleared at the start of the next turn and on project
  // switch, like `pendingNote`.
  confirmedNote: NoteProposal | null;
  pendingCard: CardProposalWire | null;
  // Read-modify-write the confirmed note into the proposal board, surface the
  // "记下了" acknowledgment, then let the next turn clear it.
  confirmNote: () => void;
  dismissNote: () => void;
  // Open the proposed card in the shared card sheet (records a coach turn on
  // submit); or decline it (records the decline so 印记 stops offering it).
  openCard: (cardId: string) => void;
  dismissCard: (cardId: string) => void;
  // The 提问卡 is a chatbox affordance (no longer an AI-summonable card): while
  // the research question isn't formed yet (proposal.objective empty), a "还没
  // 头绪？" button sits above the Composer and opens the QuestionCardModal via
  // openCard("question-card"). Flips false the moment 目标 is filled (the card
  // retires) — the same deterministic gate the backend uses.
  questionCardAvailable: boolean;
  // Task 7 (P2b) · 印记's per-turn `propose_question` OFFER (铁律②: proposed,
  // never auto-applied). Confirming turns it into an exploration lead
  // (`createLead`); cleared at the start of the next turn and on project switch,
  // mirroring `pendingNote`.
  pendingQuestion: QuestionProposal | null;
  confirmQuestion: () => void;
  dismissQuestion: () => void;
  // A phase-agnostic link bridge (铁律②: proposed, never auto-applied). When the
  // student drops a new URL in ANY phase, 印记 offers to read it. `readLinkOffer`
  // registers the reference and opens the reading room; `addLinkOffer` just files
  // it in the library; `dismissLinkOffer` declines. Cleared at the next turn and
  // on project switch, like the other offers.
  pendingLinkOffer: LinkOffer | null;
  readLinkOffer: () => void;
  addLinkOffer: () => void;
  dismissLinkOffer: () => void;
  // The deterministic flow router's one-tap next-step (立项 done → 写提案 …).
  // Tapping advanceToNextStep advances the status server-side (coachAdvance).
  pendingNextStep: NextStep | null;
  advanceToNextStep: () => void;
  // §3 gap G4 · a returning student's recap "继续工作" lives IN the chat (not a
  // pane banner). True while the recap landing is active; tapping re-asserts
  // 印记's current-status room.
  recapContinue?: boolean;
  onRecapContinue?: () => void;
  // §gaps G3/G5/G6 · the current scripted in-chat action (mode-choice / outline
  // intro / plan walkthrough). Producers set it; StudioTurnChips renders it.
  chatAction?: ChatAction | null;
  setChatAction?: (a: ChatAction | null) => void;
  // §gap G2 · true while the funnel generates the research plan — drives an
  // interesting rotating loader in the chat.
  generatingPlan?: boolean;
  // Advance the studio status forward to a specific FlowStatus (coachAdvance),
  // applying the new phase's greeting + directive + plan/projection refresh.
  // Used by the writing room's 完成 button (proposal→"essay", essay→"review");
  // the one-tap chip's advanceToNextStep is a thin wrapper over this. Rejects
  // on failure so the caller can surface an error.
  advanceStatusTo: (toStatus: string) => Promise<void>;
  // ── Task 5 (history pagination) · 载入更早的对话 ──────────────────────────
  // The container loads only the thread's RECENT page on open; `historyHasMore`
  // gates the 载入更早 control, `loadEarlier` fetches + prepends the next OLDER
  // page, and `loadingEarlier` drives the control's inline busy state.
  historyHasMore: boolean;
  loadEarlier: () => void;
  loadingEarlier: boolean;
  // ── Task 6 (start gate) ────────────────────────────────────────────────
  // Whether this project's ONE thread has been explicitly started
  // (`StudioState.started`). A brand-new project renders pure full-width
  // chat with NO tabs until the student taps 开始 (StudioCoachChat swaps the
  // Composer for that button while `!started`).
  started: boolean;
  // The student's 开始 tap: calls `coach/start`, appends its narrate, and
  // applies the returned directive (flips `started` → true, opens 提案 —
  // the container's applyStudioState re-renders the switcher/tabs in).
  startJourney: () => Promise<void>;
  // Busy flag around `startJourney`'s round-trip — drives the button's
  // pending/disabled state.
  starting: boolean;
  // Task 6 (P2, demo project): true for the shared, read-only demo project
  // (`WorkspaceProjection.isDemo`). The backend 403s all writes regardless —
  // this only disables the composer so the demo reads honestly as read-only
  // instead of silently failing on send.
  isDemo?: boolean;
};

export const StudioChatContext = createContext<StudioChatValue | null>(null);

/**
 * The hoisted coach store. Throws if used outside a `StudioChatContext.Provider`
 * so a room that forgot its provider fails loudly rather than silently losing
 * the conversation.
 */
export function useStudioChat(): StudioChatValue {
  const ctx = useContext(StudioChatContext);
  if (!ctx) {
    throw new Error("useStudioChat must be used within a StudioChatContext.Provider");
  }
  return ctx;
}
