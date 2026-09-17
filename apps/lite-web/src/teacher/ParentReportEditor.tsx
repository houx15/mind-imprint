import { useEffect, useRef, useState, type ReactNode } from "react";
import { flushSync } from "react-dom";
import { Download } from "lucide-react";
import { Button, Icon } from "@/ui";
import {
  getTeacherParentReport,
  patchParentReportHidden,
  patchParentReportSection,
  redraftParentReport,
  type ParentReport,
  type ParentReportHidden,
  type TeacherParentReport,
} from "../api/parentReports";
import { postWorkspaceTurn } from "../api/teacherWorkspace";
import { ParentReportPoster } from "../parentReport/ParentReportPoster";
import { ParentReportView } from "../parentReport/ParentReportView";
import { rangeLabel } from "../parentReport/range";
import { posterFileName, SECTION_LABELS, toggleHidden, visibleFacts } from "../parentReport/view";
import { exportPoster } from "../reports/exportPoster";
import { errorText, failText } from "./assignmentLogic";
import { Segmented } from "./formParts";
import { TeacherPage } from "./TeacherPage";
import { BackLink, StudioEmpty, StudioError, StudioHeading, StudioLoading } from "./StudioArtwork";
import {
  draftErrorText,
  EXPORT_BLOCKED_TEXT,
  exportBlockedReason,
  hiddenMentionText,
  isBodyBlank,
  keptSectionText,
  planReportPatch,
  posterReportFrom,
  recalledDraftError,
  rememberDraftError,
  reportArtifactPayload,
  reportPatchBody,
  runeCount,
  saveErrorText,
  SECTION_MAX_RUNES,
  showsNoDraftHint,
  studentLeftReason,
} from "./parentReportLogic";
import { createSerialQueue } from "./serialQueue";
import type { ThreadInput } from "./workspace/useWorkspaceThread";
import { useWorkspaceThread } from "./workspace/useWorkspaceThread";
import { WorkspacePanel } from "./workspace/WorkspacePanel";

type SectionState = { kind: "saving" } | { kind: "saved" } | { kind: "error"; text: string };
type Action = "redraft" | "export";
type Texts = Record<string, string>;
type Pane = "draft" | "preview";

const PANE_OPTIONS: { value: Pane; label: string }[] = [
  { value: "draft", label: "草稿" },
  { value: "preview", label: "预览" },
];

/** A turn's reply, held between the request settling and the thread
 * accepting it (see "The conversation" below). */
interface TurnPatch {
  snapshot: Texts;
  patch: Texts;
}

/** The stored text of each section the report has, blank for a missing one. */
function bodyOf(r: TeacherParentReport): Record<string, string> {
  return Object.fromEntries(r.view.sections.map((key) => [key, r.view.body[key] ?? ""]));
}

