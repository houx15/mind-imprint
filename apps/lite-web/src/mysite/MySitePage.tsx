import makers from "../home/assets/project-makers-v2.webp";
import { useEffect, useState } from "react";
import { ArrowRight, ExternalLink, Loader2, PencilLine } from "lucide-react";
import { apiErrorText } from "../api/errorText";
import { getSite, startSiteProject, type PublishedWork, type SiteState } from "../api/site";
import { navigate, projectPath } from "../routing";
import { BuiltSite } from "../site/BuiltSite";

/**
 * 她公开出去的那些作品，列在她自己这一页的底下。
 *
 * 这一块回答的是「我公开了哪些东西」。在这之前那件事只能靠一篇一篇打开报告去
 * 看分享面板 —— 一个她管不住的状态，等于一个她不知道自己有没有公开的状态。
 *
 * **发布/停止发布不在这里做。** 那一颗按钮属于作品自己的分享面板
 * （`SharePanel`），这里只给一条通往它的路。两个地方各存一份「现在到底公开
 * 没公开」的状态机，早晚会说两句不一样的话，而这件事上说错话的代价是隐私。
 *
 * 一件都没有就整块不渲染：一个写着「暂无」的空标题比没有这一块糟。
 */
function PublishedWorks({ works }: { works: PublishedWork[] }) {
  if (works.length === 0) return null;
  return (
    <section className="border-t border-mk-line px-7 py-6">
      <h2 className="text-mk-label text-mk-faint">已发布的作品</h2>
      <ul className="mt-3 flex flex-col gap-2">
        {works.map((w) => (
          <li key={w.publicPath}>
            <a
              href={w.publicPath}
              className="flex items-baseline gap-3 text-mk-body text-mk-ink hover:text-mk-accent-700"
            >
              <span className="shrink-0 text-mk-small text-mk-muted">{w.kind}</span>
              <span className="min-w-0 truncate">{w.title}</span>
              <ExternalLink size={13} className="shrink-0 text-mk-faint" />
            </a>
          </li>
        ))}
      </ul>
    </section>
  );
}

/**
 * `/site` —— 她自己的主页，她自己那一面。
 *
 * 🚨 **住在 `mysite/` 而不是 `site/`。** `site/` 里的每个文件都受
 * `site/no-ui-kit.test.ts` 那条规矩管：不许 import 产品的 UI kit、不许用一个
 * `mk-*` token —— 她的网站不许长得像做出它的那个产品。而这一页是产品的一格
 * （导航、工具栏、按钮全是产品的），它只是把 `BuiltSite` 嵌在中间。放进
 * `site/` 会逼着这条规矩为它开一个例外，而那条规矩正是靠没有例外才管用的。
 *
 * # 为什么它该是导航上的一格
 *
 * 主页发布之后项目进 keeping，她随时能回来改（PBL spec §4）。但在这之前，回到
 * 那一页的路只有一条：项目室 → 主页项目 → 侧栏那一行「我的主页」。也就是说，
 * 她**必须先想起那是一个项目**，才找得到自己的主页。
 *
 * `/p/:token` 是给别人看的那一面，不是入口 —— 它没有导航、没有任何回到产品里的
 * 路（那是有意的，见 `PublicSitePage`）。
 *
 * # 这一页只做两件事
 *
 * 把她的主页照原样摆出来（和访客看到的是同一个 `BuiltSite`），再给两条路：
 * 回项目里改，或者把公开链接复制走。**改的入口仍然在项目室**，不在这里 ——
 * 页面上每一句都必须是她的原话，所以那件事发生在她和印记的对话里。
 */
export function MySitePage() {
  const [site, setSite] = useState<SiteState | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [opening, setOpening] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let alive = true;
    getSite()
      .then((s) => {
        if (alive) setSite(s);
      })
      // 还没有主页项目时服务端就是 404。那不是错误，是「还没开始」，所以这里
      // 不显示报错，落到下面那个空状态。
      .catch(() => {
        if (alive) setSite(null);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center bg-mk-paper">
        <Loader2 size={20} className="animate-spin text-mk-muted" />
      </div>
    );
  }

  if (!site) {
    return (
      <div className="flex h-full items-center justify-center bg-mk-paper px-8">
        <div className="student-site-empty w-full max-w-[560px] text-center">
          <img src={makers} alt="" className="mx-auto mb-6 h-44 w-64 object-contain" />
          <h1 className="text-mk-display text-mk-ink">创建个人主页</h1>
          <p className="mt-4 text-mk-body text-mk-secondary">
            展示你的学习成果、兴趣与项目。印记会引导你完成主页内容。
          </p>
          <button
            type="button"
            disabled={opening}
            onClick={async () => {
              setOpening(true);
              setError("");
              try {
                const p = await startSiteProject();
                navigate(projectPath(p.id));
              } catch (err) {
                setError(apiErrorText(err));
                setOpening(false);
              }
            }}
            className="mt-8 inline-flex items-center gap-1.5 rounded-mk-full px-5 py-2.5 text-mk-body font-semibold text-white disabled:opacity-40"
            style={{ background: "var(--mk-accent-500)" }}
          >
            {opening ? "处理中" : "开始做我的主页"}
            <ArrowRight size={15} />
          </button>
          {error ? (
            <p className="mt-4 text-mk-small" style={{ color: "var(--mk-danger)" }}>
              {error}
            </p>
          ) : null}
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-full flex-col bg-mk-paper">
      {/* 一条工具栏，不是一个横幅。她来这一格是看自己的主页，不是看我们的界面。 */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-mk-line px-7 py-3">
        <div className="min-w-0">
          <p className="text-mk-body font-semibold text-mk-ink">我的主页</p>
          <p className="text-mk-small text-mk-muted">
            {site.published ? "已上线" : site.missing.length > 0
              ? `还缺 ${site.missing.length} 处你自己的话`
              : "待上线"}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {site.published && site.url ? (
            <button
              type="button"
              onClick={() => {
                void navigator.clipboard
                  .writeText(site.url)
                  .then(() => setCopied(true))
                  .catch(() => setError("复制失败"));
              }}
              className="inline-flex items-center gap-1.5 rounded-mk-full border border-mk-line px-4 py-1.5 text-mk-small text-mk-secondary transition-colors hover:bg-mk-hover"
            >
              <ExternalLink size={14} />
              {copied ? "已复制" : "复制公开链接"}
            </button>
          ) : null}
          <button
            type="button"
            onClick={() => navigate(projectPath(site.projectId))}
            className="inline-flex items-center gap-1.5 rounded-mk-full px-4 py-1.5 text-mk-small font-semibold text-white"
            style={{ background: "var(--mk-accent-500)" }}
          >
            <PencilLine size={14} />
            回项目里改
          </button>
        </div>
      </div>

      {error ? (
        <p className="px-7 pt-3 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      ) : null}

      <div className="min-h-0 flex-1 overflow-y-auto">
        <BuiltSite
          site={site.content}
          layout={site.layout}
          palette={site.palette}
          heroUrl={site.heroUrl}
        />
        <PublishedWorks works={site.works ?? []} />
      </div>
    </div>
  );
}
