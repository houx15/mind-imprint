import { useEffect, useState } from "react";
import { Button } from "@/ui";
import { api, type ClassSummary } from "@/api";
import { listAssignments, type AssignmentSummaryDTO } from "../api/assignments";
import { AssignmentCard } from "./AssignmentCard";
import { Select } from "./controls/Select";
import { StudioEmpty, StudioError, StudioHeading, StudioLoading } from "./StudioArtwork";
import { TeacherPage } from "./TeacherPage";
import { errorText, pickClassId, progressFromCounts, readLastClassId, writeLastClassId } from "./assignmentLogic";

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
  const [kind, setKind] = useState("all");
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
                setKind("all");
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
          <section className="teacher-assignment-overview" aria-label="班级作业概览">
            <div className="teacher-assignment-overview-title">
              <span>班级作业概览</span>
              <strong>{rows.length}<small>份作业</small></strong>
              <p>按作业统计，每名学生可计入多份作业。</p>
            </div>
            <div className="teacher-assignment-overview-metrics">
              {([
                ["已完成", rows.reduce((sum, a) => sum + progressFromCounts(a.counts).done, 0), "done"],
                ["进行中", rows.reduce((sum, a) => sum + progressFromCounts(a.counts).inProgress, 0), "active"],
                ["已逾期", rows.reduce((sum, a) => sum + progressFromCounts(a.counts).overdue, 0), "overdue"],
                ["待批改", rows.reduce((sum, a) => sum + a.toGrade, 0), "grading"],
              ] as const).map(([label, count, status]) => (
                <div key={label} data-status={status}><span>{label}</span><strong>{count}<small>人次</small></strong></div>
              ))}
            </div>
          </section>
          <div className="teacher-collection-head">
            <div className="teacher-collection-filters" role="group" aria-label="作业类型">
              {([["all", "全部"], ["reading", "阅读"], ["writing", "写作"], ["project", "项目"]] as const).map(([value, label]) => (
                <button type="button" key={value} aria-pressed={kind === value} onClick={() => setKind(value)}>
                  {label}<span>{rows.filter((a) => value === "all" || a.kind === value).length}</span>
                </button>
              ))}
            </div>
          </div>
          <div className="teacher-task-grid">
            {rows.filter((a) => kind === "all" || a.kind === kind).map((a) => (
              <AssignmentCard key={a.id} assignment={a} onOpen={() => onOpen(a.id)} />
            ))}
            <button type="button" className="teacher-task-new" onClick={openNew}>
              <strong>布置作业</strong>
              <small>用表单填写，或向印记说明要求，由印记填写。</small>
            </button>
          </div>
        </>
      )}
    </TeacherPage>
  );
}
