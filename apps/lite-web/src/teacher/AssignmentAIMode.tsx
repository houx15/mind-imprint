import { useState, type Dispatch, type SetStateAction } from "react";
import { Button } from "@/ui";
import { type ClassSummary } from "@/api";
import { createAssignment } from "../api/assignments";
import { postWorkspaceTurn, type WorkspaceCard } from "../api/teacherWorkspace";
import type { RosterRow } from "../api/teacher";
import { useAlive } from "../shared/useAlive";
import {
  buildCreateInput,
  failText,
  fillTitleIfEmpty,
  writeLastClassId,
  type AssignmentDraft,
} from "./assignmentLogic";
import { Field, INPUT_CLS } from "./formParts";
import { RecipientChecklist, SettingsFields } from "./AssignmentForm";
import { applyPatch, clampChoices, trimTurns, type Choice, type Turn } from "./workspace/workspaceLogic";
import { WorkspacePanel } from "./workspace/WorkspacePanel";

// teacher/AssignmentAIMode.tsx — the AI mode of 布置作业: the homework card
// (today's form fields, unchanged widgets) sits as `WorkspacePanel`'s
// canvas, the conversation drives it via `patch`. `AssignmentForm.tsx` owns
// `draft`/`classes`/`roster` and the mode toggle; this component only turns
// a conversation into edits on the draft it is handed.

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
  tier: "难度",
  userIds: "学生",
};

function keptLabels(kept: (keyof AssignmentDraft)[]): string {
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
}: {
  draft: AssignmentDraft;
  setDraft: Dispatch<SetStateAction<AssignmentDraft>>;
  classes: ClassSummary[];
  roster: RosterRow[] | null;
  rosterError: string | null;
  onRosterRetry: () => void;
  onCreated: (assignmentId: string) => void;
}) {
  const alive = useAlive();

  const [turns, setTurns] = useState<Turn[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [choices, setChoices] = useState<Choice[]>([]);
  const [cards, setCards] = useState<WorkspaceCard[]>([]);
  const [kept, setKept] = useState<(keyof AssignmentDraft)[]>([]);

  const [publishBusy, setPublishBusy] = useState(false);
  const [publishMessage, setPublishMessage] = useState<string | null>(null);

  async function runTurn(input: { text: string } | { choiceId: string; label: string }) {
    if (busy) return;
    const teacherText = "text" in input ? input.text : input.label;
    // Snapshot before the round trip: `applyPatch` needs to know what the
    // draft looked like when the turn started, so a card edit she makes
    // while the AI is thinking is never silently overwritten.
    const snapshot = draft;
    const nextTurns = [...turns, { role: "teacher" as const, text: teacherText }];
    setTurns(nextTurns);
    setBusy(true);
    setError(null);
    setKept([]);
    try {
      const res = await postWorkspaceTurn({
        surface: "assignment",
        classId: draft.classId,
        artifact: draft,
        // The window is the only thing bounding prompt growth — the full
        // conversation still shows in the panel, only the request is capped.
        turns: trimTurns(nextTurns),
        ...("text" in input ? { text: input.text } : { choiceId: input.choiceId }),
      });
      if (!alive.current) return;
      const { next, kept: keptFields } = applyPatch(draft, snapshot, res.patch as Partial<AssignmentDraft>);
      setDraft(next);
      setKept(keptFields);
      setChoices(clampChoices(res.choices));
      setCards(res.cards);
      setTurns((t) => [...t, { role: "ai" as const, text: res.reply }]);
    } catch (e) {
      // The server already prefixes its own message with 「对话失败：」;
      // `failText` recognizes that prefix and does not double it.
      if (alive.current) setError(failText("对话", e));
    } finally {
      if (alive.current) setBusy(false);
    }
  }

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

  const className = classes.find((c) => c.id === draft.classId)?.name ?? "";
  const keptText = kept.length > 0 ? `${keptLabels(kept)} 已保留你的修改` : null;

  return (
    <WorkspacePanel
      turns={turns}
      busy={busy}
      error={error}
      choices={clampChoices(choices)}
      onSend={(text) => void runTurn({ text })}
      onChoose={(choiceId) => {
        const choice = choices.find((c) => c.id === choiceId);
        void runTurn({ choiceId, label: choice?.label ?? choiceId });
      }}
    >
      <div className="mt-6 flex flex-col gap-5 rounded-mk-lg border border-mk-border bg-mk-surface p-4 sm:p-6">
        {className && <p className="text-mk-small text-mk-muted">班级：{className}</p>}
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
