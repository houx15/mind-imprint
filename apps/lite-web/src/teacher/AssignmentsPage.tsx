import { useEffect, useState } from "react";
import { Button } from "@/ui";
import { api, type ClassSummary } from "@/api";
import { listAssignments, type AssignmentSummaryDTO } from "../api/assignments";
import { AssignmentCard } from "./AssignmentCard";
import { Select } from "./controls/Select";
import { StudioEmpty, StudioError, StudioHeading, StudioLoading } from "./StudioArtwork";
import { TeacherPage } from "./TeacherPage";
import { errorText, pickClassId, readLastClassId, writeLastClassId } from "./assignmentLogic";

/**
 * AssignmentsPage — `/assignments`: one class's assignments as cards, each
 * with its progress per derived status. The class choice is remembered in localStorage
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
  const [kind, setKind] = useState("reading");
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

  const openNew = () => {
    writeLastClassId(classId);
    onNew(classId);
  };

  return (
    <TeacherPage width="wide">
      <StudioHeading
        kicker="作业"
        title="作业与进度"
        description="作业显示在学生首页和收件箱。请按班级查看进度，写作作业可在作业页批改。"
        kind="ideas"
        actions={
          classes && classes.length > 0 ? (
            <Button variant="primary" onClick={openNew}>
              布置作业
            </Button>
          ) : undefined
        }
      />

      {classes && classes.length > 0 && (
        <div className="teacher-toolbar">
          <label className="teacher-toolbar-field">
            班级
            <Select
              size="sm"
              ariaLabel="班级"
              value={classId}
              onChange={(id) => {
                setClassId(id);
                setKind("reading");
                writeLastClassId(id);
              }}
              options={classes.map((c) => ({ value: c.id, label: c.name }))}
            />
          </label>
          {rows && rows.length > 0 && <span className="teacher-section-aside">共 {rows.length} 份作业</span>}
        </div>
      )}

      {classesError ? (
        <StudioError message={classesError} onRetry={() => setClassesNonce((n) => n + 1)} />
      ) : classes === null ? (
        <StudioLoading />
      ) : classes.length === 0 ? (
        <StudioEmpty kind="discovery" title="暂无班级">
          布置作业需要先有班级。请联系管理员为你分配班级。
        </StudioEmpty>
      ) : rowsError ? (
        <StudioError message={rowsError} onRetry={() => setRowsNonce((n) => n + 1)} />
      ) : rows === null ? (
        <StudioLoading />
      ) : rows.length === 0 ? (
        <StudioEmpty kind="writing" title="暂无作业" action={{ label: "布置作业", onClick: openNew }}>
          作业可以是一篇阅读、一篇写作或一个项目。请为这个班布置第一份作业。
        </StudioEmpty>
      ) : (
        <>
          {(rows.some((a) => a.issueCount > 0 || a.toGrade > 0 || a.needsReadingReview > 0 || a.counts.overdue > 0)) && <div className="teacher-assignment-priority" role="status"><strong>需要关注</strong><span>{rows.reduce((n, a) => n + a.issueCount, 0)} 条材料反馈 · {rows.reduce((n, a) => n + a.toGrade, 0)} 份待批改 · {rows.reduce((n, a) => n + a.needsReadingReview, 0)} 人待继续阅读 · {rows.reduce((n, a) => n + a.counts.overdue, 0)} 人次逾期</span></div>}
          <div className="teacher-collection-head">
            <div className="teacher-collection-filters" role="group" aria-label="作业类型">
              {([["reading", "阅读"], ["writing", "写作"], ["project", "项目"], ["all", "全部"]] as const).map(([value, label]) => (
                <button type="button" key={value} aria-pressed={kind === value} onClick={() => setKind(value)}>
                  {label}<span>{rows.filter((a) => value === "all" || a.kind === value).length}</span>
                </button>
              ))}
            </div>
          </div>
          <div className="teacher-assignment-list">
            {rows.filter((a) => kind === "all" || a.kind === kind).sort((a, b) => b.issueCount - a.issueCount || b.toGrade - a.toGrade || b.needsReadingReview - a.needsReadingReview || b.counts.overdue - a.counts.overdue || Date.parse(a.dueAt) - Date.parse(b.dueAt)).map((a) => (
              <AssignmentCard key={a.id} assignment={a} compact onOpen={() => onOpen(a.id)} />
            ))}
            {rows.filter((a) => kind === "all" || a.kind === kind).length === 0 && <p className="teacher-group-empty">暂无{kind === "reading" ? "阅读" : kind === "writing" ? "写作" : "项目"}作业</p>}
          </div>
        </>
      )}
    </TeacherPage>
  );
}
