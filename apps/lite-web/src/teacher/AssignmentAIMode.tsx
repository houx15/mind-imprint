import { useRef, useState, type Dispatch, type SetStateAction } from "react";
import { Button } from "@/ui";
import { type ClassSummary } from "@/api";
import { createAssignment } from "../api/assignments";
import { postWorkspaceTurn, type WorkspaceCard } from "../api/teacherWorkspace";
import type { RosterRow } from "../api/teacher";
import { useAlive } from "../shared/useAlive";
import {
  buildCreateInput,
  draftOnClassChange,
  failText,
  fillTitleIfEmpty,
  writeLastClassId,
  type AssignmentDraft,
} from "./assignmentLogic";
import { Field, INPUT_CLS } from "./formParts";
import { RecipientChecklist, SettingsFields } from "./AssignmentForm";
import { applyPatch, clampChoices, isCurrentTurn, trimTurns, type Choice, type Turn } from "./workspace/workspaceLogic";
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

  // `runTurn` closes over `draft` at the render that created it — reading
  // that same binding after the `await` would give back whatever it was at
  // send time, not what she has now, so `current` in `applyPatch` would
  // always equal `snapshot` and `kept` could never fire. `draftRef` is kept
  // current on every render so the continuation can read the real "now".
  const draftRef = useRef(draft);
  draftRef.current = draft;

  // A generation counter, bumped on every class change. `classId` alone
  // cannot tell a turn's response is still wanted: she can switch A → B → A
  // while a turn for A is in flight, and the response lands with classId
  // matching again even though the conversation it belongs to was cleared
  // in between. `gen` catches that case — `isCurrentTurn` checks both.
  const genRef = useRef(0);

  async function runTurn(input: { text: string } | { choiceId: string; label: string; slug?: string }) {
    if (busy) return;
    const teacherText = "text" in input ? input.text : input.label;
    // Captured before the round trip: `applyPatch` needs to know what the
    // draft looked like when the turn started (so a card edit she makes
    // while the AI is thinking is never silently overwritten), and `sent`
    // is what `isCurrentTurn` compares the live session against once the
    // response lands.
    const snapshot = draft;
    const sent = { gen: genRef.current, classId: snapshot.classId };
    const nextTurns = [...turns, { role: "teacher" as const, text: teacherText }];
    setTurns(nextTurns);
    setBusy(true);
    setError(null);
    setKept([]);
    try {
      const res = await postWorkspaceTurn({
        surface: "assignment",
        classId: snapshot.classId,
        artifact: snapshot,
        // The window is the only thing bounding prompt growth — the full
        // conversation still shows in the panel, only the request is capped.
        turns: trimTurns(nextTurns),
        ...("text" in input ? { text: input.text } : { choiceId: input.choiceId, choiceSlug: input.slug }),
      });
      if (!alive.current) return;
      if (!isCurrentTurn(sent, { gen: genRef.current, classId: draftRef.current.classId })) {
        // A class change (possibly A → B → A while this was in flight)
        // already cleared the conversation on screen. The request was
        // scoped to the class she had when she sent it (the server reads
        // `classId` per turn), so this reply, patch and cards belong to an
        // abandoned session and must not land on the live one — otherwise
        // it would append an orphaned AI bubble with no teacher turn above
        // it, or a patch meant for a different sitting of the same class.
        return;
      }
      const { next, kept: keptFields } = applyPatch(draftRef.current, snapshot, res.patch as Partial<AssignmentDraft>);
      setDraft(next);
      setKept(keptFields);
      setChoices(clampChoices(res.choices));
      setCards(res.cards);
      setTurns((t) => [...t, { role: "ai" as const, text: res.reply }]);
    } catch (e) {
      // A failure for a turn whose session she has since left is not worth
      // surfacing — the conversation it belongs to is already gone.
      if (alive.current && isCurrentTurn(sent, { gen: genRef.current, classId: draftRef.current.classId })) {
        // The server already prefixes its own message with 「对话失败：」;
        // `failText` recognizes that prefix and does not double it.
        setError(failText("对话", e));
      }
    } finally {
      if (alive.current) setBusy(false);
    }
  }

  /** She picked a different class. Must go through `draftOnClassChange` —
   * setting `classId` directly would leave the old class's personalised-
   * reading `picks` attached, which can publish `{picks:{}}` built for a
   * different class (the bug that fix already exists to prevent). The
   * roster refetch + recipient reset already happen from `AssignmentForm`'s
   * effect on `draft.classId`, unconditional on mode. The conversation and
   * any pending choices/cards/kept-note are cleared here: the server scopes
   * each turn to `classId`, so a turn from the old class must not carry into
   * the new class's roster. `genRef` bumps too, so a turn already in flight
   * for the class she is leaving cannot resurface even if she switches back
   * to the same class before it resolves (`isCurrentTurn`). The `<select>`
   * is left enabled while `busy` on purpose: a teacher who notices mid-turn
   * that she picked the wrong class should be able to fix it immediately
   * rather than wait out a model call she no longer wants — the generation
   * counter is what makes that safe. */
  function onClassChange(classId: string) {
    genRef.current += 1;
    writeLastClassId(classId);
    setDraft((d) => draftOnClassChange(d, classId));
    setTurns([]);
    setChoices([]);
    setCards([]);
    setKept([]);
    setError(null);
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
        void runTurn({ choiceId, label: choice?.label ?? choiceId, slug: choice?.slug });
      }}
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
