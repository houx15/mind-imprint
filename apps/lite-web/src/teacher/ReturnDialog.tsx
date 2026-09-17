import { useEffect, useState } from "react";
import { Button } from "@/ui";
import { returnRecipient, type RecipientDTO } from "../api/assignments";
import { isoToBeijingInput } from "../shared/deadline";
import { useAlive } from "../shared/useAlive";
import { buildReturnInput, failText } from "./assignmentLogic";
import { DateField } from "./controls/DateField";
import { INPUT_CLS } from "./formParts";

const DEFAULT_EXTENSION_MS = 3 * 24 * 60 * 60 * 1000;

/**
 * ReturnDialog — 退回修改 for one student's submitted writing homework.
 * The student can edit and submit again until the new deadline. Returning
 * again replaces the previous deadline and note.
 */
export function ReturnDialog({
  assignmentId,
  recipient,
  onClose,
  onReturned,
}: {
  assignmentId: string;
  recipient: RecipientDTO;
  onClose: () => void;
  onReturned: (r: RecipientDTO) => void;
}) {
  const alive = useAlive();
  const [dueInput, setDueInput] = useState(() => isoToBeijingInput(new Date(Date.now() + DEFAULT_EXTENSION_MS).toISOString()));
  const [note, setNote] = useState(recipient.returnNote ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [busy, onClose]);

  async function submit() {
    if (busy) return;
    const built = buildReturnInput(dueInput, note, Date.now());
    if (!built.ok) {
      setError(built.error);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const updated = await returnRecipient(assignmentId, recipient.userId, built.value);
      if (alive.current) onReturned(updated);
    } catch (e) {
      if (!alive.current) return;
      setError(failText("退回", e));
      setBusy(false);
    }
  }

  const inputCls = INPUT_CLS;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: "color-mix(in srgb, var(--mk-ink) 42%, transparent)" }}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && !busy) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="return-dialog-title"
        className="flex max-h-full w-full max-w-[520px] flex-col gap-5 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface p-6 shadow-mk-lg"
      >
        <div className="flex flex-col gap-1.5">
          <h2 id="return-dialog-title" className="text-mk-h2 text-mk-ink">
            退回修改
          </h2>
          <p className="text-mk-small text-mk-muted">{recipient.displayName}</p>
          <p className="text-mk-small text-mk-secondary">退回后学生可以修改并重新提交，截止时间以此处为准。</p>
        </div>

        <label className="flex flex-col gap-1.5 text-mk-small text-mk-muted">
          新的截止时间（北京时间）
          <DateField withTime shortcuts value={dueInput} disabled={busy} onChange={setDueInput} />
        </label>

        <label className="flex flex-col gap-1.5 text-mk-small text-mk-muted">
          退回说明
          <textarea rows={4} maxLength={500} value={note} disabled={busy} onChange={(e) => setNote(e.target.value)} className={inputCls} />
        </label>

        {error && (
          <p className="break-words text-mk-small font-semibold text-mk-danger" role="alert">
            {error}
          </p>
        )}

        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="ghost" size="sm" onClick={onClose} disabled={busy}>
            取消
          </Button>
          <Button variant="primary" size="sm" onClick={() => void submit()} disabled={busy}>
            {busy ? "处理中" : "确认退回"}
          </Button>
        </div>
      </div>
    </div>
  );
}
