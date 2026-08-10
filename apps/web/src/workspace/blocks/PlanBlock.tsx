import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { LogEntry, PlanColumn, PlanItem, PlanTag, Proposal, CardTurnRef } from "@mind-imprint/contracts";
import { useStudioAiSlot } from "@/studio/ai/StudioAiSlot";
import { useStudioChat, type StudioChatMsg } from "@/studio/ai/StudioChatContext";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
import { withRecap } from "@/studio/ai/RecapHint";
import { Composer } from "@/studio/ai/Composer";
import { StudioCoachChat, StudioTurnChips } from "@/studio/ai/StudioCoachChat";
import { Segmented } from "@/ui";
import { waiveCounterpoints } from "../../api/projects";
import { Icon } from "../Icon";
import {
  PROPOSAL_DIMS,
  TAG_LABEL,
  COLUMN_LABEL,
  TIMELINE_DAYS,
  STAGES,
  STAGE_1,
} from "./mockData";
import {
  putProposal,
  getPlan,
  createPlanItem,
  patchPlanItem,
  deletePlanItem,
  getLog,
  addLog,
  type PlanItemPatch,
} from "../api/workspace";
import { CoachCardPanel, FORMING_DECK } from "./CoachCardPanel";
import { CardTurnChip } from "./CardTurnChip";
import { exportTimescale, exportActivityLog } from "../export";

// The legacy "primary" family and "accent" family were two distinct hues in
// the old two-tone design; the 2026-08-06 redesign aliases both to the same
// single accent (tailwind.config.ts LEGACY ALIASES), so a straight mechanical
// token rename would have collapsed 读/写 into one indistinguishable color.
// Reassigned onto two of the 7 macarons (spec §10) instead — the same palette
// the summon shelf (CoachCardPanel) already draws from — keeping 省 on the
// semantic success tone (review ≈ done).
const TAG_STYLE: Record<PlanTag, string> = {
  read: "bg-mk-lake-bg text-mk-lake-fg",
  write: "bg-mk-peach-bg text-mk-peach-fg",
  review: "bg-mk-success-bg text-mk-success",
};
const TAG_BAR: Record<PlanTag, string> = {
  read: "bg-mk-lake",
  write: "bg-mk-peach",
  review: "bg-mk-success",
};

// The Plan block is two phases sharing one home. Phase A ("forming") is a calm
// coach chat that turns talk into the four proposal dimensions; hitting 生成计划
// flips to Phase B ("working"), a persisted project board you return to every
// session — viewable as a Kanban, a Gantt or an activity log, and exportable.
//
// Everything here is API-backed (slice 2): proposal edits are debounced to
// PUT /proposal; the board is CRUD against /plan; the chat calls /coach.
export function PlanBlock({
  projectId,
  title,
  qualification,
  proposal,
  createdAt,
  phase,
  refreshWorkspace,
  recap,
  onStudioStateChanged,
}: {
  projectId: string;
  title: string;
  qualification: string;
  proposal: Proposal;
  createdAt?: string;
  /** Which segment mounted this room (P2a: 提案/管理 share this component —
   * WorkspaceContainer now decides via `room`, not a self-decided local
   * state). */
  phase: "forming" | "working";
  refreshWorkspace: () => void;
  /** Re-entry recap shown as 印记's opening note inside the continuous chat. */
  recap?: string | null;
  /** slice 3a · re-apply 印记's studio state (e.g. after 反例 waive triggers
   * plan-gen + advance) so the room follows the machine to 管理. */
  onStudioStateChanged?: () => void | Promise<void>;
}) {
  // Local proposal state seeded from the projection; the component is keyed on
  // projectId upstream, so this initialises once per opened project.
  const [prop, setProp] = useState<Proposal>(proposal);
  // The coach thread is HOISTED to WorkspaceContainer (persists across 立项↔写作
  // room swaps, loaded once per project). This room reads/appends the shared
  // store instead of holding its own chat state; the scripted intro is now a
  // display-only fallback (see FormingPhase's `displayChat`), never stored.
  const { messages, setMessages, sending, activeProjectIdRef, sendStudioTurn } = useStudioChat();
  // A turn resolves seconds later; if the student switched PROJECTS meanwhile,
  // don't append its reply into (or clear the busy flag of) the now-different
  // project's shared store. A plain room switch within the same project passes.
  const isActiveProject = () => activeProjectIdRef.current === projectId;
  const [draft, setDraft] = useState("");

  // Debounced persistence of proposal edits (~600ms after the last keystroke).
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  function persistProposal(next: Proposal) {
    putProposal(projectId, next)
      .then(() => refreshWorkspace())
      .catch(() => {
        /* keep the local edit; a later save or reload reconciles */
      });
  }
  function setDim(key: keyof Proposal, v: string) {
    const next = { ...prop, [key]: v };
    setProp(next);
    if (saveTimer.current) clearTimeout(saveTimer.current);
    // Null the ref when the debounce FIRES (not just when it's replaced) so
    // `saveTimer.current != null` is a true "mid-typing" signal for the
    // external-sync effect below — a fired-but-unnulled timer would wrongly
    // block every later external update.
    saveTimer.current = setTimeout(() => {
      saveTimer.current = null;
      persistProposal(next);
    }, 600);
  }
  useEffect(() => () => { if (saveTimer.current) clearTimeout(saveTimer.current); }, []);

  // Sync the local draft when the proposal projection changes from OUTSIDE this
  // room — chiefly 印记's 记进「分区」 note-confirm (confirmNote in the container
  // writes the section, then refreshWorkspace re-pushes the projection here). The
  // local `prop` state exists only to debounce keystrokes; without this it stays
  // seeded at mount, so a confirmed note didn't show until the room remounted
  // (the "appears after I go home and back" bug). Skip the sync while a local
  // edit is mid-debounce so an in-flight refresh never clobbers unsaved
  // keystrokes; the pending save (and its own refresh) reconciles right after.
  useEffect(() => {
    if (saveTimer.current) return;
    setProp(proposal);
  }, [proposal]);

  // The coach send is now the ONE container-owned loop (`sendStudioTurn`): it
  // appends the turn, calls the orchestrator, applies the returned directive,
  // and surfaces the note/card OFFERS (rendered by `StudioTurnChips` below).
  // The three entry points here just compose the right `userInput`.

  async function onSend() {
    const text = draft.trim();
    if (!text || sending) return;
    setDraft("");
    // EC · flush a pending dim autosave BEFORE the turn so 印记's spine read
    // (GetProjectProposal) sees the latest dimensions — otherwise a dim typed
    // within the 600ms debounce is invisible to this turn (race).
    if (saveTimer.current) {
      clearTimeout(saveTimer.current);
      saveTimer.current = null;
      await putProposal(projectId, prop).catch(() => {/* the turn still proceeds */});
    }
    await sendStudioTurn(text);
  }

  if (phase === "forming") {
    return (
      <FormingPhase
        proposal={prop}
        setDim={setDim}
        messages={messages}
        recap={recap}
        draft={draft}
        setDraft={setDraft}
        sending={sending}
        onSend={onSend}
        title={title}
        qualification={qualification}
        projectId={projectId}
        refreshWorkspace={refreshWorkspace}
        onStudioStateChanged={onStudioStateChanged}
        onCardReflected={(studentText, reply, card) => {
          if (!isActiveProject()) return; // student switched projects mid-reflect
          setMessages((c) => [
            ...c,
            // A card turn renders as a content-first chip (card set), falling
            // back to raw compiled text only if the server didn't echo a card.
            ...(card
              ? [{ role: "student" as const, text: studentText, card }]
              : studentText
                ? [{ role: "student" as const, text: studentText }]
                : []),
            ...(reply
              ? [{ role: "ai" as const, text: reply }]
              : card || studentText
                ? []
                : [{ role: "ai" as const, text: "这张卡还没填内容，先留着，想清楚了再来。" }]),
          ]);
        }}
      />
    );
  }

  return (
    <WorkingPhase
      projectId={projectId}
      title={title}
      qualification={qualification}
      createdAt={createdAt}
    />
  );
}

