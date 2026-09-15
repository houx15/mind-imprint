import type { CSSProperties } from "react";
import type { RosterRow } from "../api/teacher";

const areas = [
  { label: "阅读", done: "readingsDone", total: "readingsTotal", color: "var(--mk-accent-500)" },
  { label: "写作", done: "writingsDone", total: "writingsTotal", color: "var(--teacher-chart-writing)" },
  { label: "项目", done: "projectsDone", total: "projectsTotal", color: "var(--teacher-chart-project)" },
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
      <div><span>{area.label}</span><strong>{done}<small> / {total}</small></strong></div>
      <div className="teacher-bar-track" role="img" aria-label={`${area.label}：已完成 ${done} 项，共 ${total} 项`} title={`${area.label}：${done} / ${total}`}><span style={{ width: `${total > 0 ? Math.min(100, done / total * 100) : 0}%` }} /></div>
    </div>;
  })}</div>;
}

export function LearningSnapshot({ rows, compact = false, selectedDays, onSelectDays }: {
  rows: RosterRow[];
  compact?: boolean;
  selectedDays?: number | null;
  onSelectDays?: (days: number | null) => void;
}) {
  const active = rows.filter(row => row.activeDaysThisWeek > 0).length;
  const bins = Array.from({ length: 8 }, (_, days) => rows.filter(row => row.activeDaysThisWeek === days).length);
  const max = Math.max(1, ...bins);
  return <div className={`teacher-learning-snapshot${compact ? " is-compact" : ""}`}>
    <div className="teacher-participation"><Ring value={active} total={rows.length} label="本周活跃" /><p><strong>{active}</strong> / {rows.length} 位学生</p></div>
    <CompletionBars rows={rows} />
    {!compact && <div className="teacher-activity-chart"><div className="teacher-chart-caption">本周活跃天数分布 <span>人数</span></div><div className="teacher-histogram">{bins.map((count, days) => <button key={days} type="button" disabled={!onSelectDays} aria-pressed={selectedDays === days} aria-label={`活跃 ${days} 天：${count} 人`} title={`活跃 ${days} 天：${count} 人，点击筛选名单`} onClick={() => onSelectDays?.(selectedDays === days ? null : days)}><span className="teacher-histogram-value">{count}</span><span className="teacher-histogram-column"><span style={{ height: `${count / max * 100}%` }} /></span><small>{days}</small></button>)}</div><p className="teacher-chart-note">点击柱形筛选学生 · 横轴为活跃天数</p></div>}
  </div>;
}

export function StudentLearningSnapshot({ student }: { student: RosterRow }) {
  return <div className="teacher-student-snapshot"><div><p className="teacher-chart-caption">本周学习参与</p><div className="teacher-week-count"><strong>{student.activeDaysThisWeek}</strong><span>个活跃日</span></div><div className="teacher-day-segments" role="img" aria-label={`本周有 ${student.activeDaysThisWeek} 个活跃日，最多 7 天`}>{Array.from({ length: 7 }, (_, i) => <span key={i} data-active={i < student.activeDaysThisWeek} />)}</div><p className="teacher-chart-note">按活跃天数计，不代表连续学习</p></div><CompletionBars rows={[student]} /></div>;
}
