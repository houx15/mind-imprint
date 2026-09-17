import { useState, type Dispatch, type ReactNode, type SetStateAction } from "react";
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
import { DateField } from "./controls/DateField";
import { Select } from "./controls/Select";
import type { WorkspaceThread } from "./workspace/useWorkspaceThread";
import { WorkspacePanel } from "./workspace/WorkspacePanel";
import { openChoices } from "./workspace/workspaceLogic";
import { StudentsCard } from "./workspace/WorkspaceCards";

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

function WorkspaceCardView({ card }: { card: WorkspaceCard }) {
  return card.kind === "students" ? <StudentsCard card={card} /> : null;
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
  header,
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
  /** The form page's back link, title and mode switch: in AI mode this
   *  component owns the whole page, so they are drawn above both columns. */
  header: ReactNode;
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

  // The card's article when the current options arrived. Picking an article
  // on the card answers 「您想用哪篇文章？」, so its article options leave.
  const choicesKey = choices.map((c) => c.id).join("\n");
  const [offer, setOffer] = useState({ key: choicesKey, slug: draft.slug });
  if (offer.key !== choicesKey) setOffer({ key: choicesKey, slug: draft.slug });
  const shownChoices = openChoices(choices, offer.key === choicesKey ? offer.slug : draft.slug, draft.slug);

  return (
    <WorkspacePanel
      turns={turns}
      busy={busy}
      error={error}
      choices={shownChoices}
      onSend={(text) => thread.run({ text })}
      onChoose={(choiceId) => {
        const choice = choices.find((c) => c.id === choiceId);
        return thread.run({ choiceId, label: choice?.label ?? choiceId, slug: choice?.slug });
      }}
      composer={thread.composer}
      onComposerChange={thread.setComposer}
      onRetry={thread.retry}
      canRetry={thread.failed !== null}
      intro="AI 根据你的描述填写左侧的作业卡，发布前可以再修改。请输入作业要求，例如类型、材料和截止时间，也可以直接贴入文章正文。"
      header={header}
      suggestions={["这周读一篇关于气候变化的文章，周五交", "布置一篇议论文，下周一交", "给每个学生推荐适合的文章"]}
    >
      <div className="flex flex-col gap-5 rounded-mk-lg border border-mk-border bg-mk-surface p-4 sm:p-6">
        <Field label="班级">
          <Select value={draft.classId} onChange={onClassChange} options={classes.map((c) => ({ value: c.id, label: c.name }))} />
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
          <DateField withTime shortcuts value={draft.dueInput} onChange={(dueInput) => setDraft((d) => ({ ...d, dueInput }))} />
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
