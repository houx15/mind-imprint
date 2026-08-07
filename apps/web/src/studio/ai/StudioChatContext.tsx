import { createContext, useContext } from "react";
import type { Dispatch, SetStateAction } from "react";
import type { CardTurnRef } from "@mind-imprint/contracts";

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
