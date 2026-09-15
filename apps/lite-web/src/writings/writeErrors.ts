// writings/writeErrors.ts — the one-line landing spot every write catch in
// the room reaches for.
//
// Past a homework's deadline the server refuses every write the same way
// (403 writing_locked) no matter which endpoint asked — a draft autosave, a
// snippet save, a rename, 请印记看看, 完成这篇, all of it. Each catch block
// across ComposeStage/SnippetsStage/EditableTitle used to just show
// whatever the server said, which for a lock error reads as an ordinary
// mistake she could retry — she cannot, and retrying teaches her nothing.
// `handleWriteError` makes the same choice everywhere: reload past it
// (`onLocked`, which WritingRoomHost wires to its `reload`, landing her on
// the locked finished page that already explains why) when the error IS the
// lock, otherwise keep showing the server's own words as before.
//
// `onLocked` is optional: a caller with no reachable lock path (PlanningView
// writes only ever happen before a writing has been finished once, and
// `locked` requires at least one version — see `isRevising`/`showFinishedPage`
// in `finishedWriting.ts` — so a 结构-stage write can never actually hit this)
// can simply not pass it, and every call here falls through to `setError`
// exactly as before.
import { apiErrorText } from "../api/errorText";
import { isWritingLockedError } from "./finishedWriting";

export function handleWriteError(err: unknown, onLocked: (() => void) | undefined, setError: (message: string) => void): void {
  if (isWritingLockedError(err)) {
    onLocked?.();
    return;
  }
  setError(apiErrorText(err));
}
