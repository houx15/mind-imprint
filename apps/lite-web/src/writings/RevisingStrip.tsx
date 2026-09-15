import { useEffect, useState } from "react";
import { Button } from "@/ui";
import { apiErrorText } from "../api/errorText";
import { discardWritingRevision, listWritingVersions, type WritingVersionList } from "../api/writings";
import { useAlive } from "../shared/useAlive";
import { discardConfirmText, isWritingLockedError, revisingStripText } from "./finishedWriting";

/**
 * RevisingStrip — shown in the writing room (and on 结构) while she edits a
 * finished writing. Names the version already submitted and offers 放弃修改,
 * which restores the draft and title from that version after one
 * confirmation. A finished writing with no version (finished by the old API
 * during the 0153 deploy) still gets the strip: 放弃修改 then only ends
 * revising.
 *
 * `onDiscarded` doubles as the reload trigger for the locked edge case: the
 * deadline can pass while she is looking at the confirmation dialog, and the
 * server then refuses the discard with 403 `writing_locked`. That error is
 * shown here first, then the same callback reloads the room so it lands on
 * the locked finished page instead of a room that can no longer save.
 */
export function RevisingStrip({ writingId, onDiscarded }: { writingId: string; onDiscarded: () => void }) {
  const alive = useAlive();
  const [list, setList] = useState<WritingVersionList | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setList(null);
    listWritingVersions(writingId)
      .then((l) => {
        if (alive.current) setList(l);
      })
      .catch((e: unknown) => {
        if (alive.current) setError(`加载失败：${apiErrorText(e)}`);
      });
  }, [writingId, alive]);

  async function discard() {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await discardWritingRevision(writingId);
      if (alive.current) onDiscarded();
    } catch (e) {
      if (!alive.current) return;
      setError(`放弃修改失败：${apiErrorText(e)}`);
      setBusy(false);
      // The deadline passed while she was confirming: the room can no
      // longer save anything, so reload past the error into the locked page.
      if (isWritingLockedError(e)) onDiscarded();
    }
  }

  if (list === null && !error) return null;
  const latest = list?.versions[0]?.number ?? null;

  return (
    <div className="flex flex-wrap items-center gap-3 rounded-mk-md border border-mk-border bg-mk-surface px-4 py-2.5 text-mk-small text-mk-ink">
      {list !== null && <span className="font-semibold">{revisingStripText(latest)}</span>}
      {list !== null && (
        <div className="flex flex-wrap items-center gap-2 sm:ml-auto">
          {confirming ? (
            <>
              <span className="text-mk-muted">{discardConfirmText(latest)}</span>
              <Button variant="danger" size="sm" onClick={() => void discard()} disabled={busy}>
                确认放弃
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setConfirming(false)} disabled={busy}>
                取消
              </Button>
            </>
          ) : (
            <Button variant="secondary" size="sm" onClick={() => setConfirming(true)}>
              放弃修改
            </Button>
          )}
        </div>
      )}
      {error && (
        <p role="alert" className="w-full break-words font-semibold text-mk-danger">
          {error}
        </p>
      )}
    </div>
  );
}
