import { STEP_STATUS_LABELS, type StepStatus } from "../api/projectRoom";
import "../reports/report-visuals.css";

export function ProjectProgressVisual({ steps, showSteps = true, total = steps.length, done = steps.filter(step => step.status === "done").length }: {
  steps: { title: string; status: string }[];
  total?: number;
  showSteps?: boolean;
  done?: number;
}) {
  if (total === 0 && steps.length === 0) return null;
  const percent = total > 0 ? Math.min(100, Math.max(0, done / total * 100)) : 0;
  const skipped = steps.filter(step => (step.status === "skipped" || step.status === "cancelled")).length;
  return <section className="project-progress-visual" aria-label="项目完成进度">
    <div className="project-progress-heading"><div><span>项目进度</span><strong>{Math.round(percent)}<small>%</small></strong></div><p>已完成 <b>{done}</b> / {total} 个步骤{skipped > 0 && <span> · 已跳过 {skipped} 个</span>}</p></div>
    <div className="project-progress-track" role="progressbar" aria-label="已完成步骤比例" aria-valuenow={done} aria-valuemin={0} aria-valuemax={Math.max(total, done)}><span style={{ width: `${percent}%` }} /></div>
    {showSteps && steps.length > 0 && <ol className="project-progress-path">{steps.map((step, i) => <li key={i} data-status={step.status} title={step.title}><span aria-hidden="true">{step.status === "done" ? "✓" : (step.status === "skipped" || step.status === "cancelled") ? "−" : String(i + 1).padStart(2, "0")}</span><div><strong>{step.title}</strong><small>{STEP_STATUS_LABELS[step.status as StepStatus] ?? (step.status === "skipped" ? "已跳过" : "待完成")}</small></div></li>)}</ol>}
  </section>;
}
