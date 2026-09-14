import { useEffect, useRef, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { Button, Icon } from "@/ui";
import {
  getTeacherParentReport,
  patchParentReportSection,
  publishParentReport,
  redraftParentReport,
  revokeParentReportShare,
  type TeacherParentReport,
} from "../api/parentReports";
import { ParentReportView } from "../parentReport/ParentReportView";
import { rangeLabel } from "../parentReport/range";
import { SECTION_LABELS } from "../parentReport/view";
import { errorText, failText, statusChipStyle } from "./assignmentLogic";
import { safeHttpUrl } from "./format";
import {
  draftErrorText,
  errorCode,
  isBodyBlank,
  recalledDraftError,
  rememberDraftError,
  runeCount,
  saveErrorText,
  SECTION_MAX_RUNES,
  shareUrl,
  statusLabel,
} from "./parentReportLogic";

type SectionState = { kind: "saving" } | { kind: "saved" } | { kind: "error"; text: string };
type Action = "redraft" | "publish" | "revoke";

/** The stored text of each section the report has, blank for a missing one. */
function bodyOf(r: TeacherParentReport): Record<string, string> {
  return Object.fromEntries(r.view.sections.map((key) => [key, r.view.body[key] ?? ""]));
}

/**
 * ParentReportEditor — `/parent-reports/:reportId`. Left: one textarea per
 * section; right: the report exactly as a parent will see it
 * (`ParentReportView variant="teacherPreview"`), bound to the text being
 * typed rather than to the stored body.
 *
 * Autosave: a section is saved on blur, and only that section is sent (PATCH
 * merges, so a stale local copy of another section can never overwrite it).
 * Saves of one section run one after another, so the last text typed is the
 * last one stored. Publishing first saves every changed section and waits.
 *
 * Actions:
 * - 重新生成草稿 (draft only). A body with text asks first and replaces it;
 *   a blank body is redrafted without asking (the server fills it).
 * - 发布 (draft only), after a confirm.
 * - Published: the link with 复制链接 and 撤销链接, or 重新开启链接 after a
 *   revoke, which publishes again and gets a new token.
 * - After `student_left`, redraft and publish are disabled; editing and
 *   revoking stay available so a live link can still be closed.
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
  const [loadError, setLoadError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  const [texts, setTexts] = useState<Record<string, string>>({});
  const textsRef = useRef<Record<string, string>>({});
  const savedRef = useRef<Record<string, string>>({});
  const saveChain = useRef(new Map<string, Promise<boolean>>());
  const [sectionState, setSectionState] = useState<Record<string, SectionState>>({});

  const [draftMessage, setDraftMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState<Action | null>(null);
  const [confirm, setConfirm] = useState<Action | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [studentLeft, setStudentLeft] = useState(false);
  const [copyState, setCopyState] = useState<{ kind: "copied" } | { kind: "error"; text: string } | null>(null);

  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  function replaceTexts(r: TeacherParentReport) {
    const body = bodyOf(r);
    textsRef.current = body;
    savedRef.current = body;
    setTexts(body);
    setSectionState({});
  }

  useEffect(() => {
    let cancelled = false;
    setReport(null);
    setLoadError(null);
    getTeacherParentReport(reportId)
      .then((r) => {
        if (cancelled) return;
        setReport(r);
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
    const text = textsRef.current[key] ?? "";
    if (text === (savedRef.current[key] ?? "")) return true;
    setSectionState((s) => ({ ...s, [key]: { kind: "saving" } }));
    try {
      const r = await patchParentReportSection(reportId, key, text);
      if (!alive.current) return false;
      savedRef.current = { ...savedRef.current, [key]: r.view.body[key] ?? "" };
      setReport(r);
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
    const next = previous.then(() => storeSection(key));
    saveChain.current.set(key, next);
    return next;
  }

  async function saveAll(): Promise<boolean> {
    const results = await Promise.all(Object.keys(textsRef.current).map((key) => saveSection(key)));
    return results.every(Boolean);
  }

  async function refreshStatus() {
    try {
      const r = await getTeacherParentReport(reportId);
      if (alive.current) setReport(r);
    } catch {
      /* the action's own failure line is already showing */
    }
  }

  function noteRefusal(e: unknown) {
    const code = errorCode(e);
    if (code === "student_left") setStudentLeft(true);
    if (code === "already_published") void refreshStatus();
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
      // A save still on its way must not land after the new draft.
      await Promise.allSettled([...saveChain.current.values()]);
      const result = await redraftParentReport(reportId, replaceBody);
      if (!alive.current) return;
      setReport(result.report);
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

  async function runPublish() {
    setConfirm(null);
    setBusy("publish");
    setActionError(null);
    try {
      const saved = await saveAll();
      if (!alive.current) return;
      if (!saved) {
        setActionError("发布失败：部分内容保存失败");
        return;
      }
      const r = await publishParentReport(reportId);
      if (!alive.current) return;
      setReport(r);
      setCopyState(null);
    } catch (e) {
      if (!alive.current) return;
      setActionError(failText("发布", e));
      noteRefusal(e);
    } finally {
      if (alive.current) setBusy(null);
    }
  }

  async function runRevoke() {
    setConfirm(null);
    setBusy("revoke");
    setActionError(null);
    try {
      const r = await revokeParentReportShare(reportId);
      if (!alive.current) return;
      setReport(r);
      setCopyState(null);
    } catch (e) {
      if (!alive.current) return;
      setActionError(failText("撤销", e));
    } finally {
      if (alive.current) setBusy(null);
    }
  }

  async function copyLink(url: string) {
    setCopyState(null);
    try {
      if (!navigator.clipboard) throw new Error("浏览器不支持剪贴板");
      await navigator.clipboard.writeText(url);
      if (alive.current) setCopyState({ kind: "copied" });
    } catch (e) {
      if (alive.current) setCopyState({ kind: "error", text: `复制失败：${errorText(e)}` });
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

  const isDraft = report.status === "draft";
  const link = report.shareToken ? shareUrl(window.location.origin, report.shareToken) : null;
  const linkHref = link ? safeHttpUrl(link) : null;
  const sections = report.view.sections.filter((key) => SECTION_LABELS[key]);
  const anyBusy = busy !== null;
  const chip = statusChipStyle(isDraft ? "not_started" : "done");

  return (
    <div className="min-h-full">
      <div className="mx-auto max-w-[1320px] px-4 pb-16 pt-8 sm:px-8">
        {backButton}

        <p className="mt-4 text-mk-label text-mk-muted">家长报告</p>
        <div className="mt-1 flex flex-wrap items-center gap-3">
          <h1 className="text-mk-h1 tracking-tight text-mk-ink">{report.view.studentName || "—"}</h1>
          <span className="rounded-mk-full px-2.5 py-0.5 text-mk-label font-bold" style={chip}>
            {statusLabel(report.status)}
          </span>
          {isDraft && (
            <div className="flex flex-wrap gap-2 sm:ml-auto">
              <Button variant="secondary" size="sm" onClick={requestRedraft} disabled={anyBusy || studentLeft}>
                {busy === "redraft" ? "生成中" : "重新生成草稿"}
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={() => {
                  setActionError(null);
                  setConfirm("publish");
                }}
                disabled={anyBusy || studentLeft}
              >
                {busy === "publish" ? "发布中" : "发布"}
              </Button>
            </div>
          )}
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
        {confirm === "publish" && (
          <ConfirmRow
            text="发布后家长可通过链接查看，学生也会在收件箱收到这份报告。"
            confirmLabel="确认发布"
            danger={false}
            onConfirm={() => void runPublish()}
            onCancel={() => setConfirm(null)}
          />
        )}

        {!isDraft && (
          <div className="mt-4 flex flex-col gap-3 rounded-mk-lg border border-mk-border bg-mk-surface p-4 shadow-mk-xs">
            <div className="text-mk-label font-bold text-mk-muted">家长链接</div>
            {link ? (
              <>
                <div className="rounded-mk-md border border-mk-border bg-mk-paper px-3 py-2 text-mk-small text-mk-ink">
                  {linkHref ? (
                    <a href={linkHref} target="_blank" rel="noreferrer" className="break-all text-mk-accent-700 underline">
                      {link}
                    </a>
                  ) : (
                    <span className="break-all">{link}</span>
                  )}
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Button variant="secondary" size="sm" onClick={() => void copyLink(link)}>
                    复制链接
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      setActionError(null);
                      setConfirm("revoke");
                    }}
                    disabled={anyBusy}
                  >
                    {busy === "revoke" ? "撤销中" : "撤销链接"}
                  </Button>
                  {copyState?.kind === "copied" && <span className="text-mk-small text-mk-muted">已复制</span>}
                  {copyState?.kind === "error" && (
                    <span className="text-mk-small font-semibold text-mk-danger" role="alert">
                      {copyState.text}
                    </span>
                  )}
                </div>
                {confirm === "revoke" && (
                  <ConfirmRow
                    text="撤销后该链接立即失效，学生仍可在应用内查看。"
                    confirmLabel="确认撤销"
                    onConfirm={() => void runRevoke()}
                    onCancel={() => setConfirm(null)}
                  />
                )}
              </>
            ) : (
              <div className="flex flex-wrap items-center gap-3">
                <span className="text-mk-small text-mk-muted">链接已撤销</span>
                <Button variant="secondary" size="sm" onClick={() => void runPublish()} disabled={anyBusy || studentLeft}>
                  {busy === "publish" ? "处理中" : "重新开启链接"}
                </Button>
              </div>
            )}
          </div>
        )}

        {actionError && (
          <div className="mt-3 break-words text-mk-small font-semibold text-mk-danger" role="alert">
            {actionError}
          </div>
        )}

        <div className="mt-6 grid grid-cols-1 items-start gap-6 min-[900px]:grid-cols-2">
          <div className="flex min-w-0 flex-col gap-5">
            {draftMessage && (
              <div
                className="break-words rounded-mk-md border px-3 py-2 text-mk-small font-semibold text-mk-danger"
                style={{
                  borderColor: "color-mix(in srgb, var(--mk-danger) 30%, var(--mk-border))",
                  background: "color-mix(in srgb, var(--mk-danger) 6%, var(--mk-surface))",
                }}
                role="alert"
              >
                {draftMessage}
              </div>
            )}
            {sections.map((key) => {
              const text = texts[key] ?? "";
              const count = runeCount(text);
              const state = sectionState[key];
              return (
                <div key={key} className="flex flex-col gap-1.5">
                  <label htmlFor={`parent-report-${key}`} className="text-mk-small font-bold text-mk-ink">
                    {SECTION_LABELS[key]}
                  </label>
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
          </div>

          <aside className="min-w-0 min-[900px]:sticky min-[900px]:top-4" aria-label="预览">
            <div className="mb-2 text-mk-label font-bold text-mk-muted">预览</div>
            <div className="overflow-hidden rounded-mk-lg border border-mk-border bg-mk-paper min-[900px]:max-h-[calc(100vh-6rem)] min-[900px]:overflow-y-auto">
              <ParentReportView report={{ ...report.view, body: texts }} variant="teacherPreview" />
            </div>
          </aside>
        </div>
      </div>
    </div>
  );
}

function ConfirmRow({
  text,
  confirmLabel,
  danger = true,
  onConfirm,
  onCancel,
}: {
  text: string;
  confirmLabel: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <div
      className={
        "mt-3 flex flex-wrap items-center gap-2 text-mk-small font-semibold " + (danger ? "text-mk-danger" : "text-mk-ink")
      }
    >
      {text}
      <Button variant={danger ? "danger" : "primary"} size="sm" onClick={onConfirm}>
        {confirmLabel}
      </Button>
      <Button variant="ghost" size="sm" onClick={onCancel}>
        取消
      </Button>
    </div>
  );
}
