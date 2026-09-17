import type { WorkspaceCard } from "../../api/teacherWorkspace";
import { formatDeadline } from "../../shared/deadline";
import { statusChipStyle } from "../assignmentLogic";
import { CardTag, FactTile, GroupShell } from "../WeekSummaryCard";
import { assignmentCardRows, classSnapshotView, studentRows, studentsCardTitle, type SnapshotStudent } from "./homeLogic";

// teacher/workspace/WorkspaceCards.tsx — tool result cards on a workspace
// canvas. `StudentsCard` is shared by the homework card and the class
// conversation; the other two are the class conversation's (§12.5).

const CARD = "teacher-panel";

export function StudentsCard({ card }: { card: WorkspaceCard }) {
  const rows = studentRows(card.rows);
  return (
    <div className={CARD}>
      <p className="text-mk-label font-bold text-mk-muted">
        {studentsCardTitle(card.filter)} · {rows.length} 人
      </p>
      {rows.length > 0 && (
        <ul className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-mk-small text-mk-ink">
          {rows.map((s) => (
            <li key={s.id}>{s.name}</li>
          ))}
        </ul>
      )}
    </div>
  );
}

function SnapshotGroup({
  title,
  rows,
  onOpenStudent,
}: {
  title: string;
  rows: SnapshotStudent[];
  onOpenStudent: (userId: string) => void;
}) {
  return (
    <GroupShell title={title} count={rows.length} emptyText="暂无">
      {rows.map((c) => (
        <button
          key={`${c.userId}:${c.code}`}
          type="button"
          disabled={!c.userId}
          onClick={() => onOpenStudent(c.userId)}
          className="w-full rounded-mk-md border border-transparent bg-mk-paper p-3 text-left transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
        >
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-mk-body font-bold text-mk-ink">{c.name}</span>
            {c.label && <CardTag kind={c.kind} label={c.label} />}
          </div>
          {c.evidence && <p className="mt-1 text-mk-small text-mk-muted">{c.evidence}</p>}
        </button>
      ))}
    </GroupShell>
  );
}

export function ClassSnapshotCard({
  card,
  onOpenStudent,
}: {
  card: WorkspaceCard;
  onOpenStudent: (userId: string) => void;
}) {
  const v = classSnapshotView(card.rows);
  if (!v) return null;
  return (
    <div className={CARD}>
      <p className="text-mk-label font-bold text-mk-muted">本周概况</p>
      <div className="teacher-facts">
        <FactTile label="活跃学生" value={`${v.activeStudents}/${v.classSize} 人`} />
        <FactTile label="完成项目数" value={`${v.finished} 项`} />
        <FactTile label="作业完成率" value={v.assignmentRate < 0 ? "—" : `${v.assignmentRate}%`} />
      </div>
      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <SnapshotGroup title="值得表扬" rows={v.praise} onOpenStudent={onOpenStudent} />
        <SnapshotGroup title="需要建议" rows={v.watch} onOpenStudent={onOpenStudent} />
      </div>
    </div>
  );
}

export function AssignmentsCard({
  card,
  onOpenAssignment,
}: {
  card: WorkspaceCard;
  onOpenAssignment: (assignmentId: string) => void;
}) {
  const rows = assignmentCardRows(card.rows);
  return (
    <div className={CARD}>
      <p className="text-mk-label font-bold text-mk-muted">作业 {rows.length} 份</p>
      {rows.length > 0 && (
        <div className="mt-2 flex flex-col gap-2">
          {rows.map((a) => (
            <button
              key={a.id}
              type="button"
              onClick={() => onOpenAssignment(a.id)}
              className="w-full rounded-mk-md border border-transparent bg-mk-paper p-3 text-left transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
            >
              <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                {a.kindLabel && <span className="text-mk-small text-mk-muted">{a.kindLabel}</span>}
                <span className="text-mk-body font-bold text-mk-ink">{a.title}</span>
                {a.dueAt && <span className="text-mk-small text-mk-muted">截止 {formatDeadline(a.dueAt)}</span>}
              </div>
              {a.counts.length > 0 && (
                <div className="mt-1.5 flex flex-wrap gap-1.5">
                  {a.counts.map((c) => (
                    <span
                      key={c.status}
                      className="inline-block whitespace-nowrap rounded-mk-full px-2.5 py-0.5 text-mk-small font-bold tabular-nums"
                      style={statusChipStyle(c.status)}
                    >
                      {c.label} {c.n}
                    </span>
                  ))}
                </div>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
