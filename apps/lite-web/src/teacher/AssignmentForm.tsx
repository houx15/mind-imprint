import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { Button, Icon } from "@/ui";
import { api, type ClassSummary } from "@/api";
import { createAssignment, extractWritingFields, type AssignmentKind } from "../api/assignments";
import { getRoster, type RosterRow } from "../api/teacher";
import { useAlive } from "../shared/useAlive";
import { AssignmentAIMode } from "./AssignmentAIMode";
import { Field, INPUT_CLS, Segmented } from "./formParts";
import { LibraryPicker } from "./LibraryPicker";
import { PersonalizedPicker } from "./PersonalizedPicker";
import { RubricFields } from "./RubricFields";
import { rubricDefaultNote } from "./rubricLogic";
import { StudioEmpty } from "./StudioArtwork";
import { TeacherPage } from "./TeacherPage";
import { UploadSourceField } from "./UploadSourceField";
import {
  buildCreateInput,
  draftOnClassChange,
  emptySettings,
  errorText,
  failText,
  fillTitleIfEmpty,
  pickClassId,
  readAssignmentMode,
  readLastClassId,
  toggleId,
  writeAssignmentMode,
  writeLastClassId,
  type AssignmentDraft,
  type AssignmentMode,
  type ReadingSource,
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

const KIND_OPTIONS: { value: AssignmentKind; label: string }[] = [
  { value: "reading", label: "阅读" },
  { value: "writing", label: "写作" },
  { value: "project", label: "项目" },
];

const MODE_OPTIONS: { value: AssignmentMode; label: string }[] = [
  { value: "traditional", label: "传统" },
  { value: "ai", label: "AI" },
];

const SOURCE_OPTIONS: { value: ReadingSource; label: string }[] = [
  { value: "library", label: "分级阅读库" },
  { value: "url", label: "链接" },
  { value: "text", label: "正文" },
  { value: "file", label: "上传文件" },
  { value: "personalized", label: "个性化" },
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
            <LibraryPicker slug={value.slug} tier={value.tier} onChange={(next) => set(next)} />
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
          <Field label="题目">
            <textarea value={value.prompt} onChange={(e) => set({ prompt: e.target.value })} rows={4} className={INPUT_CLS} />
          </Field>
          <div className="flex flex-col gap-4 sm:flex-row">
            <div className="sm:w-[200px]">
              <Field label="目标字数">
                <input
                  type="number"
                  min={1}
                  max={100000}
                  step={1}
                  inputMode="numeric"
                  value={value.targetWords}
                  onChange={(e) => set({ targetWords: e.target.value })}
                  className={INPUT_CLS}
                />
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
        setDraft((d) => ({ ...d, classId }));
      })
      .catch((e: unknown) => {
        if (!cancelled) setClassesError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [initialClassId, classesNonce]);

  // Roster of the chosen class; switching class re-checks every student of
  // the new class (DEC-4 default).
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
        setDraft((d) => ({ ...d, userIds: rows.map((s) => s.id) }));
      })
      .catch((e: unknown) => {
        if (!cancelled) setRosterError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
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

  return (
    <TeacherPage width="narrow">
      <button
        type="button"
        onClick={onBack}
        className="flex items-center gap-1.5 rounded-mk-sm text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      >
        <Icon icon={ArrowLeft} size={15} />
        返回
      </button>

      <h1 className="teacher-page-title mt-4">布置作业</h1>
      <p className="mt-2 text-mk-small text-mk-muted">学生会在收件箱和对应页面顶部看到这份作业。</p>

      <div className="mt-4">
        <Segmented label="模式" options={MODE_OPTIONS} value={mode} onChange={setMode} />
      </div>

      {classesError ? (
        <div className="mt-6 text-mk-small font-semibold text-mk-danger">
          加载失败：{classesError}{" "}
          <button type="button" onClick={() => setClassesNonce((n) => n + 1)} className="cursor-pointer underline">
            重试
          </button>
        </div>
      ) : classes === null ? (
        <div className="mt-6 text-mk-body text-mk-muted">加载中…</div>
      ) : classes.length === 0 ? (
        <StudioEmpty kind="discovery">暂无班级。请联系管理员为你分配班级。</StudioEmpty>
      ) : mode === "ai" ? (
        <AssignmentAIMode
          draft={draft}
          setDraft={setDraft}
          classes={classes}
          roster={roster}
          rosterError={rosterError}
          onRosterRetry={() => setRosterNonce((n) => n + 1)}
          onCreated={onCreated}
        />
      ) : (
        <form
          // noValidate: the browser's own tooltips (type=url, number min)
          // would block submit before buildCreateInput shows its message.
          noValidate
          className="mt-6 flex flex-col gap-5 rounded-mk-lg border border-mk-border bg-mk-surface p-4 sm:p-6"
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <Field label="班级">
            <select
              value={draft.classId}
              onChange={(e) => {
                const classId = e.target.value;
                writeLastClassId(classId);
                setDraft((d) => draftOnClassChange(d, classId));
              }}
              className={INPUT_CLS}
            >
              {classes.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </Field>

          <KindField value={draft.kind} onChange={(kind) => setDraft((d) => ({ ...d, kind }))} />

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

          <Field label="截止时间（北京时间）">
            <input
              type="datetime-local"
              value={draft.dueInput}
              onChange={(e) => setDraft((d) => ({ ...d, dueInput: e.target.value }))}
              className={INPUT_CLS}
            />
          </Field>

          <SettingsFields
            value={draft}
            onChange={(update) => setDraft((d) => ({ ...d, ...update(d) }))}
            onExtractedTitle={(title) => setDraft((d) => ({ ...d, title: fillTitleIfEmpty(d.title, title) }))}
            classId={draft.classId}
            recipientIds={draft.userIds}
          />

          <RecipientChecklist
            roster={roster}
            error={rosterError}
            onRetry={() => setRosterNonce((n) => n + 1)}
            selected={draft.userIds}
            onChange={(userIds) => setDraft((d) => ({ ...d, userIds }))}
          />

          <div className="flex flex-wrap items-center gap-3 border-t border-mk-border pt-4">
            <Button type="submit" variant="primary" disabled={busy}>
              {busy ? "发布中" : "发布作业"}
            </Button>
            {message && (
              <span className="text-mk-small font-semibold text-mk-danger" role="alert">
                {message}
              </span>
            )}
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
        <div className="text-mk-small font-semibold text-mk-danger">
          加载失败：{error}{" "}
          <button type="button" onClick={onRetry} className="cursor-pointer underline">
            重试
          </button>
        </div>
      ) : roster === null ? (
        <div className="text-mk-small text-mk-muted">加载中…</div>
      ) : roster.length === 0 ? (
        <div className="text-mk-small text-mk-muted">暂无学生</div>
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
          <input type="checkbox" checked={selected.includes(s.id)} onChange={() => onToggle(s.id)} />
          {s.displayName}
        </label>
      ))}
    </div>
  );
}
