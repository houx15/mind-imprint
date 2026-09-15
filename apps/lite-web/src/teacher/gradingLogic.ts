// teacher/gradingLogic.ts — pure rules behind the 批改 tab and the grading view.

import {
  LETTER_GRADES,
  type GradingContent,
  type GradingPoint,
  type GradingRow,
  type PointKind,
  type QueueGradingsResult,
  type Rubric,
  type SendGradingsResult,
} from "../api/gradings";

export const POLL_MS = 5000;

export type GradingRowStatus = "not_submitted" | "pending" | "running" | "draft" | "reviewed" | "sent" | "failed";

export const GRADING_STATUS_LABEL: Record<GradingRowStatus, string> = {
  not_submitted: "未提交",
  pending: "待批改",
  running: "批改中",
  draft: "草稿",
  reviewed: "已审阅",
  sent: "已发送",
  failed: "批改失败",
};

/**
 * One row's status. A failed regrade that kept the teacher's previous draft
 * (controller ruling) comes back from the server with `status: "draft"` and
 * `error` still set — it lands in the ordinary draft/reviewed branch here,
 * same as any other draft; nothing in this function needs to special-case
 * it, since `error` is read independently of `status` (`GradingRow.grading`
 * carries both). The failure text is shown alongside the editor via
 * `failureText(row.grading?.error)`, not through a distinct status.
 */
export function gradingRowStatus(row: Pick<GradingRow, "version" | "grading">): GradingRowStatus {
  if (!row.version) return "not_submitted";
  const g = row.grading;
  if (!g) return "pending";
  switch (g.status) {
    case "queued":
    case "running":
      return "running";
    case "failed":
      return "failed";
    case "sent":
      return "sent";
    default:
      return g.reviewedAt ? "reviewed" : "draft";
  }
}

export function gradingStatusHue(status: GradingRowStatus): string {
  switch (status) {
    case "sent":
    case "reviewed":
      return "var(--mk-success)";
    case "failed":
      return "var(--mk-danger)";
    case "running":
    case "draft":
      return "var(--mk-warning)";
    default:
      return "var(--mk-muted)";
  }
}

export function shouldPoll(statuses: readonly string[]): boolean {
  return statuses.some((s) => s === "queued" || s === "running");
}

export function reviewedDraftIds(rows: readonly GradingRow[]): string[] {
  return rows.filter((r) => gradingRowStatus(r) === "reviewed").map((r) => r.grading?.id ?? "");
}

export function failedCount(rows: readonly GradingRow[]): number {
  return rows.filter((r) => gradingRowStatus(r) === "failed").length;
}

export function pendingCount(rows: readonly GradingRow[]): number {
  return rows.filter((r) => gradingRowStatus(r) === "pending").length;
}

export const REGRADE_CONFIRM = "重新批改会覆盖当前修改";

export function failureText(error: string | null): string {
  return `批改失败：${error ?? "没有更多信息"}`;
}

export function sendAllConfirmText(n: number): string {
  return `将发送 ${n} 份已审阅的批改`;
}

export function queuedText(n: number): string {
  return n > 0 ? `已加入批改队列：${n} 份` : "没有待批改的作业";
}

/**
 * 一键AI批改's 200 result, as a single line: `queued`/`failed`/`error` from
 * the server (controller ruling — never a silent no-op). When some or all
 * eligible recipients' enqueue attempts rolled back, the failure count and
 * the server's own `入队失败：…` message are shown alongside whatever did
 * queue. A 503 `grading_enqueue_failed` (every recipient failed) is a
 * different code path — the caller shows that thrown error's message
 * instead of calling this.
 */
export function queueResultText(r: QueueGradingsResult): string {
  const parts: string[] = [];
  if (r.queued > 0) parts.push(queuedText(r.queued));
  // `r.error` already reads "入队失败：{backend text}" (set server-side); shown
  // parenthesized after the count instead of re-wrapped in a second
  // "入队失败：" prefix.
  if (r.failed > 0) parts.push(`入队失败 ${r.failed} 份${r.error ? `（${r.error}）` : ""}`);
  return parts.length > 0 ? parts.join("；") : queuedText(0);
}

/** 发送全部已审阅's result, as a single line. `skipped` (a row's status
 * changed under her, or its student left the class, between opening the
 * confirm and the request landing) is explained, not left as a bare number. */
