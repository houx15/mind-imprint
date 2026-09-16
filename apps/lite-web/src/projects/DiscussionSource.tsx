import { Says, errorMarkdown } from "./Says";
import { useEffect, useState } from "react";
import { listKeepEntries, type KeepEntry } from "../api/lookback";
import { apiErrorText } from "../api/errorText";
import type { Session } from "../api/projectRoom";

/** Show the saved observation, not a generated summary of its evidence. */
export function DiscussionSource({ projectId, session }: { projectId: string; session: Session }) {
  const [entry, setEntry] = useState<KeepEntry | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setEntry(null);
    setError("");
    setLoading(true);
    listKeepEntries(projectId).then(entries => {
      if (cancelled) return;
      const source = entries.find(item => item.sessionId === session.id && item.id === session.anchorRef);
      setEntry(source ?? null);
      if (!source) setError("未找到关联记录");
    }).catch(err => {
      if (!cancelled) setError(`读取讨论依据失败：${apiErrorText(err)}`);
    }).finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [projectId, session.id, session.anchorRef, attempt]);

  return <section aria-label="讨论依据" className="rounded-2xl border border-mk-border bg-mk-paper p-4">
    <p className="text-mk-body font-semibold text-mk-ink">{session.question}</p>
    {loading ? <p className="mt-2 text-mk-small text-mk-muted">讨论依据读取中</p> : error ? <div className="mt-2 text-mk-small" role="status">
      <div><Says content={errorMarkdown(error)} /></div>
      <button type="button" className="mt-2 underline" onClick={() => setAttempt(value => value + 1)}>重新读取</button>
    </div> : entry && <details className="mt-3 text-mk-small">
      <summary className="cursor-pointer text-mk-secondary">讨论依据 · 原始记录 · {new Date(entry.createdAt).toLocaleString("zh-CN")}</summary>
      <p className="mt-3 whitespace-pre-wrap break-words border-t border-mk-border pt-3 leading-relaxed text-mk-ink">{entry.body}</p>
      {entry.expect && <p className="mt-3 whitespace-pre-wrap break-words text-mk-secondary">预期：{entry.expect}</p>}
    </details>}
  </section>;
}
