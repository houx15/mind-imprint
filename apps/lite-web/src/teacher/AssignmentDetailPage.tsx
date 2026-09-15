import { useEffect, useRef, useState, type ReactNode } from "react";
import { ArrowLeft } from "lucide-react";
import { Button, Icon } from "@/ui";
import {
  archiveAssignment,
  getAssignment,
  patchAssignment,
  type AssignmentDTO,
  type RecipientDTO,
} from "../api/assignments";
import { getLibraryShelf } from "../api/library";
import { getRoster, type RosterRow } from "../api/teacher";
import { formatDeadline, isoToBeijingInput, STATUS_LABEL } from "../shared/deadline";
import { useAlive } from "../shared/useAlive";
import { KindField, SettingsFields, StudentChecklist } from "./AssignmentForm";
import { kindLabel, safeHttpUrl } from "./format";
import { Field, INPUT_CLS } from "./formParts";
import { GradingTab } from "./GradingTab";
import { ReturnDialog } from "./ReturnDialog";
import { RubricFields } from "./RubricFields";
import { rubricScaleLabel } from "./rubricLogic";
import { TeacherPage } from "./TeacherPage";
import {
  buildPatchInput,
  canEditSettings,
  errorText,
  failText,
  fillTitleIfEmpty,
  isArchiveSuccess,
  recipientReadingText,
  settingsFromAssignment,
  settingsSummary,
  statusChipStyle,
  tabAfterAssignmentChange,
  toggleId,
  unassignedStudents,
  type EditDraft,
} from "./assignmentLogic";

/**
 * AssignmentDetailPage — `/assignments/:aid`: settings, edit, archive, the
 * recipient table, and adding students.
 *
 * Owner decision (2026-09-14): a teacher never sees a student's chat with
 * 印记. The 查看 link opens plan 1's item page, which shows her produced
 * work only.
 *
 * Mutations (save, archive, add) can resolve after she has left this
 * assignment: `alive` covers an unmount, `aidRef` covers the same instance
 * showing a different assignment. A late result from the old one does
 * nothing.
 */
