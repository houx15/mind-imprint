import { useEffect, useRef, useState } from "react";
import { Download } from "lucide-react";
import { Icon } from "@/ui";
import {
  getPublicParentReport,
  ParentReportNotFoundError,
  type ParentReport,
} from "@lite/api/parentReports";
import { GuestTheme } from "@lite/learning/GuestTheme";
import { exportPoster } from "@lite/reports/exportPoster";
import { useAlive } from "@lite/shared/useAlive";
import { useNoIndex } from "@lite/shared/useNoIndex";
import { errorMessage } from "@lite/inbox/inboxLogic";
import { ParentReportPoster } from "./ParentReportPoster";
import { ParentReportView } from "./ParentReportView";

/**
 * PublicParentReportPage — what `/r/:token` opens, for a parent with no
 * account. Mounted by `rootElementFor.tsx` directly, never inside `LiteApp`,
 * for the same reason as `PublicReportPage` (`/s/:token`): nothing about the
 * signed-in shell applies to a visitor.
 *
 * The request carries no cookie (`credentials:"omit"`). A 404 means the link
 * was revoked or never existed, which is a different sentence from a failed
 * request.
 *
 * 保存为图片 rasterizes `ParentReportPoster`, which is always rendered offscreen
 * once the report has loaded (see that file for the wrapper rule).
 */
export function PublicParentReportPage({ token }: { token: string }) {
  // She is a minor: the page carries noindex from mount to unmount, whatever
  // the load outcome (the 404 page included).
  useNoIndex();
  return (
    <GuestTheme>
      <PublicParentReportContent token={token} />
    </GuestTheme>
  );
}

type LoadState =
  | { status: "loading" }
  | { status: "done"; report: ParentReport }
  | { status: "not_found" }
  | { status: "failed"; message: string };

function PublicParentReportContent({ token }: { token: string }) {
  const [state, setState] = useState<LoadState>({ status: "loading" });
  const [exporting, setExporting] = useState(false);
  const posterRef = useRef<HTMLDivElement>(null);
  const alive = useAlive();

  useEffect(() => {
    let cancelled = false;
    setState({ status: "loading" });
    getPublicParentReport(token)
      .then((report) => {
        if (!cancelled) setState({ status: "done", report });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ParentReportNotFoundError) setState({ status: "not_found" });
        else setState({ status: "failed", message: errorMessage(err) || "没有更多信息" });
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  if (state.status === "done") {
    const { report } = state;
    async function save() {
      if (exporting) return;
      setExporting(true);
      const name = report.studentName.trim();
      await exportPoster(posterRef.current, name ? `学习报告-${name}.png` : "学习报告.png");
      if (alive.current) setExporting(false);
    }
    return (
      <div className="min-h-full w-full bg-mk-paper">
        <ParentReportView
          report={report}
          variant="public"
          actions={
            <div className="mk-rp-actions">
              <button
                type="button"
                onClick={() => void save()}
                disabled={exporting}
                className="mk-rp-action"
                aria-label={exporting ? "保存为图片，处理中" : "保存为图片"}
                title={exporting ? "处理中" : "保存为图片"}
              >
                <Icon icon={Download} size={17} />
              </button>
            </div>
          }
        />
        <footer className="mk-rp-measure pb-10 text-mk-small text-mk-muted">来自思维印记</footer>
        <ParentReportPoster ref={posterRef} report={report} />
      </div>
    );
  }

  const text =
    state.status === "not_found"
      ? "该报告链接已失效"
      : state.status === "failed"
        ? `加载失败：${state.message}`
        : "加载中…";
  return (
    <div className="flex min-h-full w-full items-center justify-center bg-mk-paper px-6">
      <p className="text-mk-body text-mk-muted" role={state.status === "loading" ? "status" : undefined}>
        {text}
      </p>
    </div>
  );
}
