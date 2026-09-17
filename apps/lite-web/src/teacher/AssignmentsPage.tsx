import { useEffect, useState } from "react";
import { Button } from "@/ui";
import { api, type ClassSummary } from "@/api";
import { listAssignments, type AssignmentSummaryDTO } from "../api/assignments";
import { formatDeadline, STATUS_LABEL } from "../shared/deadline";
import { kindLabel } from "./format";
import { Select } from "./controls/Select";
import { StudioEmpty } from "./StudioArtwork";
import { TeacherPage } from "./TeacherPage";
import { assignmentFileName, errorText, pickClassId, readLastClassId, STATUS_ORDER, writeLastClassId } from "./assignmentLogic";

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
    <TeacherPage>
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="teacher-page-title">作业</h1>
        {classes && classes.length > 0 && (
          <>
            <Select
              size="sm"
              ariaLabel="班级"
              value={classId}
              onChange={(id) => {
                setClassId(id);
                writeLastClassId(id);
              }}
              options={classes.map((c) => ({ value: c.id, label: c.name }))}
            />
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
          <StudioEmpty
            kind="writing"
            action={{
              label: "布置作业",
              onClick: () => {
                writeLastClassId(classId);
                onNew(classId);
              },
            }}
          >
            暂无作业
          </StudioEmpty>
        ) : (
          <div className="overflow-x-auto rounded-mk-lg border border-mk-border bg-mk-surface">
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
                    <td className="border-b border-mk-border px-3 py-3 text-mk-small font-bold text-mk-ink">
                      {a.title}
                      {assignmentFileName(a) && (
                        <span className="mt-0.5 block break-all font-normal text-mk-muted">{assignmentFileName(a)}</span>
                      )}
                    </td>
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
    </TeacherPage>
  );
}
