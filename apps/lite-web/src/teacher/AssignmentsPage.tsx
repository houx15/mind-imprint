import { useEffect, useState } from "react";
import { Button } from "@/ui";
import { api, type ClassSummary } from "@/api";
import { listAssignments, type AssignmentSummaryDTO } from "../api/assignments";
import { formatDeadline, STATUS_LABEL } from "../shared/deadline";
import { kindLabel } from "./format";
import { errorText, pickClassId, readLastClassId, STATUS_ORDER, writeLastClassId } from "./assignmentLogic";

/**
 * AssignmentsPage — `/assignments`: one class's assignments with a count per
 * derived status. The class choice is remembered in localStorage
 * (`readLastClassId`/`writeLastClassId`, both try/catch) and shared with the
 * create form, so 布置作业 opens on the class she was looking at.
 */
export function AssignmentsPage({
  onNew,
  onOpen,
}: {
  onNew: (classId: string) => void;
  onOpen: (assignmentId: string) => void;
}) {
  const [classes, setClasses] = useState<ClassSummary[] | null>(null);
  const [classesError, setClassesError] = useState<string | null>(null);
  const [classesNonce, setClassesNonce] = useState(0);
  const [classId, setClassId] = useState("");

  const [rows, setRows] = useState<AssignmentSummaryDTO[] | null>(null);
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
    listAssignments(classId)
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
    <div className="min-h-full">
      <div className="mx-auto max-w-[980px] px-4 pb-16 pt-8 sm:px-8">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-mk-h1 tracking-tight text-mk-ink">作业</h1>
          {classes && classes.length > 0 && (
            <>
              <label className="flex items-center gap-2 text-mk-small text-mk-muted">
                班级
                <select
                  value={classId}
                  onChange={(e) => {
                    setClassId(e.target.value);
                    writeLastClassId(e.target.value);
                  }}
                  className="rounded-mk-md border border-mk-border bg-mk-surface px-3 py-1.5 text-mk-small text-mk-ink outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                >
                  {classes.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </label>
              <Button
                variant="primary"
                size="sm"
                className="sm:ml-auto"
                onClick={() => {
                  writeLastClassId(classId);
                  onNew(classId);
                }}
              >
                布置作业
              </Button>
            </>
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
            <div className="text-mk-body text-mk-muted">暂无班级</div>
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
            <div className="text-mk-body text-mk-muted">暂无作业</div>
          ) : (
            <div className="overflow-x-auto rounded-mk-lg border border-mk-border bg-mk-surface shadow-mk-xs">
              <table className="w-full min-w-[820px] border-collapse">
                <thead>
                  <tr>
                    {["类型", "标题", "截止时间", ...STATUS_ORDER.map((s) => STATUS_LABEL[s])].map((h) => (
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
                  {rows.map((a) => (
                    <tr
                      key={a.id}
                      tabIndex={0}
                      role="link"
                      aria-label={a.title}
                      onClick={() => onOpen(a.id)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          onOpen(a.id);
                        }
                      }}
                      // A keyboard focus needs more than the hover tint. Chrome
                      // draws no box-shadow ring on a <tr>, so this uses an
                      // inset outline instead.
                      className="cursor-pointer hover:bg-mk-accent-50 focus-visible:bg-mk-accent-50 focus-visible:outline focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-[color:var(--mk-accent-500)]"
                    >
                      <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-muted">
                        {kindLabel(a.kind)}
                      </td>
                      <td className="border-b border-mk-border px-3 py-3 text-mk-small font-bold text-mk-ink">{a.title}</td>
                      <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                        {formatDeadline(a.dueAt)}
                      </td>
                      {STATUS_ORDER.map((s) => {
                        const n = a.counts[s];
                        const alarm = s === "overdue" && n > 0;
                        return (
                          <td
                            key={s}
                            className={
                              "whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small tabular-nums " +
                              (alarm ? "font-bold text-mk-danger" : "text-mk-ink")
                            }
                          >
                            {n}
                          </td>
                        );
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