export function sendResultText(r: SendGradingsResult): string {
  if (r.skipped === 0) return `已发送 ${r.sent} 份`;
  return `已发送 ${r.sent} 份，跳过 ${r.skipped} 份（批改状态已变化或学生已不在班级）`;
}

/** Same rule as liteassign.GradeInScale. */
export function gradeInScale(rubric: Rubric, grade: string): boolean {
  if (rubric.scale === "letter") return (LETTER_GRADES as readonly string[]).includes(grade);
  if (!/^(0|[1-9]\d*)$/.test(grade)) return false;
  return Number(grade) <= (rubric.max ?? 0);
}

export type GradingAction =
  | { type: "load"; content: GradingContent }
  | { type: "overallGrade"; value: string }
  | { type: "overallComment"; value: string }
  | { type: "dimensionGrade"; index: number; value: string }
  | { type: "dimensionComment"; index: number; value: string }
  | { type: "pointKind"; index: number; value: PointKind }
  | { type: "pointQuote"; index: number; value: string | null }
  | { type: "pointText"; index: number; value: string }
  | { type: "pointAction"; index: number; value: string }
  | { type: "deletePoint"; index: number }
  | { type: "addPoint" };

function editPoint(c: GradingContent, index: number, patch: Partial<GradingPoint>): GradingContent {
  return { ...c, points: c.points.map((p, i) => (i === index ? { ...p, ...patch } : p)) };
}

function editDimension(c: GradingContent, index: number, patch: Partial<GradingContent["dimensions"][number]>): GradingContent {
  return { ...c, dimensions: c.dimensions.map((d, i) => (i === index ? { ...d, ...patch } : d)) };
}

export function gradingContentReducer(state: GradingContent, action: GradingAction): GradingContent {
  switch (action.type) {
    case "load":
      return action.content;
    case "overallGrade":
      return { ...state, overall: { ...state.overall, grade: action.value } };
    case "overallComment":
      return { ...state, overall: { ...state.overall, comment: action.value } };
    case "dimensionGrade":
      return editDimension(state, action.index, { grade: action.value });
    case "dimensionComment":
      return editDimension(state, action.index, { comment: action.value });
    case "pointKind": {
      // A "good" point's action is nulled at save time (`contentForSave`),
      // not here — leaving whatever text she already typed untouched in
      // memory means switching issue → good → issue restores it, instead of
      // silently losing it to a reset "" (fix round 1).
      if (action.value === "good") return editPoint(state, action.index, { kind: "good" });
      const kept = state.points[action.index]?.action ?? "";
      return editPoint(state, action.index, { kind: "issue", action: kept });
    }
    case "pointQuote":
      return editPoint(state, action.index, { quote: action.value });
    case "pointText":
      return editPoint(state, action.index, { text: action.value });
    case "pointAction":
      return editPoint(state, action.index, { action: action.value });
    case "deletePoint":
      return { ...state, points: state.points.filter((_, i) => i !== action.index) };
    case "addPoint":
      return { ...state, points: [...state.points, { kind: "issue", quote: null, text: "", action: "", source: "teacher" }] };
  }
}

const blankToNull = (v: string | null): string | null => {
  const t = v?.trim() ?? "";
  return t === "" ? null : t;
};

/** The body PATCH sends: trimmed, blank quote/action as null, good points without an action. */
export function contentForSave(c: GradingContent): GradingContent {
  return {
    overall: { grade: c.overall.grade.trim(), comment: c.overall.comment.trim() },
    dimensions: c.dimensions.map((d) => ({ name: d.name, grade: d.grade.trim(), comment: d.comment.trim() })),
    points: c.points.map((p) => ({
      kind: p.kind,
      quote: blankToNull(p.quote),
      text: p.text.trim(),
      action: p.kind === "good" ? null : blankToNull(p.action),
      source: p.source,
    })),
  };
}

/** The checks worth making before a request; the server's shape check is the authority. */
export function validateGradingContent(c: GradingContent, rubric: Rubric): string | null {
  if (!gradeInScale(rubric, c.overall.grade.trim())) return `总评的等级不在评分标准内：${c.overall.grade}`;
  for (const d of c.dimensions) {
    if (!gradeInScale(rubric, d.grade.trim())) return `维度「${d.name}」的等级不在评分标准内：${d.grade}`;
  }
  for (let i = 0; i < c.points.length; i++) {
    if (c.points[i]!.text.trim() === "") return `第 ${i + 1} 条意见的说明为空`;
  }
  return null;
}