/**
 * ParentReportEditor — `/parent-reports/:reportId`. Left: one textarea per
 * section, then the 金句 and keywords with a 隐藏/显示 toggle each; right: the
 * report as the exported picture will show it (`ParentReportView`), bound to
 * the text being typed and to the visible facts. There is no parent end: the
 * teacher exports the report (导出图片) and sends it herself.
 *
 * ## One write lane
 *
 * Every request that changes or reads back the report after load — a section
 * save, a hide toggle, a redraft, the export snapshot — goes through ONE
 * serial queue (`createSerialQueue`). Each starts after the previous settled,
 * so responses are applied strictly in request order and a stale response can
 * never overwrite a newer one. (Two chains that waited on each other
 * deadlocked: fix round 1.) A task never awaits a task it enqueues.
 *
 * Autosave: a section is saved on blur, and only that section is sent (PATCH
 * merges). The task reads `textsRef` and the report's `sections` when it RUNS:
 * a key not in `sections` (e.g. `interests` while every keyword is hidden) is
 * not sent, and its local text is kept for when a keyword is shown again.
 *
 * Hidden items: a toggle PATCHes the whole `hidden` set, built from the latest
 * applied report when the task runs, and applies the response, so `sections`,
 * `hidden` and `hiddenMentions` all come from the server.
 *
 * ## Export
 *
 * While exporting, the textareas and toggles are disabled. Every changed
 * section is saved through the lane; any failure blocks the export. Then a
 * fresh GET, also through the lane, gives the snapshot. If it still quotes a
 * hidden item the export stops with `EXPORT_BLOCKED_TEXT` and the poster is
 * never mounted. Otherwise the poster is built ONLY from that snapshot
 * (`posterReportFrom`) and mounted offscreen for the duration of the export.
 *
 * - 重新生成草稿. A body with text asks first and replaces it; a blank body is
 *   redrafted without asking (the server fills it).
 * - After `student_left`, redraft and the conversation are disabled; editing,
 *   hiding and export stay available.
 *
 * ## The conversation (§12.6, D3)
 *
 * The editor sits in `WorkspacePanel` beside a `parentReport` thread. A turn
 * sends each shown section's current text (unsaved typing included) and gets
 * back a patch of revised sections; the server never writes the report.
 *
 * The patch is applied here, not by the thread. The thread's own
 * `applyPatch` compares against the artifact of the last render, while a
 * section's text lives in `textsRef`, which a keystroke updates before the
 * next render. So `post` keeps the snapshot and the patch in `turnRef` and
 * hands the thread an empty patch; the thread calls `setArtifact` only for a
 * current reply, and that call runs `planReportPatch` against `textsRef`:
 * - a section she changed during the turn keeps her text and gets
 *   「{段名} 已保留你的修改」;
 * - every other patched section is set in `textsRef` and saved with
 *   `saveSection`, the same queue and the same `storeSection` her typing
 *   uses. The save reads `textsRef` when it runs, so typing that lands after
 *   the patch is what gets saved, and a save of hers already in flight
 *   finishes first. A failed save shows under its section like any other.
 *
 * While a turn is in flight, redraft and export are disabled (both replace
 * or snapshot the whole body); while either runs, the composer is paused.
 * So a patch is never applied while the textareas are locked.
 *
 * ## Width
 *
 * The AI sidebar takes the right 400–500px (`WorkspacePanel`), which leaves
 * the canvas about 800px at 1440px — too narrow for the draft and the preview side by side.
 * 草稿 / 预览 is a `Segmented` switch at every width; both panes stay mounted
 * and only their display changes.
 */
