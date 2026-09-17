import { useEffect, useMemo, useReducer, useRef, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { Button, Icon, Pebble } from "@/ui";
import { getAssignment, type RecipientDTO } from "../api/assignments";
import {
  getGrading,
  LETTER_GRADES,
  patchGrading,
  regradeGrading,
  sendGrading,
  type GradingContent,
  type Rubric,
  type TeacherGrading,
} from "../api/gradings";
import { formatDeadline } from "../shared/deadline";
import { highlightSegments, MARK_STYLE, pickableSentences, PIECE_CLS, quoteRanges, unmarkedPointQuotes } from "../shared/gradingText";
import { useAlive } from "../shared/useAlive";
import { errorText, failText } from "./assignmentLogic";
import { INPUT_CLS } from "./formParts";
import {
  contentForSave,
  failureText,
  gradingContentReducer,
  gradingDoneText,
  gradingPointLabel,
  GRADING_RUNNING_KEEPS_TEXT,
  GRADING_RUNNING_TEXT,
  LEAVE_UNSAVED_CONFIRM,
  POLL_MS,
  regradeConfirmText,
  regradeLabel,
  shouldPoll,
  validateGradingContent,
  type GradingAction,
} from "./gradingLogic";
import { ReturnDialog } from "./ReturnDialog";
import { TeacherPage } from "./TeacherPage";

const EMPTY: GradingContent = { overall: { grade: "", comment: "" }, dimensions: [], points: [] };

/**
 * GradingPage — `/gradings/:gid`. Left: the graded version with quoted
 * sentences highlighted (`MARK_STYLE`/`PIECE_CLS` shared with
 * FinishedWritingPage, Task 14 — defined once in shared/gradingText.ts, not
 * redefined here). Right: the editable grading and its actions. While a
 * teacher point picks its quote, the left side lists her sentences as
 * buttons.
 */
export function GradingPage({
  gradingId,
  onBack,
  onDirtyChange,
}: {
  gradingId: string;
  onBack: (g: TeacherGrading | null) => void;
  /** Reports unsaved-edit state up to the shell, which needs it
   *  synchronously (inside a click/popstate handler, not a render) to guard
   *  navigation that does not go through this page's own 返回 button — rail
   *  links, the brand link, browser Back. */
  onDirtyChange?: (dirty: boolean) => void;
}) {
  const alive = useAlive();
  const [grading, setGrading] = useState<TeacherGrading | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [content, dispatch] = useReducer(gradingContentReducer, EMPTY);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  // The result of the last action: an error, or what just happened.
  const [message, setMessage] = useState<{ tone: "error" | "done"; text: string } | null>(null);
  const [confirmRegrade, setConfirmRegrade] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [picking, setPicking] = useState<number | null>(null);
  const [returning, setReturning] = useState<RecipientDTO | null>(null);
  const [nonce, setNonce] = useState(0);

  // Mirrors `dirty` for the poll path below, which must always read the
  // CURRENT value even though its effect only re-runs on [gradingId, nonce]
  // (fix round 1: reading `dirty` directly there captured whatever it was
  // when the effect was created, not the latest keystroke, so a poll could
  // silently overwrite her typing after her first edit).
  const dirtyRef = useRef(dirty);
  dirtyRef.current = dirty;

  useEffect(() => {
    onDirtyChange?.(dirty);
    return () => onDirtyChange?.(false);
    // `onDirtyChange` is a setter into the shell's ref, not a value this
    // effect should re-run for on every shell render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dirty]);

  // Closing or reloading the tab bypasses every in-app guard below.
  useEffect(() => {
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (!dirty) return;
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [dirty]);

  function apply(g: TeacherGrading, replaceContent: boolean) {
    setGrading(g);
    if (replaceContent && g.content) {
      dispatch({ type: "load", content: g.content });
      setDirty(false);
    }
  }

  // Polling is a self-scheduling chain, not a fixed-tick `setInterval`:
  // `getGrading` only reschedules the NEXT tick after it settles (success
  // or error), so a response slower than 5s is never discarded by a new
  // request starting on top of it, and two requests are never in flight at
  // once. It stops on its own once the row is no longer queued/running.
  useEffect(() => {
    let cancelled = false;
    let timer: number | undefined;

    async function load() {
      try {
        const g = await getGrading(gradingId);
        if (cancelled) return;
        setLoadError(null);
        // A poll must not overwrite what she is typing.
        apply(g, !dirtyRef.current);
        if (shouldPoll([g.status])) {
          timer = window.setTimeout(() => void load(), POLL_MS);
        }
      } catch (e) {
        if (!cancelled) setLoadError(errorText(e));
      }
    }

    void load();
    return () => {
      cancelled = true;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [gradingId, nonce]);

  const edit = (action: Parameters<typeof dispatch>[0]) => {
    dispatch(action);
    setDirty(true);
    setMessage((m) => (m?.tone === "done" ? null : m));
    // A point picking its quote is identified by index; deleting a point
    // shifts every later index, so a picker left open across a delete would
    // silently attach its pick to a DIFFERENT point.
    if (action.type === "deletePoint") setPicking(null);
  };

  const ranges = useMemo(() => (grading ? quoteRanges(grading.body, content.points.map((p) => p.quote)) : []), [grading, content.points]);
  // Ruling 3: a point whose quote got no highlight (not found in the text,
  // or only overlapped an earlier point's already-taken range) shows
  // 「未在正文中标出」 on its own card instead of silently looking fine.
  const unmarked = useMemo(() => new Set(unmarkedPointQuotes(content.points.map((p) => p.quote), ranges)), [content.points, ranges]);

  async function run(verb: string, action: GradingAction, task: () => Promise<TeacherGrading>) {
    if (busy) return;
    const wasSent = grading?.status === "sent";
    setBusy(true);
    setMessage(null);
    try {
      const g = await task();
      if (alive.current) {
        apply(g, true);
        const done = gradingDoneText(action, wasSent);
        if (done) setMessage({ tone: "done", text: done });
        // A regrade (or anything else `run` drives) can hand back a row
        // that is now queued/running — the poll chain only reschedules
        // itself from INSIDE its own effect (fix round 1), so nothing
        // restarts it on its own once it has already stopped. Bumping
        // `nonce` re-runs that effect; its existing cleanup (`cancelled` +
        // `clearTimeout`) means this can never leave two chains running.
        if (shouldPoll([g.status])) setNonce((n) => n + 1);
      }
    } catch (e) {
      if (alive.current) setMessage({ tone: "error", text: failText(verb, e) });
    } finally {
      if (alive.current) setBusy(false);
    }
  }

  function saveContent(): Promise<TeacherGrading> {
    const body = contentForSave(content);
    const invalid = grading ? validateGradingContent(body, grading.rubric) : null;
    if (invalid) return Promise.reject(new Error(invalid));
    return patchGrading(gradingId, body);
  }

  async function openReturn() {
    if (!grading?.assignmentId) return;
    try {
      const { recipients } = await getAssignment(grading.assignmentId);
      const r = recipients.find((x) => x.userId === grading.userId);
      if (alive.current) setReturning(r ?? null);
      if (alive.current && !r) setMessage({ tone: "error", text: "退回失败：这名学生不在这份作业中" });
    } catch (e) {
      if (alive.current) setMessage({ tone: "error", text: failText("退回", e) });
    }
  }

  const back = (
    <button
      type="button"
      onClick={() => {
        if (dirty) setConfirmLeave(true);
        else onBack(grading);
      }}
      className="flex items-center gap-1.5 rounded-mk-sm text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <Icon icon={ArrowLeft} size={15} />
      返回
    </button>
  );

  const leaveConfirmBanner = confirmLeave && (
    <div className="mt-2 flex flex-wrap items-center gap-2 text-mk-small font-semibold text-mk-danger">
      {LEAVE_UNSAVED_CONFIRM}
      <Button
        variant="danger"
        size="sm"
        onClick={() => {
          setConfirmLeave(false);
          // The shell's own `go()` reads its dirty ref synchronously, before
          // this page has a chance to unmount — reset it here so leaving
          // does not also trip its cross-component confirm a second time.
          onDirtyChange?.(false);
          onBack(grading);
        }}
      >
        确认离开
      </Button>
      <Button variant="ghost" size="sm" onClick={() => setConfirmLeave(false)}>
        取消
      </Button>
    </div>
  );

  if (loadError) {
    return (
      <TeacherPage>
        {back}
        {leaveConfirmBanner}
        <div className="mt-4 text-mk-small font-semibold text-mk-danger">
          加载失败：{loadError}{" "}
          <button type="button" onClick={() => setNonce((n) => n + 1)} className="cursor-pointer underline">
            重试
          </button>
        </div>
      </TeacherPage>
    );
  }
  if (!grading) {
    return (
      <TeacherPage>
        {back}
        <p className="mt-4 text-mk-body text-mk-muted">加载中…</p>
      </TeacherPage>
    );
  }

  // Ruling 1: a `draft` row can carry a leftover `error` (a failed regrade
  // that kept her previous content) — the editor stays open and usable in
  // that case. Only a row with no content at all (a first grading that
  // failed outright, or one still queued/running) has no editor.
  const editable = grading.content !== null && (grading.status === "draft" || grading.status === "sent");
  const running = grading.status === "queued" || grading.status === "running";
  const canRegrade = grading.status === "draft" || grading.status === "failed";

  return (
    <TeacherPage width="wide" fill>
      <div className="shrink-0">
        {back}
        {leaveConfirmBanner}
        <div className="mt-4 flex flex-wrap items-center gap-3">
          <h1 className="teacher-page-title">
            {grading.displayName} · {grading.title}
          </h1>
          <span className="rounded-mk-full bg-mk-accent-50 px-2.5 py-0.5 text-mk-label font-semibold text-mk-accent-700">
            {grading.source === "teacher" ? "人工批改" : "AI 批改"}
          </span>
          <span className="text-mk-small text-mk-muted">
            v{grading.versionNumber}
            {grading.latestVersionNumber > grading.versionNumber ? ` · 最新 v${grading.latestVersionNumber}` : ""}
          </span>
        </div>
        <p className="mt-1 text-mk-small text-mk-muted">
          {grading.status === "sent" && grading.sentAt
            ? `已发送 ${formatDeadline(grading.sentAt)} · ${grading.studentSeenAt ? "学生已读" : "学生未读"}`
            : running
              ? "批改中"
              : grading.reviewedAt
                ? "已审阅"
                : grading.status === "draft"
                  ? "草稿"
                  : ""}
        </p>
        {grading.error && (
          <p role="alert" className="mt-2 break-words text-mk-small font-semibold text-mk-danger">
            {failureText(grading.error)}
          </p>
        )}
      </div>

      {/* Wide (≥900px): the page is as tall as the window; the essay and the
          editor each scroll on their own, and the buttons with their result
          line sit under the editor, always on screen. Before 2026-09-17 the
          whole right column scrolled as one box and the result of 保存并发送
          appeared below its visible edge — the teacher saw nothing happen. */}
      <div className="teacher-grading-grid mt-6 grid min-h-0 flex-1 grid-cols-1 gap-8 min-[900px]:grid-cols-[minmax(0,44rem)_minmax(20rem,1fr)]">
        <article className="teacher-grading-article mk-scroll min-w-0">
          {picking !== null ? (
            <div className="flex flex-col gap-2">
              <p className="text-mk-small text-mk-muted">请选择一句作为第 {picking + 1} 条意见的引文</p>
              {pickableSentences(grading.body).map((s, i) => (
                <button
                  key={i}
                  type="button"
                  onClick={() => {
                    edit({ type: "pointQuote", index: picking, value: s });
                    setPicking(null);
                  }}
                  className="rounded-mk-sm border border-mk-border px-3 py-2 text-left text-mk-body text-mk-ink hover:bg-mk-paper"
                >
                  {s}
                </button>
              ))}
              <div>
                <Button variant="ghost" size="sm" onClick={() => setPicking(null)}>
                  取消选择
                </Button>
              </div>
            </div>
          ) : (
            <div className={PIECE_CLS}>
              {highlightSegments(grading.body, ranges).map((seg, i) =>
                seg.index === null ? (
                  <span key={i}>{seg.text}</span>
                ) : (
                  <mark key={i} id={`grading-quote-${seg.index}`} style={MARK_STYLE}>
                    {seg.text}
                  </mark>
                ),
              )}
            </div>
          )}
        </article>

        <aside className="teacher-grading-aside flex min-h-0 min-w-0 flex-col gap-4">
          <div className="teacher-grading-editor mk-scroll flex min-h-0 flex-col gap-4 min-[900px]:flex-1 min-[900px]:pr-1">
            {running && (
              <div role="status" className="flex items-start gap-3 rounded-mk-lg border border-mk-border bg-mk-surface p-4">
                <span className="mt-0.5 shrink-0">
                  <Pebble state="thinking" size={28} />
                </span>
                <div className="flex min-w-0 flex-col gap-1.5">
                  <p className="flex items-center gap-2 text-mk-body font-semibold text-mk-ink">
                    批改中
                    <span className="mk-think-dot" />
                    <span className="mk-think-dot [animation-delay:0.15s]" />
                    <span className="mk-think-dot [animation-delay:0.3s]" />
                  </p>
                  <p className="text-mk-small text-mk-muted">{GRADING_RUNNING_TEXT}</p>
                  {grading.content !== null && <p className="text-mk-small text-mk-muted">{GRADING_RUNNING_KEEPS_TEXT}</p>}
                </div>
              </div>
            )}
            {editable ? (
              <GradingEditor
                rubric={grading.rubric}
                content={content}
                unmarked={unmarked}
                onEdit={edit}
                onPickQuote={setPicking}
                onShowQuote={(i) => document.getElementById(`grading-quote-${i}`)?.scrollIntoView({ block: "center", behavior: "smooth" })}
              />
            ) : null}
          </div>

          <div className="flex shrink-0 flex-col gap-2 border-t border-mk-border pt-4">
            <div className="flex flex-wrap gap-2">
              {editable && (
                <Button variant="primary" size="sm" disabled={busy} onClick={() => void run("保存", "save", saveContent)}>
                  {grading.status === "sent" ? "保存并发送" : "保存"}
                </Button>
              )}
              {editable && grading.status === "draft" && !grading.reviewedAt && (
                <Button variant="secondary" size="sm" disabled={busy} onClick={() => void run("审阅", "review", () => (dirty ? saveContent() : patchGrading(gradingId)))}>
                  标记已审阅
                </Button>
              )}
              {editable && grading.status === "draft" && (
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={busy}
                  onClick={() =>
                    void run("发送", "send", async () => {
                      if (dirty) {
                        // Apply the PATCH result now — if `sendGrading` then
                        // fails, the page must show the already-saved content
                        // (reviewedAt set, no stale `dirty`), not the state
                        // from before this save.
                        const saved = await saveContent();
                        if (alive.current) apply(saved, true);
                      }
                      return sendGrading(gradingId);
                    })
                  }
                >
                  发送
                </Button>
              )}
              {canRegrade && (
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={busy}
                  onClick={() => {
                    // A picker left open across a regrade would apply its
                    // pick to whatever content lands after the regrade
                    // completes, not the point she was actually looking at.
                    setPicking(null);
                    // Only a row with content to lose needs the confirm — a
                    // `failed` row (no content) has nothing to overwrite.
                    if (grading.content !== null) setConfirmRegrade(true);
                    else void run(regradeLabel(grading.source), "regrade", () => regradeGrading(gradingId));
                  }}
                >
                  {regradeLabel(grading.source)}
                </Button>
              )}
              {grading.assignmentId && (
                <Button variant="secondary" size="sm" disabled={busy} onClick={() => void openReturn()}>
                  退回修改
                </Button>
              )}
            </div>
            {confirmRegrade && (
              <div className="flex flex-wrap items-center gap-2 text-mk-small font-semibold text-mk-danger">
                {regradeConfirmText(grading.source)}
                <Button
                  variant="danger"
                  size="sm"
                  disabled={busy}
                  onClick={() => {
                    setConfirmRegrade(false);
                    setPicking(null);
                    void run(regradeLabel(grading.source), "regrade", () => regradeGrading(gradingId));
                  }}
                >
                  确认
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setConfirmRegrade(false)}>
                  取消
                </Button>
              </div>
            )}
            {message && (
              <p
                role={message.tone === "error" ? "alert" : "status"}
                className={`break-words text-mk-small font-semibold ${message.tone === "error" ? "text-mk-danger" : "text-mk-success"}`}
              >
                {message.text}
              </p>
            )}
          </div>
        </aside>
      </div>

      {returning && grading.assignmentId && (
        <ReturnDialog
          assignmentId={grading.assignmentId}
          recipient={returning}
          onClose={() => setReturning(null)}
          onReturned={() => {
            setReturning(null);
            setNonce((n) => n + 1);
          }}
        />
      )}
    </TeacherPage>
  );
}

function GradeInput({ rubric, value, onChange, label }: { rubric: Rubric; value: string; onChange: (v: string) => void; label: string }) {
  if (rubric.scale === "letter") {
    return (
      <select aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} className={`${INPUT_CLS} w-24`}>
        {!(LETTER_GRADES as readonly string[]).includes(value) && <option value={value}>{value || "—"}</option>}
        {LETTER_GRADES.map((g) => (
          <option key={g} value={g}>
            {g}
          </option>
        ))}
      </select>
    );
  }
  return (
    <input aria-label={label} type="number" min={0} max={rubric.max} step={1} value={value} onChange={(e) => onChange(e.target.value)} className={`${INPUT_CLS} w-24`} />
  );
}

function GradingEditor({
  rubric,
  content,
  unmarked,
  onEdit,
  onPickQuote,
  onShowQuote,
}: {
  rubric: Rubric;
  content: GradingContent;
  unmarked: ReadonlySet<number>;
  onEdit: (a: Parameters<typeof gradingContentReducer>[1]) => void;
  onPickQuote: (index: number) => void;
  onShowQuote: (index: number) => void;
}) {
  return (
    <div className="flex flex-col gap-4">
      <section className="flex flex-col gap-2">
        <h2 className="text-mk-small font-bold text-mk-ink">总评</h2>
        <GradeInput rubric={rubric} label="总评等级" value={content.overall.grade} onChange={(v) => onEdit({ type: "overallGrade", value: v })} />
        <textarea aria-label="总评评语" rows={3} value={content.overall.comment} onChange={(e) => onEdit({ type: "overallComment", value: e.target.value })} className={INPUT_CLS} />
      </section>

      <section className="flex flex-col gap-2">
        <h2 className="text-mk-small font-bold text-mk-ink">维度</h2>
        {content.dimensions.map((d, i) => (
          <div key={d.name} className="flex flex-col gap-1.5">
            <h3 className="text-mk-small font-semibold text-mk-ink">{d.name}</h3>
            <GradeInput rubric={rubric} label={`${d.name}等级`} value={d.grade} onChange={(v) => onEdit({ type: "dimensionGrade", index: i, value: v })} />
            <textarea aria-label={`${d.name}评语`} rows={2} value={d.comment} onChange={(e) => onEdit({ type: "dimensionComment", index: i, value: e.target.value })} className={INPUT_CLS} />
          </div>
        ))}
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-mk-small font-bold text-mk-ink">意见</h2>
        {content.points.map((p, i) => (
          <div key={i} className="flex flex-col gap-2 rounded-mk-md border border-mk-border bg-mk-surface p-3">
            <div className="flex flex-wrap items-center gap-2">
              <select
                aria-label={gradingPointLabel(i, "类型")}
                value={p.kind}
                onChange={(e) => onEdit({ type: "pointKind", index: i, value: e.target.value === "good" ? "good" : "issue" })}
                className={`${INPUT_CLS} w-24`}
              >
                <option value="good">优点</option>
                <option value="issue">问题</option>
              </select>
              <span className="text-mk-label text-mk-muted">{p.source === "ai" ? "AI" : "老师"}</span>
              <Button variant="ghost" size="sm" aria-label={gradingPointLabel(i, "删除")} onClick={() => onEdit({ type: "deletePoint", index: i })}>
                删除
              </Button>
            </div>
            <div className="flex flex-wrap items-center gap-2 text-mk-small">
              <span className="text-mk-muted">引文</span>
              {p.quote ? (
                <button type="button" onClick={() => onShowQuote(i)} className="min-w-0 text-left text-mk-ink underline">
                  「{p.quote}」
                </button>
              ) : (
                <span className="text-mk-muted">—</span>
              )}
              <Button variant="link" size="sm" aria-label={gradingPointLabel(i, "选择引文")} onClick={() => onPickQuote(i)}>
                选择引文
              </Button>
              {p.quote && (
                <Button
                  variant="link"
                  size="sm"
                  aria-label={gradingPointLabel(i, "清除引文")}
                  onClick={() => onEdit({ type: "pointQuote", index: i, value: null })}
                >
                  清除
                </Button>
              )}
            </div>
            {p.quote && unmarked.has(i) && <p className="text-mk-small text-mk-danger">未在正文中标出</p>}
            <textarea
              aria-label={gradingPointLabel(i, "说明")}
              rows={2}
              value={p.text}
              onChange={(e) => onEdit({ type: "pointText", index: i, value: e.target.value })}
              className={INPUT_CLS}
            />
            {p.kind === "issue" && (
              <textarea
                aria-label={gradingPointLabel(i, "修改建议")}
                placeholder="修改建议"
                rows={2}
                value={p.action ?? ""}
                onChange={(e) => onEdit({ type: "pointAction", index: i, value: e.target.value })}
                className={INPUT_CLS}
              />
            )}
          </div>
        ))}
        <div>
          <Button variant="secondary" size="sm" onClick={() => onEdit({ type: "addPoint" })}>
            添加意见
          </Button>
        </div>
      </section>
    </div>
  );
}
