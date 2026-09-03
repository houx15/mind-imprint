import { useCallback, useEffect, useState } from "react";
import { ExternalLink, Loader2, Plus, Trash2 } from "lucide-react";
import { Icon } from "@/ui";
import {
  SITE_REFS_WANTED,
  addSiteRef,
  deleteSiteRef,
  listSiteRefs,
  setSiteRefSaid,
  type SiteRef,
} from "../../../api/siteRefs";
import { apiErrorText } from "../../../api/errorText";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Sites —— 主页项目第二关：去看真的个人网站。
 *
 * 产品负责人 2026-09-03：「collect data about the websites they love. let them
 * search, and give AI the urls.」
 *
 * ## 这一屏为什么不是表单
 *
 * 她在这里**打的字只有网址**。搜索在她自己的浏览器里——我们没有比浏览器更好的
 * 搜索，假装有一个只会给她一个更差的。她粘一条，服务端真去读那一页，印记回一张
 * 卡：这一站在做什么、结构是什么、最狠的一处在哪。她的动作是挑、是贴、是删。
 *
 * 一屏最多一个输入位：底下那个网址框；「说一句」的框只在她点开某一张卡时出现，
 * 而且同时只有一张。见 docs/2026-09-03-website-project-owner-requirements.md 附录。
 *
 * ## 起点是六个真站，不是三张空壳
 *
 * 上来就给一个空输入框，等于让她凭空想「我喜欢哪个个人网站」——大多数中学生
 * 一个都想不起来，因为她从来没被带着看过。所以印记先动：六个真的个人站摆在
 * 那儿，点开就能看，看中了一键贴进来。
 *
 * 🚨 这里不渲染我们自己的三个版式当样例。样例需要内容，而这个产品里唯一合法的
 * 内容是**她自己的**——原型正是因为塞了一份示例内容（林知遥），让每个学生的页面
 * 都变成了别人的。宁可给六个真站的链接，也不造一个假学生。
 */

/** 起点。都是真的个人网站，spec §4 那六个。 */
const STARTERS: { url: string; note: string }[] = [
  { url: "https://lilianweng.github.io", note: "研究者的长文站" },
  { url: "https://www.5ime.cn", note: "密排的索引式首页" },
  { url: "https://d-d.design", note: "设计师的作品优先" },
  { url: "https://keyork.cn", note: "工整的技术博客" },
  { url: "https://terrifyzhao.github.io", note: "极简，只有文章列表" },
  { url: "https://www.lixiaolai.com", note: "长期更新的个人主页" },
];

