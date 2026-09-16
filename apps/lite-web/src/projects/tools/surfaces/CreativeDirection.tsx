import { Says, errorMarkdown } from "../../Says";
import { useEffect, useRef, useState } from "react";
import { beforeNavigate } from "../../../routing";
import { getCreativeDirection, saveCreativeDirection, suggestCreativeMotifs, refineHeroPrompt, type CreativeDraft, type CreativeDirection as Direction } from "../../../api/creativeDirection";
import { apiErrorText } from "../../../api/errorText";
import { GrowingTextarea } from "../../../shared/GrowingTextarea";
import { generateHeroCode, type CodeVersion } from "../../../api/codeVersions";
import { CodePreview } from "./CodePreview";
import { listSiteRefs, type SiteRef } from "../../../api/siteRefs";
import { HeroBriefCard } from "./HeroBriefCard";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";
const empty: CreativeDraft = { revision: 0, document: { stage: "feeling", feeling: "", motifs: [], suggestions: [] } };
const chip = "rounded-full border border-mk-border px-4 py-2 text-mk-body aria-pressed:border-mk-accent-500 aria-pressed:bg-mk-accent-50";
export function CreativeDirection({ projectId, onClose, onFinish }: ToolSurfaceProps) {
 const [draft,setDraft] = useState(empty);
 const [loaded,setLoaded] = useState(false);
 const savedDocument=useRef("");
 const [busy,setBusy] = useState(false);
 const [error,setError] = useState("");
 const [newest,setNewest] = useState<CodeVersion|null>(null);

 const [references,setReferences] = useState<SiteRef[]>([]);
 const [referenceError,setReferenceError] = useState("");
 useEffect(()=>{let live=true;listSiteRefs(projectId).then(rows=>{if(live)setReferences(rows);}).catch(err=>{if(live)setReferenceError(apiErrorText(err));});return()=>{live=false;};},[projectId]);
 const [remote,setRemote] = useState<CreativeDraft|null>(null);
 const doc = draft.document;
 const custom=doc.pendingMotif??"";
 useEffect(()=>{let active=true;getCreativeDirection(projectId).then(value=>{if(active){savedDocument.current=JSON.stringify(value.document);setDraft(value);setLoaded(true);}}).catch(err=>{if(active)setError(apiErrorText(err));});return()=>{active=false;};},[projectId]);
 const change=(patch:Partial<Direction>)=>setDraft(prev=>({...prev,document:{...prev.document,...patch,...((patch.feeling!==undefined||patch.motifs!==undefined)&&prev.document.hero?{hero:{...prev.document.hero,prompt:""}}:{})}}));
 const persist=async(document=doc)=>{
  const value=await saveCreativeDirection(projectId,{document,revision:draft.revision});savedDocument.current=JSON.stringify(value.document);setDraft(value);return value;
 };
 const saveBeforeLeaving=useRef<()=>Promise<void>>(async()=>{});
 const navigationPending=useRef<Promise<void>|null>(null);
 saveBeforeLeaving.current=async()=>{
  if(navigationPending.current)return navigationPending.current;
  if(!loaded)return;
  if(busy)throw new Error("创作构思正在处理中");
  if(JSON.stringify(doc)===savedDocument.current)return;
  setBusy(true);
  navigationPending.current=(async()=>{try{await persist();}catch(err){setError(apiErrorText(err));throw err;}finally{navigationPending.current=null;setBusy(false);}})();
  return navigationPending.current;
 };
 useEffect(()=>beforeNavigate(()=>saveBeforeLeaving.current()),[]);
 useEffect(()=>{
  if(!loaded||JSON.stringify(doc)===savedDocument.current)return;
  const warn=(event:BeforeUnloadEvent)=>event.preventDefault();
  window.addEventListener("beforeunload",warn);
  return()=>window.removeEventListener("beforeunload",warn);
 },[loaded,doc]);
 const inspectSaved=async()=>{
  if(busy)return;
  setBusy(true);
  try{setRemote(await getCreativeDirection(projectId));}catch(err){setError(apiErrorText(err));}finally{setBusy(false);}
 };
 const run=async(action:()=>Promise<void>)=>{if(busy)return;setBusy(true);setError("");try{await action();}catch(err){setError(apiErrorText(err));}finally{setBusy(false);}};
 const hero=doc.hero??{mode:"" as const,scene:"",action:"",prompt:""};
 const ready=Boolean(doc.feeling.trim()&&doc.motifs.length&&hero.mode&&hero.scene.trim()&&hero.prompt.trim());
 return <ToolFrame title="主页创作构思" task={doc.stage==="feeling"?"请描述喜欢的感觉":doc.stage==="motifs"?"请选择或补充具体意象":"请构思第一幕并核对提示词"} why="风格和意象将用于构思主页的第一幕、自我介绍和作品展示。" todo={ready?"":doc.stage==="hero"?"请完成第一幕构思并核对提示词":"请完成风格与意象构思"} finishLabel="确认创作方向" busy={busy||!loaded} onClose={()=>{if(!busy)onClose();}} onFinish={()=>void run(async()=>{const saved=await persist();onFinish(saved.document,`创作方向已确认。学生原话：${saved.document.feeling}；选择的意象：${saved.document.motifs.join("、")}。第一幕构思：${saved.document.hero?.scene}；交互：${saved.document.hero?.action||"未指定"}；提示词：${saved.document.hero?.prompt}。${saved.document.trial?`学生已保留试用版本，判断依据：${saved.document.trial.observation}。这不代表已发布。`:"本次确认的是制作说明；代码草稿的试用结果与发布状态需另行核对。"}`);})}>
  <div className="mx-auto max-w-[720px] space-y-6 py-4">
   {(error||remote)&&<section role="alert" className="rounded-mk-lg border border-mk-border p-4"><Says content={errorMarkdown(error)} /><button disabled={busy} className="mt-3 underline" onClick={()=>void inspectSaved()}>读取已保存版本进行比较</button>{remote&&<div className="mt-3 space-y-3"><div className="grid gap-4 md:grid-cols-2"><CreativeDraftSummary title="当前输入" document={doc}/><CreativeDraftSummary title="已保存版本" document={remote.document}/></div><button className={chip} onClick={()=>{savedDocument.current=JSON.stringify(remote.document);setDraft(remote);setRemote(null);setLoaded(true);setError("");}}>使用已保存版本</button><button className={chip} onClick={()=>{setDraft(prev=>({...prev,revision:remote.revision}));setRemote(null);setError("");}}>保留当前输入，继续编辑</button></div>}</section>}
   {!loaded?<p>正在读取创作构思…</p>:<fieldset disabled={busy} className="min-w-0 space-y-6">
    <p className="text-mk-small text-mk-muted">{doc.stage==="feeling"?"1 / 3 · 喜欢的感觉":doc.stage==="motifs"?"2 / 3 · 具体意象":"3 / 3 · 第一幕 Hero"}</p>
    {doc.stage==="feeling"?<>
     <h3 className="text-mk-h2">你希望主页给人什么感觉？</h3>
     <p className="text-mk-body text-mk-secondary">可以是酷炫、严谨、可爱、动漫，也可以是植物生长的感觉。请用自己的语言描述，可以混合不同想法。</p>
     <GrowingTextarea aria-label="喜欢的感觉" maxLength={2000} value={doc.feeling} onChange={e=>change({feeling:e.target.value,suggestions:[]})} placeholder="例如：像植物在生长，但又有一点科幻……" className="w-full rounded-mk-lg border border-mk-border bg-mk-surface p-4 text-mk-body" />
     <button className={chip} disabled={!doc.feeling.trim()} onClick={()=>void run(async()=>{await persist({...doc,stage:"motifs"});})}>构思意象 →</button>
    </>:doc.stage==="motifs"?<>
     <button className="text-mk-small underline" onClick={()=>void run(async()=>{await persist({...doc,stage:"feeling"});})}>← 修改风格描述</button>
     <blockquote className="rounded-mk-lg bg-mk-accent-50 p-4 text-mk-body whitespace-pre-wrap">{doc.feeling}</blockquote>
     <h3 className="text-mk-h2">这些感觉让你想到什么？</h3>
     <p className="text-mk-body text-mk-secondary">请用名词表达，例如宇宙、飞船、大树、温室，或你喜欢的动漫人物。可以组合多个意象。</p>
     <div className="flex flex-wrap gap-2">{doc.motifs.map(word=><button key={word} className={chip} aria-pressed="true" aria-label={`移除意象：${word}`} onClick={()=>change({motifs:doc.motifs.filter(w=>w!==word)})}>{word} ×</button>)}</div>
     <form className="flex gap-2" onSubmit={e=>{e.preventDefault();if(custom.trim()&&doc.motifs.length<12){change({motifs:[...new Set([...doc.motifs,custom.trim()])],pendingMotif:""});}}}><input aria-label="补充意象" value={custom} maxLength={60} onChange={e=>change({pendingMotif:e.target.value})} placeholder="输入一个具体名词" className="min-w-0 flex-1 rounded-mk-lg border border-mk-border p-3"/><button disabled={!custom.trim()||doc.motifs.length>=12} className={chip}>添加</button></form>
     <div className="border-t border-mk-border pt-5"><button className={chip} onClick={()=>void run(async()=>{const saved=await persist();setDraft(await suggestCreativeMotifs(projectId,saved.revision));})}>{busy?"正在联想…":doc.suggestions.length?"再想一些意象":"请 AI 一起联想"}</button><div className="mt-4 flex flex-wrap gap-2">{doc.suggestions.filter(word=>!doc.motifs.includes(word)).map(word=><button key={word} className={chip} disabled={doc.motifs.length>=12} onClick={()=>change({motifs:[...doc.motifs,word]})}>＋ {word}</button>)}</div></div>
     <button className={chip} disabled={!doc.motifs.length} onClick={()=>void run(async()=>{await persist({...doc,stage:"hero"});})}>构思第一幕 →</button>
    </>:<>
     <button className="text-mk-small underline" onClick={()=>void run(async()=>{await persist({...doc,stage:"motifs"});})}>← 修改意象</button>
     <p className="text-mk-small text-mk-secondary">选择的意象：{doc.motifs.join("、")}</p>
     <HeroBriefCard references={references} referenceError={referenceError} hero={hero} busy={busy} onChange={patch=>change({hero:{...hero,...patch}})} onRefine={()=>void run(async()=>{const saved=await persist();setDraft(await refineHeroPrompt(projectId,saved.revision));})}/>
     <button className={chip} disabled={!ready||busy} onClick={()=>void run(async()=>{const saved=await persist();setNewest(await generateHeroCode(projectId,saved.revision));})}>{busy?"正在生成…":"生成第一幕草稿"}</button>
     <CodePreview includeComparison={Boolean(doc.includeComparison)} onIncludeComparison={includeComparison=>void run(async()=>{await persist({...doc,includeComparison});})} responses={doc.responseDrafts??{}} onResponseChange={(version,response)=>change({responseDrafts:{...doc.responseDrafts,[version]:response}})} mode={hero.mode} includeProcess={Boolean(doc.includeProcess)} onIncludeProcess={includeProcess=>void run(async()=>{await persist({...doc,includeProcess,includeComparison:includeProcess?doc.includeComparison:false});})} onCompose={version=>void run(async()=>{const saved=await persist();setNewest(await generateHeroCode(projectId,saved.revision,version,"保留当前第一幕与交互，加入已保存的自我介绍和作品内容。",true));})} projectId={projectId} newest={newest} busy={busy} trial={doc.trial} onAccept={(versionId,observation)=>void run(async()=>{await persist({...doc,trial:{versionId,observation}});})} onRevise={(version,feedback,redrawImage)=>void run(async()=>{const saved=await persist();setNewest(await generateHeroCode(projectId,saved.revision,version,feedback,false,redrawImage));})}/>

    </>}
    <p className="text-mk-small text-mk-muted">切换步骤、确认或关闭时保存。</p>
   </fieldset>}
  </div>
 </ToolFrame>;
}

