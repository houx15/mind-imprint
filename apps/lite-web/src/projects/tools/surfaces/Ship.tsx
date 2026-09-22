import { ProcessComparison } from "../../../site/ProcessComparison";
import { Says, errorMarkdown } from "../../Says";
import { useCallback, useEffect, useState } from "react";
import { Check, Copy, Link2, Monitor, RefreshCw, Smartphone } from "lucide-react";
import QRCode from "qrcode";
import { Icon } from "@/ui";
import { getSite, publishSite, revokeSite, type SiteState } from "../../../api/site";
import { listCodeVersions, codePreviewURL, type CodeVersion } from "../../../api/codeVersions";
import { getCreativeDirection } from "../../../api/creativeDirection";
import { apiErrorText } from "../../../api/errorText";
import { BuiltSite } from "../../../site/BuiltSite";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

// Publish a reviewed version; subsequent drafts remain private until explicitly published.
export function Ship({ tool, onFinish, onClose }: ToolSurfaceProps) {
  const [state, setState] = useState<SiteState | null>(null);
  const [versions,setVersions]=useState<CodeVersion[]>([]);
  const [selected,setSelected]=useState("");
  const [versionError,setVersionError]=useState("");
  const [narrow, setNarrow] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [qr, setQr] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const load = useCallback(async () => {
    try {
      const site=await getSite(); setState(site);
      if(site.projectId) {
       try {
        const rows=await listCodeVersions(site.projectId); setVersions(rows??[]);setVersionError("");
        if(rows?.length) {
         const direction=await getCreativeDirection(site.projectId);
         setSelected(previous=>rows.some(v=>v.id===previous)?previous:direction.document.trial?.versionId||site.publishedVersionId||rows[0]!.id);
        }
       } catch(err) {setVersionError(apiErrorText(err));}
      }
    } catch (err) {
      setError(apiErrorText(err));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const act = useCallback(
    async (fn: () => Promise<unknown>) => {
      setBusy(true);
      setError(null);
      try {
        await fn();
        setState(await getSite());
      } catch (err) {
        setError(apiErrorText(err));
      } finally {
        setBusy(false);
      }
    },
    [],
  );

  // 二维码跟着链接走：上线了就有，撤回了就没有。
  //
  // 🚨 画不出来不算失败。二维码是这条链接的另一种给法，不是链接本身——画图失败
  // 时她仍然拿得到那一行地址，所以这里不报错、不挡上线，只是不显示那张图。
  // 和 SharePanel 对分享报告的处理是同一条。
  const shareUrl = state?.published ? state.url : "";
  useEffect(() => {
    if (!shareUrl) {
      setQr(null);
      return;
    }
    let cancelled = false;
    QRCode.toDataURL(shareUrl)
      .then((img) => {
        if (!cancelled) setQr(img);
      })
      .catch(() => {
        if (!cancelled) setQr(null);
      });
    return () => {
      cancelled = true;
    };
  }, [shareUrl]);

  // 撤回之后再上线拿到的是同一条链接，但「已复制」是上一次的事，得清掉。
  useEffect(() => {
    setCopied(false);
  }, [shareUrl]);

  const copyLink = useCallback(async () => {
    if (!shareUrl) return;
    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      // 第 8 条：动词 + 失败，再接后台原话。她和我们看到的是同一句。
      setError(`复制失败：${err instanceof Error ? err.message : String(err)}`);
    }
  }, [shareUrl]);

  if (!state) {
    return (
      <ToolFrame
        title="上线"
        task="请预览并检查主页，确认后发布"
        why={tool.reason}
        todo="处理中"
        onFinish={() => onFinish({}, "")}
        onClose={onClose}
      >
        <div className="p-4">
        {error ? (
            <div className="text-mk-small" style={{ color: "var(--mk-danger)" }}><Says content={errorMarkdown(error)} /></div>
          ) : null}
        </div>
      </ToolFrame>
    );
  }

  const missing = versionError ? ["作品版本读取失败，请刷新预览"] : selected ? versions.find(v=>v.id===selected)?.publicationMissing ?? ["正在读取版本发布条件"] : state.publishMissing ?? state.missing ?? [];
  const selectedPublished=state.published&&(!selected||selected===state.publishedVersionId);
  return (
    <ToolFrame
      title="上线"
      task={selectedPublished ? "所选版本已发布，请确认无误" : "请检查所选版本，确认后发布"}
      why={tool.reason}
      // 还缺她自己的字的时候，「完成」不给按——服务端也拦着（发布会被拒），
      // 这里只是把话说在前面。
      todo={missing.length ? `${missing.length} 项待完成` : !selectedPublished ? "所选版本尚未发布" : ""}
      onFinish={() =>
        onFinish(
          { published: selectedPublished, url: selectedPublished?state.url:"" },
          selectedPublished ? `主页所选版本已上线：${state.url}` : "所选版本尚未发布",
        )
      }
      onClose={onClose}
      busy={busy}
    >
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex flex-wrap items-center gap-2 border-b border-mk-border px-4 py-2.5">
          <span className="text-mk-label text-mk-secondary">
            {selectedPublished ? "所选版本已上线" : "所选版本待上线"}
          </span>
          <div className="flex-1" />
          <button
            type="button"
            onClick={() => setNarrow(false)}
            aria-label="宽屏预览"
            className="rounded-mk-full p-1.5"
            style={{ color: narrow ? "var(--mk-faint)" : "var(--mk-accent-500)" }}
          >
            <Icon icon={Monitor} size={16} />
          </button>
          <button
            type="button"
            onClick={() => setNarrow(true)}
            aria-label="手机预览"
            className="rounded-mk-full p-1.5"
            style={{ color: narrow ? "var(--mk-accent-500)" : "var(--mk-faint)" }}
          >
            <Icon icon={Smartphone} size={16} />
          </button>
          <button
            type="button"
            onClick={() => void load()}
            aria-label="刷新预览"
            className="rounded-mk-full p-1.5 text-mk-faint hover:text-mk-secondary"
          >
            <Icon icon={RefreshCw} size={16} />
          </button>
        </div>

          {versions.length>0&&<section className="space-y-2 border-b border-mk-border p-4"><label className="block text-mk-small">发布版本<select value={selected} disabled={busy} onChange={e=>setSelected(e.target.value)} className="ml-3 rounded border border-mk-border p-2">{versions.map(v=><option key={v.id} value={v.id}>{new Date(v.created_at).toLocaleString()}{v.id===state.publishedVersionId?" · 当前发布版本":""}</option>)}</select></label><p className="text-mk-small text-mk-secondary">请核对所选版本的内容与交互。发布需要有该版本的试用判断；可在创作构思中查看或补充。后续草稿修改不会自动更新公开页面。</p></section>}
        {versionError&&<div role="alert" className="p-4 text-mk-danger"><Says content={errorMarkdown(`读取作品版本失败：${versionError}`)} /></div>}
        {error ? (
          <div className="px-4 pt-3 text-mk-small" style={{ color: "var(--mk-danger)" }}><Says content={errorMarkdown(error)} /></div>
        ) : null}

        {missing.length ? (
          <div className="mx-4 mt-3 rounded-mk-lg border border-mk-border p-3">
            <h3 className="text-mk-label text-mk-secondary">上线前需要完成</h3>
            <ul className="mt-1.5 space-y-1">
              {missing.map((m) => (
                <li key={m} className="text-mk-small text-mk-secondary">
                  · {m}
                </li>
              ))}
            </ul>
            {/* 🚨 不给输入框。缺的那几处回对话里说给印记，它摆上去——这一屏
                是判断的地方，不是填空的地方。 */}
            <p className="mt-2 text-mk-small text-mk-muted">
              {selected?"请返回创作构思，将已保存内容加入选定版本，并记录试用判断。":"请补充主页内容并确认视觉方案。"}
            </p>
          </div>
        ) : null}

        <div className="min-h-0 flex-1 overflow-auto p-4">
          <div
            className="mx-auto overflow-hidden rounded-mk-lg border border-mk-border"
            style={{ width: narrow ? 390 : "100%", maxWidth: "100%" }}
          >
            {/* 🚨 narrow 要传下去。它是 prop 而不是媒体查询：`md:` 读到的是
                真实视口，会把 390px 的手机框排成桌面版——恰好在她检查手机效果
                的那一刻排错。见 BuiltSite 的文件头。 */}
            {selected?<iframe title="待发布主页预览" src={codePreviewURL(state.projectId,selected)} sandbox="allow-scripts" referrerPolicy="no-referrer" allow="camera 'none'; microphone 'none'; geolocation 'none'; payment 'none'" className="h-[650px] w-full border-0"/>:<BuiltSite
              site={state.content}
              layout={state.layout}
              palette={state.palette}
              heroUrl={state.heroUrl}
              narrow={narrow}
              editing
            />}
            {selected&&versions.find(v=>v.id===selected)?.comparison&&<ProcessComparison comparison={versions.find(v=>v.id===selected)!.comparison!} source={codePreviewURL(state.projectId,selected)}/>}
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-3 border-t border-mk-border px-4 py-3">
          {state.published ? (
            <>
              {/* 🚨 二维码底色写死白色，不跟主题走。深色模式下把黑白反过来，
                  多数手机的相机就认不出来了——而这张图存在的唯一理由是被扫。 */}
              {qr ? (
                <img
                  src={qr}
                  alt="二维码"
                  width={76}
                  height={76}
                  className="shrink-0 border border-mk-border"
                  style={{ background: "#fff", borderRadius: 6, padding: 3 }}
                />
              ) : null}
              <div className="flex min-w-0 flex-col items-start gap-1">
                <a
                  href={state.url}
                  target="_blank"
                  rel="noreferrer"
                  className="flex max-w-full items-center gap-1.5 text-mk-small text-mk-accent-600"
                >
                  <Icon icon={Link2} size={14} />
                  <span className="truncate">{state.url}</span>
                </a>
                <button
                  type="button"
                  onClick={() => void copyLink()}
                  className="flex items-center gap-1.5 text-mk-small text-mk-muted"
                >
                  <Icon icon={copied ? Check : Copy} size={13} />
                  {copied ? "已复制" : "复制链接"}
                </button>
              </div>
              <div className="flex-1" />
              <button
                type="button"
                disabled={busy}
                onClick={() => void act(revokeSite)}
                className="rounded-mk-full border border-mk-border px-4 py-2 text-mk-body text-mk-secondary disabled:opacity-40"
              >
                撤回链接
              </button>
              {selected&&selected!==state.publishedVersionId&&<button disabled={busy||missing.length>0} onClick={()=>void act(()=>publishSite(selected))} className="rounded-full border border-mk-border px-4 py-2 disabled:opacity-40">更新为所选版本</button>}
            </>
          ) : (
            <>
              <span className="text-mk-small text-mk-muted">
                链接不可索引，只有拿到它的人打得开。随时可以撤回。
              </span>
              <div className="flex-1" />
              <button
                type="button"
                disabled={busy || missing.length > 0}
                onClick={() => void act(()=>publishSite(selected||undefined))}
                className="flex items-center gap-1.5 rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
                style={{ background: "var(--mk-accent-500)" }}
              >
                <Icon icon={Check} size={15} />
                上线
              </button>
            </>
          )}
        </div>
      </div>
    </ToolFrame>
  );
}
