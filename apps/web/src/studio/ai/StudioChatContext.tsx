import { createContext, useContext } from "react";
import type { Dispatch, MutableRefObject, SetStateAction } from "react";
import type { CardTurnRef, NoteProposal, CardProposalWire, QuestionProposal } from "@mind-imprint/contracts";

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
 */
export type StudioChatMsg = {
  role: "ai" | "student";
  text: string;
  card?: CardTurnRef | null;
  quotedPart?: string;
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
  pendingCard: CardProposalWire | null;
  // Read-modify-write the confirmed note into the proposal board, then clear it.
  confirmNote: () => void;
  dismissNote: () => void;
  // Open the proposed card in the shared card sheet (records a coach turn on
  // submit); or decline it (records the decline so 印记 stops offering it).
  openCard: (cardId: string) => void;
  dismissCard: (cardId: string) => void;
  // Task 7 (P2b) · 印记's per-turn `propose_question` OFFER (铁律②: proposed,
  // never auto-applied). Confirming turns it into an exploration lead
  // (`createLead`); cleared at the start of the next turn and on project switch,
  // mirroring `pendingNote`.
  pendingQuestion: QuestionProposal | null;
  confirmQuestion: () => void;
  dismissQuestion: () => void;
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
