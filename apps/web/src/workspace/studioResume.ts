import type { OpenTool } from "@mind-imprint/contracts";
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