const clamp = (n: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, n));

/* ---------- #14: plan-timeline calendar dates ---------- */
const DAY_MS = 86400000;
const WEEKDAY_ZH = ["日", "一", "二", "三", "四", "五", "六"];
function addDays(base: Date, n: number): Date {
  const d = new Date(base);
  d.setDate(d.getDate() + n);
  return d;
}
function fmtMD(d: Date): string {
  return `${d.getMonth() + 1}/${d.getDate()}`;
}
// Whole-day index of `target` relative to `anchor` (both truncated to local
// midnight), so "today" lands on the right column regardless of clock time.
function dayIndexFromAnchor(anchor: Date, target: Date): number {
  const a = new Date(anchor.getFullYear(), anchor.getMonth(), anchor.getDate());
  const t = new Date(target.getFullYear(), target.getMonth(), target.getDate());
  return Math.round((t.getTime() - a.getTime()) / DAY_MS);
}

/* ---------- Phase A · forming ---------- */

function FormingPhase(props: {
  proposal: Proposal;
  setDim: (key: keyof Proposal, v: string) => void;
  messages: StudioChatMsg[];
  recap?: string | null;
  draft: string;
  setDraft: (s: string) => void;
  sending: boolean;
  onSend: () => void;
  title: string;
  qualification: string;
  projectId: string;
  refreshWorkspace?: () => void;
  onStudioStateChanged?: () => void | Promise<void>;
  onCardReflected: (studentText: string, reply: string, card?: CardTurnRef) => void;
}) {
  const {
    proposal, setDim, messages, recap, draft, setDraft, sending, onSend,
    projectId, onCardReflected, refreshWorkspace, onStudioStateChanged,
  } = props;
  const [waiving, setWaiving] = useState(false);
  async function onWaiveCounterpoints() {
    if (waiving) return;
    setWaiving(true);
    try {
      await waiveCounterpoints(projectId);
      refreshWorkspace?.();
      await onStudioStateChanged?.();
    } catch {
      setWaiving(false);
    }
  }
  // §framework · all FIVE parts count toward the tracker (目标/缘由/活动与时间/
  // 资源/反例). The 5th (反例) is skippable — plan-gen still only requires the four,
  // so `requiredCovered` drives readiness/waive while `covered` drives the /5 view.
  const requiredDims = PROPOSAL_DIMS.filter((d) => d.required);
  const requiredCovered = requiredDims.filter((d) => proposal[d.key].trim().length > 0).length;
  const covered = PROPOSAL_DIMS.filter((d) => proposal[d.key].trim().length > 0).length;
  // The room→panel contract (Task 4, spec §17): this room's WORK — the 开题
  // proposal panel + its actions — renders directly below, in <main>; its
  // COACH (the chat conversation) is portaled into the constant AiPanel via
  // `useStudioAiSlot`. `slot` is null when the panel is collapsed or this
  // component renders outside a studio shell (e.g. some tests) — in either
  // case the coach content simply doesn't render, never crashes.
  const slot = useStudioAiSlot();
  // slice 3a · the 提问卡 manual-open affordance. Shown ONLY while 目标 is empty —
  // that's the student-manual regime (all-statuses.md §2 D); once 目标 is filled
  // the coach proposes it instead. Routes through openCard → QuestionCardModal.
  const { openCard } = useStudioChat();
  const objectiveEmpty = proposal.objective.trim() === "";
  // The 提案 framing is now the live agent's job (delivered by `coach/start`'s
  // narrate, seeded into the hoisted store before this room ever mounts) —
  // no local scripted-intro fallback. An empty thread simply renders empty.
  const displayChat: StudioChatMsg[] = messages;
  return (
    <>
      {/* WORK — the 开题 panel: proposal's four dimensions + actions. */}
      <div className="relative mx-auto flex h-full w-full max-w-2xl flex-col gap-5 overflow-y-auto px-10 py-9">
        <header>
          <p className="text-[12px] font-semibold uppercase tracking-[0.18em] text-mk-faint">立项 · 先想清楚再动手</p>
          <h1 className="mt-1 font-sans text-[26px] font-bold leading-tight text-mk-ink">先搭好研究的大框架</h1>
          <p className="mt-1.5 text-[14px] text-mk-muted">把这几件事聊清楚，计划会据此长出来。</p>
        </header>

        {objectiveEmpty && (
          <button
            type="button"
            onClick={() => openCard("question-card")}
            className="flex items-center gap-2 self-start rounded-mk-md border border-mk-accent bg-mk-accent-50 px-3.5 py-2 text-[14px] font-bold text-mk-accent hover:bg-mk-accent-100"
          >
            <Icon name="spark" size={15} /> 还没头绪？用提问卡帮你想想
          </button>
        )}

        <div className="flex min-h-0 flex-1 flex-col rounded-mk-lg border border-mk-border bg-mk-surface p-5 shadow-mk-xs">
          <div className="mb-1 flex items-center justify-between">
            <div className="flex items-center gap-2 text-mk-accent">
              <Icon name="spark" size={16} />
              <span className="text-[14px] font-bold tracking-wide">开题 · 想清楚这几件事</span>
            </div>
            <span className="text-[12px] font-bold text-mk-faint">{covered}/5 已聊到</span>
          </div>
          <p className="mb-4 text-[12px] text-mk-faint">不用写正式开题报告——把这几件事聊清楚就行。</p>
          <div className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto">
            {PROPOSAL_DIMS.map((d) => (
              <DimField key={d.key} label={d.label} hint={d.hint} filled={proposal[d.key].trim().length > 0} value={proposal[d.key]} onChange={(v) => setDim(d.key, v)} />
            ))}
          </div>
          {/* §framework · 反例 is the skippable 5th part: once the four required
              dims are in but 反例 is still empty, a student who can't think of one
              may skip it and generate the plan (铁律②). */}
          {requiredCovered === 4 && proposal.counterpoints.trim() === "" && (
            <button
              type="button"
              disabled={waiving}
              onClick={() => void onWaiveCounterpoints()}
              className="mt-3 self-start text-[13px] font-semibold text-mk-faint underline decoration-dotted hover:text-mk-accent disabled:opacity-50"
            >
              {waiving ? "生成中……" : "想不到反例？跳过这一步，先生成计划"}
            </button>
          )}
        </div>
      </div>

      {/* COACH — portaled into the constant AiPanel (Task 4), now on the
          shared `ChatLog`/`Composer` (Task 5) instead of a bespoke chat log +
          textarea. A card-turn message maps to a "system"-role ChatMessage
          carrying only `node` — ChatLog's system row has no bubble chrome
          (no bg/padding/radius), so `CardTurnChip` supplies its own chip
          styling untouched instead of nesting inside a second bubble. */}
      {slot &&
        createPortal(
          <div className="flex h-full flex-col gap-3 p-4">
            <div className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto pr-1">
              <ChatLog messages={withRecap(recap, toChatMessages(displayChat))} thinking={sending} />
              {/* 印记's per-turn note/card OFFERS come from the ONE container store
                  (StudioTurnChips) — the same chips render in chat-first and 写作. */}
              <StudioTurnChips />
              {/* The self-summon 工具卡 shelf: the student picks a forming card
                  herself (separate from 印记's proposal above). No AI proposal is
                  threaded here anymore — that path is StudioTurnChips → the shared
                  card sheet. */}
              {!sending && (
                <CoachCardPanel projectId={projectId} proposal={null} onProposalConsumed={() => {}} onReflected={onCardReflected} surface="forming" deck={FORMING_DECK} />
              )}
            </div>

            <Composer
              value={draft}
              onChange={setDraft}
              onSend={onSend}
              state={sending ? "replying" : undefined}
              placeholder="说说你的想法……（Shift+Enter 换行）"
              className="flex-none"
            />
          </div>,
          slot,
        )}
    </>
  );
}

