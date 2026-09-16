import { GuestTheme } from "../learning/GuestTheme";
import { useEffect, useState } from "react";
import {
  getPublicReport,
  PublicReportNotFoundError,
  type PublicReport,
  type PublicTranscriptLine,
} from "@lite/api/reports";
import { useAlive } from "@lite/shared/useAlive";
import { ReportView } from "./ReportView";
import { ArticleView } from "./ArticleView";
import { liteRoutePath, navigate, parseLiteRoute } from "@lite/routing";

/**
 * PublicReportPage — what a shared report link actually opens, for someone
 * with NO account at all (a parent scanning the QR code on a printed poster,
 * a link pasted into a family chat). For the `/s/:token` route this is
 * mounted directly by `rootElementFor.tsx` (the composition root, called
 * from `main.tsx`) instead of `LiteApp` — that module parses the pathname
 * itself and picks between this component and `LiteApp` before either one
 * renders, so `LiteApp`'s own hooks and auth boot effect never run at all
 * for a visitor on a share link. See `rootElementFor.tsx`'s own comment for
 * why that decision lives at the composition root rather than as an early
 * return inside `LiteApp`.
 *
 * Deliberately thin: fetch the report, mount the same `ReportView` the
 * signed-in student sees (Task 7 — pure presentation, no fetching, no auth),
 * add one quiet footer line naming the product, and nothing else. No nav, no
 * 「登录」 button, no 「去写作」, no link back into the app's authenticated
 * surfaces — a stranger who opens a child's shared report should reach
 * exactly one thing: that report. Signing in from here would also be
 * pointless: this token is not tied to any session, so there is nothing an
 * account would unlock on this page.
 *
 * Two distinct failure sentences, on purpose (per `getPublicReport`'s own
 * doc comment): a 404 means the link is dead (revoked or never existed) and
 * a network failure means we simply couldn't reach the server right now.
 * Telling someone their link is gone when their wifi merely dropped is a
 * lie, so the two states never share copy. Neither gets error styling, a
 * stack, or a retry button — just one plain sentence, matching how quiet the
 * rest of this page is.
 *
 * Fetches on mount with `useAlive`, not a `useRef` latch + per-invocation
 * `cancelled` flag — that combination is a known StrictMode trap (see
 * `useAlive.ts`'s doc comment): the latch blocks the second invocation while
 * the first's cleanup marks its own closure cancelled, so the one real
 * response never lands anywhere.
 */
export function PublicReportPage({ token, view }: { token: string; view: "article" | "record" }) {
  return <GuestTheme><PublicReportContent token={token} view={view} /></GuestTheme>;
}

