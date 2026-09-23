import { useEffect, useRef, useState } from "react";
import { ArrowDown, ArrowUp, Check, Copy, ExternalLink, Loader2, Monitor, Smartphone } from "lucide-react";
import { ApiError } from "../api/client";
import { apiErrorText } from "../api/errorText";
import { getShowcase, publishShowcase, revokeShowcase, saveShowcase, type ShowcaseGuideDestination, type ShowcaseGuideStage, type ShowcaseState } from "../api/showcase";
import { beforeNavigate, navigate } from "../routing";
import { GrowingTextarea } from "../shared/GrowingTextarea";
import { Says, errorMarkdown } from "../projects/Says";
import { Showcase } from "../site/Showcase";
import { ShowcaseComponentEditor } from "./ShowcaseComponentEditor";
import { ShowcaseLinkEditor } from "./ShowcaseLinkEditor";
import { PortfolioShareDialog } from "../shared/PortfolioShareDialog";
import { GUIDE_STEPS, ShowcaseDesignGuide } from "./ShowcaseDesignGuide";
import { ShowcaseImagePicker } from "./ShowcaseImagePicker";
import { SHOWCASE_PRESETS, SHOWCASE_ILLUSTRATIONS } from "../site/showcasePresets";
import { SHOWCASE_THEMES, SHOWCASE_FONT_STACKS } from "../site/showcaseThemes";
import type { ShowcaseConfig, ShowcasePortfolioLayout, ShowcaseWork } from "../site/showcaseTypes";
import "./showcaseEditor.css";
import { parseShowcaseInterests } from "./showcaseDraft";
import { destinationEditor, initialShowcaseGuide, type ShowcaseGuidePhase } from "./showcaseGuideFlow";

const sections = { writing: "写作", reading: "阅读", project: "项目" } as const;
const layouts = [
  { id: "folio", label: "作品画廊", detail: "醒目的介绍，舒展的作品卡片", lines: [75, 45, 90] },
  { id: "journal", label: "个人刊物", detail: "编辑式排版，文字成为主角", lines: [45, 90, 90] },
  { id: "studio", label: "灵感工作室", detail: "圆角与图形，活泼的内容组合", lines: [60, 80, 65] },
] as const;
const palettes = [
  { id: "paper", label: "暖纸" },
  { id: "forest", label: "森林" },
  { id: "ocean", label: "海岸" },
  { id: "rose", label: "莓果" },
  { id: "night", label: "夜空" },
  { id: "sunshine", label: "晴日" },
] as const;
const portfolioChoices: {id:ShowcasePortfolioLayout;label:string;detail:string}[] = [
  {id:"sections",label:"分类展厅",detail:"写作、阅读、项目各有自己的版块"},
  {id:"flow",label:"流式画廊",detail:"大小错落的作品卡，适合内容丰富的主页"},
  {id:"timeline",label:"成长时间轴",detail:"按完成日期阅读作品和变化"},
  {id:"film",label:"胶片放映",detail:"横向翻阅几件精选作品"},
  {id:"list",label:"清晰目录",detail:"以标题和简介为主，查找最方便"},
  {id:"planets",label:"作品星系",detail:"每件作品是一颗独立的星球"},
  {id:"cloud",label:"标题云",detail:"用大大小小的标题展现创作主题"},
  {id:"calendar",label:"作品日历",detail:"按日期回看公开成果"},
];

function selectedWorkOrder(config: ShowcaseConfig, work: ShowcaseWork) {
  return config.selectedWorkIds.indexOf(work.id);
}

