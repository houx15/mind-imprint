// writings/writeErrors.ts — the shared catch for every write in the writing
// room.
//
// Two server refusals mean the room is out of date rather than that her input
// was wrong:
//   - 403 writing_locked: the homework deadline passed while she was revising.
//   - 403 writing_finished: the writing is no longer being revised, for
//     example because 完成这篇 or 放弃修改 was pressed in another tab.
// A retry fails the same way, so `handleWriteError` calls `onLocked`
// (WritingRoomHost wires it to `reload`), which re-reads the writing and opens
// the finished page. Any other error is shown with the server's own message.
//
// Every room surface passes `onLocked`, 结构 (PlanningView) included: she can
// jump back to 结构 while revising, so its writes can get either refusal. A
// caller without `onLocked` gets the message shown instead.
import { apiErrorText } from "../api/errorText";
import { isWritingClosedError } from "./finishedWriting";

// `action` names what failed (「提交」): the line reads 「提交失败：{后台原话}」
// (AGENTS.md 界面文案 rule 8). Without it the generic 「后台错误：…」 stays.
export function handleWriteError(
  err: unknown,
  onLocked: (() => void) | undefined,
  setError: (message: string) => void,
  action?: string,
): void {
  if (onLocked && isWritingClosedError(err)) {
    onLocked();
    return;
  }
  const text = apiErrorText(err);
  setError(action ? `${action}失败：${text.replace(/^后台错误：/, "")}` : text);
}
