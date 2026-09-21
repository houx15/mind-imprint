import { useEffect, useRef, useState } from "react";
import { Button } from "@/ui";
import { api, type ClassSummary } from "@/api";
import { createAssignment, extractWritingFields, type AssignmentKind } from "../api/assignments";
import { getRoster, type RosterRow } from "../api/teacher";
import { useAlive } from "../shared/useAlive";
import { postWorkspaceTurn } from "../api/teacherWorkspace";
import { AssignmentAIMode } from "./AssignmentAIMode";
import { useWorkspaceThread } from "./workspace/useWorkspaceThread";
import { Field, INPUT_CLS, Segmented } from "./formParts";
import { KIND_OPTIONS, SOURCE_OPTIONS } from "./labels";
import { LibraryPicker } from "./LibraryPicker";
import { PersonalizedPicker } from "./PersonalizedPicker";
import { RubricFields } from "./RubricFields";
import { rubricDefaultNote } from "./rubricLogic";
import { BackLink, FormStep, StudioEmpty, StudioError, StudioHeading, StudioLoading } from "./StudioArtwork";
import { TeacherPage } from "./TeacherPage";
import { UploadSourceField } from "./UploadSourceField";
import { DateField } from "./controls/DateField";
import { NumberField } from "./controls/NumberField";
import { Select } from "./controls/Select";
import { PromptLibraryPicker } from "./PromptLibraryPicker";
import {
  buildCreateInput,
  draftOnClassChange,
  emptySettings,
  errorText,
  failText,
  clearPendingRecipients,
  fillTitleIfEmpty,
  initialRecipients,
  pickClassId,
  publishSummary,
  readAssignmentMode,
  readLastClassId,
  toggleId,
  workspaceArtifactPayload,
  writeAssignmentMode,
  readPendingRecipients,
  writeLastClassId,
  type AssignmentDraft,
  type AssignmentMode,
  type PendingRecipients,
  type SettingsDraft,
} from "./assignmentLogic";

/**
 * AssignmentForm — `/assignments/new`. One page: class, kind, title,
 * instructions, Beijing deadline, the kind's settings, and the recipient
 * checklist (every enrolled student checked by default, spec DEC-4).
 *
 * The request body is built and checked by `buildCreateInput`
 * (assignmentLogic.ts); this file only holds state and renders.
 */

const MODE_OPTIONS: { value: AssignmentMode; label: string }[] = [
  { value: "traditional", label: "传统" },
  { value: "ai", label: "AI" },
];

/** 类型. Rendered by the form right after 班级 (and first in the detail page's
 * edit), ahead of 标题 / 说明 / 截止时间, per the spec's field order. */
export function KindField({ value, onChange }: { value: AssignmentKind; onChange: (kind: AssignmentKind) => void }) {
  return <Segmented label="类型" options={KIND_OPTIONS} value={value} onChange={onChange} />;
}

/** The chosen kind's settings. Shared by this form and the detail page's
 * edit. `onChange` takes an updater so an extraction that returns after the
 * teacher kept typing does not overwrite the other fields with a stale copy. */