export function MySitePage({managerMode=false}:{managerMode?:boolean}) {
  const [state, setState] = useState<ShowcaseState | null>(null);
  const [draft, setDraft] = useState<ShowcaseConfig | null>(null);
  const [interestText, setInterestText] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [conflict, setConflict] = useState(false);
  const [busy, setBusy] = useState(false);
  const [imageBusy, setImageBusy] = useState(false);
  const [imageUrls, setImageUrls] = useState<Record<string, string>>({});
  const busyRef = useRef(false);
  const controlledNavigation = useRef(false);
  const [tab, setTab] = useState<ShowcaseGuideStage>("design");
  const [guidePhase, setGuidePhase] = useState<ShowcaseGuidePhase>("welcome");
  const [profileTab, setProfileTab] = useState("content");
  const [heroTab, setHeroTab] = useState("text");
  const [shareUrl, setShareUrl] = useState("");
  const [mobileView, setMobileView] = useState<"edit" | "preview">("edit");
  const [narrow, setNarrow] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const previewAreaRef = useRef<HTMLElement>(null);
  const [componentRequest,setComponentRequest]=useState<{id:number;prompt:string}|null>(null);
  const interestInput = parseShowcaseInterests(interestText);
  const effectiveDraft = draft ? { ...draft, interests: interestInput.interests } : null;
  const availableWorks: ShowcaseWork[] = [...(state?.availableWorks??[]).filter(work=>!work.id.startsWith("external:")), ...(draft?.customWorks??[]).map(work=>({id:`external:${work.id}`,kind:"project" as const,title:work.title,summary:work.summary,externalUrl:work.url,date:work.date}))];
  const dirty = !!state && !!effectiveDraft && JSON.stringify(effectiveDraft) !== JSON.stringify(state.draft);

  function accept(next: ShowcaseState) {
    setState(next); setDraft(next.draft);
    setImageUrls(current => ({...current, ...(next.draft.heroImageKey && next.heroImageUrl ? {[next.draft.heroImageKey]:next.heroImageUrl}:{}), ...(next.draft.avatarKey && next.avatarUrl ? {[next.draft.avatarKey]:next.avatarUrl}:{})})); setInterestText(next.draft.interests.join("、"));
  }
  useEffect(() => {
    let cancelled = false;
    setLoading(true); setError(""); setConflict(false);
    getShowcase().then(next => { if (!cancelled) { accept(next); const guide = initialShowcaseGuide(next); setGuidePhase(guide.phase); setTab(guide.stage); } })
      .catch(err => { if (!cancelled) setError(`读取失败：${apiErrorText(err)}`); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [attempt]);
  useEffect(() => {
    if (!dirty && !busy && !imageBusy) return;
    const remove = beforeNavigate(async () => {
      if (controlledNavigation.current) return;
      setError(busy || imageBusy ? "正在处理，请稍后离开。" : "修改尚未保存，请保存草稿或撤销修改后离开。");
      throw new Error("showcase editor pending");
    });
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); };
    window.addEventListener("beforeunload", warn);
    return () => { remove(); window.removeEventListener("beforeunload", warn); };
  }, [dirty, busy, imageBusy]);
  useEffect(() => {
    if (managerMode || loading || (mobileView === "edit" && window.matchMedia("(max-width: 700px)").matches)) return;
    const frame = previewAreaRef.current;
    if (!frame) return;
    const selector = tab === "hero" || tab === "design" || tab === "revise" || guidePhase === "welcome" ? ".showcase-hero" : tab === "profile" ? profileTab === "tree" ? ".showcase-interest" : ".showcase-about" : tab === "works" ? ".showcase-main" : tab === "components" ? ".showcase-custom-components" : ".showcase";
    const target = frame.querySelector<HTMLElement>(selector) ?? frame.querySelector<HTMLElement>(".showcase");
    if (!target) return;
    const top = target.getBoundingClientRect().top - frame.getBoundingClientRect().top + frame.scrollTop - 18;
    frame.scrollTo({top:Math.max(0,top),behavior:"smooth"});
  }, [tab,profileTab,mobileView,loading,managerMode,guidePhase]);

  async function action(kind: "save" | "publish" | "revoke") {
    if (!state || !effectiveDraft || busyRef.current || imageBusy) return;
    if (kind !== "revoke" && interestInput.error) { setError(interestInput.error); setTab("profile"); return; }
    busyRef.current = true; setBusy(true); setError(""); setNotice("");
    try {
      let next = state;
      if (kind === "save") next = await saveShowcase(effectiveDraft, state.revision);
      if (kind === "publish") {
        if (dirty) { setError("请先保存草稿，再发布当前版本。"); return; }
        next = await publishShowcase(state.revision);
        setShareUrl(next.url);
        setGuidePhase("revise"); setTab("revise");
      }
      if (kind === "revoke") next = await revokeShowcase();
      // Revocation changes visibility only; retain unsaved local edits.
      if (kind === "revoke") setState(next); else accept(next);
      setNotice(kind === "save" ? "草稿已保存，公开页面未改变" : kind === "publish" ? "主页已发布" : "主页已停止发布");
    } catch (err) { if (err instanceof ApiError && err.status === 409) setConflict(true); setError(`${kind === "save" ? "保存" : kind === "publish" ? "发布" : "停止发布"}失败：${apiErrorText(err)}`); }
    finally { busyRef.current = false; setBusy(false); }
  }
  function patch(values: Partial<ShowcaseConfig>) { setDraft(current => current ? { ...current, ...values } : current); setNotice(""); }
  function startGuide() { setGuidePhase("build"); setTab("design"); patch({guideStage:"design"}); }
  function goToStep(stage: ShowcaseGuideStage) {
    setTab(stage);
    if (guidePhase === "build" && stage !== "revise") patch({guideStage:stage});
  }
  function goToDestination(destination: ShowcaseGuideDestination) {
    const editor = destinationEditor(destination);
    if (editor.heroTab) setHeroTab(editor.heroTab);
    if (editor.profileTab) setProfileTab(editor.profileTab);
    goToStep(editor.stage);
    if (window.matchMedia("(max-width: 700px)").matches) setMobileView("edit");
  }
  async function completeGuide() {
    if (!state || !effectiveDraft || busyRef.current || imageBusy) return;
    if (!effectiveDraft.name.trim() || !(effectiveDraft.bio.trim() || effectiveDraft.tagline.trim())) {setError("请先填写展示名称，以及个人简介或开场介绍。");goToStep("profile");return;}
    if (interestInput.error) {setError(interestInput.error); goToStep("profile"); return;}
    busyRef.current=true; setBusy(true); setError("");
    try {
      const next=await saveShowcase({...effectiveDraft,guideStage:"finish",guideCompleted:true},state.revision);
      accept(next);setGuidePhase("revise");setTab("revise");setNotice("初版已保存。可以告诉印记你想修改哪里，或发布主页。");
    } catch(err) {if (err instanceof ApiError && err.status===409)setConflict(true);setError(`保存失败：${apiErrorText(err)}`);}
    finally {busyRef.current=false;setBusy(false);}
  }
  async function openWorksManager() {
    if (!state || !effectiveDraft || busyRef.current || imageBusy) return;
    if (interestInput.error) { setError(interestInput.error); goToStep("profile"); return; }
    busyRef.current = true; setBusy(true); setError("");
    try {
      if (dirty) accept(await saveShowcase(effectiveDraft, state.revision));
      controlledNavigation.current = true;
      navigate("/site/works");
      window.setTimeout(() => { controlledNavigation.current = false; }, 0);
    } catch (err) {
      controlledNavigation.current = false;
      if (err instanceof ApiError && err.status === 409) setConflict(true);
      setError(`保存失败：${apiErrorText(err)}`);
    } finally { busyRef.current = false; setBusy(false); }
  }
  function moveSection(index: number, offset: number) {
    if (!draft) return;
    const order = [...draft.sectionOrder];
    [order[index], order[index + offset]] = [order[index + offset]!, order[index]!];
    patch({ sectionOrder: order });
  }
  function moveWork(id: string, offset: number, kind: ShowcaseWork["kind"]) {
    if (!draft || !state) return;
    const inSection = availableWorks.filter(w => w.kind === kind && draft.selectedWorkIds.includes(w.id)).sort((a, b) => selectedWorkOrder(draft, a) - selectedWorkOrder(draft, b));
    const index = inSection.findIndex(w => w.id === id);
    const other = inSection[index + offset];
    if (!other) return;
    const ids = [...draft.selectedWorkIds];
    const a = ids.indexOf(id), b = ids.indexOf(other.id);
    [ids[a], ids[b]] = [ids[b]!, ids[a]!]; patch({ selectedWorkIds: ids });
  }

  if (loading) return <div className="showcase-loading" role="status"><Loader2 className="animate-spin" size={22} />正在加载主页</div>;
  if (!state || !draft || !effectiveDraft) return <div className="showcase-loading"><div role="alert"><Says content={errorMarkdown(error)} /></div><button onClick={() => setAttempt(n => n + 1)}>重新加载</button></div>;
  const picked = availableWorks.filter(w => effectiveDraft.selectedWorkIds.includes(w.id));
  const unavailableIds = draft.selectedWorkIds.filter(id => !availableWorks.some(work => work.id === id));
  const canPublish = !!draft.name.trim() && !!(draft.bio.trim() || draft.tagline.trim());

  const worksControls = <>

            <div className="showcase-heading"><h2>管理公开作品</h2><p>选择要放进主页的写作和阅读报告，调整展示形式。报告的公开状态可在对应的报告页面修改。</p></div>
            <label className="showcase-field">首页展示数量<select value={draft.homeWorkLimit??6} onChange={e=>patch({homeWorkLimit:Number(e.target.value)})}>{[3,6,9,12].map(n=><option key={n} value={n}>{n} 件</option>)}</select><small>勾选的全部作品可在“全部作品”页查看。</small></label>
            <ShowcaseLinkEditor items={draft.customWorks??[]} onChange={customWorks=>{const selectedWorkIds=draft.selectedWorkIds.filter(id=>!id.startsWith("external:")||customWorks.some(work=>`external:${work.id}`===id));patch({customWorks,selectedWorkIds,featuredWorkIds:(draft.featuredWorkIds??[]).filter(id=>selectedWorkIds.includes(id))});}}/>
            <div className="showcase-field-group"><h3>作品呈现</h3><p className="showcase-mode-help">先选一种展示方式，在主页制作页预览。时间轴和日历使用作品的完成日期。</p><div className="showcase-portfolio-choices">{portfolioChoices.map(choice=><button type="button" key={choice.id} aria-pressed={(draft.portfolioLayout??"sections")===choice.id} onClick={()=>patch({portfolioLayout:choice.id})}><span className="showcase-portfolio-choice-art" data-mode={choice.id} aria-hidden="true"><i/><i/><i/></span><strong>{choice.label}</strong><small>{choice.detail}</small></button>)}</div></div>
            <div className="showcase-featured-editor"><div><h3>重点作品</h3><span>{(draft.featuredWorkIds??[]).length} / 2</span></div><p>可从已选作品中指定最多两件，放在作品区开头详细介绍。保持空白就是纯粹的列表或画廊。</p>{(draft.featuredWorkIds??[]).length>0&&<button type="button" onClick={()=>patch({featuredWorkIds:[]})}>取消全部重点作品</button>}</div>
            {(draft.portfolioLayout??"sections")==="sections" && <><div className="showcase-field-group"><h3>写作展示</h3><div className="showcase-segments">{([["cards","文章卡片"],["list","文章列表"]] as const).map(([id,label])=><button key={id} aria-pressed={draft.writingStyle===id} onClick={()=>patch({writingStyle:id})}>{label}</button>)}</div></div><div className="showcase-field-group"><h3>阅读展示</h3><div className="showcase-segments">{([["shelf","阅读书架"],["list","阅读列表"]] as const).map(([id,label])=><button key={id} aria-pressed={draft.readingStyle===id} onClick={()=>patch({readingStyle:id})}>{label}</button>)}</div></div></>}
            {unavailableIds.length > 0 && <div className="showcase-editor-empty"><p>{unavailableIds.length} 件已选作品已停止发布或暂不可用，不会在主页展示。</p><button className="underline mt-2" onClick={() => patch({selectedWorkIds: draft.selectedWorkIds.filter(id => !unavailableIds.includes(id))})}>移除不可用作品</button></div>}
            {draft.sectionOrder.map((kind, index) => {
              const works = availableWorks.filter(w => w.kind === kind).sort((a, b) => {
                const ai = selectedWorkOrder(draft, a), bi = selectedWorkOrder(draft, b);
                return (ai < 0 ? Infinity : ai) - (bi < 0 ? Infinity : bi);
              });
              const selected = works.filter(w => draft.selectedWorkIds.includes(w.id));
              return <section className="showcase-work-group" key={kind}><header><h3>{sections[kind]}</h3><span>版块顺序</span><button aria-label={`上移${sections[kind]}版块`} disabled={index === 0} onClick={() => moveSection(index, -1)}><ArrowUp size={15} /></button><button aria-label={`下移${sections[kind]}版块`} disabled={index === draft.sectionOrder.length - 1} onClick={() => moveSection(index, 1)}><ArrowDown size={15} /></button></header>
                {works.length === 0 ? <p className="showcase-editor-empty">{kind === "project" ? "完成项目后，可在这里选择展示。" : `发布${kind === "writing" ? "文章" : "阅读成果"}后，可在这里选择展示。`}</p> : works.map(work => {
                  const checked = draft.selectedWorkIds.includes(work.id);
                  const featured=(draft.featuredWorkIds??[]).includes(work.id);
                  return <div className="showcase-work-option" key={work.id}><div className="showcase-work-copy"><label><input type="checkbox" checked={checked} onChange={() => patch({ selectedWorkIds: checked ? draft.selectedWorkIds.filter(id => id !== work.id) : [...draft.selectedWorkIds, work.id], featuredWorkIds: checked ? (draft.featuredWorkIds??[]).filter(id=>id!==work.id) : (draft.featuredWorkIds??[]) })} /><span><strong>{work.title}</strong>{work.summary && <small>{work.summary}</small>}</span></label>{checked&&<button type="button" className="showcase-featured-toggle" aria-pressed={featured} disabled={!featured&&(draft.featuredWorkIds??[]).length>=2} onClick={()=>patch({featuredWorkIds:featured?(draft.featuredWorkIds??[]).filter(id=>id!==work.id):[...(draft.featuredWorkIds??[]),work.id]})}>{featured?"★ 已设为重点":"☆ 设为重点"}</button>}{work.managePath&&<button type="button" className="showcase-manage-report" onClick={()=>navigate(work.managePath!)}>在报告页管理公开状态</button>}</div>{checked && <div className="showcase-work-order"><button aria-label={`上移作品 ${work.title}`} disabled={selected[0]?.id === work.id} onClick={() => moveWork(work.id, -1, kind)}><ArrowUp size={13} /></button><button aria-label={`下移作品 ${work.title}`} disabled={selected[selected.length - 1]?.id === work.id} onClick={() => moveWork(work.id, 1, kind)}><ArrowDown size={13} /></button></div>}</div>;
                })}</section>;
            })}
  </>;

  if (managerMode) return <div className="showcase-editor showcase-works-page">
    <header className="showcase-toolbar"><div><p className="showcase-eyebrow">个人展示</p><h1>作品管理</h1><p className="showcase-status">{dirty ? "有未保存的修改" : state.published && state.hasUnpublishedChanges ? "草稿已保存 · 待更新发布" : "选择主页要展示的作品"}</p></div>
      <div className="showcase-actions"><button type="button" onClick={()=>navigate("/site")}>返回主页制作</button><button disabled={busy || !dirty} onClick={()=>void action("save")}>保存草稿</button><button className="showcase-primary" disabled={busy || dirty || !canPublish || (state.published && !state.hasUnpublishedChanges)} onClick={()=>void action("publish")}>{state.published ? "更新发布" : "发布主页"}</button></div>
    </header>
    {(error || notice) && <div className={`showcase-feedback ${error ? "is-error" : ""}`} role={error ? "alert" : "status"}>{error ? <Says content={errorMarkdown(error)} /> : <><Check size={16} />{notice}</>}</div>}
    {conflict && <div className="showcase-feedback"><span>当前修改已保留。</span><button className="underline" disabled={busy} onClick={() => {setAttempt(n => n + 1); setNotice("");}}>放弃本地修改，读取最新版本</button></div>}
    {shareUrl && <PortfolioShareDialog url={shareUrl} onClose={()=>setShareUrl("")}/>}
    <main className="showcase-works-content"><fieldset disabled={busy} className="showcase-fields">{worksControls}</fieldset></main>
  </div>;

  return <div className="showcase-editor">
    <header className="showcase-toolbar">
      <div><p className="showcase-eyebrow">个人展示</p><h1>我的主页</h1><p className="showcase-status">{dirty ? "有未保存的修改" : state.published ? state.hasUnpublishedChanges ? "草稿已保存 · 待更新发布" : "已发布" : "仅自己可见"}</p></div>
      <div className="showcase-actions">
        {state.url && (state.published || state.hasLegacySite) && <a href={state.url} target="_blank" rel="noreferrer">查看公开页<ExternalLink size={14} /></a>}
        <button type="button" disabled={busy || imageBusy} onClick={()=>void openWorksManager()}>管理作品</button>
        {dirty && <button disabled={busy || imageBusy} onClick={() => {accept(state); setError(""); setNotice("修改已撤销");}}>撤销修改</button>}
        <button disabled={busy || imageBusy || !dirty || !!interestInput.error} onClick={() => void action("save")}>{busy ? "处理中" : "保存草稿"}</button>
        <button className="showcase-primary" disabled={busy || imageBusy || dirty || !!interestInput.error || !canPublish || (state.published && !state.hasUnpublishedChanges)} onClick={() => void action("publish")}>{state.published || state.hasLegacySite ? "更新发布" : "发布主页"}</button>
        {(state.published || state.hasLegacySite) && <><button disabled={busy || imageBusy} onClick={() => void navigator.clipboard.writeText(state.url).then(() => setNotice("公开链接已复制")).catch(() => setError("复制失败，请从公开页面复制地址"))}><Copy size={14} />复制链接</button><button disabled={busy || imageBusy} onClick={() => void action("revoke")}>停止发布</button></>}
      </div>
    </header>
    {(error || notice) && <div className={`showcase-feedback ${error ? "is-error" : ""}`} role={error ? "alert" : "status"}>{error ? <Says content={errorMarkdown(error)} /> : <><Check size={16} />{notice}</>}</div>}
    {conflict && <div className="showcase-feedback"><span>当前修改已保留。</span><button className="underline" disabled={busy || imageBusy} onClick={() => {setAttempt(n => n + 1); setNotice("");}}>放弃本地修改，读取最新版本</button></div>}
    <div className="showcase-mobile-switch" aria-label="编辑或预览">{(["edit", "preview"] as const).map(v => <button key={v} aria-pressed={mobileView === v} onClick={() => setMobileView(v)}>{v === "edit" ? "编辑主页" : "查看预览"}</button>)}</div>
    {shareUrl && <PortfolioShareDialog url={shareUrl} onClose={()=>setShareUrl("")}/>}
    <div className="showcase-workspace" data-mobile-view={mobileView}>
      <aside className="showcase-controls">
        {guidePhase !== "welcome" && <nav className="showcase-tabs" aria-label="主页制作步骤">{guidePhase === "revise" && <button aria-pressed={tab === "revise"} onClick={() => goToStep("revise")}>修改需求</button>}{GUIDE_STEPS.map((step,index) => <button key={step.id} aria-pressed={tab === step.id} onClick={() => goToStep(step.id)}><span>{index+1}</span>{step.title}</button>)}</nav>}
        <ShowcaseDesignGuide phase={guidePhase} stage={tab} draft={effectiveDraft} interestTree={state.availableInterestTree} available={state.aboutChatAvailable===true} disabled={busy||imageBusy} onBusy={setImageBusy} onStart={startGuide} onStep={goToStep} onNavigate={goToDestination} onComplete={()=>void completeGuide()} onComponentIdea={prompt=>{patch({componentPrompt:prompt});setComponentRequest(current=>({id:(current?.id??0)+1,prompt}));}} onConversation={messages=>patch({guideConversation:messages})} onApply={proposal=>{const {reason,...changes}=proposal;patch(changes);if(changes.interests)setInterestText(changes.interests.join("、"));setNotice("设计建议已应用，请预览并保存草稿");}}>
        {tab === "profile" && <nav className="showcase-subtabs" aria-label="介绍设置">{([["content","内容"],["tree","兴趣树"],["avatar","头像"]] as const).map(([id,label])=><button key={id} aria-pressed={profileTab===id} onClick={()=>setProfileTab(id)}>{label}</button>)}</nav>}
        {tab === "hero" && <nav className="showcase-subtabs" aria-label="开场设置">{([["text","文字"],["art","系统配图"],["image","自定义图片"]] as const).map(([id,label])=><button key={id} aria-pressed={heroTab===id} onClick={()=>setHeroTab(id)}>{label}</button>)}</nav>}
        <fieldset disabled={busy || imageBusy} className="showcase-fields">

          {tab === "design" && <>
            <div className="showcase-heading"><h2>选择喜欢的样子</h2><p>从一套视觉方案开始，再调整配图、字体和版式。</p></div>
            <div className="showcase-presets">{SHOWCASE_PRESETS.map(preset => {
              const art = SHOWCASE_ILLUSTRATIONS.find(item => item.id === preset.config.illustration);
              return <button key={preset.id} className={`showcase-preset preset-${preset.id}`} aria-pressed={(draft.style ?? "classic") === preset.id} onClick={() => patch({...preset.config, heroImageKey:""})}><span className="showcase-preset-art">{art?.src ? <img src={art.src} alt="" loading="lazy" /> : <span>Aa<br />留白</span>}</span><strong>{preset.name}</strong><small>{preset.description}</small></button>;
            })}</div>
            <div className="showcase-field-group"><h3>版式</h3><div className="showcase-layouts">{layouts.map(layout => <button key={layout.id} className={`showcase-layout ${draft.layout === layout.id ? "is-selected" : ""}`} aria-pressed={draft.layout === layout.id} onClick={() => patch({ layout: layout.id })}><span className={`showcase-layout-sketch sketch-${layout.id}`} aria-hidden="true"><i /><span>{layout.lines.map((w, i) => <b key={i} style={{ width: `${w}%` }} />)}</span></span><span><strong>{layout.label}</strong><small>{layout.detail}</small></span>{draft.layout === layout.id && <Check size={15} />}</button>)}</div></div>
            <div className="showcase-field-group"><h3>色调</h3><div className="showcase-palette-grid">{palettes.map(palette => <button key={palette.id} aria-pressed={draft.palette === palette.id} className={draft.palette === palette.id ? "is-selected" : ""} onClick={() => patch({ palette: palette.id })}><span className="showcase-swatches" aria-hidden="true">{[SHOWCASE_THEMES[palette.id].paper, SHOWCASE_THEMES[palette.id].ink, SHOWCASE_THEMES[palette.id].accent].map(color => <i key={color} style={{ background: color }} />)}</span>{palette.label}{draft.palette === palette.id && <Check size={12} />}</button>)}</div></div>
            <div className="showcase-field-group"><h3>字体</h3><div className="showcase-font-grid">{([["sans", "清晰"], ["serif", "书卷"], ["mono", "等宽"]] as const).map(([id, label]) => <button key={id} style={{fontFamily:SHOWCASE_FONT_STACKS[id]}} aria-pressed={( ["rounded", "handwritten", "display"].includes(draft.font) ? "sans" : draft.font) === id} onClick={() => patch({ font: id })}>{label}</button>)}</div></div>
          </>}
          {tab === "hero" && <>
            <div className="showcase-heading"><h2>欢迎来到你的空间</h2><p>全屏开场之后，访客可以继续查看个人介绍和作品。</p></div>
            <div hidden={heroTab !== "text"}><label className="showcase-field">开场标题<GrowingTextarea value={draft.heroTitle ?? ""} maxLength={200} onChange={e => patch({heroTitle:e.target.value})} placeholder={`欢迎来到${draft.name || "我的"}的空间`} /></label>
            <label className="showcase-field">开场介绍<GrowingTextarea value={draft.tagline} maxLength={200} onChange={e => patch({tagline:e.target.value})} placeholder="请写下想对访客说的话。" /></label>
            </div><div hidden={heroTab !== "image"}><div className="showcase-field-group"><h3>自定义图片</h3><ShowcaseImagePicker prompt={draft.heroImagePrompt ?? ""} onPromptChange={value => patch({heroImagePrompt:value})} purpose="hero" currentUrl={draft.heroImageKey ? imageUrls[draft.heroImageKey] : ""} disabled={busy || imageBusy} onBusy={setImageBusy} onPick={(key,url) => {if(key)setImageUrls(current=>({...current,[key]:url})); patch({heroImageKey:key});}} /></div>
            </div><div hidden={heroTab !== "art"}><div className="showcase-field-group"><h3>开场配图</h3><div className="showcase-art-options">{SHOWCASE_ILLUSTRATIONS.map(art => <button key={art.id} aria-pressed={!draft.heroImageKey && (draft.illustration ?? "none") === art.id} onClick={() => patch({illustration:art.id,heroImageKey:""})}>{art.src ? <img src={art.src} alt="" loading="lazy" /> : <span>留白</span>}<small>{art.name}</small></button>)}</div></div>
          </div></>}
          <div className="showcase-profile-panel" hidden={tab !== "profile"}>
            <div className="showcase-heading"><h2>{profileTab === "content" ? "介绍内容与版式" : profileTab === "avatar" ? "个人照片或头像" : "兴趣树展示"}</h2><p>{profileTab === "content" ? "请填写愿意公开的基本信息，再与印记讨论如何呈现。" : profileTab === "avatar" ? "上传图片，或用文字描述生成新头像。" : "选择在主页分享哪些兴趣内容。"}</p></div>
            <div hidden={profileTab !== "content"}>
            <div className="showcase-field-group"><h3>介绍版式</h3><div className="showcase-segments">{([["classic","名字与简介"],["orbit","照片与关键词"]] as const).map(([id,label])=><button key={id} aria-pressed={(draft.aboutLayout??"classic")===id} onClick={()=>patch({aboutLayout:id})}>{label}</button>)}</div></div>
            <label className="showcase-field mt-6">展示名称<input value={draft.name} maxLength={80} onChange={e => patch({ name: e.target.value })} placeholder="名字或昵称" /></label>

            <label className="showcase-field">个人简介<GrowingTextarea value={draft.bio} maxLength={2000} rows={4} onChange={e => patch({ bio: e.target.value })} placeholder="请介绍你的兴趣、正在探索的事情，或想分享的经历。" /></label>
            <label className="showcase-field">兴趣关键词<input value={interestText} onChange={e => {setInterestText(e.target.value); setNotice("");}} maxLength={500} placeholder="动漫、科幻、植物、摄影" /><small>用顿号或逗号分隔，最多 12 个。已填写 {interestInput.interests.length} 个。</small>{interestInput.error && <small role="alert" className="showcase-field-error">{interestInput.error}</small>}</label>
            </div>
            <div hidden={profileTab !== "tree"}><div className="showcase-field-group"><h3>兴趣树展示</h3><div className="showcase-mode-options">{([["none","不展示"],["tree","展示兴趣树"],["keywords","展示关键词"]] as const).map(([id,label])=><button type="button" key={id} aria-pressed={(draft.interestTreeMode??"none")===id} onClick={()=>patch({interestTreeMode:id})}>{label}</button>)}</div><p className="showcase-mode-help">选择展示后，发布时会分享兴趣树中的领域与关键词，不包含对话、来源或学习记录。后续变化需要重新发布。</p>{draft.interestTreeMode && draft.interestTreeMode !== "none" && !state.availableInterestTree?.keywords.length && <p className="showcase-editor-empty">兴趣树暂无关键词。产生新的兴趣关键词后，可在这里预览并发布。</p>}</div>
            </div><div hidden={profileTab !== "avatar"}><div className="showcase-field-group"><h3>个人照片或头像</h3><ShowcaseImagePicker prompt={draft.avatarImagePrompt ?? ""} onPromptChange={value => patch({avatarImagePrompt:value})} purpose="avatar" currentUrl={draft.avatarKey ? imageUrls[draft.avatarKey] : ""} disabled={busy || imageBusy} onBusy={setImageBusy} onPick={(key,url)=>{if(key)setImageUrls(current=>({...current,[key]:url}));patch({avatarKey:key});}} /></div>
          </div></div>
          {tab === "components" && <ShowcaseComponentEditor components={draft.components??[]} disabled={busy||imageBusy} onBusy={setImageBusy} prompt={draft.componentPrompt??""} onPromptChange={componentPrompt=>patch({componentPrompt})} style={draft.style} palette={draft.palette} generationRequest={componentRequest} onRequestConsumed={()=>setComponentRequest(null)} onChange={components=>patch({components})}/>}
          {tab === "works" && <div className="showcase-works-entrance"><h2>管理主页作品</h2><p>请在作品管理页选择公开报告、调整作品顺序和展示方式。</p><button type="button" disabled={busy || imageBusy} onClick={()=>void openWorksManager()}>打开作品管理</button></div>}
          {tab === "finish" && <div className="showcase-heading"><h2>检查并发布</h2><p>右侧是访客将看到的主页。请检查文字、图片、作品与组件；保存草稿后，再决定是否发布。</p><p className="showcase-mode-help">保存草稿仅自己可见。发布后，持有链接的人可以查看你选择的作品和组件。</p></div>}
        </fieldset>
        </ShowcaseDesignGuide>
      </aside>
      <section ref={previewAreaRef} className="showcase-preview-area" aria-label="主页预览">
        <div className="showcase-preview-toolbar"><span><i />{guidePhase === "welcome" ? "空间预览" : "实时预览"}</span><div><button aria-label="宽屏预览" aria-pressed={!narrow} disabled={guidePhase === "welcome"} onClick={() => setNarrow(false)}><Monitor size={16} /></button><button aria-label="手机预览" aria-pressed={narrow} disabled={guidePhase === "welcome"} onClick={() => setNarrow(true)}><Smartphone size={16} /></button></div><span>{guidePhase === "welcome" ? "准备开始" : `${picked.length} 件作品`}</span></div>
        <div className={`showcase-preview-frame ${narrow ? "is-narrow" : ""} ${guidePhase === "welcome" ? "is-dormant" : ""}`}><div className="showcase-preview-canvas" aria-hidden={guidePhase === "welcome"}><Showcase config={effectiveDraft} works={availableWorks} interestTree={draft.interestTreeMode && draft.interestTreeMode !== "none" && state.availableInterestTree ? {...state.availableInterestTree, mode:draft.interestTreeMode, title:draft.interestTreeMode === "tree" ? "兴趣树" : "兴趣"} : undefined} heroImageUrl={draft.heroImageKey ? imageUrls[draft.heroImageKey] : undefined} avatarUrl={draft.avatarKey ? imageUrls[draft.avatarKey] : undefined} narrow={narrow || undefined} editing /></div>{guidePhase === "welcome" && <div className="showcase-preview-dormant"><span>个人作品空间</span><p>从风格开始，逐步完成你的主页。</p></div>}</div>
      </section>
    </div>
  </div>;
}
