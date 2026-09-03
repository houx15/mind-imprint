import { Essay } from "./Essay";
import { Ledger } from "./Ledger";
import { Magazine } from "./Magazine";
import { themeFor } from "./themes";
import type { SiteContent, SiteLayout } from "./types";

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
  narrow = false,
  editing = false,
}: {
  site: SiteContent;
  layout: SiteLayout;
  narrow?: boolean;
  /** 她自己在看 = true（会显示「这里还没写」）；访客 = false。 */
  editing?: boolean;
}) {
  const props = { site, theme: themeFor(layout), narrow, editing };
  if (layout === "ledger") return <Ledger {...props} />;
  if (layout === "magazine") return <Magazine {...props} />;
  return <Essay {...props} />;
}