function PublicReportContent({ token, view }: { token: string; view: "article" | "record" }) {
  const [state, setState] = useState<"loading" | "done" | "not_found" | "failed">("loading");
  const [payload, setPayload] = useState<PublicReport | null>(null);
  const alive = useAlive();

  /**
   * Which of the two pages is showing.
   *
   * 🚨 This has to live in state here, seeded from the prop — it cannot just
   * BE the prop. `rootElementFor` runs exactly once, from `main.tsx`'s
   * module-scope render, so a `pushState` between `/s/:token` and
   * `/s/:token/record` would change the URL and re-render nothing at all.
   * `LiteApp` has its own popstate listener for the same reason; the share
   * route is mounted outside it, so it needs its own.
   *
   * Listening to `popstate` (which `navigate` dispatches synthetically, and
   * which the browser fires on a real Back) rather than tracking the clicks
   * means the browser's own Back button moves between the article and the
   * record correctly, instead of jumping the reader out of the shared link
   * entirely.
   */
  const [currentView, setCurrentView] = useState<"article" | "record">(view);
  useEffect(() => setCurrentView(view), [view]);
  useEffect(() => {
    const onPop = () => {
      const route = parseLiteRoute(window.location.pathname);
      if (route.tab === "share") setCurrentView(route.view);
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  useEffect(() => {
    setState("loading");
    setPayload(null);
    getPublicReport(token)
      .then((p) => {
        if (!alive.current) return;
        setPayload(p);
        setState("done");
      })
      .catch((err) => {
        if (!alive.current) return;
        setState(err instanceof PublicReportNotFoundError ? "not_found" : "failed");
      });
  }, [token, alive]);

  if (state === "done" && payload) {
    const report = payload.report;
    // Two REAL pages behind one link. `/s/:token` is her article; appending
    // `/record` opens 这一篇是怎么写出来的. Navigation is a genuine pushState
    // (routing.ts's `navigate`), so a reader can send either page on, and the
    // browser's own Back works between them — which a view-state toggle in
    // this component would have quietly broken.
    //
    // A reading has no article: it goes straight to the record, and passes no
    // way back to one. So does a writing report generated before `piece`
    // existed (no backfill) — `hasArticle` is what keeps that case off a door
    // to an empty room.
    const hasArticle = report.kind === "writing" && report.piece.trim() !== "";
    const body =
      hasArticle && currentView === "article" ? (
        <ArticleView
          report={report}
          onOpenRecord={() => navigate(liteRoutePath({ tab: "share", token, view: "record" }))}
        />
      ) : (
        <ReportView
          report={report}
          viewer="guest"
          onBackToArticle={
            hasArticle
              ? () => navigate(liteRoutePath({ tab: "share", token, view: "article" }))
              : undefined
          }
        />
      );
    // 对话只有在她勾过的时候才会跟着负载回来（服务端没勾连键都不发），所以
    // 这里不需要再判一次「该不该显示」—— 有就是她要给人看的。
    const transcript =
      currentView !== "article" && payload.transcript.length > 0 ? (
        <PublicTranscript lines={payload.transcript} name={report.studentName} />
      ) : null;
    return (
      <div className="min-h-full w-full bg-mk-paper">
        {body}
        {transcript}
        <footer className="mk-rp-measure pb-10 text-mk-small text-mk-faint">
          来自思维印记
        </footer>
      </div>
    );
  }

  if (state === "not_found") {
    return <QuietMessage text="这份记录不存在，或者已经被收回了。" />;
  }

  if (state === "failed") {
    return <QuietMessage text="网络好像断开了，请稍后再试一次。" />;
  }

  // "loading" — no spinner copy of its own; a blank paper background is
  // enough for the brief instant before the fetch resolves.
  return <div className="min-h-full w-full bg-mk-paper" />;
}

/**
 * 她公开出来的那段对话。
 *
 * 只在她勾过「公开我和印记的对话」时存在（迁移 0174，默认关，停止分享时一起
 * 收回）。🚨 每条都标明是谁说的，而且访客那一面用的是**她的名字**而不是
 * 「我」—— 看这一页的人不是她。
 */
function PublicTranscript({ lines, name }: { lines: PublicTranscriptLine[]; name: string }) {
  return (
    <section className="mk-rp-measure flex flex-col gap-4 pb-10">
      <h2 className="text-mk-label text-mk-faint">这次的完整对话</h2>
      {lines.map((line, i) => (
        <div
          key={i}
          className={line.who === "student" ? "flex flex-col items-end gap-1" : "flex flex-col items-start gap-1"}
        >
          <span className="text-mk-label text-mk-faint">{line.who === "student" ? name : "印记"}</span>
          <div
            className="max-w-[46rem] rounded-mk-lg px-5 py-4 text-mk-body text-mk-ink"
            style={{
              background: line.who === "student" ? "var(--mk-accent-50)" : "var(--mk-surface)",
              border: line.who === "student" ? "none" : "1px solid var(--mk-border)",
            }}
          >
            <p className="whitespace-pre-wrap">{line.text}</p>
          </div>
        </div>
      ))}
    </section>
  );
}

/** One plain sentence, centered, with no error chrome — used for both the
 *  404 and the network-failure states so neither reads as a broken page. */
function QuietMessage({ text }: { text: string }) {
  return (
    <div className="flex min-h-full w-full items-center justify-center bg-mk-paper px-6">
      <p className="text-mk-body text-mk-muted">{text}</p>
    </div>
  );
}
