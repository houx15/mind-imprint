import type { CSSProperties } from "react";
import type { RosterRow } from "../api/teacher";

const areas = [
  // `unit` keeps 「3 / 4」 from reading as a head count in a class of four:
  // these count items, not students (real-user walk, 2026-09-17).
  { label: "阅读", unit: "篇", done: "readingsDone", total: "readingsTotal", color: "var(--mk-accent-500)" },
  { label: "写作", unit: "篇", done: "writingsDone", total: "writingsTotal", color: "var(--teacher-chart-writing)" },
  { label: "项目", unit: "个", done: "projectsDone", total: "projectsTotal", color: "var(--teacher-chart-project)" },
] as const;

function Ring({ value, total, label }: { value: number; total: number; label: string }) {
  const percent = total > 0 ? Math.min(100, Math.max(0, value / total * 100)) : 0;
  return <div className="teacher-ring" role="img" aria-label={`${label}：${value} / ${total}`}>
    <svg viewBox="0 0 120 120" aria-hidden="true"><circle className="teacher-ring-track" cx="60" cy="60" r="49" /><circle className="teacher-ring-fill" cx="60" cy="60" r="49" pathLength="100" strokeDasharray={`${percent} 100`} transform="rotate(-90 60 60)" /></svg>
    <div><strong>{total > 0 ? `${Math.round(percent)}%` : "—"}</strong><span>{label}</span></div>
  </div>;
}

function CompletionBars({ rows }: { rows: RosterRow[] }) {
  return <div className="teacher-completion-bars"><p className="teacher-chart-caption">学习完成情况 <span>已完成 / 总数</span></p>{areas.map(area => {
    const done = rows.reduce((sum, row) => sum + row[area.done], 0);
    const total = rows.reduce((sum, row) => sum + row[area.total], 0);
    return <div className="teacher-completion-row" key={area.label} style={{ "--chart-color": area.color } as CSSProperties}>
      <div><span>{area.label}</span><strong>{done}<small> / {total} {area.unit}</small></strong></div>
      <div className="teacher-bar-track" role="img" aria-label={`${area.label}：已完成 ${done} ${area.unit}，共 ${total} ${area.unit}`} title={`${area.label}：${done} / ${total}`}><span style={{ width: `${total > 0 ? Math.min(100, done / total * 100) : 0}%` }} /></div>
    </div>;
  })}</div>;
}

export function LearningSnapshot({ rows, compact = false, selectedDays, onSelectDays, onOpenStudent }: {
  rows: RosterRow[];
  compact?: boolean;
  selectedDays?: number | null;
  onSelectDays?: (days: number | null) => void;
  onOpenStudent?: (userId: string) => void;
}) {
  const active = rows.filter(row => row.activeDaysThisWeek > 0).length;
  const bins = Array.from({ length: 8 }, (_, days) => rows.filter(row => row.activeDaysThisWeek === days).length);
  const max = Math.max(1, ...bins);
  const leaders = rows.filter((row) => row.activeDaysThisWeek > 0)
    .sort((a, b) => b.activeDaysThisWeek - a.activeDaysThisWeek || b.minutesThisWeek - a.minutesThisWeek)
    .slice(0, 3);
  return <div className={`teacher-learning-snapshot${compact ? " is-compact" : ""}`}>
    <div className="teacher-participation"><Ring value={active} total={rows.length} label="本周活跃" /><p><strong>{active}</strong> / {rows.length} 位学生</p></div>
    <CompletionBars rows={rows} />
    {!compact && <div className="teacher-active-students"><p className="teacher-chart-caption">本周活跃学生 <span>按活跃天数排序</span></p>{leaders.length ? leaders.map((student, i) => <button key={student.id} type="button" onClick={() => onOpenStudent?.(student.id)} disabled={!onOpenStudent} className="teacher-active-student"><span className="teacher-active-rank">0{i + 1}</span><span className="teacher-avatar">{Array.from(student.displayName)[0]}</span><span className="teacher-active-name"><strong>{student.displayName}</strong><small>阅读 {student.readingsDone} · 写作 {student.writingsDone} · 项目 {student.projectsDone}</small></span><b>{student.activeDaysThisWeek} 天</b></button>) : <p className="teacher-chart-note">本周暂无学生活跃记录</p>}<details className="teacher-activity-details"><summary>查看全班活跃天数分布</summary><div className="teacher-histogram">{bins.map((count, days) => <button key={days} type="button" disabled={!onSelectDays} aria-pressed={selectedDays === days} aria-label={`活跃 ${days} 天：${count} 人`} onClick={() => onSelectDays?.(selectedDays === days ? null : days)}><span className="teacher-histogram-value">{count}</span><span className="teacher-histogram-column"><span style={{ height: `${count / max * 100}%` }} /></span><small>{days}</small></button>)}</div></details></div>}
  </div>;
}

export function StudentLearningSnapshot({ student }: { student: RosterRow }) {
  return <div className="teacher-student-snapshot"><div><p className="teacher-chart-caption">本周学习参与</p><div className="teacher-week-count"><strong>{student.activeDaysThisWeek}</strong><span>个活跃日</span></div><div className="teacher-day-segments" role="img" aria-label={`本周有 ${student.activeDaysThisWeek} 个活跃日，最多 7 天`}>{Array.from({ length: 7 }, (_, i) => <span key={i} data-active={i < student.activeDaysThisWeek} />)}</div><p className="teacher-chart-note">按活跃天数计，不代表连续学习</p></div><CompletionBars rows={[student]} /></div>;
}