function CreativeDraftSummary({title,document}:{title:string;document:Direction}) {
 const fields=[
  ["喜欢的感觉",document.feeling], ["意象",document.motifs.join("、")], ["待添加意象",document.pendingMotif],
  ["呈现方式",document.hero?.mode==="code"?"代码效果":document.hero?.mode==="image"?"一张图片":document.hero?.mode==="mixed"?"图片与代码":""],
  ["第一幕画面",document.hero?.scene], ["第一幕互动",document.hero?.action],
  ["生成提示词",document.hero?.prompt], ["保留理由",document.trial?.observation],
  ["展示制作过程",document.includeProcess?"已选择":"未选择"],
  ["未提交的试用意见",Object.entries(document.responseDrafts??{}).map(([version,draft])=>`版本 ${version}\n${draft.response==="retain"?"准备保留":draft.response==="revise"?"准备修改":"未选择"}\n修改意见：${draft.feedback||"未填写"}\n保留理由：${draft.observation||"未填写"}\n重新生成图片：${draft.redrawImage?"是":"否"}`).join("\n\n")],
 ];
 return <section className="min-w-0 rounded-mk-lg border border-mk-border p-3"><h3 className="mb-3 font-semibold">{title}</h3><dl className="space-y-3 text-mk-small">{fields.map(([label,value])=><div key={label}><dt className="font-semibold">{label}</dt><dd className="mt-1 whitespace-pre-wrap break-words text-mk-secondary">{value||"未填写"}</dd></div>)}</dl></section>;
}
