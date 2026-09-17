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
        <div className="teacher-task-grid">
          {rows.map((a) => (
            <AssignmentCard key={a.id} assignment={a} onOpen={() => onOpen(a.id)} />
          ))}
          <button type="button" className="teacher-task-new" onClick={openNew}>
            <strong>布置作业</strong>
            <small>用表单填写，或向印记说明要求，由印记填写。</small>
          </button>
        </div>
      )}
    </TeacherPage>
  );
}