// DimField — §framework · the note defaults to a READ-ONLY view showing all the
// content 印记 filled in; the student double-clicks (or taps 编辑) to edit, and
// leaving the box (blur) returns to the view. This keeps the framework page a
// calm "here's what we agreed" surface rather than a wall of edit boxes.
function DimField({ label, hint, value, filled, onChange }: { label: string; hint: string; value: string; filled: boolean; onChange: (v: string) => void }) {
  const [editing, setEditing] = useState(false);
  const taRef = useRef<HTMLTextAreaElement | null>(null);
  useEffect(() => {
    if (editing && taRef.current) {
      taRef.current.focus();
      const n = taRef.current.value.length;
      taRef.current.setSelectionRange(n, n);
    }
  }, [editing]);
  return (
    <div className="block">
      <span className="mb-1 flex items-center gap-1.5">
        <span className={`h-1.5 w-1.5 rounded-full ${filled ? "bg-mk-success" : "border border-mk-faint"}`} />
        <span className="text-[12px] font-bold text-mk-ink">{label}</span>
        <span className="text-[12px] font-normal text-mk-faint">· {hint}</span>
        {!editing && (
          <button type="button" onClick={() => setEditing(true)} className="ml-auto text-[12px] font-semibold text-mk-faint hover:text-mk-accent">编辑</button>
        )}
      </span>
      {editing ? (
        <textarea
          ref={taRef}
          value={value}
          placeholder="跟印记聊几句，这里会慢慢填上"
          onChange={(e) => onChange(e.target.value)}
          onBlur={() => setEditing(false)}
          rows={3}
          className="w-full resize-none rounded-mk-md border border-mk-accent bg-mk-surface px-3 py-2 text-[14px] leading-relaxed text-mk-ink outline-none transition placeholder:text-mk-faint"
        />
      ) : (
        <div
          onDoubleClick={() => setEditing(true)}
          title="双击编辑"
          className="w-full cursor-text whitespace-pre-line rounded-mk-md border border-mk-border bg-mk-paper px-3 py-2 text-[14px] leading-relaxed"
        >
          {value.trim() ? <span className="text-mk-ink">{value}</span> : <span className="text-mk-faint">跟印记聊几句，这里会慢慢填上</span>}
        </div>
      )}
    </div>
  );
}

