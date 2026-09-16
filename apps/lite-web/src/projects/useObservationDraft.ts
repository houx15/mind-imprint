import { useCallback, useEffect, useRef, useState } from "react";
import type { ObservationRow, ObservationDraft } from "../api/observationDraft";
const emptyObservation = (): ObservationRow[] => [];
import { getObservationDraft, saveObservationDraft, submitObservationDraft } from "../api/observationDraft";
import { ApiError } from "../api/client";
import { apiErrorText } from "../api/errorText";
import { beforeNavigate } from "../routing";

export function useObservationDraft(projectId: string, toolId: string) {
  const [document, setDocument] = useState(emptyObservation);
  const [loaded, setLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [savedText, setSavedText] = useState("");
  const [error, setError] = useState("");
  const [remote, setRemote] = useState<ObservationDraft | null>(null);
  const latest = useRef(document);
  const revision = useRef(0);
  const saved = useRef("");
  const pending = useRef<Promise<void> | null>(null);
  const failed = useRef(false);
  const change = (value: ObservationRow[]) => { latest.current = value; setDocument(value); };
  const accept = (value: ObservationDraft) => {
    revision.current = value.revision;
    saved.current = JSON.stringify(value.document);
    setSavedText(saved.current);
    change(value.document);
    failed.current = false; setError(""); setLoaded(true);
  };
  useEffect(() => {
    let live = true;
    getObservationDraft(projectId, toolId).then(value => { if (live) accept(value); })
      .catch(err => { if (live) setError(apiErrorText(err)); });
    return () => { live = false; };
  }, [projectId, toolId]);
  const flush = useCallback((): Promise<void> => {
    if (pending.current) return pending.current;
    if (!loaded) return Promise.resolve();
    if (failed.current) return Promise.reject(new Error("草稿保存失败：请先处理保存错误"));
    pending.current = Promise.resolve().then(async () => {
      setSaving(true);
      try {
        while (saved.current !== JSON.stringify(latest.current)) {
          const document = latest.current;
          const result = await saveObservationDraft(projectId, toolId, {document, revision: revision.current});
          revision.current = result.revision;
          saved.current = JSON.stringify(document);
          setSavedText(saved.current);
        }
      } catch (err) {
        failed.current = true; setError(apiErrorText(err)); throw err;
      } finally { pending.current = null; setSaving(false); }
    });
    return pending.current;
  }, [loaded, projectId, toolId]);
  const dirty = loaded && JSON.stringify(document) !== savedText;
  useEffect(() => {
    if (!dirty || failed.current) return;
    const timer = window.setTimeout(() => { void flush().catch(() => {}); }, 400);
    return () => window.clearTimeout(timer);
  }, [document, dirty, flush]);
  useEffect(() => beforeNavigate(flush), [flush]);
  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);
  const inspect = async () => {
    try { await pending.current?.catch(() => {}); setRemote(await getObservationDraft(projectId, toolId)); }
    catch (err) { setError(apiErrorText(err)); }
  };
  const resolve = async (keepLocal: boolean) => {
    if (!remote) return;
    const local = latest.current;
    accept(remote); setRemote(null);
    if (keepLocal) {
      change(local);
      try { await flush(); } catch { /* error already visible */ }
    }
  };
  const submit = async () => {
    await flush();
    try {
      const submittedRevision = revision.current;
      const result = await submitObservationDraft(projectId, toolId, submittedRevision);
      accept(result.draft);
      return {notes: result.notes, submittedRevision};
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) { failed.current = true; setError(apiErrorText(err)); }
      throw err;
    }
  };
  return {document, change, loaded, saving, dirty, error, remote, inspect, resolve, flush, submit};
}
