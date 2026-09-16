import { Says, errorMarkdown } from "../../Says";
import { useCallback, useEffect, useState } from "react";
import { ExternalLink, Loader2, Plus, Trash2 } from "lucide-react";
import { Icon } from "@/ui";
import { addSiteRef, bookmarkSiteRef, deleteSiteRef, listSiteRefs, setSiteRefSaid, type SiteRef } from "../../../api/siteRefs";
import { apiErrorText } from "../../../api/errorText";
import { GrowingTextarea } from "../../../shared/GrowingTextarea";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

// Optional inspiration: collecting a link never asserts that its contents were seen.
export function Sites({ projectId, onFinish, onClose }: ToolSurfaceProps) {
  const [refs, setRefs] = useState<SiteRef[]>([]);
  const [url, setUrl] = useState("");
  const [searchTerms, setSearchTerms] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saying, setSaying] = useState<string | null>(null);
  const [said, setSaid] = useState("");
  const reload = useCallback(async () => setRefs(await listSiteRefs(projectId)), [projectId]);
  useEffect(() => { void reload().catch(err => setError(apiErrorText(err))); }, [reload]);
  const run = async (action: () => Promise<void>) => {
    if (busy) return;
    setBusy(true); setError(null);
    try { await action(); } catch (err) { setError(apiErrorText(err)); }
    finally { setBusy(false); }
  };
  const saveJudgment = async () => {
    if (!saying) return;
    const saved = await setSiteRefSaid(projectId, saying, said);
    setRefs(current => current.map(ref => ref.id === saved.id ? saved : ref));
    setSaying(null); setSaid("");
  };
  const collect = () => void run(async () => {
    if (!url.trim()) return;
    if (saying) await saveJudgment();
    const saved = await bookmarkSiteRef(projectId, url.trim());
    setUrl(""); await reload(); setSaying(saved.id); setSaid(saved.sheSaid);
  });
  return <ToolFrame title="灵感采集" task="请探索感兴趣的画面或互动效果，记录对你的启发"
    why="观察真实效果可以帮助你把意象变成具体设计。这个任务可选，不限制参考数量或网站类型。"
    todo="" busy={busy}
    onClose={() => void run(async () => { if (saying) await saveJudgment(); onClose(); })}
    onFinish={() => void run(async () => {
      if (saying) await saveJudgment();
      const current = await listSiteRefs(projectId);
      onFinish({sites: current.map(r => ({url:r.url,title:r.title,judgment:r.sheSaid}))}, `已收藏 ${current.length} 个灵感链接，记录 ${current.filter(r => r.sheSaid.trim()).length} 条学生观察或设计取舍。收藏不代表已浏览或试用；请区分待探索、学生观察与AI正文分析。`);
    })}>
    <div className="flex h-full min-h-0 flex-col">
      <div className="space-y-3 border-b border-mk-border px-4 py-3">
        <p className="text-mk-small text-mk-secondary">可选任务 · 已收藏 {refs.length} 个参考</p>
        <p className="text-mk-small text-mk-muted">请搜索想了解的效果，例如植物生长动画、宇宙场景或鼠标互动。参考可以来自任何类型的网站。</p>
        <div className="flex gap-2">
          <input aria-label="灵感搜索词" value={searchTerms} onChange={e=>setSearchTerms(e.target.value)} placeholder="请输入意象或效果" className="min-w-0 flex-1 rounded-mk-lg border border-mk-border px-3 py-2" />
          <a href={`https://www.baidu.com/s?wd=${encodeURIComponent(searchTerms.trim() || "网页 互动效果 灵感")}`} target="_blank" rel="noreferrer" className="rounded-mk-full border border-mk-border px-3 py-2 text-mk-small">搜索灵感</a>
        </div>
        <p className="text-mk-small text-mk-muted">收藏链接不需要 AI 读懂页面。请亲自体验动画或互动，再记录看见了什么、想用在哪里；还没看过请注明待探索。</p>
      </div>
      {error && <div role="alert" className="px-4 pt-3 text-mk-small text-mk-danger"><Says content={errorMarkdown(error)} /></div>}
      <div className="min-h-0 flex-1 overflow-auto px-4 py-3 space-y-4">
        {refs.map(r => <section key={r.id} className="rounded-mk-lg border border-mk-border p-4 space-y-3">
          <div className="flex items-start justify-between gap-2">
            <a href={r.url} target="_blank" rel="noreferrer" className="flex min-w-0 items-center gap-1 break-all font-semibold">{r.title || r.url}<Icon icon={ExternalLink} size={13}/></a>
            <button disabled={busy} aria-label="移除参考" onClick={() => void run(async () => { await deleteSiteRef(projectId,r.id); if(saying === r.id){setSaying(null);setSaid("");} await reload(); })} className="shrink-0 p-1"><Icon icon={Trash2} size={14}/></button>
          </div>
          {r.sheSaid && saying !== r.id && <p className="rounded-mk-md bg-mk-paper p-3 text-mk-small whitespace-pre-wrap">我的观察与取舍：{r.sheSaid}</p>}
          {saying === r.id ? <div className="space-y-2">
            <label className="block text-mk-small">我的观察与设计取舍<GrowingTextarea autoFocus value={said} onChange={e=>setSaid(e.target.value)} placeholder="实际看见或操作了什么？准备怎样用在自己的设计里？未体验的部分请注明。" className="mt-2 w-full rounded-mk-md border border-mk-border bg-mk-surface p-3"/></label>
            <button disabled={busy} className="rounded-mk-full bg-mk-accent-500 px-3 py-2 text-mk-small text-white" onClick={()=>void run(saveJudgment)}>保存判断</button>
          </div> : <button disabled={busy} className="text-mk-small text-mk-accent-600" onClick={()=>void run(async()=>{if(saying)await saveJudgment();setSaying(r.id);setSaid(r.sheSaid);})}>{r.sheSaid ? "修改判断" : "记录观察与取舍"}</button>}
          <details className="border-t border-mk-border pt-3">
            <summary className="cursor-pointer text-mk-small">AI 正文分析（可选）</summary>
            <p className="my-2 text-mk-small text-mk-muted">读取正文不等于看见画面或操作效果。没有可读文字时，仍可以保留链接和自己的观察。</p>
            {r.what || r.structure || r.best ? <dl className="space-y-2">{[["材料内容",r.what],["正文中的组织或步骤",r.structure],["可供考虑的做法",r.best]].map(([label,value])=><div key={label}><dt className="text-mk-label text-mk-faint">{label}</dt><dd className="text-mk-small">{value}</dd></div>)}</dl> : <p className="text-mk-small">尚未分析正文。</p>}
            <button disabled={busy} className="mt-3 text-mk-small underline" onClick={()=>void run(async()=>{await addSiteRef(projectId,r.url);await reload();})}>{r.what ? "重新分析正文" : "请 AI 分析正文"}</button>
          </details>
        </section>)}
      </div>
      <div className="flex gap-2 border-t border-mk-border px-4 py-3">
        <input aria-label="灵感网址" value={url} onChange={e=>setUrl(e.target.value)} onKeyDown={e=>{if(e.key==="Enter")collect();}} placeholder="粘贴灵感页面的网址" className="min-w-0 flex-1 rounded-mk-full border border-mk-border bg-mk-surface px-3.5 py-2"/>
        <button disabled={busy || !url.trim()} onClick={collect} className="flex items-center gap-1.5 rounded-mk-full bg-mk-accent-500 px-4 py-2 font-semibold text-white disabled:opacity-40"><Icon icon={busy ? Loader2 : Plus} size={15}/>{busy ? "处理中" : "收藏链接"}</button>
      </div>
    </div>
  </ToolFrame>;
}
