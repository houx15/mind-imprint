import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { flushSync } from "react-dom";
import { Pebble } from "@/ui";
import { getReportEnvelope, type AtomKind, type LiteReport } from "../api/reports";
import { useAlive } from "../shared/useAlive";
import { exportPoster } from "./exportPoster";
import { ReportPoster } from "./ReportPoster";
import { ReportView } from "./ReportView";
import { ArticleView } from "./ArticleView";
import { SharePanel } from "./SharePanel";
import { ReportActions } from "./ReportActions";
import { ExperienceStars } from "./ExperienceStars";
import { TreeProposals } from "./TreeProposals";

/**
 * ReportPanel — the thing `FinishedReadingPanel` / `FinishedWritingPanel`
 * mount where 「这次阅读/写作的报告还在路上」 used to sit. Owns the fetch and
 * the loading/absent/failed states; `ReportView` itself stays pure
 * presentation (props in, markup out — Task 7). Once the report has
 * arrived, also mounts `SharePanel` underneath it (Task 10) — the opt-in
 * that lets her publish this exact report to anyone with the link, and take
 * it back — and an 导出图片 button (Task 11) that mounts an offscreen
 * `ReportPoster` and hands it to `exportPoster`. Neither is ever offered
 * while the report is still loading or absent: there is nothing to share or
 * export yet.
 *
 * The poster is mounted only for the instant of export — into a fresh,
 * detached container appended to `document.body` and torn down right after
 * — rather than kept sitting in the tree for the whole time a report is on
 * screen. `ReportPoster`'s own root already carries the offscreen
 * `position:fixed; left:-99999px` sizing that keeps it out of the visible
 * page (see `ReportPoster.tsx`); mounting it only on demand additionally
 * means nobody — least of all a screen reader, which position:fixed alone
 * does nothing to hide from — ever has to contend with a duplicate, inert
 * copy of the title/name/stats/quotes sitting in the page the whole time
 * she's just reading her own report.
 *
 * Fetches on mount with `useAlive`, NOT a `useRef` latch paired with a
 * per-invocation `cancelled` flag — that exact combination is a known
 * StrictMode trap (see `useAlive.ts`'s own doc comment): the latch blocks
 * the second invocation while the first's cleanup marks it cancelled, so
 * the one real response lands in a closure that was told to drop it and the
 * loading state never clears.
 *
 * Also carries F2's share state: `getReportEnvelope` returns `shareToken`
 * alongside the report, and it is handed to `SharePanel` as
 * `initialShareToken` so a report she already shared in an earlier sitting
 * reopens with the link, the QR, and 停止分享 already showing — not the
 * closed {phase:"off"} state a plain reload used to fall back to, whose
 * off-state copy describes sharing as hypothetical when it may already be
 * live and whose only path back to 停止分享 was a button that reads as
 * "publish", not "manage".
 *
 * The first open is the one that GENERATES the report server-side — a
 * flagship model call that can run tens of seconds — so the waiting copy
 * says that honestly, the way 印记 talks (a real explanation of what's
 * happening, not a bare spinner and not "加载中").
 *
 * A failed fetch never shouts: she has already finished her work, and an
 * error banner about a report she didn't ask for would be worse than a
 * quiet absence. `getReport` returning `null` (the atom's report generator
 * declining because it isn't finished yet — see api/reports.ts) gets the
 * exact same quiet treatment, since neither case is something she can act
 * on from here.
 */
