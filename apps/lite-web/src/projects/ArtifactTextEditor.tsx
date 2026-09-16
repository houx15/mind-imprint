import { useEffect, useState } from "react";
import { editArtifactText, type Artifact } from "../api/artifacts";
import { apiErrorText } from "../api/errorText";
import { beforeNavigate } from "../routing";
import { GrowingTextarea } from "../shared/GrowingTextarea";
import { Says, errorMarkdown } from "./Says";

export function ArtifactTextEditor({projectId, artifact, disabled, beforeSave, registerGuard, onSaved}: {
  projectId: string; artifact: Artifact; disabled: boolean;
  beforeSave: () => Promise<void>;
  registerGuard: (guard: () => Promise<void>) => () => void;
  onSaved: (artifact: Artifact) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(artifact.payload.body ?? "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const dirty = editing && text !== artifact.payload.body;
  useEffect(() => {
    if (!dirty && !saving) return;
    const guard = async () => {
      setError("请先保存或取消正文修改，再离开当前成果。");
      throw new Error("unsaved artifact edit");
    };
    const removeLocal = registerGuard(guard);
    const removeNavigation = beforeNavigate(guard);
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => { removeLocal(); removeNavigation(); window.removeEventListener("beforeunload", warn); };
  }, [dirty, saving, registerGuard]);
  if (!editing) return <button className="rounded border border-mk-border px-3 py-2 disabled:opacity-40" disabled={disabled || artifact.superseded} onClick={() => {setText(artifact.payload.body ?? ""); setError(""); setEditing(true);}}>编辑正文</button>;
  return <section className="space-y-3 rounded-xl border border-mk-border p-4">
    <h4 className="font-semibold">编辑正文</h4>
    <p className="text-mk-small text-mk-secondary">保存为学生修改的新版本，旧版和原审核记录保留。原有假设与局限会沿用，请检查它们是否仍适用。</p>
    <GrowingTextarea aria-label="成果正文" value={text} disabled={saving} onChange={e => setText(e.target.value)} className="w-full rounded border border-mk-border bg-mk-surface p-3 font-mono text-mk-small" />
    {error && <div role="alert"><Says content={errorMarkdown(error)} /></div>}
    <div className="flex gap-3">
      <button className="rounded border border-mk-border px-3 py-2 disabled:opacity-40" disabled={saving || disabled || !dirty || !text.trim()} onClick={async () => {
        setSaving(true); setError("");
        try { await beforeSave(); const saved = await editArtifactText(projectId, artifact.id, text); setEditing(false); onSaved(saved); }
        catch (err) { setError(apiErrorText(err)); }
        finally { setSaving(false); }
      }}>{saving ? "保存中" : "保存新版本"}</button>
      <button disabled={saving} onClick={() => {setEditing(false); setError("");}}>取消修改</button>
    </div>
  </section>;
}
