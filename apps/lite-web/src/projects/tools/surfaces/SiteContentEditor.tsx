import { useEffect, useRef, useState } from "react";
import { getSite, putSiteContent } from "../../../api/site";
import type { SiteDraft } from "../../../site/types";
import { GrowingTextarea } from "../../../shared/GrowingTextarea";
import { beforeNavigate } from "../../../routing";
import { apiErrorText } from "../../../api/errorText";
import { Says, errorMarkdown } from "../../Says";

/** Student-written copy is saved separately from immutable generated versions. */
export function SiteContentEditor({ projectId, onPendingChange }: { projectId:string; onPendingChange:(pending:boolean)=>void }) {
 const [draft,setDraft]=useState<SiteDraft|null>(null);
 const saved=useRef<SiteDraft|null>(null);
 const current=useRef<SiteDraft|null>(null);
 const saving=useRef<Promise<void>|null>(null);
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState("");
 const [status,setStatus]=useState("");
 const [selected,setSelected]=useState("about");
 const dirty=Boolean(draft&&JSON.stringify(draft)!==JSON.stringify(saved.current));
 const load=async()=>{
  setBusy(true);setError("");
  try {const site=await getSite();if(site.projectId!==projectId)throw new Error("主页与当前项目不一致");saved.current=site.draft;current.current=site.draft;setDraft(site.draft);setStatus("");}
  catch(e){setError(apiErrorText(e));}finally{setBusy(false);}
 };
 useEffect(()=>{void load();},[projectId]);
 useEffect(()=>{onPendingChange(!draft||dirty||busy);},[draft,dirty,busy,onPendingChange]);
 const change=(next:SiteDraft)=>{current.current=next;setDraft(next);setStatus("");};
 const persist=async()=>{
  if(saving.current)return saving.current;
  if(!current.current||JSON.stringify(current.current)===JSON.stringify(saved.current))return;
  const value=current.current;
  setBusy(true);setError("");
  saving.current=(async()=>{try{const site=await putSiteContent(value,saved.current??undefined);const index=value.sections?.findIndex(section=>section.key===selected)??-1;if(index>=0&&site.draft.sections?.[index])setSelected(site.draft.sections[index].key);saved.current=site.draft;current.current=site.draft;setDraft(site.draft);setStatus("内容已保存，生成版本尚未更新");}catch(e){setError(apiErrorText(e));throw e;}finally{setBusy(false);saving.current=null;}})();
  return saving.current;
 };
 const guard=useRef(persist);guard.current=persist;
 useEffect(()=>beforeNavigate(()=>guard.current()),[]);
 useEffect(()=>{if(!dirty)return;const warn=(e:BeforeUnloadEvent)=>e.preventDefault();window.addEventListener("beforeunload",warn);return()=>window.removeEventListener("beforeunload",warn);},[dirty]);
 const section=draft?.sections?.find(s=>s.key===selected);
 return <details className="rounded-mk-lg border border-mk-border bg-white p-4">
  <summary className="cursor-pointer font-semibold">编辑主页内容{dirty?" · 未保存":""}</summary>
  <p className="my-3 text-mk-small text-mk-secondary">这些文字会出现在主页中。请写下希望读者了解的自己，或一件作品的内容与制作过程。</p>
  {error&&<div role="alert"><Says content={errorMarkdown(`保存或读取内容失败：${error}`)}/><p className="my-2 text-mk-small">当前输入仍保留。若其他位置已有修改，请先复制需要保留的文字，再读取最新内容。</p><button type="button" disabled={busy} className="underline" onClick={()=>void load()}>读取最新内容，替换当前输入</button></div>}
  {draft&&<fieldset disabled={busy} className="space-y-3 disabled:opacity-60">
   <label className="block text-mk-small">内容模块<select aria-label="主页内容模块" className="mt-1 block w-full rounded-mk-lg border border-mk-border p-3" value={selected} onChange={e=>setSelected(e.target.value)}><option value="about">自我介绍</option>{draft.sections?.map(s=><option key={s.key} value={s.key}>{s.title||"未命名模块"}</option>)}</select></label>
   {section&&<label className="block text-mk-small">标题<input aria-label="模块标题" maxLength={2000} value={section.title} onChange={e=>change({...draft,sections:draft.sections?.map(s=>s.key===selected?{...s,title:e.target.value}:s)})} className="mt-1 block w-full rounded-mk-lg border border-mk-border p-3"/></label>}
   <GrowingTextarea aria-label={selected==="about"?"主页自我介绍":"主页模块正文"} maxLength={2000} value={selected==="about"?(draft.about??[]).join("\n\n"):section?.body??""} placeholder={selected==="about"?"例如：我喜欢观察植物，正在尝试做一个能互动的太空温室主页。":"请介绍作品、你的尝试，以及希望读者看到的内容。"} onChange={e=>change(selected==="about"?{...draft,about:e.target.value.split("\n\n")}:{...draft,sections:draft.sections?.map(s=>s.key===selected?{...s,body:e.target.value}:s)})} className="min-h-32 w-full rounded-mk-lg border border-mk-border p-3"/>
   <div className="flex flex-wrap gap-3"><button type="button" disabled={!dirty} onClick={()=>void persist().catch(()=>{})} className="rounded-full bg-mk-accent-500 px-4 py-2 text-white disabled:opacity-40">{busy?"保存中…":"保存内容"}</button><button type="button" disabled={(draft.sections?.length??0)>=60} className="rounded-full border border-mk-border px-4 py-2" onClick={()=>{const key=crypto.randomUUID();change({...draft,sections:[...(draft.sections??[]),{key,title:"新作品",depth:0,body:""}]});setSelected(key);}}>添加作品模块</button></div>
  </fieldset>}
  {status&&<p role="status" className="mt-3 text-mk-small">{status}</p>}
 </details>;
}