// PlanBlock's own ChatMsg[] history → the shared ChatLog's ChatMessage[]. A
// card-turn (msg.card set) maps to a "system"-role message carrying only
// `node` — ChatLog's system row has no bubble background/padding, letting
// `CardTurnChip` be the whole message (its own content-first accent chip,
// never raw compiled text) instead of nesting inside a second bubble. Every
// other AI turn renders through the shared `ChatMarkdown` (bold/lists/links/
// code, not just `**bold**`); a student turn stays plain text — she types
// prose, not markup, and shouldn't have `*`/`#` silently swallowed.
function toChatMessages(chat: StudioChatMsg[]): ChatMessage[] {
  return chat.map((m, i) =>
    m.card
      ? { id: String(i), role: "system", node: <CardTurnChip card={m.card} /> }
      : {
          id: String(i),
          role: m.role === "ai" ? "assistant" : "student",
          node:
            m.role === "ai" ? (
              <ChatMarkdown text={m.text} />
            ) : (
              <span className="whitespace-pre-wrap">{m.text}</span>
            ),
        },
  );
}

/* ---------- Phase B · working (kanban / gantt / log) ---------- */

const COLUMNS: PlanColumn[] = ["todo", "doing", "done"];
type PlanView = "kanban" | "gantt" | "log";

function WorkingPhase(props: {
  projectId: string;
  title: string;
  qualification: string;
  seedBoard?: PlanItem[];
  createdAt?: string;
}) {
  const { projectId, title, qualification, seedBoard, createdAt } = props;
  // The room→panel contract (same as FormingPhase/ReadingBlock/WritingBlock/
  // ReviewBlock, Task 4, spec §17): 管理 previously portaled nothing into the
  // shared AiPanel slot, so it showed an empty 印记 panel — this is the SAME
  // continuous thread every other room shows (StudioCoachChat reads the
  // hoisted store via `useStudioChat`), not a second conversation.
  const slot = useStudioAiSlot();
  // #14: anchor the plan timeline to real calendar dates. Fall back to today
  // when the project has no creation timestamp (older mocks).
  const anchor = useMemo(() => (createdAt ? new Date(createdAt) : new Date()), [createdAt]);
  // §3 · the management page opens on the 甘特图 by default (the whole-plan recap
  // view), then the student can switch to 看板 / 活动日志.
  const [view, setView] = useState<PlanView>("gantt");

  // The board — seeded from a fresh 生成计划 when we arrive that way, otherwise
  // loaded on enter; mutated optimistically then reconciled.
  const [board, setBoard] = useState<PlanItem[]>(seedBoard ?? []);
  const [loadingPlan, setLoadingPlan] = useState(!seedBoard);
  // The card the student is viewing/editing in the detail popover.
  const [editingId, setEditingId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoadingPlan(true);
    getPlan(projectId)
      .then((items) => { if (!cancelled) setBoard(items); })
      .catch(() => { /* leave empty; empty board is a valid state */ })
      .finally(() => { if (!cancelled) setLoadingPlan(false); });
    return () => { cancelled = true; };
  }, [projectId]);

  function reconcile() {
    getPlan(projectId).then(setBoard).catch(() => {});
  }
  // Optimistically apply a partial edit, then swap in the server's row (or
  // reconcile from scratch if the write failed).
  function patchItem(id: string, patch: PlanItemPatch) {
    setBoard((xs) => xs.map((x) => (x.id === id ? { ...x, ...patch } : x)));
    patchPlanItem(projectId, id, patch)
      .then((updated) => setBoard((xs) => xs.map((x) => (x.id === id ? updated : x))))
      .catch(() => reconcile());
  }
  function addTask(stage: string) {
    createPlanItem(projectId, { title: "写：新任务", tag: "write", column: "todo", stage, start: 0, days: 2 })
      .then(() => reconcile())
      .catch(() => {});
  }
  // #3 — Gantt's own add: land in the last stage, on the timeline after the
  // last-scheduled item, so a new bar appears where work is heading.
  function addTaskGantt() {
    const lastStage = board.length ? board[board.length - 1]!.stage : STAGE_1;
    const maxEnd = board.reduce((m, i) => Math.max(m, i.start + i.days), 0);
    const start = clamp(maxEnd, 0, TIMELINE_DAYS - 2);
    createPlanItem(projectId, { title: "新任务", tag: "write", column: "todo", stage: lastStage, start, days: 2 })
      .then(() => reconcile())
      .catch(() => {});
  }
  function removeItem(id: string) {
    const prev = board;
    setBoard((xs) => xs.filter((x) => x.id !== id));
    deletePlanItem(projectId, id).catch(() => setBoard(prev));
  }
  // Stage options the editor offers: the canonical stages plus any custom
  // stage already present on the board (e.g. from a generated plan).
  const stageOptions = Array.from(new Set([...STAGES, ...board.map((i) => i.stage)]));
  const editing = board.find((i) => i.id === editingId) ?? null;

  // The activity log — loaded lazily the first time the tab is opened (also
  // needed for its export).
  const [log, setLog] = useState<LogEntry[] | null>(null);
  useEffect(() => {
    if (view === "log" && log === null) {
      getLog(projectId).then(setLog).catch(() => setLog([]));
    }
  }, [view, log, projectId]);
  function addLogEntry(text: string) {
    addLog(projectId, text)
      .then((entry) => setLog((l) => [...(l ?? []), entry]))
      .catch(() => {});
  }

  // Real .xlsx / .docx exports (slice 6). The heavy libs load lazily inside the
  // export fns; a busy flag guards the round-trip and a caught error keeps the
  // room alive if generation ever fails.
  const [exporting, setExporting] = useState(false);
  async function runExport(fn: () => Promise<unknown>) {
    if (exporting) return;
    setExporting(true);
    try {
      await fn();
    } catch {
      /* a failed export must never crash the room */
    } finally {
      setExporting(false);
    }
  }
  function exportPlan() {
    void runExport(() => exportTimescale(board, { title }, TIMELINE_DAYS));
  }
  function exportLog() {
    void runExport(() => exportActivityLog(log ?? [], { title }));
  }

  return (
    <>
      <div className="relative flex h-full flex-col px-10 py-8">
        {/* Slim goal header */}
        <div className="mb-6 flex items-center gap-3 rounded-mk-lg border border-mk-border bg-mk-surface px-5 py-3.5 shadow-mk-xs">
          <span className="rounded-full bg-mk-accent-50 px-2.5 py-1 text-[12px] font-bold text-mk-accent">{qualification}</span>
          <h1 className="font-sans text-[18px] font-bold text-mk-ink">{title}</h1>
        </div>

        {/* §3 gap G4 · the recap "继续工作" continue button now lives in the AI
            chat (StudioTurnChips), not a pane banner. */}

        {/* Toolbar: title + view toggle + export */}
        <div className="mb-4 flex items-center justify-between">
          <div>
            <h2 className="font-sans text-[20px] font-bold text-mk-ink">项目管理</h2>
            <p className="mt-0.5 text-[14px] text-mk-muted">{view === "log" ? "项目一路上发生了什么——大多自动记下，你也能补一笔。" : "拖动来编辑：看板换列、甘特图挪动/拉长。点任务卡查看或修改。"}</p>
          </div>
          <div className="flex items-center gap-3">
            <Segmented
              options={[
                { value: "kanban", label: "看板" },
                { value: "gantt", label: "甘特图" },
                { value: "log", label: "活动日志" },
              ]}
              value={view}
              onChange={(v) => setView(v as PlanView)}
            />
            <button type="button" onClick={view === "log" ? exportLog : exportPlan} disabled={exporting} className="rounded-mk-md border border-mk-border bg-mk-surface px-3.5 py-2 text-[14px] font-semibold text-mk-muted hover:text-mk-accent disabled:opacity-60">
              {exporting ? "导出中…" : "导出"}
            </button>
          </div>
        </div>

        {view === "kanban" && <KanbanView board={board} loading={loadingPlan} anchor={anchor} onMove={(id, column) => patchItem(id, { column })} onAddTask={addTask} onEditItem={(i) => setEditingId(i.id)} />}
        {view === "gantt" && <GanttView board={board} anchor={anchor} onReschedule={(id, start) => patchItem(id, { start })} onResize={(id, days) => patchItem(id, { days })} onAddTask={addTaskGantt} onEditItem={(i) => setEditingId(i.id)} />}
        {view === "log" && <ActivityLogView log={log} onAdd={addLogEntry} />}

        {editing && (
          <PlanItemEditor
            item={editing}
            stageOptions={stageOptions}
            onPatch={(patch) => patchItem(editing.id, patch)}
            onDelete={() => { removeItem(editing.id); setEditingId(null); }}
            onClose={() => setEditingId(null)}
          />
        )}
      </div>

      {/* COACH — portaled into the constant AiPanel, same contract as every
          other room (Task 4). 管理 has no room-specific coach chrome to add,
          so it reuses the shared `StudioCoachChat` body as-is (mirrors
          ReadingBlock). */}
      {slot && createPortal(<StudioCoachChat />, slot)}
    </>
  );
}

