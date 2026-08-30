import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { flushSync } from "react-dom";
import { getReportEnvelope, type AtomKind, type LiteReport } from "../api/reports";
import { useAlive } from "../shared/useAlive";
import { exportPoster } from "./exportPoster";
import { ReportPoster } from "./ReportPoster";
import { ReportView } from "./ReportView";
import { SharePanel } from "./SharePanel";
import { ReportActions } from "./ReportActions";
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
  const [shareOpen, setShareOpen] = useState(false);
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
        {/* 导出 / 分享 are two small icons in the report's own upper-right
            corner, handed to `ReportView` as slots; the share panel drops in
            right under the hero so it opens next to the icon that opened it.
            `PublicReportPage` mounts the same `ReportView` with NEITHER slot,
            which is what keeps a visitor from ever seeing controls over
            someone else's report. */}
        <ReportView
          report={report}
          actions={
            <ReportActions
              exporting={exporting}
              onExport={handleExport}
              shareOpen={shareOpen}
              shared={shareToken !== null}
              onToggleShare={() => setShareOpen((v) => !v)}
            />
          }
          sharePanel={
            shareOpen ? (
              <SharePanel
                kind={kind}
                atomId={atomId}
                initialShareToken={shareToken}
                // Keeps the share icon's "already published" dot honest after a
                // mint or a revoke, and keeps `initialShareToken` valid across
                // the unmount/remount this toggle causes — without giving two
                // components two copies of the same state machine.
                onSharedChange={setShareToken}
              />
            ) : null
          }
        />

        <div className="mk-rp-measure flex flex-col gap-5 pb-4">
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

  const verb = kind === "reading" ? "读" : "写";
  // Carries the same gutters as the report it is standing in for — the hosts
  // no longer wrap this panel in a padded column, so an unwrapped <p> would
  // sit flush against the window edge.
  return (
    <div className="mk-rp-measure py-10">
      <p className="text-mk-small text-mk-muted">
        印记正在把这次{verb}的东西整理成一份报告，稍等一下。
      </p>
    </div>
  );
}
