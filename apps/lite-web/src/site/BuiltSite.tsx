import { useEffect, useRef, useState } from "react";
import { StructuredSite } from "./StructuredSite";
import { Essay } from "./Essay";
import { Ledger } from "./Ledger";
import { Magazine } from "./Magazine";
import { themeFor } from "./themes";
import type { SiteContent, SiteLayout, SitePalette } from "./types";

/**
 * 她的网站 — 主页项目真正做出来的那个东西。
 *
 * ## 三个版式，因为她是在三个页面之间做的选择
 * 第一版把三个方案渲染成一个版式换三套配色，于是她写了理由的那个决定，在页面
 * 上看不出来。现在选择决定的是页面本身：
 *
 *   essay    长页、无导航、很大的一句话
 *   ledger   一张密表，等宽字
 *   magazine 头图 + 卡片 + 侧栏（经典博客的形状）
 *
 * ## 一个组件，三个地方
 * 挑版式时的预览、她自己的「我的主页」、以及 `/p/:token` 那个访客页面，渲染的
 * 都是这一个组件。**预览如果是另一个组件，它迟早会变成一句谎话**——原型里她挑
 * 版式那一步看到的是三条灰色骨架，真正的页面要到第六步才出现，于是她是在为一
 * 个自己没见过的东西写理由。
 *
 * `narrow` 是 prop 而不是媒体查询：预览要在 1400px 的窗口里画一个 390px 的手机
 * 框，`md:` 断点读到的是真实视口，会把手机预览排成桌面版——恰好在她检查手机效
 * 果的那一刻排错。
 */
export function BuiltSite({
  site,
  layout,
  palette,
  heroUrl,
  narrow,
  editing = false,
}: {
  site: SiteContent;
  layout: SiteLayout;
  /** 她第三关定下的配色。没有就用版式自带的那一套。 */
  palette?: SitePalette | null;
  /** 她第三关生成的头图。空 = 她没要头图，那是一个合法的选择。 */
  heroUrl?: string;
  narrow?: boolean;
  /** 她自己在看 = true（会显示「这里还没写」）；访客 = false。 */
  editing?: boolean;
}) {
  const container = useRef<HTMLDivElement>(null);
  const [autoNarrow, setAutoNarrow] = useState(() => typeof window !== "undefined" && window.innerWidth < 700);
  useEffect(() => {
    if (narrow !== undefined || !container.current) return;
    const measure = () => setAutoNarrow(container.current!.getBoundingClientRect().width < 700);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(container.current);
    return () => observer.disconnect();
  }, [narrow]);
  // Explicit preview widths win; normal pages respond to their actual space,
  // including the student's navigation rail beside the published site.
  const props = { site, theme: themeFor(layout, palette), narrow: narrow ?? autoNarrow, editing, heroUrl };
  return <div ref={container} style={{ width: "100%", minWidth: 0 }}>
    {site.sections?.length ? <StructuredSite {...props} layout={layout} /> : layout === "ledger" ? <Ledger {...props} /> : layout === "magazine" ? <Magazine {...props} /> : <Essay {...props} />}
  </div>;
}