/* ----- Kanban (HTML5 drag between columns) ----- */

function KanbanView({ board, loading, anchor, onMove, onAddTask, onEditItem }: { board: PlanItem[]; loading: boolean; anchor: Date; onMove: (id: string, c: PlanColumn) => void; onAddTask: (stage: string) => void; onEditItem: (i: PlanItem) => void }) {
  const [dragId, setDragId] = useState<string | null>(null);
  const [over, setOver] = useState<PlanColumn | null>(null);
  return (
    <div className="grid min-h-0 flex-1 grid-cols-3 gap-5">
      {COLUMNS.map((col) => {
        const colItems = board.filter((i) => i.column === col);
        const isOver = over === col;
        return (
          <section
            key={col}
            onDragOver={(e) => { e.preventDefault(); setOver(col); }}
            onDragLeave={() => setOver((o) => (o === col ? null : o))}
            onDrop={() => { if (dragId) onMove(dragId, col); setDragId(null); setOver(null); }}
            className={`flex min-h-0 flex-col rounded-mk-lg border-2 p-3 transition ${isOver ? "border-mk-accent/50 bg-mk-accent-50" : "border-transparent bg-mk-surface"}`}
          >
            <header className="mb-3 flex items-center justify-between px-1">
              <span className="text-[14px] font-bold text-mk-ink">{COLUMN_LABEL[col]}</span>
              <span className="rounded-full bg-mk-paper px-2 py-0.5 text-[12px] font-semibold text-mk-faint">{colItems.length}</span>
            </header>
            <div className="flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto pr-0.5">
              {colItems.map((item) => (
                <PlanCard key={item.id} item={item} anchor={anchor} dragging={dragId === item.id} onEdit={() => onEditItem(item)} onDragStart={() => setDragId(item.id)} onDragEnd={() => { setDragId(null); setOver(null); }} />
              ))}
              {colItems.length === 0 && (
                <div className="rounded-mk-md border border-dashed border-mk-border px-3 py-6 text-center text-[12px] text-mk-faint">
                  {loading ? "加载中…" : "拖到这里"}
                </div>
              )}
            </div>
            {col === "todo" && (
              <button type="button" onClick={() => onAddTask(STAGE_1)} className="mt-2 rounded-mk-md border border-dashed border-mk-border py-2 text-[14px] font-semibold text-mk-faint hover:border-mk-accent hover:text-mk-accent">+ 添加任务</button>
            )}
          </section>
        );
      })}
    </div>
  );
}

