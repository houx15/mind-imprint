import { useEffect, useState } from "react";
import { getReport, type AtomKind, type LiteReport } from "../api/reports";
import { useAlive } from "../shared/useAlive";
import { ReportView } from "./ReportView";

/**
 * ReportPanel — the thing `FinishedReadingPanel` / `FinishedWritingPanel`
 * mount where 「这次阅读/写作的报告还在路上」 used to sit. Owns the fetch and
 * the loading/absent/failed states; `ReportView` itself stays pure
 * presentation (props in, markup out — Task 7).
 *
 * Fetches on mount with `useAlive`, NOT a `useRef` latch paired with a
 * per-invocation `cancelled` flag — that exact combination is a known
 * StrictMode trap (see `useAlive.ts`'s own doc comment): the latch blocks
 * the second invocation while the first's cleanup marks it cancelled, so
 * the one real response lands in a closure that was told to drop it and the
 * loading state never clears.
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
export function ReportPanel({ kind, atomId }: { kind: AtomKind; atomId: string }) {
  const [report, setReport] = useState<LiteReport | null>(null);
  const [state, setState] = useState<"loading" | "done" | "quiet">("loading");
  const alive = useAlive();

  useEffect(() => {
    setState("loading");
    setReport(null);
    getReport(kind, atomId)
      .then((r) => {
        if (!alive.current) return;
        setReport(r);
        setState(r ? "done" : "quiet");
      })
      .catch(() => {
        if (!alive.current) return;
        setState("quiet");
      });
  }, [kind, atomId, alive]);

  if (state === "done" && report) return <ReportView report={report} />;
  if (state === "quiet") return null;

  const verb = kind === "reading" ? "读" : "写";
  return (
    <p className="text-mk-small text-mk-muted">
      印记正在把这次{verb}的东西整理成一份报告，稍等一下。
    </p>
  );
}
