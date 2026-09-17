import { useEffect, useState } from "react";
import { ArrowRight } from "lucide-react";
import { Icon } from "@/ui";
import { api, type ClassSummary } from "@/api";
import { listClassParentReports, type ParentReportSummary } from "../api/parentReports";
import { publishedMonthDay, rangeLabel } from "../parentReport/range";
import { errorText, pickClassId, readLastClassId, writeLastClassId } from "./assignmentLogic";
import { Select } from "./controls/Select";
import { StudioEmpty, StudioError, StudioHeading, StudioLoading } from "./StudioArtwork";
import { TeacherPage } from "./TeacherPage";

/**
 * ParentReportsPage — `/parent-reports`: one class's parent reports, newest
 * first as the server sends them. Reports of students who have left the class
 * are included, so they can still be edited and exported. The class choice is
 * the same remembered choice the assignment pages use.
 */
export function ParentReportsPage({
  onOpen,
  onOpenClass,
}: {
  onOpen: (reportId: string) => void;
  /** 选择学生: the class page, whose roster opens a student. */
  onOpenClass: (classId: string) => void;
}) {
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

  const className = classes?.find((c) => c.id === classId)?.name ?? "";

  return (
    <TeacherPage width="wide">
      <StudioHeading
        kicker="家长沟通"
        title="家长报告"
        description="家长报告按所选日期汇总学生的学习记录，由印记起草文字，导出前可修改。请在学生页面生成。"
        kind="keepsake"
      />

      {classes && classes.length > 0 && (
        <div className="teacher-toolbar">
          <label className="teacher-toolbar-field">
            班级
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
          </label>
          {rows && rows.length > 0 && (
            <button type="button" className="teacher-link" onClick={() => onOpenClass(classId)}>
              生成新报告：选择学生
              <Icon icon={ArrowRight} size={14} />
            </button>
          )}
        </div>
      )}

      {classesError ? (
        <StudioError message={classesError} onRetry={() => setClassesNonce((n) => n + 1)} />
      ) : classes === null ? (
        <StudioLoading />
      ) : classes.length === 0 ? (
        <StudioEmpty kind="discovery" title="暂无班级">
          家长报告按班级中的学生生成。请联系管理员为你分配班级。
        </StudioEmpty>
      ) : rowsError ? (
        <StudioError message={rowsError} onRetry={() => setRowsNonce((n) => n + 1)} />
      ) : rows === null ? (
        <StudioLoading />
      ) : rows.length === 0 ? (
        <StudioEmpty kind="keepsake" title="暂无家长报告" action={{ label: "选择学生", onClick: () => onOpenClass(classId) }}>
          {`报告按单个学生生成。请在${className ? `「${className}」的` : "班级"}名单中选择学生，然后点击「生成家长报告」。`}
        </StudioEmpty>
      ) : (
        <div className="teacher-row-list">
          {rows.map((r) => (
            <button
              key={r.id}
              type="button"
              className="teacher-row-card teacher-report-card"
              aria-label={`${r.studentName} ${rangeLabel(r.rangeStart, r.rangeEnd)}`}
              onClick={() => onOpen(r.id)}
            >
              <span className="teacher-avatar">{Array.from(r.studentName || "—")[0]}</span>
              <span className="teacher-row-card-copy">
                <strong>{r.studentName || "—"}</strong>
                <small>
                  <span>{rangeLabel(r.rangeStart, r.rangeEnd) || "—"}</span>
                  <span>创建于 {publishedMonthDay(r.createdAt) || "—"}</span>
                </small>
              </span>
              <Icon icon={ArrowRight} size={18} />
            </button>
          ))}
        </div>
      )}
    </TeacherPage>
  );
}