// A plan card: the body (tag row + title) opens the edit popover.
function PlanCard({ item, anchor, dragging, onEdit, onDragStart, onDragEnd }: { item: PlanItem; anchor: Date; dragging: boolean; onEdit: () => void; onDragStart: () => void; onDragEnd: () => void }) {
  // #14: the card's scheduled window as calendar dates.
  const startDate = addDays(anchor, item.start);
  const endDate = addDays(anchor, item.start + Math.max(1, item.days) - 1);
  return (
    <div
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      className={`group relative cursor-grab rounded-mk-md border border-mk-border bg-mk-surface p-3 shadow-mk-xs transition active:cursor-grabbing hover:border-mk-accent/40 hover:shadow-mk-sm ${dragging ? "opacity-40" : ""}`}
    >
      <button type="button" onClick={onEdit} title="查看 / 修改任务" className="mb-2 flex w-full items-center gap-1.5 text-left">
        <span className={`rounded px-1.5 py-0.5 text-[12px] font-bold ${TAG_STYLE[item.tag]}`}>{TAG_LABEL[item.tag]}</span>
        <span className="ml-auto truncate text-[12px] text-mk-faint">{item.stage.split(" · ")[0]}</span>
      </button>
      <button type="button" onClick={onEdit} className="block w-full text-left text-[14px] font-medium leading-snug text-mk-ink hover:text-mk-accent">{item.title}</button>
      <div className="mt-1.5 text-[12px] font-medium text-mk-faint">📅 {fmtMD(startDate)} – {fmtMD(endDate)}</div>
    </div>
  );
}

/* ----- Gantt (drag to move, resize handle to change duration), by stage ----- */

function GanttView({ board, anchor, onReschedule, onResize, onAddTask, onEditItem }: { board: PlanItem[]; anchor: Date; onReschedule: (id: string, start: number) => void; onResize: (id: string, days: number) => void; onAddTask: () => void; onEditItem: (i: PlanItem) => void }) {
  const days = Array.from({ length: TIMELINE_DAYS }, (_, i) => i);
  // #14: today's day-index from the anchor drives the today-line. When it
  // falls outside [0, TIMELINE_DAYS) no column matches, so nothing highlights.
  const todayIdx = dayIndexFromAnchor(anchor, new Date());
  // Render every stage present on the board (a generated plan may use stage
  // names beyond the two canonical ones), ordered by where the stage actually
  // sits on the timeline — its earliest task start — so 阶段二 never renders
  // after 阶段三 just because its first task happened to be created/loaded later
  // (#10). Ties fall back to first-appearance order (Array.sort is stable).
  const stages = Array.from(new Set(board.map((i) => i.stage)));
  const stageStart = (s: string) => Math.min(...board.filter((i) => i.stage === s).map((i) => i.start));
  stages.sort((a, b) => stageStart(a) - stageStart(b));
  return (
    <div className="min-h-0 flex-1 overflow-auto rounded-mk-lg border border-mk-border bg-mk-surface">
      <div className="min-w-[820px]">
        {/* Day header */}
        <div className="sticky top-0 z-10 grid grid-cols-[240px,1fr] border-b border-mk-border bg-mk-surface">
          <div className="flex flex-col justify-center px-4 py-1.5 text-[12px] font-bold text-mk-faint">
            任务
            <span className="text-[12px] font-medium text-mk-faint/80">{fmtMD(anchor)} 起 · 今天已在时间线上标出</span>
          </div>
          <div className="grid" style={{ gridTemplateColumns: `repeat(${TIMELINE_DAYS}, 1fr)` }}>
            {days.map((d) => {
              const date = addDays(anchor, d);
              const dow = date.getDay();
              const isWeekend = dow === 0 || dow === 6;
              const isToday = d === todayIdx;
              const showMonth = d === 0 || date.getDate() === 1;
              return (
                <div key={d} className={`border-l border-mk-border py-1.5 text-center ${isWeekend ? "text-mk-faint" : "text-mk-muted"} ${isToday ? "bg-mk-accent-50" : ""}`}>
                  <div className="text-[9px] leading-tight opacity-70">{WEEKDAY_ZH[dow]}</div>
                  <div className={`text-[12px] font-semibold leading-tight ${isToday ? "text-mk-accent" : ""}`}>{showMonth ? fmtMD(date) : date.getDate()}</div>
                </div>
              );
            })}
          </div>
        </div>
        {/* Stage groups */}
        {stages.map((stage) => {
          const items = board.filter((i) => i.stage === stage);
          if (items.length === 0) return null;
          return (
            <div key={stage}>
              <div className="grid grid-cols-[240px,1fr] border-b border-mk-border bg-mk-paper">
                <div className="px-4 py-1.5 text-[12px] font-bold uppercase tracking-wider text-mk-muted">{stage}</div>
                <div />
              </div>
              {items.map((item) => (
                <div key={item.id} className="group grid grid-cols-[240px,1fr] items-center border-b border-mk-border hover:bg-mk-paper">
                  <div className="flex items-center gap-1 px-4 py-3">
                    {/* label click = view/edit */}
                    <button type="button" onClick={() => onEditItem(item)} title="查看 / 修改任务" className="flex min-w-0 flex-1 items-center gap-2 text-left">
                      <span className={`flex-none rounded px-1.5 py-0.5 text-[12px] font-bold ${TAG_STYLE[item.tag]}`}>{TAG_LABEL[item.tag]}</span>
                      <span className="truncate text-[14px] font-medium text-mk-ink hover:text-mk-accent">{item.title.replace(/^[读写省]：/, "")}</span>
                    </button>
                  </div>
                  <div data-track className="relative h-11">
                    <div className="absolute inset-0 grid" style={{ gridTemplateColumns: `repeat(${TIMELINE_DAYS}, 1fr)` }}>
                      {days.map((d) => {
                        const dow = addDays(anchor, d).getDay();
                        const isWeekend = dow === 0 || dow === 6;
                        const isToday = d === todayIdx;
                        return (
                          <div
                            key={d}
                            className={`${isToday ? "border-l-2 border-l-mk-accent/60 bg-mk-accent-50" : `border-l border-mk-border ${isWeekend ? "bg-mk-paper" : ""}`}`}
                          />
                        );
                      })}
                    </div>
                    <GanttBar item={item} onReschedule={onReschedule} onResize={onResize} />
                  </div>
                </div>
              ))}
            </div>
          );
        })}
        {/* #3 — add a task straight from the Gantt (Kanban already has one) */}
        <div className="grid grid-cols-[240px,1fr] border-b border-mk-border">
          <button type="button" onClick={onAddTask} className="px-4 py-2.5 text-left text-[14px] font-semibold text-mk-faint hover:text-mk-accent">+ 添加任务</button>
          <div />
        </div>
      </div>
    </div>
  );
}

