import { ProcessComparison, type ComparisonText } from "./ProcessComparison";
import { useEffect, useState } from "react";
import { getPublicSite, publicCodeSiteURL } from "../api/site";
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
    { kind: "loading" } | { kind: "generated"; renderKey:string; comparison?:ComparisonText|null } | { kind: "ok"; layout: SiteLayout; palette: SitePalette; heroUrl: string; content: SiteContent } | { kind: "gone" }
  >({ kind: "loading" });

  // noindex 在挂载时加上，卸载时收回。它只属于这一个页面。
  useNoIndex();

  useEffect(() => {
    let cancelled = false;
    getPublicSite(token)
      .then((res) => {
        if (cancelled) return;
        if (res.generated) { setState({kind:"generated",renderKey:res.renderKey,comparison:res.comparison}); document.title="个人主页"; return; }
        setState({ kind: "ok", layout: res.layout, palette: res.palette, heroUrl: res.heroUrl, content: res.content });
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

  if(state.kind==="generated") return <>{state.comparison&&<nav aria-label="主页内容" style={{padding:"12px 24px",background:"#f4f0e6",display:"flex",gap:24}}><a href="#site-work">作品</a><a href="#process-comparison">修改过程</a></nav>}<iframe id="site-work" title="个人主页" src={`${publicCodeSiteURL(token)}?publication=${encodeURIComponent(state.renderKey)}`} sandbox="allow-scripts" referrerPolicy="no-referrer" allow="camera 'none'; microphone 'none'; geolocation 'none'; payment 'none'" style={{display:"block",width:"100%",height:"100dvh",border:0}}/>{state.comparison&&<ProcessComparison comparison={state.comparison} source={`${publicCodeSiteURL(token)}?publication=${encodeURIComponent(state.renderKey)}`}/>}</>;

  return (
    <BuiltSite
      site={state.content}
      layout={state.layout}
      palette={state.palette}
      heroUrl={state.heroUrl}
      editing={false}
    />
  );
}