export function Sites({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [refs, setRefs] = useState<SiteRef[]>([]);
  const [url, setUrl] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // 同时只给一张卡开「说一句」的框——一屏一个输入位。
  const [saying, setSaying] = useState<string | null>(null);
  const [said, setSaid] = useState("");

  const reload = useCallback(async () => {
    try {
      setRefs(await listSiteRefs(projectId));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const collect = useCallback(
    async (raw: string) => {
      const target = raw.trim();
      if (!target || busy) return;
      setBusy(true);
      setError(null);
      try {
        await addSiteRef(projectId, target);
        setUrl("");
        await reload();
      } catch (err) {
        setError(apiErrorText(err));
      } finally {
        setBusy(false);
      }
    },
    [busy, projectId, reload],
  );

  const left = Math.max(0, SITE_REFS_WANTED - refs.length);
  const taken = new Set(refs.map((r) => r.url.replace(/\/$/, "")));

  return (
    <ToolFrame
      title="站点采集"
      task="请找三个你真的喜欢的个人网站，把网址贴进来"
      why={tool.reason}
      // 真的闸：三个之前收不了工。服务端那边第二关也不算完。
      todo={left ? `还差 ${left} 个站` : ""}
      onFinish={() =>
        onFinish(
          { sites: refs.map((r) => ({ url: r.url, title: r.title })) },
          `她收了 ${refs.length} 个她喜欢的个人网站`,
        )
      }
      onClose={onClose}
      busy={busy}
    >
      <div className="flex h-full min-h-0 flex-col">
        {/* 进度。三个点，收一个亮一个。 */}
        <div className="flex items-center gap-2 border-b border-mk-border px-4 py-2.5">
          <span className="text-mk-label text-mk-secondary">已收集</span>
          <div className="flex items-center gap-1">
            {Array.from({ length: SITE_REFS_WANTED }).map((_, i) => (
              <span
                key={i}
                className="h-2 w-2 rounded-mk-full"
                style={{
                  background:
                    i < refs.length ? "var(--mk-accent-500)" : "var(--mk-border)",
                }}
              />
            ))}
          </div>
          <span className="text-mk-small text-mk-muted">
            {refs.length} / {SITE_REFS_WANTED}
          </span>
        </div>

        {error ? (
          <p className="px-4 pt-3 text-mk-small" style={{ color: "var(--mk-danger)" }}>
            {error}
          </p>
        ) : null}

        <div className="min-h-0 flex-1 overflow-auto px-4 py-3">
          {/* 印记先动：六个真站摆在这儿，点开就能看。 */}
          <h3 className="text-mk-label text-mk-secondary">可以从这几个开始</h3>
          <p className="mt-1 text-mk-small text-mk-muted">
            请点开看一看，喜欢的直接收进来。也可以自己去搜，把网址贴到下面。
          </p>
          <div className="mt-2 flex flex-wrap gap-2">
            {STARTERS.filter((s) => !taken.has(s.url.replace(/\/$/, ""))).map((s) => (
              <span
                key={s.url}
                className="flex items-center gap-1.5 rounded-mk-full border border-mk-border px-2.5 py-1"
              >
                <a
                  href={s.url}
                  target="_blank"
                  rel="noreferrer"
                  className="flex items-center gap-1 text-mk-small text-mk-secondary"
                >
                  {s.note}
                  <Icon icon={ExternalLink} size={12} />
                </a>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => void collect(s.url)}
                  className="text-mk-small font-semibold text-mk-accent-600 disabled:opacity-40"
                >
                  收进来
                </button>
              </span>
            ))}
          </div>

          {/* 收到的每一站：印记读完之后的三句。 */}
          <div className="mt-5 space-y-3">
            {refs.map((r) => (
              <div key={r.id} className="rounded-mk-lg border border-mk-border p-3">
                <div className="flex items-start justify-between gap-2">
                  <a
                    href={r.url}
                    target="_blank"
                    rel="noreferrer"
                    className="flex items-center gap-1 text-mk-body font-semibold text-mk-ink"
                  >
                    {r.title || r.url}
                    <Icon icon={ExternalLink} size={13} />
                  </a>
                  <button
                    type="button"
                    onClick={async () => {
                      await deleteSiteRef(projectId, r.id).catch((e) =>
                        setError(apiErrorText(e)),
                      );
                      await reload();
                    }}
                    aria-label="拿掉这一站"
                    className="rounded-mk-full p-1 text-mk-faint hover:text-mk-secondary"
                  >
                    <Icon icon={Trash2} size={14} />
                  </button>
                </div>
                <dl className="mt-2 space-y-1.5">
                  {[
                    ["在做什么", r.what],
                    ["结构", r.structure],
                    ["最值得学", r.best],
                  ].map(([k, v]) => (
                    <div key={k}>
                      <dt className="text-mk-label text-mk-faint">{k}</dt>
                      <dd className="text-mk-small text-mk-secondary">{v}</dd>
                    </div>
                  ))}
                </dl>

                {r.sheSaid ? (
                  <p className="mt-2 rounded-mk-md px-2.5 py-1.5 text-mk-small text-mk-secondary"
                     style={{ background: "var(--mk-paper)" }}>
                    你说：{r.sheSaid}
                  </p>
                ) : null}

                {saying === r.id ? (
                  <div className="mt-2 flex gap-2">
                    <input
                      autoFocus
                      value={said}
                      onChange={(e) => setSaid(e.target.value)}
                      placeholder="你喜欢它哪一点"
                      className="min-w-0 flex-1 rounded-mk-md border border-mk-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none focus:border-mk-accent-200"
                    />
                    <button
                      type="button"
                      onClick={async () => {
                        await setSiteRefSaid(projectId, r.id, said).catch((e) =>
                          setError(apiErrorText(e)),
                        );
                        setSaying(null);
                        setSaid("");
                        await reload();
                      }}
                      className="rounded-mk-full px-3 py-1.5 text-mk-small font-semibold text-white"
                      style={{ background: "var(--mk-accent-500)" }}
                    >
                      记下
                    </button>
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={() => {
                      setSaying(r.id);
                      setSaid(r.sheSaid);
                    }}
                    className="mt-2 text-mk-small text-mk-accent-600"
                  >
                    {r.sheSaid ? "改一句" : "说一句"}
                  </button>
                )}
              </div>
            ))}
          </div>
        </div>

        {/* 唯一那个输入位。 */}
        <div className="flex gap-2 border-t border-mk-border px-4 py-3">
          <input
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void collect(url);
            }}
            placeholder="粘一个网址"
            className="min-w-0 flex-1 rounded-mk-full border border-mk-border bg-mk-surface px-3.5 py-2 text-mk-body text-mk-ink outline-none focus:border-mk-accent-200"
          />
          <button
            type="button"
            disabled={busy || !url.trim()}
            onClick={() => void collect(url)}
            className="flex items-center gap-1.5 rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
            style={{ background: "var(--mk-accent-500)" }}
          >
            <Icon icon={busy ? Loader2 : Plus} size={15} />
            {busy ? "读取中" : "加入"}
          </button>
        </div>
      </div>
    </ToolFrame>
  );
}
