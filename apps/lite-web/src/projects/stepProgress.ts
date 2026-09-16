import { STEP_STATUS_LABELS, type StepStatus } from "../api/projectRoom";

export interface StepProgressInput {
  status: string;
  progress?: "todo" | "doing" | "done";
}

/** Plan commitment and actual execution are separate. Explicit execution wins,
 * except cancelled steps which remain skipped rather than counted as finished. */
export function executionStatus(step: StepProgressInput): string {
  if (step.status === "cancelled" || step.status === "skipped") return step.status;
  return step.progress ?? step.status;
}

export function executionLabel(step: StepProgressInput): string {
  const status = executionStatus(step);
  if (status === "todo") return "待开始";
  if (status === "doing") return "进行中";
  if (status === "skipped") return "已跳过";
  return STEP_STATUS_LABELS[status as StepStatus] ?? "待完成";
}

export function completedSteps(steps: StepProgressInput[]): number {
  return steps.filter(step => executionStatus(step) === "done").length;
}
