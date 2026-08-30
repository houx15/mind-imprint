import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { flushSync } from "react-dom";
import { getReportEnvelope, type AtomKind, type LiteReport } from "../api/reports";
import { useAlive } from "../shared/useAlive";
import { exportPoster } from "./exportPoster";
import { ReportPoster } from "./ReportPoster";
import { ReportView } from "./ReportView";
import { SharePanel } from "./SharePanel";
import { ExperienceStars } from "./ExperienceStars";

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
    return (
      <>
        <ReportView report={report} />
        {/* Same 1180px measure and gutters as `ReportView` itself, so the
            export button, the share panel and the stars line up with the
            report's own left edge instead of sitting in a narrower column
            under a wide page. */}
        <div className="mx-auto flex w-full max-w-[1180px] flex-col gap-5 px-5 pb-4 sm:px-8">
          <button
            type="button"
            onClick={handleExport}
            disabled={exporting}
            className="w-fit rounded-mk-full px-4 py-2 text-mk-small text-white disabled:cursor-not-allowed disabled:opacity-60"
            style={{ background: "var(--mk-accent-500)" }}
          >
            {exporting ? "生成图片中…" : "导出图片"}
          </button>
          <SharePanel kind={kind} atomId={atomId} initialShareToken={shareToken} />
          {/* The five stars, at the very bottom — she reads the report first,
              then says how the session felt. Reading only for now: the writing
              room has no endpoint for it yet, and a scorer that silently drops
              her answer is worse than not asking. */}
          {kind === "reading" && <ExperienceStars atomId={atomId} initial={rating} />}
        </div>
      </>
    );
  }
  if (state === "quiet") return <>{fallback ?? null}</>;

  const verb = kind === "reading" ? "读" : "写";
  // Carries the same gutters as the report it is standing in for — the hosts
  // no longer wrap this panel in a padded column, so an unwrapped <p> would
  // sit flush against the window edge.
  return (
    <div className="mx-auto w-full max-w-[1180px] px-5 py-10 sm:px-8">
      <p className="text-mk-small text-mk-muted">
        印记正在把这次{verb}的东西整理成一份报告，稍等一下。
      </p>
    </div>
  );
}
