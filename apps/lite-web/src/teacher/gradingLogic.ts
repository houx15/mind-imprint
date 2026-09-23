// teacher/gradingLogic.ts — pure rules behind the 批改 tab and the grading view.

import {
  LETTER_GRADES,
  type GradingContent,
  type GradingPoint,
  type GradingRow,
  type GradingStatus,
  type PointKind,
  type QueueGradingsResult,
  type GradingSource,
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

export type GradingStepState = "done" | "current" | "todo";

/**
 * The 批改 → 审阅 → 发送 guide above the 批改 tab: one state and one count
 * line per step. The current step is the first one with work left: an
 * ungraded or failed submission (批改), else a draft (审阅), else a reviewed
 * draft (发送). With nothing submitted every step is `todo`; with every
 * submission sent every step is `done`.
 */
export function gradingSteps(rows: readonly Pick<GradingRow, "version" | "grading">[]): {
  submitted: number;
  steps: { key: "grade" | "review" | "send"; label: string; state: GradingStepState; note: string }[];
} {
  const count = (s: GradingRowStatus) => rows.filter((r) => gradingRowStatus(r) === s).length;
  const submitted = rows.filter((r) => r.version !== null).length;
  const toGrade = count("pending") + count("failed");
  const running = count("running");
  const drafts = count("draft");
  const reviewed = count("reviewed");
  const sent = count("sent");
  const work = [toGrade + running > 0, drafts > 0, reviewed > 0];
  const current = submitted === 0 ? -1 : work.findIndex(Boolean);
  const state = (i: number): GradingStepState => {
    if (submitted === 0) return "todo";
    if (current === -1) return "done";
    return i < current ? "done" : i === current ? "current" : "todo";
  };
  const gradeNote = [toGrade > 0 ? `待批改 ${toGrade}` : "", running > 0 ? `批改中 ${running}` : ""].filter(Boolean).join(" · ");
  return {
    submitted,
    steps: [
      { key: "grade", label: "批改", state: state(0), note: gradeNote || "无待批改" },
      { key: "review", label: "审阅", state: state(1), note: drafts > 0 ? `待审阅 ${drafts}` : "无待审阅" },
      { key: "send", label: "发送", state: state(2), note: `已发送 ${sent}/${submitted}` },
    ],
  };
}

/**
 * 起草 → 审阅 → 发送 on the grading page, from one grading's state. A
 * failed first run (no content) stays on 起草; a sent grading is done and
 * says whether the student has read it.
 */
export function gradingPageSteps(g: {
  status: GradingStatus;
  hasContent: boolean;
  reviewedAt: string | null;
  studentSeenAt: string | null;
}): { label: string; state: GradingStepState; note?: string }[] {
  const at = (current: number, notes: (string | undefined)[] = []) =>
    ["起草", "审阅", "发送"].map((label, i) => ({
      label,
      state: (i < current ? "done" : i === current ? "current" : "todo") as GradingStepState,
      note: notes[i],
    }));
  switch (g.status) {
    case "queued":
    case "running":
      return at(0, ["批改中"]);
    case "failed":
      return g.hasContent ? at(g.reviewedAt ? 2 : 1) : at(0, ["批改失败"]);
    case "sent":
      return at(3, [undefined, undefined, g.studentSeenAt ? "学生已读" : "学生未读"]);
    default:
      return g.reviewedAt ? at(2, [undefined, "已审阅"]) : at(1, [undefined, "待审阅"]);
  }
}

export const REGRADE_CONFIRM = "重新批改会覆盖当前修改";

/** The button that asks the AI to grade a row that already exists: a 人工批改
 *  was never graded by the AI, so 「重新」 would be wrong there. */
export function regradeLabel(source: GradingSource): string {
  return source === "teacher" ? "AI 批改" : "重新批改";
}

export function regradeConfirmText(source: GradingSource): string {
  return source === "teacher" ? "AI 批改会覆盖当前内容" : REGRADE_CONFIRM;
}

/** Shown in place of the editor while the AI is grading. The second line
 *  only applies to a regrade: SetLiteGradingFailed keeps the old content. */
export const GRADING_RUNNING_TEXT = "AI 正在批改，通常需要 1 分钟左右。可以先离开此页，稍后在作业的「批改」中查看。";
export const GRADING_RUNNING_KEEPS_TEXT = "批改失败时保留当前内容。";

export type GradingAction =
  | "save"
  | "review"
  | "send"
  | "regrade"
  | "manual";

/**
 * The line shown after an action on the grading page succeeds. Before
 * 2026-09-17 a successful save showed nothing, so 保存并发送 looked like it
 * did nothing although it had re-sent the grading to the student.
 * `wasSent`: the row was already sent before this save (保存并发送).
 */
export function gradingDoneText(action: GradingAction, wasSent: boolean): string | null {
  switch (action) {
    case "save":
      return wasSent ? "已保存并重新发送，学生端显示为未读" : "已保存";
    case "review":
      return "已标记为已审阅";
    case "send":
      return "已发送给学生";
    case "regrade":
      return null;
    case "manual":
      return "已改为人工批改，请填写等级和意见后保存";
  }
}

/** Shown before leaving the grading view with unsaved edits — both the
 *  page's own inline 返回 confirm and the shell's cross-component navigation
 *  guard (rail links, browser back) use this exact text, so the two guards
 *  read as one behaviour rather than two differently-worded prompts. */
export const LEAVE_UNSAVED_CONFIRM = "有未保存的修改，确定要离开吗？";

/**
 * Names one point's repeated controls (删除/选择引文/清除, 类型/说明/修改建议)
 * for their `aria-label`s — every point card renders the same control names,
 * which read identically to a screen reader across every point. Positional,
 * not quoted, since (unlike a rubric dimension) a point has no name of its
 * own to quote — same idea as rubricLogic's `rubricDimensionLabel`.
 */
export function gradingPointLabel(index: number, field: string): string {
  return `意见 ${index + 1} ${field}`;
}

export function failureText(error: string | null): string {
  return `批改失败：${error ?? "没有更多信息"}`;
}

/**
 * Whether one point has anything to show in the 依据 modal: which rubric
 * dimension it belongs to, which symptom it matched, or the sentence it
 * quotes. All three can be blank — 人工批改 has none of them, and a model
 * point can lose one or both to SanitizeProvenance (apps/api/internal/
 * litegrade/check.go) when the value it gave didn't match anything real.
 *
 * 🚨 Nothing behind it → no button. A modal that opens empty tells her less
 * than not offering one at all (task-3-brief, 2026-09-23).
 */
export function pointHasBasis(p: Pick<GradingPoint, "dimension" | "symptom" | "quote">): boolean {
  return p.dimension.trim() !== "" || p.symptom.trim() !== "" || (p.quote ?? "").trim() !== "";
}

/** The grading card's two views: the form, or the finished grading as the
 *  student will read it. */
export type GradingMode = "edit" | "preview";

/**
 * Which view the card opens on. A grading she has already sent or marked
 * 已审阅 is something she comes back to READ — it opens on 预览, where the
 * whole grading is one page of text instead of a column of input boxes. Work
 * still in progress opens on 编辑.
 */
export function initialGradingMode(status: GradingStatus, reviewedAt: string | null): GradingMode {
  return status === "sent" || reviewedAt ? "preview" : "edit";
}

/** A field's value in 预览, or what is still missing. Blank is never silently
 *  rendered as nothing — 预览 has to show the same gaps the form does. */
export function previewValue(value: string | null): { text: string; filled: boolean } {
  const t = (value ?? "").trim();
  return t === "" ? { text: "待填写", filled: false } : { text: t, filled: true };
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

export type GradingContentAction =
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

export function gradingContentReducer(state: GradingContent, action: GradingContentAction): GradingContent {
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
      return {
        ...state,
        // A point she types herself has no model provenance to show.
        points: [...state.points, { kind: "issue", quote: null, text: "", action: "", source: "teacher", dimension: "", symptom: "" }],
      };
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
      // Not editable from this page — carried through as received, the
      // server re-derives/clears them from the rubric and the symptom
      // table on save regardless (litegrade.SanitizeProvenance).
      dimension: p.dimension,
      symptom: p.symptom,
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
