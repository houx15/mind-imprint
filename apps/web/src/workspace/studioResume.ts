import type { OpenTool, StudioState } from "@mind-imprint/contracts";
import type { BlockKey } from "./blocks/mockData";

/**
 * Map 印记's `open_tool` directive to the interactive-area room to mount
 * (agentic studio, spec §2/§6). This is the resume/morphing seam: the load
 * effect calls it once on open, and Task 9 reuses it after every turn so a
 * fresh `open_tool` swaps the room.
 *
 * `chat` maps to "plan" only as a safe fallback — the chat-first render owns
 * the chat state (an empty landing, not a board), so this branch is never used
 * to mount a room for chat. It exists so the return type stays a real BlockKey.
 */
export function openToolToRoom(tool: OpenTool): BlockKey {
  switch (tool) {
    case "forming":
      return "forming";
    case "reading":
      return "reading";
    case "writing":
      return "writing";
    case "reflection":
      return "reflection";
    case "plan":
    case "chat":
    default:
      return "plan";
  }
}

/**
 * Resume mapping. 印记 now emits `forming`(提案) vs `plan`(管理) explicitly (P2b),
 * so any real room openTool — including `forming` — wins directly. The stage
 * heuristic remains ONLY for `plan`/`chat`: an older/ambiguous `plan` directive
 * during proposal_forming still lands on 提案 rather than a bare board.
 */
export function roomForResume(state: StudioState): BlockKey {
  if (state.openTool !== "plan" && state.openTool !== "chat") {
    return openToolToRoom(state.openTool);
  }
  return state.stage === "topic_discussion" || state.stage === "proposal_forming"
    ? "forming"
    : "plan";
}
