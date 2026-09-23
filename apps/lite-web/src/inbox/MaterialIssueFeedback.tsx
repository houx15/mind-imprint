import { useId, useState, type FormEvent } from "react";
import { reportAssignmentIssue } from "../api/assignments";

/** Student-initiated report for a reading material that opens but looks wrong. */
export function MaterialIssueFeedback({ assignmentId }: { assignmentId: string }) {
  const inputId = useId();
  const [open, setOpen] = useState(false);
  const [detail, setDetail] = useState("");
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const text = detail.trim();
    if (!text || busy) return;
    setBusy(true);
    setError(null);
    try {
      await reportAssignmentIssue(assignmentId, text);
      setSent(true);
      setOpen(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return <div className="mt-1 text-mk-small">
    <button type="button" className="font-semibold text-mk-accent-700 underline underline-offset-2" onClick={() => { setOpen(!open); setError(null); }}>
      {sent ? "已反馈材料问题 · 再次反馈" : "反馈材料问题"}
    </button>
    {open && <form onSubmit={(e) => void submit(e)} className="mt-2 flex max-w-md flex-col gap-2">
      <label htmlFor={inputId} className="text-mk-muted">请说明网页打不开、正文缺失或其他材料问题</label>
      <textarea id={inputId} value={detail} onChange={(e) => setDetail(e.target.value)} maxLength={500} rows={2} className="w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-ink" />
      {error && <span role="alert" className="text-mk-danger">反馈失败：{error}</span>}
      <div className="flex gap-3"><button type="submit" disabled={busy || !detail.trim()} className="font-semibold text-mk-accent-700 disabled:opacity-40">{busy ? "发送中" : "发送给老师"}</button><button type="button" onClick={() => setOpen(false)} className="text-mk-muted">取消</button></div>
    </form>}
  </div>;
}
