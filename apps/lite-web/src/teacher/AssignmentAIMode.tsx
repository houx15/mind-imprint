import { useState, type Dispatch, type SetStateAction } from "react";
import { Button } from "@/ui";
import { type ClassSummary } from "@/api";
import { createAssignment } from "../api/assignments";
import { type WorkspaceCard } from "../api/teacherWorkspace";
import type { RosterRow } from "../api/teacher";
import { useAlive } from "../shared/useAlive";
import {
  buildCreateInput,
  draftOnKindChange,
  failText,
  fillTitleIfEmpty,
  writeLastClassId,
  type AssignmentDraft,
} from "./assignmentLogic";
import { Field, INPUT_CLS } from "./formParts";
import { KindField, RecipientChecklist, SettingsFields } from "./AssignmentForm";
import { StudentViewPreview } from "./StudentViewPreview";
import type { WorkspaceThread } from "./workspace/useWorkspaceThread";
import { WorkspacePanel } from "./workspace/WorkspacePanel";

// teacher/AssignmentAIMode.tsx — the AI mode of 布置作业: the homework card
// (today's form fields, unchanged widgets) sits as `WorkspacePanel`'s
// canvas, the conversation drives it via `patch`. `AssignmentForm.tsx` owns
// `draft`/`classes`/`roster`, the mode toggle and the conversation
// (`useWorkspaceThread`), so the conversation outlives this component when
// she switches to 传统 and back.

/** Patch keys the server sends are `AssignmentDraft` field names
 * (apps/api/internal/api/lite_teacher_workspace.go's `run.write` calls) —
 * never show one of those English names to a teacher. */
const FIELD_LABELS: Partial<Record<keyof AssignmentDraft, string>> = {
  kind: "类型",
  title: "标题",
  instructions: "说明",
  dueInput: "截止时间",
  readingSource: "来源",
  slug: "文章",
  text: "正文",
  tier: "难度",
  userIds: "学生",
};

export function keptLabels(kept: (keyof AssignmentDraft)[]): string {
  return kept.map((k) => FIELD_LABELS[k] ?? String(k)).join("、");
}

interface StudentCardRow {
  id: string;
  name: string;
}

/** `WorkspaceCard.rows` is opaque on the wire (§ teacherWorkspace.ts) — a
 * "students" card's actual shape is `liteworkspace.Student` (apps/api's
 * workspace.go), narrowed defensively here so a malformed row cannot crash
 * the canvas. */
function studentRows(raw: unknown): StudentCardRow[] {
  if (!Array.isArray(raw)) return [];
  const out: StudentCardRow[] = [];
  for (const r of raw) {
    if (!r || typeof r !== "object") continue;
    const o = r as Record<string, unknown>;
    if (typeof o.id === "string" && typeof o.name === "string") out.push({ id: o.id, name: o.name });
  }
  return out;
}

function WorkspaceCardView({ card }: { card: WorkspaceCard }) {
  if (card.kind !== "students") return null;
  const rows = studentRows(card.rows);
  return (
    <div className="rounded-mk-md border border-mk-border bg-mk-paper p-3">
      <p className="text-mk-label font-bold text-mk-muted">学生 {rows.length} 人</p>
      {rows.length > 0 && (
        <ul className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-mk-small text-mk-ink">
          {rows.map((s) => (
            <li key={s.id}>{s.name}</li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function AssignmentAIMode({
  draft,
  setDraft,
  classes,
  roster,
  rosterError,
  onRosterRetry,
  onCreated,
  thread,
  onClassChange,
}: {
  draft: AssignmentDraft;
  setDraft: Dispatch<SetStateAction<AssignmentDraft>>;
  classes: ClassSummary[];
  roster: RosterRow[] | null;
  rosterError: string | null;
  onRosterRetry: () => void;
  onCreated: (assignmentId: string) => void;
  /** Owned by `AssignmentForm`; see `useWorkspaceThread`. */
  thread: WorkspaceThread<AssignmentDraft>;
  /** `AssignmentForm.changeClass`: `draftOnClassChange` + `thread.reset()`. */
  onClassChange: (classId: string) => void;
}) {
  const alive = useAlive();
  const { turns, busy, error, choices, cards, kept } = thread;

  const [publishBusy, setPublishBusy] = useState(false);
  const [publishMessage, setPublishMessage] = useState<string | null>(null);

  async function publish() {
    if (publishBusy) return;
    const built = buildCreateInput(draft);
    if (!built.ok) {
      setPublishMessage(built.error);
      return;
    }
    setPublishMessage(null);
    setPublishBusy(true);
    const classId = draft.classId;
    try {
      const created = await createAssignment(classId, built.value);
      if (!alive.current) return;
      writeLastClassId(classId);
      onCreated(created.id);
    } catch (e) {
      if (alive.current) setPublishMessage(failText("发布", e));
    } finally {
      if (alive.current) setPublishBusy(false);
    }
  }

  const keptText = kept.length > 0 ? `${keptLabels(kept)} 已保留你的修改` : null;

  return (
    <WorkspacePanel
      turns={turns}
      busy={busy}
      error={error}
      choices={choices}
      onSend={(text) => thread.run({ text })}
      onChoose={(choiceId) => {
        const choice = choices.find((c) => c.id === choiceId);
        return thread.run({ choiceId, label: choice?.label ?? choiceId, slug: choice?.slug });
      }}
      composer={thread.composer}
      onComposerChange={thread.setComposer}
      onRetry={thread.retry}
      canRetry={thread.failed !== null}
    >
      <div className="mt-6 flex flex-col gap-5 rounded-mk-lg border border-mk-border bg-mk-surface p-4 sm:p-6">
        <Field label="班级">
          <select value={draft.classId} onChange={(e) => onClassChange(e.target.value)} className={INPUT_CLS}>
            {classes.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </Field>

        {/* 类型 sits where the traditional form puts it — after 班级, before
            标题 — so the two modes read the same way. It is the same KindField,
            not a second one: a divergent copy is how the two modes would come
            to disagree about what a homework can be.

            Without it this card had no way to correct a type the conversation
            had got wrong, and the type decides which cells exist below: switch
            a writing card to 阅读 and SettingsFields renders the material row.

            It goes through `draftOnKindChange` because the card she edits is
            the card the next turn shows the model. Setting `kind` alone left a
            chosen article on a writing draft, where nothing renders it — the
            same contradictory state the server's `set_fields` clears, arriving
            through her control instead of a tool's. */}
        <KindField value={draft.kind} onChange={(kind) => setDraft((d) => draftOnKindChange(d, kind))} />

        {keptText && (
          <p className="text-mk-small font-semibold text-mk-accent-700" role="status">
            {keptText}
          </p>
        )}

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
          onRetry={onRosterRetry}
          selected={draft.userIds}
          onChange={(userIds) => setDraft((d) => ({ ...d, userIds }))}
        />

        {cards.map((c, i) => (
          <WorkspaceCardView key={i} card={c} />
        ))}

        <StudentViewPreview draft={draft} classes={classes} />

        <div className="flex flex-wrap items-center gap-3 border-t border-mk-border pt-4">
          <Button variant="primary" onClick={() => void publish()} disabled={publishBusy}>
            {publishBusy ? "发布中" : "发布作业"}
          </Button>
          {publishMessage && (
            <span className="text-mk-small font-semibold text-mk-danger" role="alert">
              {publishMessage}
            </span>
          )}
        </div>
      </div>
    </WorkspacePanel>
  );
}