export function ParentReportEditor({
  reportId,
  onBack,
}: {
  reportId: string;
  /** Receives the report's class once it has loaded. */
  onBack: (classId: string | null) => void;
}) {
  const [report, setReport] = useState<TeacherParentReport | null>(null);
  const reportRef = useRef<TeacherParentReport | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  const [texts, setTexts] = useState<Record<string, string>>({});
  const textsRef = useRef<Record<string, string>>({});
  const savedRef = useRef<Record<string, string>>({});
  const queue = useRef(createSerialQueue());
  const [sectionState, setSectionState] = useState<Record<string, SectionState>>({});

  const [draftMessage, setDraftMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState<Action | null>(null);
  const [confirm, setConfirm] = useState<Action | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  /** The server's reason once a redraft or a turn was refused with
   * `student_left`; null until then. */
  const [leftReason, setLeftReason] = useState<string | null>(null);
  const [hiddenBusy, setHiddenBusy] = useState(false);
  const [kept, setKept] = useState<string[]>([]);
  const turnRef = useRef<TurnPatch | null>(null);
  const [pane, setPane] = useState<Pane>("draft");
  const [hiddenError, setHiddenError] = useState<string | null>(null);

  const posterRef = useRef<HTMLDivElement>(null);
  const [poster, setPoster] = useState<ParentReport | null>(null);

  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  function applyReport(r: TeacherParentReport) {
    reportRef.current = r;
    setReport(r);
    // The blocked-export line is about the body as it was; once a response
    // shows no section quoting a hidden item, it no longer applies.
    if (!exportBlockedReason(r)) setActionError((e) => (e === EXPORT_BLOCKED_TEXT ? null : e));
  }

  function replaceTexts(r: TeacherParentReport) {
    const body = bodyOf(r);
    textsRef.current = body;
    savedRef.current = body;
    setTexts(body);
    setSectionState({});
  }

  useEffect(() => {
    let cancelled = false;
    reportRef.current = null;
    setReport(null);
    setLoadError(null);
    getTeacherParentReport(reportId)
      .then((r) => {
        if (cancelled) return;
        applyReport(r);
        replaceTexts(r);
        // A failed generate hands its message over (see `rememberDraftError`).
        // Once a draft exists the message is out of date.
        setDraftMessage(r.hasDraft ? null : draftErrorText(recalledDraftError(reportId)));
      })
      .catch((e: unknown) => {
        if (!cancelled) setLoadError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [reportId, nonce]);

  function editText(key: string, value: string) {
    textsRef.current = { ...textsRef.current, [key]: value };
    setTexts(textsRef.current);
  }

  /** One section save. Runs inside the lane; reads everything when it runs. */
  async function storeSection(key: string): Promise<boolean> {
    const sections = reportRef.current?.view.sections ?? [];
    if (!sections.includes(key)) return true;
    const text = textsRef.current[key] ?? "";
    if (text === (savedRef.current[key] ?? "")) return true;
    if (alive.current) setSectionState((s) => ({ ...s, [key]: { kind: "saving" } }));
    try {
      const r = await patchParentReportSection(reportId, key, text);
      if (!alive.current) return false;
      savedRef.current = { ...savedRef.current, [key]: r.view.body[key] ?? "" };
      applyReport(r);
      setSectionState((s) => ({ ...s, [key]: { kind: "saved" } }));
      return true;
    } catch (e) {
      if (!alive.current) return false;
      setSectionState((s) => ({ ...s, [key]: { kind: "error", text: saveErrorText(e) } }));
      return false;
    }
  }

  function saveSection(key: string): Promise<boolean> {
    return queue.current.enqueue(() => storeSection(key));
  }

  /** Enqueues a save for every local section and waits for all of them. The
   * saves are the newest tasks in the lane, so once they settle every write
   * made before them has landed too. */
  async function saveAll(): Promise<boolean> {
    const results = await Promise.all(Object.keys(textsRef.current).map((key) => saveSection(key)));
    return results.every(Boolean);
  }

  function toggle(list: keyof ParentReportHidden, text: string) {
    const current = reportRef.current;
    if (!current || hiddenBusy) return;
    setHiddenBusy(true);
    setHiddenError(null);
    const hiding = !current.hidden[list].includes(text);
    void queue.current
      .enqueue(async () => {
        // Built from the latest applied report, after every earlier write.
        const latest = reportRef.current ?? current;
        const r = await patchParentReportHidden(reportId, toggleHidden(latest.hidden, list, text));
        if (!alive.current) return;
        applyReport(r);
        // Local text stays authoritative. A section that came back (a keyword
        // shown again) and has no local text takes the stored one.
        let restored = false;
        const nextTexts = { ...textsRef.current };
        for (const key of r.view.sections) {
          if (!(key in nextTexts)) {
            nextTexts[key] = r.view.body[key] ?? "";
            savedRef.current = { ...savedRef.current, [key]: nextTexts[key] ?? "" };
            restored = true;
          }
        }
        if (restored) {
          textsRef.current = nextTexts;
          setTexts(nextTexts);
        }
      })
      .catch((e: unknown) => {
        if (alive.current) setHiddenError(failText(hiding ? "隐藏" : "显示", e));
      })
      .finally(() => {
        if (alive.current) setHiddenBusy(false);
      });
  }

  function noteRefusal(e: unknown) {
    const reason = studentLeftReason(e);
    if (reason !== null && alive.current) setLeftReason(reason);
  }

  /** The thread's `setArtifact`, called once per current reply. `next` is the
   * thread's copy with an empty patch applied and is not used: the patch is
   * in `turnRef` and is planned against `textsRef` (see the doc above). */
  function applyTurn() {
    const turn = turnRef.current;
    turnRef.current = null;
    if (!turn || !alive.current) return;
    const plan = planReportPatch(textsRef.current, turn.snapshot, turn.patch, reportRef.current?.view.sections ?? []);
    setKept(plan.kept);
    if (plan.write.length === 0) return;
    textsRef.current = plan.texts;
    setTexts(plan.texts);
    for (const key of plan.write) void saveSection(key);
  }

  const thread = useWorkspaceThread<Texts>({
    artifact: texts,
    setArtifact: applyTurn,
    // The shell mounts this editor with `key={reportId}`, so the scope never
    // changes during its life.
    scopeOf: () => reportId,
    post: ({ artifact, turns, input }) => {
      const current = reportRef.current;
      turnRef.current = null;
      return postWorkspaceTurn({
        surface: "parentReport",
        classId: current?.classId ?? "",
        reportId,
        artifact: reportArtifactPayload(artifact, current?.view.sections ?? []),
        turns,
        ...("text" in input ? { text: input.text } : { choiceId: input.choiceId, choiceSlug: input.slug, choiceLabel: input.label }),
      }).then(
        (res) => {
          turnRef.current = { snapshot: artifact, patch: reportPatchBody(res.patch) };
          return { reply: res.reply, choices: res.choices, cards: [], patch: {} };
        },
        (e: unknown) => {
          noteRefusal(e);
          throw e;
        },
      );
    },
    // The server prefixes its own failures with 「对话失败：」; `failText`
    // does not double it.
    describeError: (e) => failText("对话", e),
  });

  function runTurn(input: ThreadInput): boolean {
    const accepted = thread.run(input);
    if (accepted) {
      setKept([]);
      // A pending 确认重新生成 would replace the body under the turn.
      setConfirm(null);
    }
    return accepted;
  }

  function requestRedraft() {
    setActionError(null);
    if (isBodyBlank(textsRef.current) && isBodyBlank(savedRef.current)) void runRedraft(false);
    else setConfirm("redraft");
  }

  async function runRedraft(replaceBody: boolean) {
    setConfirm(null);
    setBusy("redraft");
    setActionError(null);
    try {
      // In the lane: a save or toggle made before it lands first.
      const result = await queue.current.enqueue(() => redraftParentReport(reportId, replaceBody));
      if (!alive.current) return;
      applyReport(result.report);
      replaceTexts(result.report);
      // The notices are about text the redraft just replaced.
      setKept([]);
      rememberDraftError(reportId, result.draftError);
      setDraftMessage(draftErrorText(result.draftError));
    } catch (e) {
      if (!alive.current) return;
      setActionError(failText("重新生成", e));
      noteRefusal(e);
    } finally {
      if (alive.current) setBusy(null);
    }
  }

  async function runExport() {
    setConfirm(null);
    setBusy("export");
    setActionError(null);
    try {
      const saved = await saveAll();
      if (!alive.current) return;
      if (!saved) {
        setActionError("导出失败：部分内容保存失败");
        return;
      }
      let snapshot: TeacherParentReport;
      try {
        snapshot = await queue.current.enqueue(() => getTeacherParentReport(reportId));
      } catch (e) {
        if (alive.current) setActionError(failText("导出", e));
        return;
      }
      if (!alive.current) return;
      applyReport(snapshot);
      const blocked = exportBlockedReason(snapshot);
      if (blocked) {
        setActionError(blocked);
        return;
      }
      // Mounted synchronously so the ref is set before rasterizing. Built from
      // the snapshot alone, never from local text.
      flushSync(() => setPoster(posterReportFrom(snapshot)));
      const failure = await exportPoster(posterRef.current, posterFileName(snapshot.view.studentName));
      if (!alive.current) return;
      if (failure) setActionError(`导出失败：${failure}`);
    } catch (e) {
      // A save flush or the render threw: say so instead of leaving an
      // unhandled rejection behind a button that just stops spinning.
      if (alive.current) setActionError(`导出失败：${e instanceof Error ? e.message : String(e)}`);
    } finally {
      if (alive.current) {
        setPoster(null);
        setBusy(null);
      }
    }
  }

  const backButton = <BackLink label="返回家长报告" onClick={() => onBack(report?.classId ?? null)} />;

  if (loadError || report === null) {
    return (
      <TeacherPage width="full">
        {backButton}
        {loadError ? (
          <StudioError message={loadError} onRetry={() => setNonce((n) => n + 1)} />
        ) : (
          <StudioLoading />
        )}
      </TeacherPage>
    );
  }

  const sections = report.view.sections.filter((key) => SECTION_LABELS[key]);
  const anyBusy = busy !== null;
  const moments = report.view.facts.moments.filter((m) => m.quote.trim());
  const keywords = report.view.facts.keywords.filter((k) => k.text.trim());
  const preview: ParentReport = {
    ...report.view,
    facts: visibleFacts(report.view.facts, report.hidden),
    body: texts,
  };

  return (
    <>
      <WorkspacePanel
        turns={thread.turns}
        busy={thread.busy}
        error={thread.error}
        choices={thread.choices}
        onSend={(text) => runTurn({ text })}
        onChoose={(choiceId) => {
          const choice = thread.choices.find((c) => c.id === choiceId);
          return runTurn({ choiceId, label: choice?.label ?? choiceId, slug: choice?.slug });
        }}
        composer={thread.composer}
        onComposerChange={thread.setComposer}
        onRetry={() => {
          // `thread.retry` is `run(failed)`; this goes through `runTurn` so a
          // retry clears the same state a new turn does.
          if (thread.failed) runTurn(thread.failed);
        }}
        canRetry={thread.failed !== null && leftReason === null}
        paused={anyBusy}
        closedReason={leftReason}
        intro="印记按学习记录改写左侧报告的段落，结果先显示在编辑器中，段落不能增删。请说明要改哪一段、怎么改。"
        suggestions={["让下一步建议更具体", "把总体概述改短一些"]}
        header={
          <>
            {backButton}

            <StudioHeading
              kicker="家长报告"
              title={report.view.studentName || "—"}
              description="AI 起草的文字可能有误。请逐段审核，在「预览」中确认后再导出图片。"
              actions={
                <>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={requestRedraft}
                  disabled={anyBusy || thread.busy || leftReason !== null}
                >
                  {busy === "redraft" ? "生成中" : "重新生成草稿"}
                </Button>
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => void runExport()}
                  disabled={anyBusy || thread.busy}
                  iconStart={<Icon icon={Download} size={15} />}
                >
                  {busy === "export" ? "处理中" : "导出图片"}
                </Button>
                </>
              }
            >
              <p className="text-mk-small text-mk-muted">
                {[report.view.className, rangeLabel(report.view.rangeStart, report.view.rangeEnd)].filter(Boolean).join(" · ")}
              </p>
            </StudioHeading>

            {confirm === "redraft" && (
              <ConfirmRow
                text="重新生成会覆盖当前文字，确定？"
                confirmLabel="确认重新生成"
                onConfirm={() => void runRedraft(true)}
                onCancel={() => setConfirm(null)}
              />
            )}

            {actionError && (
              <div className="mt-3 break-words text-mk-small font-semibold text-mk-danger" role="alert">
                {actionError}
              </div>
            )}
          </>
        }
      >
        <div>
          <Segmented label="视图" options={PANE_OPTIONS} value={pane} onChange={setPane} />
        </div>

        <div className="mt-5 grid grid-cols-1 items-start gap-6">
          <div className={"min-w-0 flex-col gap-5 " + (pane === "draft" ? "flex" : "hidden")}>
            {draftMessage && <DangerNote role="alert">{draftMessage}</DangerNote>}
            {showsNoDraftHint(report.hasDraft, texts, draftMessage) && (
              <StudioEmpty
                kind="keepsake"
                title="暂无草稿"
                action={leftReason === null && !anyBusy && !thread.busy ? { label: "重新生成草稿", onClick: requestRedraft } : undefined}
              >
                草稿由印记按学习记录起草。请重新生成草稿，再逐段修改。
              </StudioEmpty>
            )}
            {sections.map((key) => {
              const text = texts[key] ?? "";
              const count = runeCount(text);
              const state = sectionState[key];
              const mentions = report.hiddenMentions[key];
              return (
                <div key={key} className="flex flex-col gap-1.5">
                  <label htmlFor={`parent-report-${key}`} className="text-mk-small font-bold text-mk-ink">
                    {SECTION_LABELS[key]}
                  </label>
                  {kept.includes(key) && (
                    <p className="text-mk-small font-semibold text-mk-accent-700" role="status">
                      {keptSectionText(key)}
                    </p>
                  )}
                  {mentions && mentions.length > 0 && (
                    <DangerNote role="status">{hiddenMentionText(mentions)}</DangerNote>
                  )}
                  <textarea
                    id={`parent-report-${key}`}
                    value={text}
                    rows={6}
                    disabled={anyBusy}
                    onChange={(e) => editText(key, e.target.value)}
                    onBlur={() => void saveSection(key)}
                    className="w-full resize-y rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-body text-mk-ink outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:text-mk-muted"
                  />
                  <div className="flex flex-wrap items-center justify-between gap-2 text-mk-small">
                    <span className="min-w-0 break-words">
                      {state?.kind === "saving" && <span className="text-mk-muted">保存中</span>}
                      {state?.kind === "saved" && <span className="text-mk-muted">已保存</span>}
                      {state?.kind === "error" && (
                        <span className="font-semibold text-mk-danger" role="alert">
                          {state.text}{" "}
                          <button type="button" onClick={() => void saveSection(key)} className="cursor-pointer underline">
                            重试
                          </button>
                        </span>
                      )}
                    </span>
                    <span className={count > SECTION_MAX_RUNES ? "font-semibold text-mk-danger" : "text-mk-muted"}>
                      {count}/{SECTION_MAX_RUNES}
                    </span>
                  </div>
                </div>
              );
            })}

            {(moments.length > 0 || keywords.length > 0) && (
              <section
                aria-label="隐藏内容"
                className="flex flex-col gap-4 rounded-mk-lg border border-mk-border bg-mk-surface p-4"
              >
                <p className="text-mk-small text-mk-muted">隐藏的内容不会出现在预览和导出的图片中。</p>
                {hiddenError && (
                  <p className="break-words text-mk-small font-semibold text-mk-danger" role="alert">
                    {hiddenError}
                  </p>
                )}

                {moments.length > 0 && (
                  <div className="flex flex-col gap-2">
                    <h2 className="text-mk-small font-bold text-mk-ink">学生原话</h2>
                    <ul className="flex flex-col gap-2">
                      {moments.map((m, i) => {
                        const isHidden = report.hidden.moments.includes(m.quote);
                        return (
                          <li
                            key={`${m.quote}-${i}`}
                            className="flex items-start justify-between gap-3 rounded-mk-md border border-mk-border bg-mk-paper px-3 py-2"
                          >
                            <span
                              className={"min-w-0 text-mk-small " + (isHidden ? "text-mk-muted line-through" : "text-mk-ink")}
                              style={{ overflowWrap: "anywhere", opacity: isHidden ? 0.6 : 1 }}
                            >
                              {m.quote}
                              {m.itemTitle && <span className="ml-1 text-mk-muted">《{m.itemTitle}》</span>}
                            </span>
                            <ToggleButton
                              hidden={isHidden}
                              disabled={hiddenBusy || anyBusy}
                              onClick={() => toggle("moments", m.quote)}
                            />
                          </li>
                        );
                      })}
                    </ul>
                  </div>
                )}

                {keywords.length > 0 && (
                  <div className="flex flex-col gap-2">
                    <h2 className="text-mk-small font-bold text-mk-ink">兴趣关键词</h2>
                    <ul className="flex flex-wrap gap-2">
                      {keywords.map((k, i) => {
                        const isHidden = report.hidden.keywords.includes(k.text);
                        return (
                          <li
                            key={`${k.text}-${i}`}
                            className="flex items-center gap-2 rounded-mk-full border border-mk-border bg-mk-paper py-1 pl-3 pr-1"
                          >
                            <span
                              className={"text-mk-small " + (isHidden ? "text-mk-muted line-through" : "text-mk-ink")}
                              style={{ opacity: isHidden ? 0.6 : 1 }}
                            >
                              {k.text}
                            </span>
                            <ToggleButton
                              hidden={isHidden}
                              disabled={hiddenBusy || anyBusy}
                              onClick={() => toggle("keywords", k.text)}
                            />
                          </li>
                        );
                      })}
                    </ul>
                  </div>
                )}
              </section>
            )}
          </div>

          <aside
            className={
              "min-w-0 " +
              (pane === "preview" ? "block" : "hidden")
            }
            aria-label="预览"
          >
            <div className="mb-2 text-mk-label font-bold text-mk-muted">预览</div>
            <div className="overflow-hidden rounded-mk-lg border border-mk-border bg-mk-paper">
              <ParentReportView report={preview} />
            </div>
          </aside>
        </div>
      </WorkspacePanel>

      {/* Mounted only while an export runs. The offscreen offset lives on the
          poster's own wrapper, never on the node that is rasterized. */}
      {poster && <ParentReportPoster ref={posterRef} report={poster} />}
    </>
  );
}

/** A line with a muted danger tint. No left colour bar. `role` is `alert` for
 * a failure, `status` for a standing note such as a hidden-item mention.
 * Plain `--mk-danger` on this tint reads 4.05:1 (lite light) and 3.69:1 (lite
 * dark); mixed 70% into ink it reads 5.84:1 and 5.00:1. */
function DangerNote({ children, role }: { children: ReactNode; role: "alert" | "status" }) {
  return (
    <div
      className="break-words rounded-mk-md border px-3 py-2 text-mk-small font-semibold"
      style={{
        color: "color-mix(in srgb, var(--mk-danger) 70%, var(--mk-ink))",
        borderColor: "color-mix(in srgb, var(--mk-danger) 30%, var(--mk-border))",
        background: "color-mix(in srgb, var(--mk-danger) 6%, var(--mk-surface))",
      }}
      role={role}
    >
      {children}
    </div>
  );
}

function ToggleButton({ hidden, disabled, onClick }: { hidden: boolean; disabled: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-pressed={hidden}
      className="shrink-0 rounded-mk-full border border-mk-border bg-mk-surface px-2.5 py-0.5 text-mk-label text-mk-secondary transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:cursor-not-allowed disabled:opacity-60"
    >
      {hidden ? "显示" : "隐藏"}
    </button>
  );
}

function ConfirmRow({
  text,
  confirmLabel,
  onConfirm,
  onCancel,
}: {
  text: string;
  confirmLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <div className="mt-3 flex flex-wrap items-center gap-2 text-mk-small font-semibold text-mk-danger">
      {text}
      <Button variant="danger" size="sm" onClick={onConfirm}>
        {confirmLabel}
      </Button>
      <Button variant="ghost" size="sm" onClick={onCancel}>
        取消
      </Button>
    </div>
  );
}
