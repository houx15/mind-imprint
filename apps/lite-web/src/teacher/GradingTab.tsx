import { useEffect, useState } from "react";
import { Button } from "@/ui";
import { listAssignmentGradings, queueAssignmentGradings, sendReviewedGradings, type GradingRow } from "../api/gradings";
import { formatDeadline } from "../shared/deadline";
import { useAlive } from "../shared/useAlive";
import { errorText, failText, tintedChipStyle } from "./assignmentLogic";
import {
  failedCount,
  failureText,
  GRADING_STATUS_LABEL,
  gradingRowStatus,
  gradingStatusHue,
  pendingCount,
  POLL_MS,
  queueResultText,
  reviewedDraftIds,
  sendAllConfirmText,
  sendResultText,
  shouldPoll,
} from "./gradingLogic";

/**
 * 批改 tab of a writing homework: one row per student. Polls every 5s while
 * any row is queued or running.
 *
 * Polling is a self-scheduling chain, not a fixed-tick `setInterval`: each
 * tick's `listAssignmentGradings` call only reschedules the NEXT tick after
 * it settles (success or error), so a response slower than 5s is never
 * discarded by a new request starting on top of it, and two requests are
 * never in flight at once. It stops on its own once no row is queued/
 * running, and the effect's cleanup (assignmentId change, or unmount)
 * always clears the pending timer.
 */
export function GradingTab({ assignmentId, onOpenGrading }: { assignmentId: string; onOpenGrading: (gradingId: string) => void }) {
  const alive = useAlive();
  const [rows, setRows] = useState<GradingRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  // `message` is a result line (queued/sent counts, role="status"); a
  // thrown 批改失败/发送失败 goes in `actionError` instead (role="alert",
  // danger styling) so the two don't read as the same kind of line.
  const [message, setMessage] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmSend, setConfirmSend] = useState(false);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    let timer: number | undefined;

    async function load() {
      try {
        const r = await listAssignmentGradings(assignmentId);
        if (cancelled) return;
        setRows(r);
        setError(null);
        if (shouldPoll(r.map((row) => row.grading?.status ?? ""))) {
          timer = window.setTimeout(() => void load(), POLL_MS);
        }
      } catch (e) {
        if (!cancelled) setError(errorText(e));
      }
    }

    void load();
    return () => {
      cancelled = true;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [assignmentId, nonce]);

  async function queue(retryFailed: boolean) {
    if (busy) return;
    setBusy(true);
    setMessage(null);
    setActionError(null);
    try {
      const r = await queueAssignmentGradings(assignmentId, retryFailed);
      if (alive.current) setMessage(queueResultText(r));
    } catch (e) {
      // Covers both 503 grading_queue_unavailable (the queue never started)
      // and 503 grading_enqueue_failed (every eligible recipient's enqueue
      // rolled back) — an AI/backend failure must surface, never a silent
      // no-op (controller ruling).
      if (alive.current) setActionError(failText("批改", e));
    } finally {
      if (alive.current) {
        setBusy(false);
        setNonce((n) => n + 1);
      }
    }
  }

  const reviewed = rows ? reviewedDraftIds(rows) : [];

  async function sendAll() {
    if (busy || reviewed.length === 0) return;
    setBusy(true);
    setMessage(null);
    setActionError(null);
    try {
      const r = await sendReviewedGradings(assignmentId, reviewed);
      if (alive.current) setMessage(sendResultText(r));
    } catch (e) {
      if (alive.current) setActionError(failText("发送", e));
    } finally {
      if (alive.current) {
        setBusy(false);
        setConfirmSend(false);
        setNonce((n) => n + 1);
      }
    }
  }

  if (error) {
    return (
      <div className="mt-4 text-mk-small font-semibold text-mk-danger">
        加载失败：{error}{" "}
        <button type="button" onClick={() => setNonce((n) => n + 1)} className="cursor-pointer underline">
          重试
        </button>
      </div>
    );
  }
  if (rows === null) return <p className="mt-4 text-mk-small text-mk-muted">加载中…</p>;

  return (
    <section className="mt-4 flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="primary" size="sm" onClick={() => void queue(false)} disabled={busy || pendingCount(rows) === 0}>
          一键AI批改
        </Button>
        <Button variant="secondary" size="sm" onClick={() => void queue(true)} disabled={busy || failedCount(rows) === 0}>
          重试失败
        </Button>
        <Button variant="secondary" size="sm" onClick={() => setConfirmSend(true)} disabled={busy || reviewed.length === 0}>
          发送全部已审阅
        </Button>
      </div>
      {message && (
        <p role="status" className="break-words text-mk-small font-semibold text-mk-ink">
          {message}
        </p>
      )}
      {actionError && (
        <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
          {actionError}
        </p>
      )}
      {confirmSend && (
        <div className="flex flex-wrap items-center gap-2 text-mk-small font-semibold text-mk-ink">
          {sendAllConfirmText(reviewed.length)}
          <Button variant="primary" size="sm" onClick={() => void sendAll()} disabled={busy}>
            确认发送
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setConfirmSend(false)} disabled={busy}>
            取消
          </Button>
        </div>
      )}
      <div className="overflow-x-auto rounded-mk-lg border border-mk-border bg-mk-surface">
        <table className="w-full min-w-[640px] border-collapse">
          <thead>
            <tr>
              {["学生", "版本", "状态", "总评", "操作"].map((h) => (
                <th key={h} className="whitespace-nowrap border-b border-mk-border px-3 py-2.5 text-left text-mk-label font-bold text-mk-muted">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => {
              const status = gradingRowStatus(r);
              // Ruling 1: a draft can carry a leftover error (a failed
              // regrade that kept her previous content) — show the failure
              // text whenever `error` is set, not only when the row's own
              // status is "failed".
              const rowError = r.grading?.error ?? null;
              return (
                <tr key={r.userId}>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small font-bold text-mk-ink">{r.displayName}</td>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                    {r.version ? `v${r.version.number} · ${formatDeadline(r.version.submittedAt)}` : "—"}
                  </td>
                  <td className="border-b border-mk-border px-3 py-3 text-mk-small">
                    <span className="inline-block whitespace-nowrap rounded-mk-full px-2.5 py-0.5 font-bold" style={tintedChipStyle(gradingStatusHue(status))}>
                      {GRADING_STATUS_LABEL[status]}
                    </span>
                    {rowError && <p className="mt-1 break-words text-mk-danger">{failureText(rowError)}</p>}
                  </td>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">{r.grading?.overallGrade ?? "—"}</td>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small">
                    {r.grading ? (
                      <Button variant="link" size="sm" onClick={() => onOpenGrading(r.grading?.id ?? "")}>
                        查看
                      </Button>
                    ) : (
                      <span className="text-mk-muted">—</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}
