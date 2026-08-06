import { createContext, useContext } from "react";

/**
 * StudioAiSlot (studio agentic rebuild, spec §17 / Task 4).
 *
 * The room→panel portal contract. `WorkspaceContainer` owns the constant
 * `AiPanel` chrome; it hands down the DOM element of the panel's BODY (via a
 * callback ref stored in state — see WorkspaceContainer's `aiSlotRef` — so
 * this context value updates whenever that element mounts/unmounts: room
 * switch, panel collapse, or an AI-side flip all cause a remount).
 *
 * A room calls `useStudioAiSlot()` and, when it gets back a non-null
 * element, `createPortal`s its coach UI (chat log + composer + summon
 * shelf) into it — while rendering its own WORK content directly in
 * `<main>`. `null` means there is nowhere to portal into right now (panel
 * collapsed, or the room isn't mounted inside a studio shell, e.g. in
 * isolation in a test) — a room must treat that as "don't render the coach
 * content", never crash.
 *
 * The room component itself stays mounted across a panel collapse/flip (only
 * the portal target changes), so its own coach STATE (chat history, draft
 * text, pending offers) is preserved — this is the whole point of the portal
 * approach over swapping which component renders the coach.
 */
export const StudioAiSlotContext = createContext<HTMLDivElement | null>(null);

export function useStudioAiSlot(): HTMLDivElement | null {
  return useContext(StudioAiSlotContext);
}
