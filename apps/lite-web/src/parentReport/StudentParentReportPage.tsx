import { useEffect, useState } from "react";
import { getStudentParentReport, markParentReportSeen, type ParentReport } from "@lite/api/parentReports";
import { errorMessage } from "@lite/inbox/inboxLogic";
import { requestInboxReload } from "@lite/inbox/useInbox";
import { ParentReportView } from "./ParentReportView";

/**
 * `/parent-reports/:id` — the report her teacher published, read by the
 * student herself. The same view a parent sees at `/r/:token`.
 *
 * On mount it marks the report seen. A failure there is ignored: it only
 * affects the unread dot, never whether she can read the page. After the
 * call the inbox is asked to reload so the dot clears.
 */
export function StudentParentReportPage({ id }: { id: string }) {
  const [report, setReport] = useState<ParentReport | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setReport(null);
    setError(null);
    getStudentParentReport(id)
      .then((r) => {
        if (!cancelled) setReport(r);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(errorMessage(err) || "没有更多信息");
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  useEffect(() => {
    let cancelled = false;
    markParentReportSeen(id)
      .catch(() => undefined)
      .then(() => {
        if (!cancelled) requestInboxReload();
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  if (report) return <ParentReportView report={report} variant="student" />;
  return (
    <div className="flex min-h-full w-full items-center justify-center px-6 py-16">
      <p className="text-mk-body text-mk-muted" role={error ? "alert" : "status"}>
        {error ? `加载失败：${error}` : "加载中…"}
      </p>
    </div>
  );
}