export function SettingsFields({
  value,
  onChange,
  onExtractedTitle,
  classId,
  recipientIds,
}: {
  value: SettingsDraft;
  onChange: (update: (d: SettingsDraft) => SettingsDraft) => void;
  /** Called with the uploaded document's title; the page fills 标题 only when it is empty. */
  onExtractedTitle?: (title: string) => void;
  /** The class whose students the personalised preview covers. */
  classId: string;
  /** Checked recipients: the preview table and the saved picks keep only these. */
  recipientIds: string[];
}) {
  const set = (patch: Partial<SettingsDraft>) => onChange((d) => ({ ...d, ...patch }));
  return (
    <>
      {value.kind === "reading" && (
        <>
          <Segmented
            label="来源"
            options={SOURCE_OPTIONS}
            value={value.readingSource}
            onChange={(readingSource) => set({ readingSource })}
          />
          {value.readingSource === "library" && (
            <LibraryPicker classId={classId} slug={value.slug} tier={value.tier} onChange={(next) => set(next)} />
          )}
          {value.readingSource === "url" && (
            <Field label="链接">
              <input
                type="url"
                inputMode="url"
                value={value.url}
                onChange={(e) => set({ url: e.target.value })}
                placeholder="https://"
                className={INPUT_CLS}
              />
            </Field>
          )}
          {value.readingSource === "text" && (
            <Field label="正文">
              <textarea
                value={value.text}
                onChange={(e) => set({ text: e.target.value })}
                rows={10}
                className={INPUT_CLS}
              />
            </Field>
          )}
          {value.readingSource === "file" && (
            <UploadSourceField
              text={value.text}
              fileName={value.fileName}
              onExtracted={(r) => {
                onChange((d) => ({ ...d, text: r.text, fileName: r.fileName }));
                onExtractedTitle?.(r.title);
              }}
              onTextChange={(text) => set({ text })}
            />
          )}
          {value.readingSource === "personalized" && (
            <PersonalizedPicker value={value} onChange={onChange} classId={classId} recipientIds={recipientIds} />
          )}
        </>
      )}

      {value.kind === "writing" && (
        <>
          <WritingExtract
            onFill={(r) =>
              onChange((d) => ({
                ...d,
                prompt: r.prompt,
                // No count in the instructions → the field stays empty.
                targetWords: r.targetWords === null ? "" : String(r.targetWords),
                lang: r.lang,
              }))
            }
          />
          {/* 第三条路：从 705 道真题里挑一道。终点和上面那条一样 ——
              都只是把字填进下面那个 textarea，填完她照样能改。 */}
          <PromptLibraryPicker
            onFill={(r) =>
              onChange((d) => ({
                ...d,
                prompt: r.prompt,
                targetWords: r.targetWords === null ? "" : String(r.targetWords),
                lang: r.lang,
              }))
            }
          />
          <Field label="题目">
            <textarea value={value.prompt} onChange={(e) => set({ prompt: e.target.value })} rows={4} className={INPUT_CLS} />
          </Field>
          <div className="flex flex-col gap-4 sm:flex-row">
            <div className="sm:w-[200px]">
              <Field label="目标字数">
                <NumberField min={50} max={100000} step={50} value={value.targetWords} onChange={(targetWords) => set({ targetWords })} className="w-full" />
              </Field>
            </div>
            <Segmented
              label="语言"
              options={[
                { value: "zh", label: "中文" },
                { value: "en", label: "英文" },
              ]}
              value={value.lang}
              onChange={(lang) => set({ lang })}
            />
          </div>
          {value.rubric ? (
            <RubricFields
              value={value.rubric}
              onChange={(update) => onChange((d) => (d.rubric ? { ...d, rubric: update(d.rubric) } : d))}
            />
          ) : (
            <p className="text-mk-small text-mk-muted">{rubricDefaultNote(value.lang)}</p>
          )}
        </>
      )}

      {value.kind === "project" && (
        <>
          <Field label="驱动问题">
            <textarea
              value={value.drivingQuestion}
              onChange={(e) => set({ drivingQuestion: e.target.value })}
              rows={3}
              className={INPUT_CLS}
            />
          </Field>
          <Field label="补充说明">
            <textarea
              value={value.description}
              onChange={(e) => set({ description: e.target.value })}
              rows={4}
              className={INPUT_CLS}
            />
          </Field>
        </>
      )}
    </>
  );
}

function WritingExtract({
  onFill,
}: {
  onFill: (r: { prompt: string; targetWords: number | null; lang: "zh" | "en" }) => void;
}) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const alive = useAlive();

  async function run() {
    if (!text.trim() || busy) return;
    setBusy(true);
    setError(null);
    try {
      const r = await extractWritingFields(text);
      if (alive.current) onFill(r);
    } catch (e) {
      if (alive.current) setError(failText("提取", e));
    } finally {
      if (alive.current) setBusy(false);
    }
  }

  return (
    <details className="rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2">
      <summary className="cursor-pointer text-mk-small font-bold text-mk-ink">从作业说明提取</summary>
      <div className="mt-3 flex flex-col gap-2 pb-1">
        <Field label="作业说明">
          <textarea value={text} onChange={(e) => setText(e.target.value)} rows={5} className={INPUT_CLS} />
        </Field>
        <div className="flex flex-wrap items-center gap-3">
          <Button variant="secondary" size="sm" onClick={() => void run()} disabled={busy || !text.trim()}>
            {busy ? "提取中" : "提取"}
          </Button>
          {error && <span className="text-mk-small font-semibold text-mk-danger">{error}</span>}
        </div>
      </div>
    </details>
  );
}

