import { BookOpen, Clock3, MessageCircle, PenLine } from "lucide-react";
import type { ReportStat } from "../api/reports";
import "./report-visuals.css";

/** Counts are compared only within a count-based activity chart. Minutes and
 * words remain independent measures; no score or invented goal is inferred. */
export function ReportVisualSummary({ stats }: { stats: ReportStat[] }) {
  const visible = stats.filter(stat => Number.isFinite(stat.value) && stat.value > 0);
  if (!visible.length) return null;
  const activityKeys = new Set(["highlights", "notes", "lenses", "outline", "snippets", "comments", "stepsDone"]);
  const activities = visible.filter(stat => activityKeys.has(stat.key));
  const metrics = visible.filter(stat => !activityKeys.has(stat.key));
  const max = Math.max(1, ...activities.map(stat => stat.value));
  return <section className="report-visual-summary" aria-label="学习数据概览">
    {metrics.length > 0 && <div className="report-metric-grid">{metrics.map(stat => {
      const Icon = stat.key === "focusMinutes" ? Clock3 : stat.key === "chatTurns" ? MessageCircle : stat.key === "words" ? PenLine : BookOpen;
      return <div className="report-metric" key={stat.key}><Icon size={23} aria-hidden="true" /><div><strong>{stat.value.toLocaleString("zh-CN")}</strong><span>{stat.unit}</span></div><p>{stat.label}</p></div>;
    })}</div>}
    {activities.length > 0 && <div className="report-activity"><header><h3>学习活动记录</h3><span>数量 · 0—{max}</span></header>{activities.map((stat, i) => <div className="report-activity-row" key={stat.key} style={{ "--activity-color": ["var(--mk-accent-500)", "var(--mk-taro-fg)", "var(--mk-peach-fg)"][i % 3] } as React.CSSProperties}><div><span>{stat.label}</span><strong>{stat.value} <small>{stat.unit}</small></strong></div><div className="report-activity-track" role="img" aria-label={`${stat.label}：${stat.value}${stat.unit}`} title={`${stat.label}：${stat.value}${stat.unit}`}><span style={{ width: `${stat.value / max * 100}%` }} /></div></div>)}<p>各项活动分别统计，可在同一次学习中发生。</p></div>}
  </section>;
}
