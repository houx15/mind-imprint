import { useEffect, useState } from "react";
import { api, type ClassSummary } from "@/api";
import { listClassParentReports, type ParentReportSummary } from "../api/parentReports";
import { publishedMonthDay, rangeLabel } from "../parentReport/range";
import { errorText, pickClassId, readLastClassId, writeLastClassId } from "./assignmentLogic";
import { Select } from "./controls/Select";
import { StudioEmpty } from "./StudioArtwork";
import { TeacherPage } from "./TeacherPage";

/**
 * ParentReportsPage — `/parent-reports`: one class's parent reports, newest
 * first as the server sends them. Reports of students who have left the class
 * are included, so they can still be edited and exported. The class choice is
 * the same remembered choice the assignment pages use.
 */
export function ParentReportsPage({ onOpen }: { onOpen: (reportId: string) => void }) {
  const [classes, setClasses] = useState<ClassSummary[] | null>(null);
  const [classesError, setClassesError] = useState<string | null>(null);
  const [classesNonce, setClassesNonce] = useState(0);
  const [classId, setClassId] = useState("");

  const [rows, setRows] = useState<ParentReportSummary[] | null>(null);
  const [rowsError, setRowsError] = useState<string | null>(null);
  const [rowsNonce, setRowsNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setClasses(null);
    setClassesError(null);
    api
      .listClasses()
      .then((list) => {
        if (cancelled) return;
        setClasses(list);
        setClassId(pickClassId(list.map((c) => c.id), null, readLastClassId()));
      })
      .catch((e: unknown) => {
        if (!cancelled) setClassesError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [classesNonce]);

  useEffect(() => {
    if (!classId) return;
    let cancelled = false;
    setRows(null);
    setRowsError(null);
    listClassParentReports(classId)
      .then((list) => {
        if (!cancelled) setRows(list);
      })
      .catch((e: unknown) => {
        if (!cancelled) setRowsError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [classId, rowsNonce]);

  return (
    <TeacherPage>
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="teacher-page-title">家长报告</h1>
        {classes && classes.length > 0 && (
          <Select
            size="sm"
            ariaLabel="班级"
            className="max-w-[260px]"
            value={classId}
            onChange={(id) => {
              setClassId(id);
              writeLastClassId(id);
            }}
            options={classes.map((c) => ({ value: c.id, label: c.name }))}
          />
        )}
      </div>

      <div className="mt-6">
        {classesError ? (
          <div className="text-mk-small font-semibold text-mk-danger">
            加载失败：{classesError}{" "}
            <button type="button" onClick={() => setClassesNonce((n) => n + 1)} className="cursor-pointer underline">
              重试
            </button>
          </div>
        ) : classes === null ? (
          <div className="text-mk-body text-mk-muted">加载中…</div>
        ) : classes.length === 0 ? (
          <StudioEmpty kind="discovery">暂无班级。请联系管理员为你分配班级。</StudioEmpty>
        ) : rowsError ? (
          <div className="text-mk-small font-semibold text-mk-danger">
            加载失败：{rowsError}{" "}
            <button type="button" onClick={() => setRowsNonce((n) => n + 1)} className="cursor-pointer underline">
              重试
            </button>
          </div>
        ) : rows === null ? (
          <div className="text-mk-body text-mk-muted">加载中…</div>
        ) : rows.length === 0 ? (
          <StudioEmpty kind="keepsake">暂无家长报告。请在学生页面为单个学生生成报告。</StudioEmpty>
        ) : (
          <div className="overflow-x-auto rounded-mk-lg border border-mk-border bg-mk-surface">
            <table className="w-full border-collapse">
              <thead>
                <tr>
                  {["学生", "日期范围", "创建时间"].map((h) => (
                    <th
                      key={h}
                      className="whitespace-nowrap border-b border-mk-border px-3 py-2.5 text-left text-mk-label font-bold text-mk-muted"
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr
                    key={r.id}
                    tabIndex={0}
                    role="link"
                    aria-label={`${r.studentName} ${rangeLabel(r.rangeStart, r.rangeEnd)}`}
                    onClick={() => onOpen(r.id)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        onOpen(r.id);
                      }
                    }}
                    className="cursor-pointer hover:bg-mk-accent-50 focus-visible:bg-mk-accent-50 focus-visible:outline focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-[color:var(--mk-accent-500)]"
                  >
                    <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small font-bold text-mk-ink">
                      {r.studentName || "—"}
                    </td>
                    <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                      {rangeLabel(r.rangeStart, r.rangeEnd) || "—"}
                    </td>
                    <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-muted">
                      {publishedMonthDay(r.createdAt) || "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </TeacherPage>
  );
}