export function AssignmentForm({
  initialClassId,
  onBack,
  onCreated,
}: {
  initialClassId?: string;
  onBack: () => void;
  onCreated: (assignmentId: string) => void;
}) {
  const [mode, setModeState] = useState<AssignmentMode>(() => readAssignmentMode());
  function setMode(next: AssignmentMode) {
    writeAssignmentMode(next);
    setModeState(next);
  }

  const [classes, setClasses] = useState<ClassSummary[] | null>(null);
  const [classesError, setClassesError] = useState<string | null>(null);
  const [classesNonce, setClassesNonce] = useState(0);

  // Shared by both modes — switching `mode` never touches `draft`, so
  // whatever she typed on either side survives the toggle.
  const [draft, setDraft] = useState<AssignmentDraft>(() => ({
    ...emptySettings("reading"),
    classId: "",
    title: "",
    instructions: "",
    dueInput: "",
    userIds: [],
  }));

  // The AI mode's conversation. Held here, above the mode toggle, so
  // switching to 传统 and back keeps it (AssignmentAIMode unmounts).
  const thread = useWorkspaceThread<AssignmentDraft>({
    artifact: draft,
    setArtifact: setDraft,
    scopeOf: (d) => d.classId,
    post: ({ artifact, turns, input }) =>
      postWorkspaceTurn({
        surface: "assignment",
        classId: artifact.classId,
        // Only the fields the endpoint reads — never the whole draft,
        // which can carry a 50000-rune pasted-text material that has
        // nothing to do with what the model needs to see.
        artifact: workspaceArtifactPayload(artifact),
        turns,
        ...("text" in input ? { text: input.text } : { choiceId: input.choiceId, choiceSlug: input.slug, choiceLabel: input.label }),
      }).then((res) => ({ ...res, patch: res.patch as Partial<AssignmentDraft> })),
    // The server already prefixes its message with 「对话失败：」; `failText`
    // does not double it.
    describeError: (e) => failText("对话", e),
  });

  /** She picked a different class, in either mode. Goes through
   * `draftOnClassChange` — setting `classId` directly would leave the old
   * class's personalised-reading `picks` attached. The conversation is reset
   * in both modes: the server scopes each turn to `classId`, so turns from
   * the old class must not be sent with the new one, and `reset()` bumps the
   * generation so a turn in flight for the old class is dropped even if she
   * switches back to it before it lands. The roster refetch happens in the
   * effect on `draft.classId` below. The AI mode's `<select>` stays enabled
   * while a turn is in flight; the generation check is what makes that safe. */
  function changeClass(classId: string) {
    writeLastClassId(classId);
    switchClass(classId);
  }

  /** `changeClass` without remembering the class: used when the page, not
   * she, replaces the class (the class-list effect below). */
  function switchClass(classId: string) {
    thread.reset();
    setDraft((d) => draftOnClassChange(d, classId));
  }

  // The class id as of this render, for the class-list effect: it runs in a
  // `.then` and must know whether a class was already set.
  const classIdRef = useRef(draft.classId);
  classIdRef.current = draft.classId;

  const [roster, setRoster] = useState<RosterRow[] | null>(null);
  const [rosterError, setRosterError] = useState<string | null>(null);
  const [rosterNonce, setRosterNonce] = useState(0);

  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const alive = useAlive();

  useEffect(() => {
    let cancelled = false;
    setClasses(null);
    setClassesError(null);
    api
      .listClasses()
      .then((list) => {
        if (cancelled) return;
        setClasses(list);
        const classId = pickClassId(
          list.map((c) => c.id),
          initialClassId,
          readLastClassId(),
        );
        // First load (no class yet, so no conversation): set it directly.
        // Replacing a class she already had (e.g. `initialClassId` changed,
        // or 重试 after a list error) goes through `switchClass`, so the
        // conversation resets and a turn in flight is dropped.
        if (classIdRef.current && classIdRef.current !== classId) switchClass(classId);
        else setDraft((d) => ({ ...d, classId }));
      })
      .catch((e: unknown) => {
        if (!cancelled) setClassesError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [initialClassId, classesNonce]);

  // Students handed over by the class chat (「给这些学生布置作业」), read
  // once when the page opens and used on the first roster of their class.
  const [pending, setPending] = useState<PendingRecipients | null>(() => readPendingRecipients());
  useEffect(() => clearPendingRecipients(), []);
  const [preselectNote, setPreselectNote] = useState<string | null>(null);

  // Roster of the chosen class; switching class re-checks every student of
  // the new class (DEC-4 default), unless the class chat named students.
  useEffect(() => {
    if (!draft.classId) return;
    let cancelled = false;
    setRoster(null);
    setRosterError(null);
    setDraft((d) => ({ ...d, userIds: [] }));
    getRoster(draft.classId)
      .then((rows) => {
        if (cancelled) return;
        setRoster(rows);
        const ids = rows.map((s) => s.id);
        const picked = initialRecipients(ids, draft.classId, pending);
        if (pending && pending.classId === draft.classId) {
          setPending(null);
          setPreselectNote(picked.length < ids.length ? `已按班级对话选中 ${picked.length} 名学生` : null);
        } else {
          setPreselectNote(null);
        }
        setDraft((d) => ({ ...d, userIds: picked }));
      })
      .catch((e: unknown) => {
        if (!cancelled) setRosterError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
    // `pending` is read when the class's roster loads, not watched.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draft.classId, rosterNonce]);

  async function submit() {
    if (busy) return;
    const built = buildCreateInput(draft);
    if (!built.ok) {
      setMessage(built.error);
      return;
    }
    setMessage(null);
    setBusy(true);
    const classId = draft.classId;
    try {
      const created = await createAssignment(classId, built.value);
      if (!alive.current) return;
      writeLastClassId(classId);
      onCreated(created.id);
    } catch (e) {
      if (alive.current) setMessage(failText("发布", e));
    } finally {
      if (alive.current) setBusy(false);
    }
  }

  const header = (
    <>
      <BackLink label="返回作业" onClick={onBack} />

      <StudioHeading
        kicker="作业"
        title="布置作业"
        description={
          mode === "ai"
            ? "作业会显示在学生首页和收件箱。请在右侧向印记说明要求，印记填写左侧的作业卡，发布前可修改。"
            : "作业会显示在学生首页和收件箱。请按顺序填写四项内容，然后发布。"
        }
        kind="ideas"
        actions={<Segmented label="模式" options={MODE_OPTIONS} value={mode} onChange={setMode} />}
      />
      {preselectNote && (
        <p className="mb-2 text-mk-small font-semibold text-mk-accent-700" role="status">
          {preselectNote}
        </p>
      )}
    </>
  );

  // AI mode is a workspace page of its own (`WorkspacePanel` draws the page,
  // with this header above both columns). Nesting it inside this page's
  // `TeacherPage` applied the page padding twice: the panel sat 33px right of
  // the title and 30px lower than the canvas.
  if (mode === "ai" && classes !== null && classes.length > 0 && !classesError) {
    return (
      <AssignmentAIMode
        draft={draft}
        setDraft={setDraft}
        classes={classes}
        roster={roster}
        rosterError={rosterError}
        onRosterRetry={() => setRosterNonce((n) => n + 1)}
        onCreated={onCreated}
        thread={thread}
        onClassChange={changeClass}
        header={header}
      />
    );
  }

  const className = classes?.find((c) => c.id === draft.classId)?.name ?? "";

  return (
    <TeacherPage width="narrow">
      {header}

      {classesError ? (
        <StudioError message={classesError} onRetry={() => setClassesNonce((n) => n + 1)} />
      ) : classes === null ? (
        <StudioLoading />
      ) : classes.length === 0 ? (
        <StudioEmpty kind="discovery" title="暂无班级">
          布置作业需要先有班级。请联系管理员为你分配班级。
        </StudioEmpty>
      ) : (
        <form
          // noValidate: the browser's own tooltips (type=url, number min)
          // would block submit before buildCreateInput shows its message.
          noValidate
          className="teacher-form-card"
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <FormStep index={1} title="班级与类型" note="类型决定下面要填写的材料。">
            <Field label="班级">
              <Select value={draft.classId} onChange={changeClass} options={classes.map((c) => ({ value: c.id, label: c.name }))} />
            </Field>
            <KindField value={draft.kind} onChange={(kind) => setDraft((d) => ({ ...d, kind }))} />
          </FormStep>

          <FormStep index={2} title="标题与说明" note="显示在学生的作业卡上。">
            <Field label="标题">
              <input
                value={draft.title}
                onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))}
                maxLength={200}
                className={INPUT_CLS}
              />
            </Field>

            <Field label="说明">
              <textarea
                value={draft.instructions}
                onChange={(e) => setDraft((d) => ({ ...d, instructions: e.target.value }))}
                rows={3}
                className={INPUT_CLS}
              />
            </Field>
          </FormStep>

          <FormStep
            index={3}
            title={draft.kind === "reading" ? "阅读材料" : draft.kind === "writing" ? "写作要求" : "项目要求"}
            note={
              draft.kind === "reading"
                ? "从分级阅读库选择文章，或提供链接、正文、文档。"
                : draft.kind === "writing"
                  ? "题目、字数与评分标准，AI 批改时使用。"
                  : "驱动问题是学生立项的起点。"
            }
          >
            <SettingsFields
              value={draft}
              onChange={(update) => setDraft((d) => ({ ...d, ...update(d) }))}
              onExtractedTitle={(title) => setDraft((d) => ({ ...d, title: fillTitleIfEmpty(d.title, title) }))}
              classId={draft.classId}
              recipientIds={draft.userIds}
            />
          </FormStep>

          <FormStep index={4} title="学生与截止时间" note="默认选中班级全部学生。">
            <Field label="截止时间（北京时间）">
              <DateField withTime shortcuts value={draft.dueInput} onChange={(dueInput) => setDraft((d) => ({ ...d, dueInput }))} />
            </Field>

            <RecipientChecklist
              roster={roster}
              error={rosterError}
              onRetry={() => setRosterNonce((n) => n + 1)}
              selected={draft.userIds}
              onChange={(userIds) => setDraft((d) => ({ ...d, userIds }))}
            />
          </FormStep>

          <div className="teacher-publish-bar">
            <p>
              <strong>{publishSummary(className, draft.userIds.length, draft.dueInput)}</strong>
              {message && (
                <span className="mt-1 block font-semibold text-mk-danger" role="alert">
                  {message}
                </span>
              )}
            </p>
            <div>
              <Button type="submit" variant="primary" disabled={busy}>
                {busy ? "发布中" : "发布作业"}
              </Button>
            </div>
          </div>
        </form>
      )}
    </TeacherPage>
  );
}

export function RecipientChecklist({
  roster,
  error,
  onRetry,
  selected,
  onChange,
}: {
  roster: RosterRow[] | null;
  error: string | null;
  onRetry: () => void;
  selected: string[];
  onChange: (ids: string[]) => void;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex flex-wrap items-center gap-3">
        <span className="text-mk-label font-bold text-mk-muted">学生</span>
        {roster && roster.length > 0 && (
          <>
            <span className="text-mk-small text-mk-muted">
              已选 {selected.length}/{roster.length}
            </span>
            <Button variant="link" size="sm" onClick={() => onChange(roster.map((s) => s.id))}>
              全选
            </Button>
            <Button variant="link" size="sm" onClick={() => onChange([])}>
              全不选
            </Button>
          </>
        )}
      </div>
      {error ? (
        <StudioError message={error} onRetry={onRetry} />
      ) : roster === null ? (
        <StudioLoading />
      ) : roster.length === 0 ? (
        <div className="text-mk-small text-mk-muted">本班暂无学生。学生凭邀请码加入后才能布置。</div>
      ) : (
        <StudentChecklist students={roster} selected={selected} onToggle={(id) => onChange(toggleId(selected, id))} />
      )}
    </div>
  );
}

export function StudentChecklist({
  students,
  selected,
  onToggle,
}: {
  students: RosterRow[];
  selected: string[];
  onToggle: (id: string) => void;
}) {
  return (
    <div className="grid max-h-[320px] grid-cols-1 gap-1 overflow-y-auto rounded-mk-md border border-mk-border p-2 sm:grid-cols-2">
      {students.map((s) => (
        <label
          key={s.id}
          className="flex cursor-pointer items-center gap-2 rounded-mk-sm px-2 py-1.5 text-mk-small text-mk-ink hover:bg-mk-accent-50"
        >
          <input type="checkbox" className="tc-check" checked={selected.includes(s.id)} onChange={() => onToggle(s.id)} />
          {s.displayName}
        </label>
      ))}
    </div>
  );
}
