import { Says, errorMarkdown } from "./Says";
import { useCallback, useEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { listArtifacts, pendingArtifacts, type Artifact } from "../api/artifacts";
import { downloadCodingAgentBrief, artifactStatus, downloadArtifact, reviewEntries, type ArtifactExportScope } from "../api/artifactExport";
import { apiErrorText } from "../api/errorText";
import { getReview, type ReviewPlan } from "../api/review";
import { ArtifactChanges } from "./ArtifactChanges";
import { PaperPreview } from "./PaperPreview";
import { readPaperLayout } from "../api/paperLayout";
import { FoldoutPreview } from "./FoldoutPreview";
import { readPrintLayout } from "../api/printLayout";
import { ArtifactTrial } from "./ArtifactTrial";
import { ArtifactPresentation } from "./ArtifactPresentation";
import { ArtifactTextEditor } from "./ArtifactTextEditor";

export function ArtifactLibrary({projectId,onClose,onReview,onOpenSession,busy=false}:{projectId:string;onClose:()=>void;onReview:()=>Promise<void>;onOpenSession:(id:string)=>void;busy?:boolean}) {
  const trialFlush = useRef<(() => Promise<void>) | null>(null);
  const editGuard = useRef<(() => Promise<void>) | null>(null);
  const registerEditGuard = useCallback((guard: () => Promise<void>) => {
    editGuard.current = guard;
    return () => { if (editGuard.current === guard) editGuard.current = null; };
  }, []);
  const registerTrialFlush = useCallback((flush: () => Promise<void>) => {
    trialFlush.current = flush;
    return () => { if (trialFlush.current === flush) trialFlush.current = null; };
  }, []);
  const [leaving, setLeaving] = useState(false);
  const afterSave = async (action: () => void | Promise<void>) => {
    if (leaving) return;
    setLeaving(true);
    try {
      try { await editGuard.current?.(); } catch { return; }
      try { await trialFlush.current?.(); } catch { return; } // The draft displays its own recovery controls.
      setError("");
      await action();
    }
    catch (err) { setError(apiErrorText(err)); }
    finally { setLeaving(false); }
  };
  const [items,setItems]=useState<Artifact[]>([]);
  const [exportScope, setExportScope] = useState<ArtifactExportScope>("complete");
  const [selected,setSelected]=useState<string|null>(null);
  const [error,setError]=useState("");
  const [loading,setLoading]=useState(true);
  const [opening,setOpening]=useState(false);
  const contentRef = useRef<HTMLDivElement>(null);
  const [reviewRecord, setReviewRecord] = useState<{id: string; plan?: ReviewPlan; error?: string} | null>(null);
  const [reviewReload, setReviewReload] = useState(0);
  useEffect(()=>{
    if (busy) return;
    let cancelled=false;
    setLoading(true);
    setError("");
    listArtifacts(projectId).then(all=>{
      if (cancelled) return;
      const latestFirst = all.slice().reverse();
      setItems(latestFirst);
      // Keep the current artifact mounted when generation returns so its
      // trial draft is saved through afterSave before a deliberate switch.
      setSelected(id => id ?? latestFirst[0]?.id ?? null);
    })
      .catch(e=>{if(!cancelled)setError(apiErrorText(e));})
      .finally(()=>{if(!cancelled)setLoading(false);});
    return()=>{cancelled=true;};
  },[projectId,busy]);
  const latest = items[0];
  const current=items.find(x=>x.id===selected)??latest;
  const artifactId = current?.id;
  useEffect(() => {
    if (!artifactId) return;
    let cancelled = false;
    setReviewRecord(null);
    getReview(projectId, artifactId).then(plan => {
      if (!cancelled) setReviewRecord({id: artifactId, plan});
    }).catch(e => {
      if (!cancelled) setReviewRecord({id: artifactId, error: apiErrorText(e)});
    });
    return () => { cancelled = true; };
  }, [projectId, artifactId, reviewReload]);
  const review = reviewRecord?.id === artifactId ? reviewRecord?.plan : undefined;
  const reviewError = reviewRecord?.id === artifactId ? reviewRecord?.error : undefined;
  const entries = reviewEntries(review);
  const hasPrintPreview = current && (readPrintLayout(current.payload.printLayout) || readPaperLayout(current.payload.paperLayout));
  const body = current?.payload.body ? <article className="max-w-none break-words leading-relaxed [&_h1]:text-2xl [&_h1]:font-semibold [&_h2]:text-lg [&_h2]:font-semibold [&_h2]:mt-6 [&_h2]:mb-3 [&_h3]:font-semibold [&_h3]:mt-4 [&_p]:my-3 [&_p]:whitespace-pre-wrap [&_ul]:list-disc [&_ol]:list-decimal [&_ul]:pl-5 [&_ol]:pl-5 [&_li]:my-2 [&_table]:my-4 [&_table]:w-full [&_table]:table-fixed [&_table]:border-collapse [&_th]:border [&_th]:border-mk-border [&_th]:p-3 [&_th]:text-left [&_td]:border [&_td]:border-mk-border [&_td]:p-3 [&_td]:align-top [&_td]:h-20"><ReactMarkdown remarkPlugins={[remarkGfm]}>{current.payload.body}</ReactMarkdown></article> : null;
  return <div className="flex h-full flex-col">
    <header className="border-b border-mk-border p-4 flex justify-between items-center"><h2 className="font-semibold">项目成果</h2><button disabled={leaving} onClick={() => void afterSave(onClose)}>返回计划</button></header>
    {latest && current?.id !== latest.id && <div className="border-b border-mk-border bg-mk-paper px-4 py-3 flex flex-wrap items-center gap-3" role="status">
      <p className="min-w-0 flex-1 text-mk-small">最新成果：{latest.title || "未命名成果"}</p>
      <button disabled={leaving || busy} className="rounded-full border border-mk-border px-3 py-1.5 text-mk-small disabled:opacity-40" onClick={() => {
        const id = latest.id;
        void afterSave(() => { setSelected(id); contentRef.current?.scrollTo({top: 0}); });
      }}>查看最新成果</button>
    </div>}
    <div ref={contentRef} className="overflow-y-auto p-4 space-y-4">
      {error&&<div role="alert" className="text-mk-danger"><Says content={errorMarkdown(error)} /></div>}
      {loading&&<p>加载中</p>}
      {!loading&&!items.length&&!error&&<p>暂无成果</p>}
      {items.length>0&&<label className="block">成果记录<select aria-label="成果记录" value={current?.id} disabled={leaving} onChange={e=>{ const id=e.target.value; void afterSave(() => setSelected(id)); }} className="mt-2 w-full rounded border border-mk-border p-2 bg-mk-surface">{items.map((a,i)=><option key={a.id} value={a.id}>{a.title||"未命名成果"} · {artifactStatus(a)} · {new Date(a.createdAt).toLocaleString()} · {items.length-i}</option>)}</select></label>}
      {current&&<>
        <div><h3 className="text-xl font-semibold">{current.title}</h3><p className="text-mk-secondary">{artifactStatus(current)}</p></div>
        {current.payload.editedByStudent === true && <p className="text-mk-small text-mk-secondary">学生修改</p>}
        {!hasPrintPreview && ["draft", "spec", "options"].includes(current.kind) && current.payload.body && <ArtifactTextEditor key={`edit-${current.id}`} projectId={projectId} artifact={current} disabled={busy || leaving} registerGuard={registerEditGuard} beforeSave={async () => { await trialFlush.current?.(); }} onSaved={saved => {
          setItems(old => [saved, ...old.filter(a => a.id !== saved.id).map(a => a.id === current.id ? {...a, superseded: true} : a)]);
          setSelected(saved.id);
        }} />}
        <ArtifactChanges artifact={current} previousArtifact={items.find(a => a.id === (current.payload.baseArtifactId ?? current.payload.replacesArtifactId))} />
        {items.some(a => a.id === (current.payload.baseArtifactId ?? current.payload.replacesArtifactId)) && <button className="underline" disabled={leaving} onClick={() => void afterSave(() => setSelected(String(current.payload.baseArtifactId ?? current.payload.replacesArtifactId)))}>查看修改前的成果</button>}
        {current.kind === "site" && <p className="text-mk-small text-mk-secondary">此处保存审核时的文字。<a href="/site" target="_blank" rel="noreferrer" className="underline">查看当前主页</a></p>}
        {current.payload.body&&<section className="rounded-xl border border-mk-border p-4 space-y-3">
          <label className="block">导出内容<select aria-label="导出内容" className="mt-2 w-full rounded border border-mk-border p-2 bg-mk-surface" value={exportScope} onChange={e => setExportScope(e.target.value as ArtifactExportScope)}>
            <option value="complete">完整记录</option><option value="material">材料正文</option>
          </select></label>
          <p className="text-mk-small text-mk-secondary">{exportScope === "material" ? "仅导出标题、审核状态和正文，便于打印使用。不附加 AI 假设、局限、修改对照和审核意见；正文中的来源说明会保留。" : "包含正文、AI 假设与局限、修改对照和已填写的审核意见，便于留档。"}</p>
          <div className="flex flex-wrap gap-3"><button disabled={exportScope === "complete" && !review} className="underline disabled:opacity-40" onClick={()=>downloadArtifact(current,"md",review,exportScope)}>导出 Markdown</button>{!readPrintLayout(current.payload.printLayout)&&!readPaperLayout(current.payload.paperLayout)&&<button disabled={exportScope === "complete" && !review} className="underline disabled:opacity-40" onClick={()=>downloadArtifact(current,"html",review,exportScope)}>导出打印版 HTML</button>}</div>
        </section>}
        {current.kind === "spec" && !hasPrintPreview && current.payload.body?.trim() && <section className="rounded-xl border border-mk-border p-4">
          <h4 className="font-semibold">交给外部工具制作网页</h4>
          <p className="my-2 text-mk-small text-mk-secondary">导出这份规格及审核意见，交给编程工具制作。完成后，请回到项目对话，带回预览和一次实际操作的试用记录，继续讨论修改。</p>
          <button disabled={!review} className="underline disabled:opacity-40" onClick={() => review && downloadCodingAgentBrief(current, review)}>导出网页制作交接说明</button>
        </section>}
        {(current.kind === "spec" || current.kind === "draft") && <details key={`trial-${current.id}`} className="rounded-xl border border-mk-border p-3">
          <summary className="cursor-pointer font-semibold">试用记录与讨论</summary>
          <ArtifactTrial projectId={projectId} artifact={current} registerFlush={registerTrialFlush} onOpenSession={id => void afterSave(() => onOpenSession(id))} disabled={busy || leaving} />
        </details>}
        {reviewError ? <div role="alert"><Says content={errorMarkdown(`审核记录加载失败：${reviewError}`)} /><button className="underline" onClick={() => setReviewReload(n => n + 1)}>重试</button></div> : !review && <p>审核记录加载中</p>}
        <PaperPreview key={`paper-${current.id}`} artifact={current}/>
        <FoldoutPreview key={current.id} artifact={current}/>
        {!hasPrintPreview && <ArtifactPresentation key={`presentation-${current.id}`} artifact={current} />}
        {body && (hasPrintPreview ? <details key={`text-${current.id}`} className="rounded-xl border border-mk-border p-3"><summary className="cursor-pointer font-semibold">文字记录</summary>{body}</details> : body)}
        {!current.payload.body&&current.payload.url&&<a className="underline" href={current.payload.url} target="_blank" rel="noreferrer">打开成果</a>}
        {current.why&&<section><h4 className="font-semibold">审核结论</h4><p className="whitespace-pre-wrap">{current.why}</p></section>}
        {entries.length > 0 && <section className="space-y-3"><h4 className="font-semibold">学生审核意见</h4>{entries.map((e, i) => <div key={i} className="rounded border border-mk-border p-3"><p className="font-medium">{e.question}</p><p className="text-mk-small text-mk-secondary">{e.source}</p>{e.quote && <blockquote className="my-2 border-l-2 border-mk-border pl-3">{e.quote}</blockquote>}<p className="whitespace-pre-wrap">{e.answer}</p></div>)}</section>}
        {current.guessed.length>0&&<section><h4 className="font-semibold">AI 标注的假设</h4><ul>{current.guessed.map((x,i)=><li key={i}>{x}</li>)}</ul></section>}
        {current.admits.length>0&&<section><h4 className="font-semibold">待核实与局限</h4><ul>{current.admits.map((x,i)=><li key={i}>{x}</li>)}</ul></section>}
        {pendingArtifacts(items).length > 0&&<button disabled={opening} className="rounded border border-mk-border px-4 py-2" onClick={async()=>{setOpening(true);try{await editGuard.current?.();await trialFlush.current?.();await onReview();}catch(e){if(!editGuard.current)setError(apiErrorText(e));}finally{setOpening(false);}}}>打开待审核成果</button>}
      </>}
    </div>
  </div>;
}