export function ReportPanel({
  kind,
  atomId,
  fallback,
}: {
  kind: AtomKind;
  atomId: string;
  /** Rendered INSTEAD of the report when there is no report to render (a
   *  failed fetch, or an atom the generator declined). The report now carries
   *  content the host used to render itself — a reading's 我的收获 is the
   *  report's own `keep`, verbatim — so without this the quiet state would
   *  silently drop her own words off the page rather than merely omitting a
   *  summary. Hosts pass the same content in its pre-report form. */
  fallback?: React.ReactNode;
}) {
  const [report, setReport] = useState<LiteReport | null>(null);
  const [shareToken, setShareToken] = useState<string | null>(null);
  const [rating, setRating] = useState<number | null>(null);
  const [state, setState] = useState<"loading" | "done" | "quiet">("loading");
  const [exporting, setExporting] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);
  /**
   * Which of her two pages is showing: the article, or the record of writing
   * it. Starts on the article — that is the piece she just finished, and the
   * thing she'd want to look at (and send) first.
   *
   * In-app this is component state rather than a URL, unlike the public share
   * link (`/s/:token` vs `/s/:token/record`, real pushState). A shared page
   * has to be linkable to be shared at all; this one is reached by finishing
   * a writing, and both pages carry an explicit way to the other. Browser
   * Back therefore leaves the room rather than stepping between the two —
   * worth promoting to a real route if that ever bites.
   */
  const [page, setPage] = useState<"article" | "record">("article");
  const alive = useAlive();

  useEffect(() => {
    setState("loading");
    setReport(null);
    setShareToken(null);
    setRating(null);
    getReportEnvelope(kind, atomId)
      .then((env) => {
        if (!alive.current) return;
        setReport(env.report);
        setShareToken(env.shareToken);
        setRating(env.rating);
        setState(env.report ? "done" : "quiet");
        /**
         * 🚨 The second fetch is what PAYS for the prose, and she is already
         * reading her report while it runs.
         *
         * The server splits report generation in two (see ensureAtomReport):
         * the first request stores and returns everything deterministic —
         * stats, her notes, her lens notes, her own 收获 — and flags
         * `prosePending`; the model call happens on the NEXT request. That
         * call is `assess`-class (`reasoning: "max"`, a 180s budget) and it
         * used to run inline, which is why 「印记正在把这次读的东西整理成一份
         * 报告，稍等一下。」 was the whole screen for up to two and a half
         * minutes — and why, past the 150s request cap, it produced no report
         * at all.
         *
         * So this is deliberately NOT a poll loop. It is one follow-up
         * request whose answer arrives when it arrives; nothing on screen is
         * blocked on it, and if it fails the report she already has stays
         * exactly as it is (the next time she opens the report, the server
         * tries the prose again).
         */
        if (env.report?.prosePending) {
          getReportEnvelope(kind, atomId)
            .then((withProse) => {
              // Only ever ADD prose to a report already on screen. A null
              // report here would mean something odd happened server-side,
              // and replacing a good report with nothing is strictly worse
              // than leaving hers alone.
              if (!alive.current || !withProse.report) return;
              setReport(withProse.report);
            })
            .catch(() => {
              /* Her report is already on screen — see above. */
            });
        }
      })
      .catch(() => {
        if (!alive.current) return;
        setState("quiet");
      });
  }, [kind, atomId, alive]);

  async function handleExport() {
    if (!report) return;
    setExporting(true);
    const mountEl = document.createElement("div");
    document.body.appendChild(mountEl);
    const root = createRoot(mountEl);
    try {
      let posterNode: HTMLDivElement | null = null;
      // `flushSync` forces the render to commit synchronously, so
      // `posterNode` is populated before `root.render` returns — no waiting
      // on an effect just to get a ref to something we're about to unmount.
      flushSync(() => {
        root.render(
          <ReportPoster
            report={report}
            ref={(el) => {
              posterNode = el;
            }}
          />,
        );
      });
      await exportPoster(posterNode, `${report.title}.png`);
    } finally {
      root.unmount();
      mountEl.remove();
      if (alive.current) setExporting(false);
    }
  }

  if (state === "done" && report) {
    // Her own two pages, the same split a shared link gets: the article she
    // wrote, and 这一篇是怎么写出来的 behind it ("make the report a new page").
    // A reading has no article and goes straight to the record; so does a
    // writing finished before `piece` existed (no backfill).
    const hasArticle = report.kind === "writing" && report.piece.trim() !== "";
    const actions = (
      <ReportActions
        exporting={exporting}
        onExport={handleExport}
        shareOpen={shareOpen}
        shared={shareToken !== null}
        onToggleShare={() => setShareOpen((v) => !v)}
      />
    );
    const sharePanel = shareOpen ? (
      <SharePanel
        kind={kind}
        atomId={atomId}
        initialShareToken={shareToken}
        // Keeps the share icon's "already published" dot honest after a mint
        // or a revoke, and keeps `initialShareToken` valid across the
        // unmount/remount this toggle causes — without giving two components
        // two copies of the same state machine.
        onSharedChange={setShareToken}
      />
    ) : null;

    if (hasArticle && page === "article") {
      return (
        <ArticleView
          report={report}
          actions={actions}
          sharePanel={sharePanel}
          onOpenRecord={() => setPage("record")}
        />
      );
    }

    return (
      <>
        {/* 导出 / 分享 are two small icons in the report's own upper-right
            corner, handed to `ReportView` as slots; the share panel drops in
            right under the hero so it opens next to the icon that opened it.
            `PublicReportPage` mounts the same views with NEITHER slot, which
            is what keeps a visitor from ever seeing controls over someone
            else's work. */}
        <ReportView
          report={report}
          onBackToArticle={hasArticle ? () => setPage("article") : undefined}
          actions={actions}
          sharePanel={sharePanel}
        />

        <div className="mk-rp-measure flex flex-col gap-5 pb-4">
          {/* 可以加进兴趣树的候选词。**只在她自己这一面**，`PublicReportPage`
              没有这一节 —— 别人打开这个链接时，能加词的是他，不是她。

              放在报告之后、五颗星之前：她先看完这一篇留下了什么，再决定哪几个
              词进树。反过来问，是在问一件她还没看到的事。 */}
          <TreeProposals atomId={atomId} />

          {/* The five stars come AFTER the report — she reads it, then says how
              the session felt; asking first would be asking about something she
              hasn't seen. With sharing moved to the top this is now the last
              thing on the page rather than the fourth, and it is styled to be
              found. Reading only: the writing room has no endpoint for it yet,
              and a scorer that silently drops her answer is worse than not
              asking. */}
          {kind === "reading" && <ExperienceStars atomId={atomId} initial={rating} />}
        </div>
      </>
    );
  }
  if (state === "quiet") return <>{fallback ?? null}</>;

  /**
   * 报告加载中。
   *
   * 🚨 这里原本只有一行静态灰字（「印记正在把这次读的东西整理成一份报告，
   * 稍等一下。」），同事试用报的两条——「报告没有loading状态」和那句话
   * 「it never finishes」——说的是同一块屏幕：没有任何东西在动，所以她无法
   * 分辨「在跑」和「卡死了」，而当时那一等确实可以长到两分半。
   *
   * 服务端拆成两段之后这一段只有毫秒级（见上面那个 effect），但它仍然要有
   * 一个**在动**的东西：一次网络抖动下，一行不动的字和一个死掉的页面长得
   * 一模一样。
   *
   * 文案按 AGENTS.md §界面文案怎么写：状态用「处理中」这类成对词，不写成
   * 印记 在跟她说话。
   */
  return (
    <div className="mk-rp-measure flex items-center gap-2 py-10" role="status" aria-live="polite">
      <span className="shrink-0">
        <Pebble state="thinking" size={22} />
      </span>
      <span className="flex items-center gap-1" aria-hidden="true">
        <span className="mk-think-dot" />
        <span className="mk-think-dot [animation-delay:0.15s]" />
        <span className="mk-think-dot [animation-delay:0.3s]" />
      </span>
      <p className="text-mk-small text-mk-muted">报告处理中</p>
    </div>
  );
}
