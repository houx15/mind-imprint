import { markSeen, startAssignment, type AssignmentInboxItem } from "../api/assignments";
import { navigate } from "../routing";
import { errorMessage, startErrorText } from "./inboxLogic";
import { roomPathForStart } from "./startAssignment";

/**
 * Open an assignment from the inbox or a 老师布置 strip: mark it seen (a
 * failure there must not stop her getting into the room), start it, and go to
 * the room the server names. Resolves to `null` on success, or the inline
 * error text on failure. `reload` refreshes the caller's inbox so the unread
 * mark reflects the seen call even when the start failed.
 */
export async function openAssignment(item: AssignmentInboxItem, reload: () => void): Promise<string | null> {
  await markSeen(item.id).catch(() => undefined);
  try {
    const result = await startAssignment(item.id);
    navigate(roomPathForStart(result));
    return null;
  } catch (err) {
    reload();
    return startErrorText(errorMessage(err));
  }
}
