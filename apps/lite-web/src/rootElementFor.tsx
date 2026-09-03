import type { ReactElement } from "react";
import { LiteApp } from "./LiteApp";
import { PublicReportPage } from "./reports/PublicReportPage";
import { PublicSitePage } from "./site/PublicSitePage";
import { parseLiteRoute } from "./routing";
import { EcoRoot } from "./eco/EcoApp";
import { isEcoPath } from "./eco/route";

/**
 * The public report viewer (`/s/:token`) and the authenticated shell
 * (`LiteApp`) are two genuinely disjoint apps — a visitor on a share link has
 * no account and no session cookie at all (a parent scanning a QR code), and
 * nothing in this codebase ever navigates TO `/s/:token` (`SharePanel`
 * renders that URL into a read-only input and a QR image, never an `<a
 * href>` or a `navigate()` call) — so the choice between them belongs at the
 * composition root (`main.tsx`), not inside `LiteApp`'s own render. Deciding
 * it here means `LiteApp` never has to reason about the share route at all:
 * its hooks are unconditionally the first thing in the function body, with
 * no early return above them gating on a branch outcome that (correctly
 * implemented) can never flip mid-instance — there is nothing left inside
 * `LiteApp` to get wrong.
 *
 * Pulled into its own module rather than living inline in `main.tsx` (or
 * exported from it) so a test can import and call it directly, without
 * pulling in `main.tsx`'s own module-scope `createRoot(...).render(...)`
 * call — that runs the moment the module loads and blows up outside a real
 * `#root` DOM element (and would be unsafe to trigger twice even with one).
 */
export function rootElementFor(pathname: string): ReactElement {
  // `/eco/*` — the ecosystem prototype (世界 / 我的树 / PBL / 个人主页). It is a
  // THIRD disjoint app: pure static frontend on mock data, no API and no
  // session, so it is decided here for the same reason the share viewer is —
  // `LiteApp` should never have to reason about a route it does not own.
  // Delete this branch and the `eco/` directory together when the prototype
  // has done its job.
  if (isEcoPath(pathname)) return <EcoRoot />;

  const route = parseLiteRoute(pathname);
  // `view` is only the STARTING page — `PublicReportPage` owns it from there,
  // because this function runs once from `main.tsx`'s module-scope render and
  // never again. See that component's `currentView` comment.
  // `/p/:token` — 她的主页，访客那一面。第四个 disjoint app，理由和 `/s/` 完全
  // 一样：打开它的人没有 session，而且这一页上不该有任何属于这个产品的外壳。
  if (route.tab === "page") return <PublicSitePage token={route.token} />;

  return route.tab === "share" ? (
    <PublicReportPage token={route.token} view={route.view} />
  ) : (
    <LiteApp />
  );
}