function GanttBar({ item, onReschedule, onResize }: { item: PlanItem; onReschedule: (id: string, start: number) => void; onResize: (id: string, days: number) => void }) {
  const drag = useRef<{ mode: "move" | "resize"; startX: number; orig: number; trackW: number; moved: boolean } | null>(null);

  function begin(mode: "move" | "resize", e: React.PointerEvent) {
    e.stopPropagation();
    const track = (e.currentTarget as HTMLElement).closest("[data-track]") as HTMLElement | null;
    if (!track) return;
    drag.current = { mode, startX: e.clientX, orig: mode === "move" ? item.start : item.days, trackW: track.getBoundingClientRect().width, moved: false };
    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", onPointerUp);
  }
  function onPointerMove(e: PointerEvent) {
    const d = drag.current;
    if (!d) return;
    const dayW = d.trackW / TIMELINE_DAYS;
    const delta = Math.round((e.clientX - d.startX) / dayW);
    if (delta !== 0) d.moved = true;
    if (d.mode === "move") onReschedule(item.id, clamp(d.orig + delta, 0, TIMELINE_DAYS - item.days));
    else onResize(item.id, clamp(d.orig + delta, 1, TIMELINE_DAYS - item.start));
  }
  function onPointerUp() {
    drag.current = null;
    window.removeEventListener("pointermove", onPointerMove);
    window.removeEventListener("pointerup", onPointerUp);
  }

  const dim = item.column === "todo" ? "opacity-50" : item.column === "doing" ? "opacity-85" : "";
  return (
    <div
      onPointerDown={(e) => begin("move", e)}
      title={`${item.title} · 第${item.start + 1}–${item.start + item.days}天`}
      className={`absolute top-1/2 flex h-6 -translate-y-1/2 cursor-grab items-center rounded-md ${TAG_BAR[item.tag]} ${dim} select-none active:cursor-grabbing`}
      style={{ left: `calc(${(item.start / TIMELINE_DAYS) * 100}% + 3px)`, width: `calc(${(item.days / TIMELINE_DAYS) * 100}% - 6px)` }}
    >
      <span className="pointer-events-none flex-1 truncate px-2 text-[12px] font-bold leading-6 text-white">
        {item.column === "done" ? "✓ " : ""}{item.days}天
      </span>
      {/* resize handle */}
      <span
        onPointerDown={(e) => begin("resize", e)}
        className="h-full w-2 cursor-ew-resize rounded-r-md border-l border-white/30 hover:bg-white/20"
      />
    </div>
  );
}

/* ----- Activity log (real EPQ deliverable; auto-seeded + student notes) ----- */

// #5 — the log a student sees should read as a timeline of real progress, not
// every "opened/viewing X" the platform happens to notice. This is a DISPLAY
// filter only (铁律④ 过程即数据): the server still writes every auto-seeded
// entry via appendAutoLog (apps/api/internal/api/*.go) and `log` (the state
// this component receives) stays the full, unfiltered list — export
// (exportActivityLog, called on the raw `log`, not on these filtered rows)
// and any future assessor/process read still see everything. We only decide,
// here, which rows EARN a spot in the student-facing timeline.
//
// The wire (contracts LogEntry) has no `type` — auto entries are free text
// templated at the call site — so milestones are recognized by the phrase
// each call site uses today. A student's own note (source="me") is never
// noise: she chose to write it.
const NOISE_PATTERNS = [
  /^打开来源/, // workspace_library.go:609 — entering the reading room, not finishing it
  /^新增计划任务/, // workspace_plan.go:187 — routine board bookkeeping, not a milestone
];
export function isMilestoneLogEntry(entry: LogEntry): boolean {
  if (entry.source === "me") return true;
  return !NOISE_PATTERNS.some((re) => re.test(entry.text));
}