export function AssignmentDetailPage({
  assignmentId,
  initialTab,
  onBack,
  onOpenItem,
  onOpenGrading,
}: {
  assignmentId: string;
  /** From the route's `?tab=grading` — set when she arrived via 返回 from
   *  the grading view, so she lands back on 批改 rather than the default
   *  学生. Read once, as the tab state's initial value; the reset effect
   *  below must NOT also reset it back to 学生 on that same first run (see
   *  `tabAfterAssignmentChange` — the effect runs after every mount, not
   *  only after a genuine assignment switch, and the fix-round-1 version of
   *  this component unconditionally called `setTab("students")` there,
   *  overwriting `initialTab` one tick after the initial render). Switching
   *  tabs by hand afterward is never overridden; navigating to a DIFFERENT
   *  assignment while this instance stays mounted still resets to 学生. */
  initialTab?: "grading";
  onBack: () => void;
  onOpenItem: (classId: string, userId: string, atomId: string) => void;
  onOpenGrading: (gradingId: string) => void;
}) {
  const [data, setData] = useState<{ assignment: AssignmentDTO; recipients: RecipientDTO[] } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  const [edit, setEdit] = useState<EditDraft | null>(null);
  const [confirmArchive, setConfirmArchive] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [returning, setReturning] = useState<RecipientDTO | null>(null);
  const [tab, setTab] = useState<"students" | "grading">(initialTab ?? "students");

  const alive = useAlive();
  const aidRef = useRef(assignmentId);
  aidRef.current = assignmentId;
  const current = (aid: string) => alive.current && aidRef.current === aid;

  // Tracks the PREVIOUS `assignmentId` across renders, so the reset effect
  // below (which also runs on the very first mount, not only on a genuine
  // switch) can tell the two apart — see `tabAfterAssignmentChange`.
  // Starts equal to the current id on purpose: the first run must see "no
  // change" and leave `tab` (seeded from `initialTab`) alone.
  const prevAssignmentIdRef = useRef(assignmentId);

  // A mutation for the previous assignment skips its own `finally` once
  // `current(aid)` is false, so the new assignment starts un-busy here.
  useEffect(() => {
    setEdit(null);
    setConfirmArchive(false);
    setMessage(null);
    setBusy(false);
    setReturning(null);
    setTab((t) => tabAfterAssignmentChange(prevAssignmentIdRef.current, assignmentId, t));
    prevAssignmentIdRef.current = assignmentId;
  }, [assignmentId]);

  useEffect(() => {
    let cancelled = false;
    setData(null);
    setError(null);
    getAssignment(assignmentId)
      .then((d) => {
        if (!cancelled) setData(d);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [assignmentId, nonce]);

  const assignment = data?.assignment ?? null;
  const recipients = data?.recipients ?? [];
  const editable = canEditSettings(recipients);

  // Library article title for the summary. Decorative: on failure the
  // summary shows the slug, which still identifies the article.
  const librarySlug =
    assignment?.kind === "reading" && assignment.payload.source === "library" && typeof assignment.payload.slug === "string"
      ? assignment.payload.slug
      : null;
  const [articleTitle, setArticleTitle] = useState<string | null>(null);
  useEffect(() => {
    setArticleTitle(null);
    if (!librarySlug) return;
    let cancelled = false;
    getLibraryShelf()
      .then((shelf) => {
        if (cancelled) return;
        const a = (Array.isArray(shelf?.articles) ? shelf.articles : []).find((x) => x.slug === librarySlug);
        setArticleTitle(a ? a.zhTitle || a.title : null);
      })
      .catch(() => {
        if (!cancelled) setArticleTitle(null);
      });
    return () => {
      cancelled = true;
    };
  }, [librarySlug]);

  // Class roster, for 添加学生.
  const classId = assignment?.classId ?? "";
  const [roster, setRoster] = useState<RosterRow[] | null>(null);
  const [rosterError, setRosterError] = useState<string | null>(null);
  const [rosterNonce, setRosterNonce] = useState(0);
  const [toAdd, setToAdd] = useState<string[]>([]);
  useEffect(() => {
    setToAdd([]);
    if (!classId) return;
    let cancelled = false;
    setRoster(null);
    setRosterError(null);
    getRoster(classId)
      .then((rows) => {
        if (!cancelled) setRoster(rows);
      })
      .catch((e: unknown) => {
        if (!cancelled) setRosterError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [classId, rosterNonce]);

  async function save() {
    if (!edit || busy) return;
    const aid = assignmentId;
    setBusy(true);
    setMessage(null);
    try {
      // A build error here must surface as a message and reset `busy`, the
      // same as a failed request — not throw uncaught past this function.
      const built = buildPatchInput(
        edit,
        editable,
        recipients.map((r) => r.userId),
      );
      if (!built.ok) {
        setMessage(built.error);
        return;
      }
      await patchAssignment(aid, built.value);
      if (!current(aid)) return;
      setEdit(null);
      setNonce((n) => n + 1);
    } catch (e) {
      if (current(aid)) setMessage(failText("修改", e));
    } finally {
      if (current(aid)) setBusy(false);
    }
  }

  async function archive() {
    if (busy) return;
    const aid = assignmentId;
    setBusy(true);
    setMessage(null);
    try {
      await archiveAssignment(aid);
      if (current(aid)) onBack();
    } catch (e) {
      if (!current(aid)) return;
      // A 404 here means it is already archived (Ruling P2-4).
      if (isArchiveSuccess(e)) onBack();
      else setMessage(failText("归档", e));
    } finally {
      if (current(aid)) setBusy(false);
    }
  }

  async function addStudents() {
    if (busy || toAdd.length === 0) return;
    const aid = assignmentId;
    setBusy(true);
    setMessage(null);
    try {
      await patchAssignment(aid, { addUserIds: toAdd });
      if (!current(aid)) return;
      setToAdd([]);
      setNonce((n) => n + 1);
    } catch (e) {
      if (current(aid)) setMessage(failText("添加", e));
    } finally {
      if (current(aid)) setBusy(false);
    }
  }

  const unassigned = roster ? unassignedStudents(roster, recipients) : null;
  const personalized = assignment?.kind === "reading" && assignment.payload.source === "personalized";

  return (
    <TeacherPage>
      <button
        type="button"
        onClick={onBack}
        className="flex items-center gap-1.5 rounded-mk-sm text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      >
        <Icon icon={ArrowLeft} size={15} />
        返回
      </button>

      {error ? (
        <div className="mt-4 text-mk-small font-semibold text-mk-danger">
          加载失败：{error}{" "}
          <button type="button" onClick={() => setNonce((n) => n + 1)} className="cursor-pointer underline">
            重试
          </button>
        </div>
      ) : assignment === null ? (
        <div className="mt-4 text-mk-body text-mk-muted">加载中…</div>
      ) : (
        <>
          {edit ? (
            <form
              // noValidate: same reason as AssignmentForm — buildPatchInput's
              // messages, not the browser's tooltips.
              noValidate
              className="mt-4 flex flex-col gap-5 rounded-mk-lg border border-mk-border bg-mk-surface p-4 sm:p-6"
              onSubmit={(e) => {
                e.preventDefault();
                void save();
              }}
            >
              {editable && (
                <KindField
                  value={edit.settings.kind}
                  onChange={(kind) => setEdit((d) => (d ? { ...d, settings: { ...d.settings, kind } } : d))}
                />
              )}
              <Field label="标题">
                <input
                  value={edit.title}
                  onChange={(e) => setEdit((d) => (d ? { ...d, title: e.target.value } : d))}
                  maxLength={200}
                  className={INPUT_CLS}
                />
              </Field>
              <Field label="说明">
                <textarea
                  value={edit.instructions}
                  onChange={(e) => setEdit((d) => (d ? { ...d, instructions: e.target.value } : d))}
                  rows={3}
                  className={INPUT_CLS}
                />
              </Field>
              <Field label="截止时间（北京时间）">
                <input
                  type="datetime-local"
                  value={edit.dueInput}
                  onChange={(e) => setEdit((d) => (d ? { ...d, dueInput: e.target.value } : d))}
                  className={INPUT_CLS}
                />
              </Field>
              {editable ? (
                <SettingsFields
                  value={edit.settings}
                  onChange={(update) => setEdit((d) => (d ? { ...d, settings: update(d.settings) } : d))}
                  onExtractedTitle={(title) => setEdit((d) => (d ? { ...d, title: fillTitleIfEmpty(d.title, title) } : d))}
                  classId={assignment.classId}
                  recipientIds={recipients.map((r) => r.userId)}
                />
              ) : (
                <>
                  <p className="text-mk-small text-mk-muted">已有学生开始这份作业，类型和设置不能再修改</p>
                  {edit.settings.kind === "writing" && edit.settings.rubric && (
                    <RubricFields
                      value={edit.settings.rubric}
                      onChange={(update) =>
                        setEdit((d) =>
                          d && d.settings.rubric ? { ...d, settings: { ...d.settings, rubric: update(d.settings.rubric) } } : d,
                        )
                      }
                    />
                  )}
                </>
              )}
              <div className="flex flex-wrap items-center gap-3 border-t border-mk-border pt-4">
                <Button type="submit" variant="primary" size="sm" disabled={busy}>
                  保存
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setEdit(null)} disabled={busy}>
                  取消
                </Button>
                {message && (
                  <span className="text-mk-small font-semibold text-mk-danger" role="alert">
                    {message}
                  </span>
                )}
              </div>
            </form>
          ) : (
            <AssignmentHeader
              assignment={assignment}
              articleTitle={articleTitle}
              busy={busy}
              confirmArchive={confirmArchive}
              message={message}
              onEdit={() => {
                setMessage(null);
                setConfirmArchive(false);
                // `originalRubric` is a snapshot taken here, once — it must
                // not be re-derived from `settings.rubric` later, since that
                // field changes as she edits the rubric fields; this is what
                // buildPatchInput compares against to decide whether the
                // rubric actually changed.
                const settings = settingsFromAssignment(assignment.kind, assignment.payload);
                setEdit({
                  title: assignment.title,
                  instructions: assignment.instructions,
                  dueInput: isoToBeijingInput(assignment.dueAt),
                  settings,
                  originalRubric: settings.rubric,
                });
              }}
              onArchive={() => {
                setMessage(null);
                setConfirmArchive(true);
              }}
              onConfirmArchive={() => void archive()}
              onCancelArchive={() => setConfirmArchive(false)}
            />
          )}

          {assignment.kind === "writing" && (
            <div role="tablist" aria-label="作业视图" className="mt-8 flex gap-2 border-b border-mk-border">
              {(["students", "grading"] as const).map((key) => (
                <button
                  key={key}
                  type="button"
                  role="tab"
                  aria-selected={tab === key}
                  onClick={() => setTab(key)}
                  className={
                    "-mb-px border-b-2 px-3 py-2 text-mk-small transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 " +
                    (tab === key ? "border-mk-accent font-bold text-mk-accent-700" : "border-transparent text-mk-muted hover:text-mk-ink")
                  }
                >
                  {key === "students" ? "学生" : "批改"}
                </button>
              ))}
            </div>
          )}
          {assignment.kind === "writing" && tab === "grading" ? (
            <GradingTab assignmentId={assignment.id} onOpenGrading={onOpenGrading} />
          ) : (
            <>
            <section className="mt-8">
              <h2 className="text-mk-h3 text-mk-ink">学生</h2>
              {recipients.length === 0 ? (
                <p className="mt-2 text-mk-small text-mk-muted">暂无学生</p>
              ) : (
                <div className="mt-3 overflow-x-auto rounded-mk-lg border border-mk-border bg-mk-surface">
                  <table className="w-full min-w-[600px] border-collapse">
                    <thead>
                      <tr>
                        {["学生", ...(personalized ? ["文章"] : []), "状态", "开始时间", "完成时间", "操作"].map((h) => (
                          <th
                            key={h}
                            className="whitespace-nowrap border-b border-mk-border px-3 py-2.5 text-left text-mk-label font-bold text-mk-muted"
                          >
                            {h}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {recipients.map((r) => (
                        <tr key={r.userId}>
                          <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small font-bold text-mk-ink">
                            {r.displayName}
                          </td>
                          {personalized && (
                            <td className="border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">{recipientReadingText(r.reading)}</td>
                          )}
                          <td className="whitespace-nowrap border-b border-mk-border px-3 py-3">
                            <StatusChip status={r.status} label={r.statusLabel || STATUS_LABEL[r.status]} />
                          </td>
                          <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                            {r.startedAt ? formatDeadline(r.startedAt) : "—"}
                          </td>
                          <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                            {r.finishedAt ? formatDeadline(r.finishedAt) : "—"}
                          </td>
                          <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small">
                            <div className="flex items-center gap-3">
                              {r.atomId ? (
                                <Button variant="link" size="sm" onClick={() => onOpenItem(assignment.classId, r.userId, r.atomId ?? "")}>
                                  查看
                                </Button>
                              ) : (
                                <span className="text-mk-muted">—</span>
                              )}
                              {assignment.kind === "writing" && r.versionCount > 0 && (
                                <Button variant="link" size="sm" onClick={() => setReturning(r)}>
                                  退回修改
                                </Button>
                              )}
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </section>

            <section className="mt-8">
              <h2 className="text-mk-h3 text-mk-ink">添加学生</h2>
              {rosterError ? (
                <div className="mt-2 text-mk-small font-semibold text-mk-danger">
                  加载失败：{rosterError}{" "}
                  <button type="button" onClick={() => setRosterNonce((n) => n + 1)} className="cursor-pointer underline">
                    重试
                  </button>
                </div>
              ) : unassigned === null ? (
                <p className="mt-2 text-mk-small text-mk-muted">加载中…</p>
              ) : unassigned.length === 0 ? (
                <p className="mt-2 text-mk-small text-mk-muted">暂无未布置的学生</p>
              ) : (
                <div className="mt-3 flex flex-col gap-3">
                  <StudentChecklist students={unassigned} selected={toAdd} onToggle={(id) => setToAdd((ids) => toggleId(ids, id))} />
                  <div>
                    <Button variant="secondary" size="sm" onClick={() => void addStudents()} disabled={busy || toAdd.length === 0}>
                      添加学生
                    </Button>
                  </div>
                </div>
              )}
            </section>
            </>
          )}
        </>
      )}

      {returning && assignment && (
        <ReturnDialog
          assignmentId={assignment.id}
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

/** Status chip: a `color-mix` tint per status, no left bar. Also used on the
 * teacher's student page. */
export function StatusChip({ status, label }: { status: string; label: string }) {
  return (
    <span className="inline-block whitespace-nowrap rounded-mk-full px-2.5 py-0.5 text-mk-small font-bold" style={statusChipStyle(status)}>
      {label}
    </span>
  );
}

function AssignmentHeader({
  assignment,
  articleTitle,
  busy,
  confirmArchive,
  message,
  onEdit,
  onArchive,
  onConfirmArchive,
  onCancelArchive,
}: {
  assignment: AssignmentDTO;
  articleTitle: string | null;
  busy: boolean;
  confirmArchive: boolean;
  message: string | null;
  onEdit: () => void;
  onArchive: () => void;
  onConfirmArchive: () => void;
  onCancelArchive: () => void;
}) {
  const { kind, payload } = assignment;
  const settings = settingsFromAssignment(kind, payload);
  const linkHref = kind === "reading" && settings.readingSource === "url" ? safeHttpUrl(settings.url) : null;

  return (
    <>
      <p className="learning-landing-kicker mt-4">{kindLabel(kind)}</p>
      <div className="mt-1 flex flex-wrap items-center gap-3">
        <h1 className="teacher-page-title">{assignment.title}</h1>
        <div className="flex flex-wrap gap-2 sm:ml-auto">
          <Button variant="secondary" size="sm" onClick={onEdit} disabled={busy}>
            修改
          </Button>
          <Button variant="secondary" size="sm" onClick={onArchive} disabled={busy}>
            归档
          </Button>
        </div>
      </div>

      {confirmArchive && (
        <div className="mt-3 flex flex-wrap items-center gap-2 text-mk-small font-semibold text-mk-danger">
          归档后学生将不再看到这份作业，已开始的内容仍保留。
          <Button variant="danger" size="sm" onClick={onConfirmArchive} disabled={busy}>
            确认归档
          </Button>
          <Button variant="ghost" size="sm" onClick={onCancelArchive} disabled={busy}>
            取消
          </Button>
        </div>
      )}
      {message && (
        <div className="mt-3 text-mk-small font-semibold text-mk-danger" role="alert">
          {message}
        </div>
      )}

      <dl className="mt-4 grid grid-cols-1 gap-3 rounded-mk-lg border border-mk-border bg-mk-surface p-4 sm:grid-cols-[120px_1fr]">
        <InfoRow label="截止时间">{formatDeadline(assignment.dueAt)}</InfoRow>
        <InfoRow label="设置">{settingsSummary(kind, payload, articleTitle)}</InfoRow>
        {kind === "reading" && settings.readingSource === "url" && (
          <InfoRow label="链接">
            {linkHref ? (
              <a href={linkHref} target="_blank" rel="noreferrer" className="break-all text-mk-accent-700 underline">
                {settings.url}
              </a>
            ) : (
              <span className="break-all">{settings.url}</span>
            )}
          </InfoRow>
        )}
        {kind === "writing" && settings.prompt && <InfoRow label="题目">{settings.prompt}</InfoRow>}
        {kind === "writing" && settings.rubric && (
          <InfoRow label="评分标准">
            {rubricScaleLabel(settings.rubric)} · {settings.rubric.dimensions.map((d) => d.name).join(" / ") || "—"}
          </InfoRow>
        )}
        {kind === "writing" && settings.rubric && settings.rubric.focus.trim() && (
          <InfoRow label="批改重点">{settings.rubric.focus}</InfoRow>
        )}
        {kind === "project" && settings.drivingQuestion && <InfoRow label="驱动问题">{settings.drivingQuestion}</InfoRow>}
        {kind === "project" && settings.description && <InfoRow label="补充说明">{settings.description}</InfoRow>}
        {assignment.instructions && <InfoRow label="说明">{assignment.instructions}</InfoRow>}
      </dl>
    </>
  );
}

function InfoRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <dt className="text-mk-label font-bold text-mk-muted">{label}</dt>
      <dd className="whitespace-pre-wrap text-mk-small text-mk-ink">{children}</dd>
    </>
  );
}
