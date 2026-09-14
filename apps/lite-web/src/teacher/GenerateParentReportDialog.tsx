import { useEffect, useRef, useState } from "react";
import { Button } from "@/ui";
import { createParentReport } from "../api/parentReports";
import { defaultRange, todayBeijing } from "../parentReport/range";
import { failText } from "./assignmentLogic";
import { generateErrorPlacement } from "./parentReportLogic";

/**
 * GenerateParentReportDialog — picks a date range and creates a report for
 * one student. The request runs the flagship model, so it can take a while;
 * the dialog stays open and busy until the server answers.
 *
 * - 201 (with or without `draftError`) → `onCreated`; the report row exists
 *   either way, and the editor shows the draft failure.
 * - A refused range (`range_before_start`, `invalid_range`) shows under the
 *   dates; any other failure next to the buttons. The dialog stays open so
 *   the teacher can change the dates and try again.
 */
export function GenerateParentReportDialog({
  classId,
  userId,
  studentName,
  onClose,
  onCreated,
}: {
  classId: string;
  userId: string;
  studentName: string;
  onClose: () => void;
  onCreated: (reportId: string, draftError: string | null) => void;
}) {
  const [initial] = useState(() => {
    const today = todayBeijing(Date.now());
    return { today, ...defaultRange(today) };
  });
  const [start, setStart] = useState(initial.start);
  const [end, setEnd] = useState(initial.end);
  const [busy, setBusy] = useState(false);
  const [rangeError, setRangeError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  // The dialog can close (route change) while the POST is still running.
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [busy, onClose]);

  async function submit() {
    if (busy || !start || !end) return;
    setBusy(true);
    setRangeError(null);
    setFormError(null);
    try {
      const result = await createParentReport(classId, userId, { rangeStart: start, rangeEnd: end });
      if (!alive.current) return;
      onCreated(result.report.id, result.draftError);
    } catch (e) {
      if (!alive.current) return;
      const text = failText("生成", e);
      if (generateErrorPlacement(e) === "range") setRangeError(text);
      else setFormError(text);
      setBusy(false);
    }
  }

  const inputCls =
    "w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-body text-mk-ink outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:text-mk-muted";

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: "color-mix(in srgb, var(--mk-ink) 42%, transparent)" }}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && !busy) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="generate-parent-report-title"
        className="flex max-h-full w-full max-w-[520px] flex-col gap-5 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface p-6 shadow-mk-lg"
      >
        <div className="flex flex-col gap-1.5">
          <h2 id="generate-parent-report-title" className="text-mk-h2 text-mk-ink">
            生成家长报告
          </h2>
          <p className="text-mk-small text-mk-muted">{studentName}</p>
          <p className="text-mk-small text-mk-secondary">报告会汇总所选日期内的学习数据，并由 AI 起草文字；发布前可以修改。</p>
        </div>

        <div className="flex flex-col gap-2">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label className="flex flex-col gap-1.5 text-mk-small text-mk-muted">
              开始日期
              <input
                type="date"
                value={start}
                max={end || initial.today}
                disabled={busy}
                onChange={(e) => {
                  setStart(e.target.value);
                  setRangeError(null);
                }}
                className={inputCls}
              />
            </label>
            <label className="flex flex-col gap-1.5 text-mk-small text-mk-muted">
              结束日期
              <input
                type="date"
                value={end}
                min={start || undefined}
                max={initial.today}
                disabled={busy}
                onChange={(e) => {
                  setEnd(e.target.value);
                  setRangeError(null);
                }}
                className={inputCls}
              />
            </label>
          </div>
          {rangeError && (
            <p className="text-mk-small font-semibold text-mk-danger" role="alert">
              {rangeError}
            </p>
          )}
        </div>

        {formError && (
          <p className="break-words text-mk-small font-semibold text-mk-danger" role="alert">
            {formError}
          </p>
        )}

        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="ghost" size="sm" onClick={onClose} disabled={busy}>
            取消
          </Button>
          <Button variant="primary" size="sm" onClick={() => void submit()} disabled={busy || !start || !end}>
            {busy ? "生成中" : "生成"}
          </Button>
        </div>
      </div>
    </div>
  );
}