export function ActivityLogView({ log, onAdd }: { log: LogEntry[] | null; onAdd: (text: string) => void }) {
  const [draft, setDraft] = useState("");
  const rows = (log ?? []).filter(isMilestoneLogEntry);
  function submit() {
    if (!draft.trim()) return;
    onAdd(draft.trim());
    setDraft("");
  }
  return (
    <div className="mx-auto flex min-h-0 w-full max-w-3xl flex-1 flex-col">
      <div className="min-h-0 flex-1 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface">
        {log === null ? (
          <div className="flex h-full items-center justify-center py-16 text-[14px] text-mk-faint">加载中…</div>
        ) : rows.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center gap-1 py-16 text-center">
            <p className="text-[14px] font-semibold text-mk-ink">还没有记录</p>
            <p className="text-[14px] text-mk-faint">你在项目里做的事会自动记下——也可以现在补一笔。</p>
          </div>
        ) : (
          // #5 — one box per date; each date's lines keep their 自动/我记的 tag.
          <div className="flex flex-col gap-4 p-4">
            {groupByDate(rows).map((g) => (
              <div key={g.date} className="rounded-mk-md border border-mk-border bg-mk-paper">
                <div className="border-b border-mk-border px-4 py-2 text-[12px] font-bold text-mk-faint">{g.date}</div>
                <div className="flex flex-col">
                  {g.entries.map((e, i) => (
                    <div key={e.id} className={`flex items-start gap-3 px-4 py-2.5 ${i < g.entries.length - 1 ? "border-b border-mk-border/60" : ""}`}>
                      <p className="flex-1 text-[14px] leading-relaxed text-mk-ink">{e.text}</p>
                      <span className={`flex-none self-start rounded-full px-2 py-0.5 text-[12px] font-bold ${e.source === "auto" ? "bg-mk-accent-50 text-mk-accent" : "bg-mk-success-bg text-mk-success"}`}>
                        {e.source === "auto" ? "自动" : "我记的"}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="mt-3 flex items-end gap-2 rounded-mk-lg border border-mk-border bg-mk-surface p-2.5">
        <input value={draft} onChange={(e) => setDraft(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter" && !e.nativeEvent.isComposing) { e.preventDefault(); submit(); } }} placeholder="补一笔：今天做了什么、想到什么……" className="flex-1 bg-transparent px-2 py-1.5 text-[14px] text-mk-ink outline-none placeholder:text-mk-faint" />
        <button
          type="button"
          onClick={submit}
          className="rounded-mk-md bg-mk-accent px-3.5 py-2 text-[14px] font-bold text-white hover:bg-mk-accent-600"
        >
          记一笔
        </button>
      </div>
    </div>
  );
}

// Group log entries into one bucket per date, preserving first-seen order.
function groupByDate(rows: LogEntry[]): { date: string; entries: LogEntry[] }[] {
  const groups: { date: string; entries: LogEntry[] }[] = [];
  for (const e of rows) {
    const g = groups.find((x) => x.date === e.date);
    if (g) g.entries.push(e);
    else groups.push({ date: e.date, entries: [e] });
  }
  return groups;
}

/* ----- Plan item detail / edit popover (#4) ----- */

// Clicking a card body (kanban) or a row label (gantt) opens this. The student
// can retitle, retag, restage, move column, and reschedule/resize — one PATCH
// on 保存 — or delete the task outright.
function PlanItemEditor({ item, stageOptions, onPatch, onDelete, onClose }: {
  item: PlanItem;
  stageOptions: string[];
  onPatch: (patch: PlanItemPatch) => void;
  onDelete: () => void;
  onClose: () => void;
}) {
  const [title, setTitle] = useState(item.title);
  const [tag, setTag] = useState<PlanTag>(item.tag);
  const [stage, setStage] = useState(item.stage);
  const [column, setColumn] = useState<PlanColumn>(item.column);
  const [start, setStart] = useState(item.start);
  const [days, setDays] = useState(item.days);

  function save() {
    const s = clamp(Math.round(start), 0, TIMELINE_DAYS - 1);
    const d = clamp(Math.round(days), 1, TIMELINE_DAYS - s);
    onPatch({ title: title.trim() || item.title, tag, stage, column, start: s, days: d });
    onClose();
  }

  const tags: PlanTag[] = ["read", "write", "review"];
  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-black/30 px-8" onClick={onClose}>
      <div className="flex w-[460px] flex-col rounded-mk-lg border border-mk-border bg-mk-surface shadow-mk-lg" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b border-mk-border px-5 py-3.5">
          <h3 className="font-sans text-[16px] font-bold text-mk-ink">任务详情</h3>
          <button type="button" onClick={onClose} className="text-[20px] leading-none text-mk-faint hover:text-mk-ink">×</button>
        </div>
        <div className="flex flex-col gap-4 px-5 py-4">
          <label className="block">
            <span className="mb-1 block text-[12px] font-bold text-mk-faint">任务</span>
            <input value={title} onChange={(e) => setTitle(e.target.value)} className="w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-[14px] text-mk-ink outline-none focus:border-mk-accent" />
          </label>

          <div>
            <span className="mb-1 block text-[12px] font-bold text-mk-faint">类别</span>
            <div className="flex gap-2">
              {tags.map((t) => (
                <button key={t} type="button" onClick={() => setTag(t)} className={`rounded-mk-md px-3 py-1.5 text-[14px] font-bold transition ${tag === t ? TAG_STYLE[t] + " ring-2 ring-mk-accent/40" : "bg-mk-paper text-mk-faint hover:text-mk-muted"}`}>{TAG_LABEL[t]}</button>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <label className="block">
              <span className="mb-1 block text-[12px] font-bold text-mk-faint">阶段</span>
              <select value={stage} onChange={(e) => setStage(e.target.value)} className="w-full rounded-mk-md border border-mk-border bg-mk-surface px-2 py-2 text-[14px] text-mk-ink outline-none focus:border-mk-accent">
                {(stageOptions.includes(stage) ? stageOptions : [stage, ...stageOptions]).map((s) => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-1 block text-[12px] font-bold text-mk-faint">状态</span>
              <select value={column} onChange={(e) => setColumn(e.target.value as PlanColumn)} className="w-full rounded-mk-md border border-mk-border bg-mk-surface px-2 py-2 text-[14px] text-mk-ink outline-none focus:border-mk-accent">
                {COLUMNS.map((c) => (<option key={c} value={c}>{COLUMN_LABEL[c]}</option>))}
              </select>
            </label>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <label className="block">
              <span className="mb-1 block text-[12px] font-bold text-mk-faint">开始（第几天）</span>
              <input type="number" min={1} max={TIMELINE_DAYS} value={start + 1} onChange={(e) => setStart((Number(e.target.value) || 1) - 1)} className="w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-[14px] text-mk-ink outline-none focus:border-mk-accent" />
            </label>
            <label className="block">
              <span className="mb-1 block text-[12px] font-bold text-mk-faint">持续（天）</span>
              <input type="number" min={1} max={TIMELINE_DAYS} value={days} onChange={(e) => setDays(Number(e.target.value) || 1)} className="w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-[14px] text-mk-ink outline-none focus:border-mk-accent" />
            </label>
          </div>
        </div>
        <div className="flex items-center justify-between border-t border-mk-border px-5 py-3.5">
          <button type="button" onClick={onDelete} className="rounded-mk-md px-3 py-2 text-[14px] font-semibold text-mk-accent hover:bg-mk-accent-50">删除任务</button>
          <div className="flex gap-2">
            <button type="button" onClick={onClose} className="rounded-mk-md border border-mk-border px-4 py-2 text-[14px] font-semibold text-mk-muted hover:text-mk-accent">取消</button>
            <button type="button" onClick={save} className="rounded-mk-md bg-mk-accent px-4 py-2 text-[14px] font-bold text-white hover:bg-mk-accent-600">保存</button>
          </div>
        </div>
      </div>
    </div>
  );
}
