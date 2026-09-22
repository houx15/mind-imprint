import { useEffect, useRef, useState } from "react";
import { ArrowDown, ArrowUp, Check, Copy, ExternalLink, Loader2, Monitor, Smartphone } from "lucide-react";
import { ApiError } from "../api/client";
import { apiErrorText } from "../api/errorText";
import { getShowcase, publishShowcase, revokeShowcase, saveShowcase, type ShowcaseState } from "../api/showcase";
import { beforeNavigate } from "../routing";
import { GrowingTextarea } from "../shared/GrowingTextarea";
import { Says, errorMarkdown } from "../projects/Says";
import { Showcase } from "../site/Showcase";
import { SHOWCASE_THEMES } from "../site/showcaseThemes";
import type { ShowcaseConfig, ShowcaseWork } from "../site/showcaseTypes";
import "./showcaseEditor.css";
import { parseShowcaseInterests } from "./showcaseDraft";

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

function selectedWorkOrder(config: ShowcaseConfig, work: ShowcaseWork) {
  return config.selectedWorkIds.indexOf(work.id);
}

export function MySitePage() {
  const [state, setState] = useState<ShowcaseState | null>(null);
  const [draft, setDraft] = useState<ShowcaseConfig | null>(null);
  const [interestText, setInterestText] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [conflict, setConflict] = useState(false);
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);
  const [tab, setTab] = useState<"profile" | "design" | "works">("design");
  const [mobileView, setMobileView] = useState<"edit" | "preview">("edit");
  const [narrow, setNarrow] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const interestInput = parseShowcaseInterests(interestText);
  const effectiveDraft = draft ? { ...draft, interests: interestInput.interests } : null;
  const dirty = !!state && !!effectiveDraft && JSON.stringify(effectiveDraft) !== JSON.stringify(state.draft);

  function accept(next: ShowcaseState) {
    setState(next); setDraft(next.draft); setInterestText(next.draft.interests.join("、"));
  }
  useEffect(() => {
    let cancelled = false;
    setLoading(true); setError(""); setConflict(false);
    getShowcase().then(next => { if (!cancelled) accept(next); })
      .catch(err => { if (!cancelled) setError(`读取失败：${apiErrorText(err)}`); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [attempt]);
  useEffect(() => {
    if (!dirty && !busy) return;
    const remove = beforeNavigate(async () => {
      setError(busy ? "正在保存，请稍后离开。" : "修改尚未保存，请保存草稿或撤销修改后离开。");
      throw new Error("showcase editor pending");
    });
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); };
    window.addEventListener("beforeunload", warn);
    return () => { remove(); window.removeEventListener("beforeunload", warn); };
  }, [dirty, busy]);

  async function action(kind: "save" | "publish" | "revoke") {
    if (!state || !effectiveDraft || busyRef.current) return;
    if (kind !== "revoke" && interestInput.error) { setError(interestInput.error); setTab("profile"); return; }
    busyRef.current = true; setBusy(true); setError(""); setNotice("");
    try {
      let next = state;
      if (kind === "save") next = await saveShowcase(effectiveDraft, state.revision);
      if (kind === "publish") {
        if (dirty) { setError("请先保存草稿，再发布当前版本。"); return; }
        next = await publishShowcase(state.revision);
      }
      if (kind === "revoke") next = await revokeShowcase();
      // Revocation changes visibility only; retain unsaved local edits.
      if (kind === "revoke") setState(next); else accept(next);
      setNotice(kind === "save" ? "草稿已保存，公开页面未改变" : kind === "publish" ? "主页已发布" : "主页已停止发布");
    } catch (err) { if (err instanceof ApiError && err.status === 409) setConflict(true); setError(`${kind === "save" ? "保存" : kind === "publish" ? "发布" : "停止发布"}失败：${apiErrorText(err)}`); }
    finally { busyRef.current = false; setBusy(false); }
  }
  function patch(values: Partial<ShowcaseConfig>) { setDraft(current => current ? { ...current, ...values } : current); setNotice(""); }
  function moveSection(index: number, offset: number) {
    if (!draft) return;
    const order = [...draft.sectionOrder];
    [order[index], order[index + offset]] = [order[index + offset]!, order[index]!];
    patch({ sectionOrder: order });
  }
  function moveWork(id: string, offset: number, kind: ShowcaseWork["kind"]) {
    if (!draft || !state) return;
    const inSection = state.availableWorks.filter(w => w.kind === kind && draft.selectedWorkIds.includes(w.id)).sort((a, b) => selectedWorkOrder(draft, a) - selectedWorkOrder(draft, b));
    const index = inSection.findIndex(w => w.id === id);
    const other = inSection[index + offset];
    if (!other) return;
    const ids = [...draft.selectedWorkIds];
    const a = ids.indexOf(id), b = ids.indexOf(other.id);
    [ids[a], ids[b]] = [ids[b]!, ids[a]!]; patch({ selectedWorkIds: ids });
  }

  if (loading) return <div className="showcase-loading" role="status"><Loader2 className="animate-spin" size={22} />正在加载主页</div>;
  if (!state || !draft || !effectiveDraft) return <div className="showcase-loading"><div role="alert"><Says content={errorMarkdown(error)} /></div><button onClick={() => setAttempt(n => n + 1)}>重新加载</button></div>;
  const picked = state.availableWorks.filter(w => effectiveDraft.selectedWorkIds.includes(w.id));
  const unavailableIds = draft.selectedWorkIds.filter(id => !state.availableWorks.some(work => work.id === id));
  const canPublish = !!draft.name.trim() && !!(draft.bio.trim() || draft.tagline.trim());

  return <div className="showcase-editor">
    <header className="showcase-toolbar">
      <div><p className="showcase-eyebrow">个人展示</p><h1>我的主页</h1><p className="showcase-status">{dirty ? "有未保存的修改" : state.published ? state.hasUnpublishedChanges ? "草稿已保存 · 待更新发布" : "已发布" : "仅自己可见"}</p></div>
      <div className="showcase-actions">
        {state.url && (state.published || state.hasLegacySite) && <a href={state.url} target="_blank" rel="noreferrer">查看公开页<ExternalLink size={14} /></a>}
        <button disabled={busy || !dirty || !!interestInput.error} onClick={() => void action("save")}>{busy ? "处理中" : "保存草稿"}</button>
        <button className="showcase-primary" disabled={busy || dirty || !!interestInput.error || !canPublish || (state.published && !state.hasUnpublishedChanges)} onClick={() => void action("publish")}>{state.published || state.hasLegacySite ? "更新发布" : "发布主页"}</button>
      </div>
    </header>
    {(error || notice) && <div className={`showcase-feedback ${error ? "is-error" : ""}`} role={error ? "alert" : "status"}>{error ? <Says content={errorMarkdown(error)} /> : <><Check size={16} />{notice}</>}</div>}
    {conflict && <div className="showcase-feedback"><span>当前修改已保留。</span><button className="underline" disabled={busy} onClick={() => {setAttempt(n => n + 1); setNotice("");}}>放弃本地修改，读取最新版本</button></div>}
    <div className="showcase-mobile-switch" aria-label="编辑或预览">{(["edit", "preview"] as const).map(v => <button key={v} aria-pressed={mobileView === v} onClick={() => setMobileView(v)}>{v === "edit" ? "编辑主页" : "查看预览"}</button>)}</div>
    <div className="showcase-workspace" data-mobile-view={mobileView}>
      <aside className="showcase-controls">
        <nav className="showcase-tabs" aria-label="主页设置">{([["design", "风格"], ["profile", "介绍"], ["works", "作品"]] as const).map(([id, label]) => <button key={id} aria-pressed={tab === id} onClick={() => setTab(id)}>{label}</button>)}</nav>
        <fieldset disabled={busy} className="showcase-fields">
          {tab === "design" && <>
            <div className="showcase-heading"><h2>选择喜欢的样子</h2><p>版式、色调和字体可以自由组合。</p></div>
            <div className="showcase-field-group"><h3>版式</h3><div className="showcase-layouts">{layouts.map(layout => <button key={layout.id} className={`showcase-layout ${draft.layout === layout.id ? "is-selected" : ""}`} aria-pressed={draft.layout === layout.id} onClick={() => patch({ layout: layout.id })}><span className={`showcase-layout-sketch sketch-${layout.id}`} aria-hidden="true"><i /><span>{layout.lines.map((w, i) => <b key={i} style={{ width: `${w}%` }} />)}</span></span><span><strong>{layout.label}</strong><small>{layout.detail}</small></span>{draft.layout === layout.id && <Check size={15} />}</button>)}</div></div>
            <div className="showcase-field-group"><h3>色调</h3><div className="showcase-palette-grid">{palettes.map(palette => <button key={palette.id} aria-pressed={draft.palette === palette.id} className={draft.palette === palette.id ? "is-selected" : ""} onClick={() => patch({ palette: palette.id })}><span className="showcase-swatches" aria-hidden="true">{[SHOWCASE_THEMES[palette.id].paper, SHOWCASE_THEMES[palette.id].ink, SHOWCASE_THEMES[palette.id].accent].map(color => <i key={color} style={{ background: color }} />)}</span>{palette.label}{draft.palette === palette.id && <Check size={12} />}</button>)}</div></div>
            <div className="showcase-field-group"><h3>字体</h3><div className="showcase-segments">{([["sans", "清晰"], ["serif", "书卷"], ["mono", "等宽"]] as const).map(([id, label]) => <button key={id} aria-pressed={draft.font === id} onClick={() => patch({ font: id })}>{label}</button>)}</div></div>
            <div className="showcase-field-group"><h3>写作展示</h3><div className="showcase-segments">{([["cards", "文章卡片"], ["list", "文章列表"]] as const).map(([id, label]) => <button key={id} aria-pressed={draft.writingStyle === id} onClick={() => patch({ writingStyle: id })}>{label}</button>)}</div></div>
            <div className="showcase-field-group"><h3>阅读展示</h3><div className="showcase-segments">{([["shelf", "阅读书架"], ["list", "阅读列表"]] as const).map(([id, label]) => <button key={id} aria-pressed={draft.readingStyle === id} onClick={() => patch({ readingStyle: id })}>{label}</button>)}</div></div>
          </>}
          {tab === "profile" && <>
            <div className="showcase-heading"><h2>关于你</h2><p>请填写愿意在主页上公开的介绍。</p></div>
            <label className="showcase-field">展示名称<input value={draft.name} maxLength={80} onChange={e => patch({ name: e.target.value })} placeholder="名字或昵称" /></label>
            <label className="showcase-field">开场介绍<GrowingTextarea value={draft.tagline} maxLength={200} onChange={e => patch({ tagline: e.target.value })} placeholder="例如：喜欢科幻，也喜欢把想象做成小作品。" /></label>
            <label className="showcase-field">个人简介<GrowingTextarea value={draft.bio} maxLength={2000} rows={4} onChange={e => patch({ bio: e.target.value })} placeholder="请介绍你的兴趣、正在探索的事情，或想分享的经历。" /></label>
            <label className="showcase-field">兴趣关键词<input value={interestText} onChange={e => {setInterestText(e.target.value); setNotice("");}} maxLength={500} placeholder="动漫、科幻、植物、摄影" /><small>用顿号或逗号分隔，最多 12 个。已填写 {interestInput.interests.length} 个。</small>{interestInput.error && <small role="alert" className="showcase-field-error">{interestInput.error}</small>}</label>
          </>}
          {tab === "works" && <>
            <div className="showcase-heading"><h2>选择展示内容</h2><p>写作与阅读从已发布的作品中选择。项目仅展示名称与简介。</p></div>
            {unavailableIds.length > 0 && <div className="showcase-editor-empty"><p>{unavailableIds.length} 件已选作品已停止发布或暂不可用，不会在主页展示。</p><button className="underline mt-2" onClick={() => patch({selectedWorkIds: draft.selectedWorkIds.filter(id => !unavailableIds.includes(id))})}>移除不可用作品</button></div>}
            {draft.sectionOrder.map((kind, index) => {
              const works = state.availableWorks.filter(w => w.kind === kind).sort((a, b) => {
                const ai = selectedWorkOrder(draft, a), bi = selectedWorkOrder(draft, b);
                return (ai < 0 ? Infinity : ai) - (bi < 0 ? Infinity : bi);
              });
              const selected = works.filter(w => draft.selectedWorkIds.includes(w.id));
              return <section className="showcase-work-group" key={kind}><header><h3>{sections[kind]}</h3><span>版块顺序</span><button aria-label={`上移${sections[kind]}版块`} disabled={index === 0} onClick={() => moveSection(index, -1)}><ArrowUp size={15} /></button><button aria-label={`下移${sections[kind]}版块`} disabled={index === draft.sectionOrder.length - 1} onClick={() => moveSection(index, 1)}><ArrowDown size={15} /></button></header>
                {works.length === 0 ? <p className="showcase-editor-empty">{kind === "project" ? "完成项目后，可在这里选择展示。" : `发布${kind === "writing" ? "文章" : "阅读成果"}后，可在这里选择展示。`}</p> : works.map(work => {
                  const checked = draft.selectedWorkIds.includes(work.id);
                  return <div className="showcase-work-option" key={work.id}><label><input type="checkbox" checked={checked} onChange={() => patch({ selectedWorkIds: checked ? draft.selectedWorkIds.filter(id => id !== work.id) : [...draft.selectedWorkIds, work.id] })} /><span><strong>{work.title}</strong>{work.summary && <small>{work.summary}</small>}</span></label>{checked && <div className="showcase-work-order"><button aria-label={`上移作品 ${work.title}`} disabled={selected[0]?.id === work.id} onClick={() => moveWork(work.id, -1, kind)}><ArrowUp size={13} /></button><button aria-label={`下移作品 ${work.title}`} disabled={selected[selected.length - 1]?.id === work.id} onClick={() => moveWork(work.id, 1, kind)}><ArrowDown size={13} /></button></div>}</div>;
                })}</section>;
            })}
          </>}
        </fieldset>
        <div className="showcase-publishing">
          {state.hasLegacySite && <p>原主页仍在公开展示。发布当前版本后，将使用这里的版式与内容；原项目记录保留。</p>}
          <p>保存草稿仅自己可见。发布后，持有链接的人可以查看主页；只展示你勾选的作品。</p>
          {!canPublish && <p>发布需要展示名称，以及开场介绍或个人简介。</p>}
          {dirty && <button disabled={busy} onClick={() => {accept(state); setError(""); setNotice("修改已撤销");}}>撤销未保存的修改</button>}
          {(state.published || state.hasLegacySite) && <div className="showcase-link-actions"><button disabled={busy} onClick={() => void navigator.clipboard.writeText(state.url).then(() => setNotice("公开链接已复制")).catch(() => setError("复制失败，请从公开页面复制地址"))}><Copy size={14} />复制链接</button><button disabled={busy} onClick={() => void action("revoke")}>停止发布</button></div>}
        </div>
      </aside>
      <section className="showcase-preview-area" aria-label="主页预览">
        <div className="showcase-preview-toolbar"><span><i />实时预览</span><div><button aria-label="宽屏预览" aria-pressed={!narrow} onClick={() => setNarrow(false)}><Monitor size={16} /></button><button aria-label="手机预览" aria-pressed={narrow} onClick={() => setNarrow(true)}><Smartphone size={16} /></button></div><span>{picked.length} 件作品</span></div>
        <div className={`showcase-preview-frame ${narrow ? "is-narrow" : ""}`}><Showcase config={effectiveDraft} works={state.availableWorks} narrow={narrow || undefined} editing /></div>
      </section>
    </div>
  </div>;
}
