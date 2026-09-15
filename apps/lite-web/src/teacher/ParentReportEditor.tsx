import { useEffect, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { ArrowLeft, Download } from "lucide-react";
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
import { ParentReportPoster } from "../parentReport/ParentReportPoster";
import { ParentReportView } from "../parentReport/ParentReportView";
import { rangeLabel } from "../parentReport/range";
import { posterFileName, SECTION_LABELS, toggleHidden, visibleFacts } from "../parentReport/view";
import { exportPoster } from "../reports/exportPoster";
import { errorText, failText } from "./assignmentLogic";
import {
  draftErrorText,
  errorCode,
  EXPORT_BLOCKED_TEXT,
  exportBlockedReason,
  hiddenMentionText,
  isBodyBlank,
  recalledDraftError,
  rememberDraftError,
  runeCount,
  savableSectionKeys,
  saveErrorText,
  SECTION_MAX_RUNES,
  showsNoDraftHint,
} from "./parentReportLogic";

type SectionState = { kind: "saving" } | { kind: "saved" } | { kind: "error"; text: string };
type Action = "redraft" | "export";

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
 * Autosave: a section is saved on blur, and only that section is sent (PATCH
 * merges, so a stale local copy of another section can never overwrite it).
 * Saves of one section run one after another, so the last text typed is the
 * last one stored. A section is sent only while it is in the report's
 * `sections`: with every keyword hidden `interests` is not, PATCH would reject
 * it, and its local text is kept for when a keyword is shown again.
 *
 * Hidden items: a toggle PATCHes the whole `hidden` set and applies the
 * response, so `sections`, `hidden` and `hiddenMentions` all come from the
 * server. Toggles and section saves are serialised (a toggle waits for pending
 * saves; a save waits for a pending toggle), so an older response never lands
 * after a newer one and a save never races a section disappearing.
 *
 * Export: saves every changed section and waits for a pending toggle, then
 * reads `hiddenMentions` from the latest response. While any section still
 * quotes a hidden item the export stops with `EXPORT_BLOCKED_TEXT`, and the
 * poster is never mounted. Otherwise the poster is mounted offscreen for the
 * duration of the export only.
 *
 * - 重新生成草稿. A body with text asks first and replaces it; a blank body is
 *   redrafted without asking (the server fills it).
 * - After `student_left`, redraft is disabled; editing, hiding and export stay
 *   available.
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
  const saveChain = useRef(new Map<string, Promise<boolean>>());
  const hiddenChain = useRef<Promise<unknown>>(Promise.resolve());
  const [sectionState, setSectionState] = useState<Record<string, SectionState>>({});

  const [draftMessage, setDraftMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState<Action | null>(null);
  const [confirm, setConfirm] = useState<Action | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [studentLeft, setStudentLeft] = useState(false);
  const [hiddenBusy, setHiddenBusy] = useState(false);
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
    // The blocked-export line is about the body as it was; once the last save
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

  async function storeSection(key: string): Promise<boolean> {
    // Read at execution: the section may have left `sections` while this save
    // waited behind a toggle. Its local text stays; there is nothing to send.
    const sections = reportRef.current?.view.sections ?? [];
    if (!sections.includes(key)) return true;
    const text = textsRef.current[key] ?? "";
    if (text === (savedRef.current[key] ?? "")) return true;
    setSectionState((s) => ({ ...s, [key]: { kind: "saving" } }));
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
    const previous = saveChain.current.get(key) ?? Promise.resolve(true);
    const next = previous.then(() => hiddenChain.current.then(() => storeSection(key)));
    saveChain.current.set(key, next);
    return next;
  }

  async function saveAll(): Promise<boolean> {
    // A toggle still running can change `sections`; wait for it first.
    await hiddenChain.current;
    const sections = reportRef.current?.view.sections ?? [];
    const results = await Promise.all(savableSectionKeys(sections, textsRef.current).map((key) => saveSection(key)));
    return results.every(Boolean);
  }

  function toggle(list: keyof ParentReportHidden, text: string) {
    const current = reportRef.current;
    if (!current || hiddenBusy) return;
    setHiddenBusy(true);
    setHiddenError(null);
    const hiding = !current.hidden[list].includes(text);
    const run = (async () => {
      // A blur save of a section that is about to disappear must land first.
      await Promise.allSettled([...saveChain.current.values()]);
      const latest = reportRef.current ?? current;
      try {
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
      } catch (e) {
        if (alive.current) setHiddenError(failText(hiding ? "隐藏" : "显示", e));
      } finally {
        if (alive.current) setHiddenBusy(false);
      }
    })();
    hiddenChain.current = run;
  }

  function noteRefusal(e: unknown) {
    if (errorCode(e) === "student_left") setStudentLeft(true);
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
      // A save or toggle still on its way must not land after the new draft.
      await Promise.allSettled([...saveChain.current.values(), hiddenChain.current]);
      const result = await redraftParentReport(reportId, replaceBody);
      if (!alive.current) return;
      applyReport(result.report);
      replaceTexts(result.report);
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
      const latest = reportRef.current;
      if (!saved || !latest) {
        setActionError("导出失败：部分内容保存失败");
        return;
      }
      const blocked = exportBlockedReason(latest);
      if (blocked) {
        setActionError(blocked);
        return;
      }
      // Mounted synchronously so the ref is set before rasterizing.
      flushSync(() =>
        setPoster({
          ...latest.view,
          facts: visibleFacts(latest.view.facts, latest.hidden),
          body: { ...textsRef.current },
        }),
      );
      const failure = await exportPoster(posterRef.current, posterFileName(latest.view.studentName));
      if (!alive.current) return;
      if (failure) setActionError(`导出失败：${failure}`);
    } finally {
      if (alive.current) {
        setPoster(null);
        setBusy(null);
      }
    }
  }

  const backButton = (
    <button
      type="button"
      onClick={() => onBack(report?.classId ?? null)}
      className="flex items-center gap-1.5 rounded-mk-sm text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <Icon icon={ArrowLeft} size={15} />
      返回
    </button>
  );

  if (loadError || report === null) {
    return (
      <div className="min-h-full">
        <div className="mx-auto max-w-[980px] px-4 pb-16 pt-8 sm:px-8">
          {backButton}
          {loadError ? (
            <div className="mt-4 text-mk-small font-semibold text-mk-danger">
              加载失败：{loadError}{" "}
              <button type="button" onClick={() => setNonce((n) => n + 1)} className="cursor-pointer underline">
                重试
              </button>
            </div>
          ) : (
            <div className="mt-4 text-mk-body text-mk-muted">加载中…</div>
          )}
        </div>
      </div>
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
    <div className="min-h-full">
      <div className="mx-auto max-w-[1320px] px-4 pb-16 pt-8 sm:px-8">
        {backButton}

        <p className="mt-4 text-mk-label text-mk-muted">家长报告</p>
        <div className="mt-1 flex flex-wrap items-center gap-3">
          <h1 className="text-mk-h1 tracking-tight text-mk-ink">{report.view.studentName || "—"}</h1>
          <div className="flex flex-wrap gap-2 sm:ml-auto">
            <Button variant="secondary" size="sm" onClick={requestRedraft} disabled={anyBusy || studentLeft}>
              {busy === "redraft" ? "生成中" : "重新生成草稿"}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={() => void runExport()}
              disabled={anyBusy}
              iconStart={<Icon icon={Download} size={15} />}
            >
              {busy === "export" ? "处理中" : "导出图片"}
            </Button>
          </div>
        </div>
        <p className="mt-1 text-mk-small text-mk-muted">
          {[report.view.className, rangeLabel(report.view.rangeStart, report.view.rangeEnd)].filter(Boolean).join(" · ")}
        </p>

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

        <div className="mt-6 grid grid-cols-1 items-start gap-6 min-[900px]:grid-cols-2">
          <div className="flex min-w-0 flex-col gap-5">
            {draftMessage && <DangerNote>{draftMessage}</DangerNote>}
            {showsNoDraftHint(report.hasDraft, texts, draftMessage) && (
              <p className="text-mk-small text-mk-muted">暂无草稿，请重新生成草稿</p>
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
                  {mentions && mentions.length > 0 && <DangerNote>{hiddenMentionText(mentions)}</DangerNote>}
                  <textarea
                    id={`parent-report-${key}`}
                    value={text}
                    rows={6}
                    disabled={busy === "redraft"}
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
                              disabled={hiddenBusy || busy !== null}
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
                              disabled={hiddenBusy || busy !== null}
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

          <aside className="min-w-0 min-[900px]:sticky min-[900px]:top-4" aria-label="预览">
            <div className="mb-2 text-mk-label font-bold text-mk-muted">预览</div>
            <div className="overflow-hidden rounded-mk-lg border border-mk-border bg-mk-paper min-[900px]:max-h-[calc(100vh-6rem)] min-[900px]:overflow-y-auto">
              <ParentReportView report={preview} />
            </div>
          </aside>
        </div>
      </div>

      {/* Mounted only while an export runs. The offscreen offset lives on the
          poster's own wrapper, never on the node that is rasterized. */}
      {poster && <ParentReportPoster ref={posterRef} report={poster} />}
    </div>
  );
}

/** A failure line with a muted danger tint. No left colour bar. */
function DangerNote({ children }: { children: React.ReactNode }) {
  return (
    <div
      className="break-words rounded-mk-md border px-3 py-2 text-mk-small font-semibold text-mk-danger"
      style={{
        borderColor: "color-mix(in srgb, var(--mk-danger) 30%, var(--mk-border))",
        background: "color-mix(in srgb, var(--mk-danger) 6%, var(--mk-surface))",
      }}
      role="alert"
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
