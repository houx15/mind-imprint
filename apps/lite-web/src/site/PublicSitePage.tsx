import { Showcase } from "./Showcase";
import type { ShowcaseConfig, ShowcaseWork } from "./showcaseTypes";
import { ProcessComparison, type ComparisonText } from "./ProcessComparison";
import { useEffect, useState } from "react";
import { getPublicSite, publicCodeSiteURL, type PublishedWork } from "../api/site";
import { useNoIndex } from "../shared/useNoIndex";
import { BuiltSite } from "./BuiltSite";
import { themeFor } from "./themes";
import type { SiteContent, SiteLayout, SitePalette } from "./types";

/**
 * `/p/:token` — 她的主页，访客看到的那一面。
 *
 * 打开这个页面的人没有账号、没有 session：她把链接发给了家人或朋友。所以它不
 * 经过 `LiteApp`，由 `rootElementFor.tsx` 直接挂载。
 *
 * 🚨 这一页上没有任何属于这个产品的东西：没有导航、没有「回到思维印记」的按钮、
 * 没有登录入口、没有横幅。她把链接发出去是让人看她做的东西，不是让人看我们。
 * 页脚那一行「用 思维印记 搭建」是版式自己的一部分，和真实个人站上的
 * 「Powered by Hexo」是同一种东西——一行小字，不是一个入口。
 *
 * 🚨 不可索引。服务端已经发了 `X-Robots-Tag: noindex`；这里再挂一个 meta，因为
 * 两者在不同的层，而她是未成年人，这条链接是给人的，不是给搜索引擎的。
 */
export function PublicSitePage({ token }: { token: string }) {
  const [state, setState] = useState<
    { kind: "loading" } | { kind: "showcase"; config: ShowcaseConfig; works: ShowcaseWork[] } | { kind: "generated"; renderKey:string; comparison?:ComparisonText|null; works: PublishedWork[] } | { kind: "ok"; layout: SiteLayout; palette: SitePalette; heroUrl: string; content: SiteContent; works: PublishedWork[] } | { kind: "gone" }
  >({ kind: "loading" });

  // noindex 在挂载时加上，卸载时收回。它只属于这一个页面。
  useNoIndex();

  useEffect(() => {
    let cancelled = false;
    getPublicSite(token)
      .then((res) => {
        if (cancelled) return;
        if ("showcase" in res && res.showcase) { setState({kind: "showcase", config: res.config, works: res.works}); document.title = res.config.name || "个人主页"; return; }
        if ("showcase" in res) return;
        if (res.generated) { setState({kind:"generated",renderKey:res.renderKey,comparison:res.comparison,works:res.works ?? []}); document.title="个人主页"; return; }
        setState({ kind: "ok", layout: res.layout, palette: res.palette, heroUrl: res.heroUrl, content: res.content, works: res.works ?? [] });
        if (res.content.name) document.title = res.content.name;
      })
      // 撤销过的链接和从来不存在的链接，在服务端就是同一个 404；这里也必须是
      // 同一句话。「这个页面已被作者收回」会告诉陌生人这里曾经有过东西。
      .catch(() => {
        if (!cancelled) setState({ kind: "gone" });
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  if (state.kind === "loading") {
    // 空白，不是转圈：页面自己带底色，闪一个加载态反而更像坏了。
    return <div style={{ minHeight: "100vh", background: themeFor("essay").paper }} />;
  }

  if (state.kind === "gone") {
    return (
      <div
        style={{
          minHeight: "100vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: "#F4F0E6",
          color: "#23201C",
          fontFamily: '"Noto Serif SC","Songti SC",Georgia,serif',
          padding: 24,
          textAlign: "center",
        }}
      >
        <p style={{ fontSize: 16, lineHeight: 1.9 }}>这个地址上没有页面。</p>
      </div>
    );
  }

  if (state.kind === "showcase") return <Showcase config={state.config} works={state.works} />;

  if(state.kind==="generated") return <>{state.comparison&&<nav aria-label="主页内容" style={{padding:"12px 24px",background:"#f4f0e6",display:"flex",gap:24}}><a href="#site-work">作品</a><a href="#process-comparison">修改过程</a></nav>}<iframe id="site-work" title="个人主页" src={`${publicCodeSiteURL(token)}?publication=${encodeURIComponent(state.renderKey)}`} sandbox="allow-scripts" referrerPolicy="no-referrer" allow="camera 'none'; microphone 'none'; geolocation 'none'; payment 'none'" style={{display:"block",width:"100%",height:"100dvh",border:0}}/><PublishedWorks works={state.works}/>{state.comparison&&<ProcessComparison comparison={state.comparison} source={`${publicCodeSiteURL(token)}?publication=${encodeURIComponent(state.renderKey)}`}/>}</>;

  return (
    <>
      <BuiltSite
        site={state.content}
        layout={state.layout}
        palette={state.palette}
        heroUrl={state.heroUrl}
        editing={false}
      />
      <PublishedWorks works={state.works} />
    </>
  );
}

/**
 * 她发布过的作品，每一条点得开。
 *
 * 🚨 **做在 iframe 外面。** 她生成的那个站挂在 `sandbox="allow-scripts"` 下，
 * 里面的链接根本跳不动（沙箱既没开弹窗也没开顶层跳转）。为了让一行链接能用
 * 去放宽一个渲染模型生成 HTML 的沙箱，是拿安全换样式。同一个文件里已经有
 * 先例：`ProcessComparison` 也挂在 iframe 外面。
 *
 * 只列**已发布**的（服务端只发这些）。她完成过但没公开的东西不在这里 ——
 * 完成和公开是两件事。
 *
 * 一件都没有就整块不渲染：一个写着「暂无作品」的空标题，比没有这一块糟。
 *
 * 样式全是行内的，不碰一个 `mk-*` token —— `site/` 里的每个文件都受
 * `site/no-ui-kit.test.ts` 管：她的网站不许长得像做出它的那个产品。
 */
function PublishedWorks({ works }: { works: PublishedWork[] }) {
  if (works.length === 0) return null;
  const theme = themeFor("essay");
  return (
    <section
      style={{
        background: theme.paper,
        color: theme.ink,
        padding: "48px 24px",
        fontFamily: '"Noto Serif SC","Songti SC",Georgia,serif',
      }}
    >
      <div style={{ margin: "0 auto", maxWidth: 720 }}>
        <h2 style={{ fontSize: 15, letterSpacing: "0.08em", opacity: 0.6, marginBottom: 16 }}>作品</h2>
        <ul style={{ listStyle: "none", margin: 0, padding: 0, display: "flex", flexDirection: "column", gap: 12 }}>
          {works.map((w) => (
            <li key={w.publicPath}>
              <a
                href={w.publicPath}
                style={{
                  display: "flex",
                  alignItems: "baseline",
                  gap: 12,
                  color: "inherit",
                  textDecoration: "none",
                  borderBottom: `1px solid ${theme.ink}22`,
                  paddingBottom: 12,
                }}
              >
                <span style={{ fontSize: 13, opacity: 0.55, flexShrink: 0 }}>{w.kind}</span>
                <span style={{ fontSize: 18, lineHeight: 1.6 }}>{w.title}</span>
              </a>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}
