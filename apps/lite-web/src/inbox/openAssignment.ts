import { markSeen, startAssignment, type AssignmentInboxItem, type GradingInboxItem } from "../api/assignments";
import { markGradingSeen } from "../api/gradings";
import { navigate, writingPath } from "../routing";
import { notifyGradingsChanged } from "../writings/gradingsChanged";
import { errorMessage, startErrorText } from "./inboxLogic";
import { roomPathForStart } from "./startAssignment";

/**
 * Open an assignment from the inbox or a 作业 strip: mark it seen (a
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

/**
 * Open a sent grading: mark it seen, then go to the writing it grades. A
 * failed seen-call must not block getting to the writing; `reload` runs
 * either way, so a mark that really failed shows as unread again.
 * `navigate` does nothing when that writing is already open, so the page is
 * also told to fetch its gradings again.
 */
export async function openGrading(item: GradingInboxItem, reload: () => void): Promise<void> {
  await markGradingSeen(item.id).catch(() => undefined);
  reload();
  notifyGradingsChanged(item.atomId);
  navigate(writingPath(item.atomId));
}
