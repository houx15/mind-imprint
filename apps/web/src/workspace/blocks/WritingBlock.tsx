import { useEffect, useMemo, useRef, useState } from "react";
import type { Proposal, ProjectStatus } from "@mind-imprint/contracts";
import { putBuffer, runDraftReview } from "../../api/writing";
import type { ReviewItem, ReviewVoice, DraftReviewResult } from "../../api/writing";
import { finishWriting, reopenWriting } from "../../api/projects";
import { ApiError } from "../../api/client";
import { getExploration } from "../../api/exploration";
import { exportDraftDocx } from "../export";
import { Icon } from "../Icon";
import type { BlockKey } from "./mockData";
import { getOutline, putOutline, getSnippets, putSnippets, getDraft, coach, getCoachHistory, reflectProjectCard, dismissProposal } from "../api/workspace";
import { MaterialsSidebar } from "./MaterialsSidebar";
import { parseSections, serializeSections, sectionsFromOutline, newSection, type DraftSection } from "./draftSections";
import type { CardProposalWire } from "../api/workspace";
import { MarkdownPreview } from "./MarkdownPreview";
import { CoachProposal } from "./CoachProposal";
import { StudioCardSheet } from "../../studio/StudioCardSheet";
import { compileCardEnvelope, compileCardForCoach } from "../../studio/compileCard";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

// One outline bullet in local edit shape — flat-with-depth, the same model the
// prototype used (the persisted OutlineNode adds a server-owned `position`,
// which the array order carries here).
type Row = { id: string; text: string; depth: number };
// #9-second · quotedPart carries a referenced paragraph SEPARATELY from the
// question text, so the rail can render it as a styled callout above the
// bubble instead of baking a literal 【就这一段】 token into the message string.
type ChatMsg = { role: "ai" | "student"; text: string; quotedPart?: string };

// Max outline nesting depth (0 = top level). Indent clamps here.
const MAX_DEPTH = 2;

// A monotonic client-side id for freshly-added rows before the server mints a
// real one. Any string is fine — the server re-assigns ids on every PUT.
let tempSeq = 0;
const tempId = () => `tmp-${tempSeq++}`;

// ── Outline keyboard reducer (pure, tested) ────────────────────────────────
// The keystroke semantics of a real outliner, factored out so the behavior can
// be unit-tested without a DOM:
//   enter     → insert a blank sibling right below at the same depth, focus it
//   indent    → depth + 1 (clamped ≤ MAX_DEPTH); caret stays put (no refocus)
//   outdent   → depth − 1 (clamped ≥ 0); caret stays put
//   backspace → only meaningful on an empty, non-first row: delete it and put
//               the caret at the end of the previous row
// `focus` is null when the caret should stay where the browser already has it
// (indent/outdent don't reorder the DOM, so focus is naturally retained).
export type OutlineKeyType = "enter" | "indent" | "outdent" | "backspace";
export type OutlineKeyResult = { rows: Row[]; focus: { id: string; atEnd: boolean } | null };

export function outlineKey(
  rows: Row[],
  type: OutlineKeyType,
  id: string,
  makeId: () => string = tempId,
): OutlineKeyResult {
  const i = rows.findIndex((r) => r.id === id);
  if (i < 0) return { rows, focus: null };
  const row = rows[i]!;
  switch (type) {
    case "enter": {
      const nid = makeId();
      const next = [...rows];
      next.splice(i + 1, 0, { id: nid, text: "", depth: row.depth });
      return { rows: next, focus: { id: nid, atEnd: false } };
    }
    case "indent": {
      const depth = Math.min(MAX_DEPTH, row.depth + 1);
      if (depth === row.depth) return { rows, focus: null };
      return { rows: rows.map((r, idx) => (idx === i ? { ...r, depth } : r)), focus: null };
    }
    case "outdent": {
      const depth = Math.max(0, row.depth - 1);
      if (depth === row.depth) return { rows, focus: null };
      return { rows: rows.map((r, idx) => (idx === i ? { ...r, depth } : r)), focus: null };
    }
    case "backspace": {
      // Only delete when the row is empty and there's a previous row to merge
      // the caret onto. Otherwise a no-op (the caller lets the default run).
      if (row.text !== "" || i === 0) return { rows, focus: null };
      const prev = rows[i - 1]!;
      return { rows: rows.filter((_, idx) => idx !== i), focus: { id: prev.id, atEnd: true } };
    }
  }
}

// The Write block: two gears — 提纲 (outline) and 写作 (a single draft panel) —
// with a slim goal strip up top so you write against your thesis, and an AI
// rail that talks about your outline/draft (never writes it). The proposal
// stays in Project Management; here we only link back to it. Outline + draft are
// API-backed (slice 4): outline edits debounce to PUT /outline, the draft
// debounces to PUT /buffer, and the rail calls POST /coach (writing scope).
export function WritingBlock({
  projectId,
  title,
  proposal,
  status,
  writingFinished,
  onOpenRoom,
  refreshWorkspace,
}: {
  projectId: string;
  title: string;
  proposal: Proposal;
  status: ProjectStatus;
  // #20 · the 完成写作 milestone — draft is read-only once true. Separate from
  // status (evaluating/done terminally lock too).
  writingFinished: boolean;
  onOpenRoom: (room: BlockKey) => void;
  // Re-pull the projection so a 完成写作 / 重新打开写作 toggle propagates to both
  // rooms (WritingBlock's lock + ReviewBlock's gate) without a full remount.
  refreshWorkspace: () => Promise<void> | void;
}) {
  const [tab, setTab] = useState<"outline" | "snippets" | "draft">("outline");
  // WC · part-by-part: the draft part the student has pinned to think through
  // with 印记 (lifted so DraftPane can set it and the rail can consume it).
  const [focusPart, setFocusPart] = useState<string | null>(null);
  // #8-second · a voice-scoped 体检 requested from the persistent rail shelf
  // (CoachRail, a tab-sibling of DraftPane) — lifted here as a queued request
  // rather than a direct call, since the shelf is reachable from any tab and
  // DraftPane may not be mounted yet. requestReview switches to 正文 (so the
  // review panel is visible) and queues the request; DraftPane's own
  // mount-effect flushes it once it's actually mounted (and its draft loaded).
  const [pendingReview, setPendingReview] = useState<{ scope?: string; voice: ReviewVoice } | null>(null);
  function requestReview(scope: string | undefined, v: ReviewVoice) {
    setTab("draft");
    setPendingReview({ scope, voice: v });
  }
  // #5/#20 · 完成写作 is a guarded moment. Clicking it opens a confirm modal;
  // confirming locks the draft read-only (writing_finished_at) AND routes to 回顾
  // (which only unlocks once writing is finished). Reversible via 重新打开写作
  // until the project is archived. Once archived (评估中 / 已完成) the draft is
  // terminally read-only — the true point of no return.
  const [showFinishModal, setShowFinishModal] = useState(false);
  const [finishingWriting, setFinishingWriting] = useState(false);
  const [finishWritingError, setFinishWritingError] = useState<string | null>(null);
  // archived = the terminal finalize path has begun (can't reopen writing then).
  const archived = status === "evaluating" || status === "done";
  const locked = archived || writingFinished;

  // #20 · confirm → lock the draft → go to 回顾. finishWriting is idempotent;
  // 422 draft_empty when there's nothing written yet.
  async function doFinishWriting() {
    if (finishingWriting) return;
    setFinishingWriting(true);
    setFinishWritingError(null);
    try {
      await finishWriting(projectId);
      await refreshWorkspace();
      onOpenRoom("reflection");
    } catch (e) {
      setFinishWritingError(
        e instanceof ApiError ? e.message || "还不能完成写作，请稍后再试。" : "刚才没接上，稍等再试一次。",
      );
    } finally {
      setFinishingWriting(false);
    }
  }

  // #20 (铁律②) · reopen — reversible until the project is archived.
  async function doReopenWriting() {
    try {
      await reopenWriting(projectId);
      await refreshWorkspace();
    } catch {
      /* best-effort; the affordance stays and can be retried */
    }
  }
  // #9 · the materials sidebar (any tab) places a fragment into the draft at the
  // caret. DraftPane registers its inserter here on mount; the sidebar calls it
  // only when 正文 is active (so DraftPane is mounted and the ref is set).
  const draftInsertRef = useRef<((t: string) => void) | null>(null);
  // #23: snippets state is lifted here so the materials sidebar (below) can
  // append a source's note as a new snippet regardless of the active tab.
  const snip = useSnippets(projectId);
  // #6 · the outline headings the student has explicitly imported as snippet
  // board sections (see importedSectionsMemo above) — lifted here so both the
  // 片段 board (renders them as foldable groups) and the materials sidebar
  // (the import button + its "already imported" state) share one source of
  // truth, and it survives switching tabs (SnippetsPane unmounts on tab-away).
  const [importedSections, setImportedSectionsState] = useState<string[]>(
    () => importedSectionsMemo.get(projectId) ?? [],
  );
  function importOutlineAsGroups(headings: string[]) {
    const merged = dedupe([...(importedSectionsMemo.get(projectId) ?? []), ...headings]);
    importedSectionsMemo.set(projectId, merged);
    setImportedSectionsState(merged);
  }
  // Section labels a card's compiled 片段 can be filed under (Item C's 收进片段
  // offer): the imported outline groups plus any label already in live use on a
  // snippet (e.g. an探索线索 the student filed one under).
  const knownSectionLabels = useMemo(
    () => dedupe([...importedSections, ...snip.snippets.map((s) => s.section).filter((x): x is string => !!x)]),
    [importedSections, snip.snippets],
  );
  return (
    <div className="flex h-full flex-col">
      {/* goal strip */}
      <div className="flex items-center gap-3 border-b border-mk-border bg-mk-surface px-8 py-2.5">
        <span className="flex-none rounded-full bg-mk-accent-tint px-2 py-0.5 text-[11px] font-bold text-mk-accent">论点</span>
        <p className="min-w-0 flex-1 truncate text-[13px] text-mk-ink">{proposal.objective || "还没有写下你的论点——先去开题里想清楚。"}</p>
        <button type="button" onClick={() => onOpenRoom("plan")} className="flex-none text-[12px] font-semibold text-mk-muted-2 hover:text-mk-primary">看开题 →</button>
        {archived ? (
          <button type="button" onClick={() => onOpenRoom("reflection")} className="flex-none rounded-mk border border-mk-border px-3 py-1 text-[12px] font-bold text-mk-muted hover:text-mk-primary">已归档 · 看回顾 →</button>
        ) : writingFinished ? (
          <>
            {/* #20 · reversible — 重新打开写作 unlocks the draft again (铁律②). */}
            <button type="button" onClick={() => void doReopenWriting()} className="flex-none rounded-mk border border-mk-border px-3 py-1 text-[12px] font-bold text-mk-muted hover:text-mk-primary" title="重新打开写作，继续修改初稿">重新打开写作</button>
            <button type="button" onClick={() => onOpenRoom("reflection")} className="flex-none rounded-mk bg-mk-primary px-3 py-1 text-[12px] font-bold text-white hover:bg-mk-primary-hover">去回顾 →</button>
          </>
        ) : (
          <button type="button" onClick={() => setShowFinishModal(true)} className="flex-none rounded-mk bg-mk-primary px-3 py-1 text-[12px] font-bold text-white hover:bg-mk-primary-hover" title="写完了？点这里锁定初稿、进入回顾（之后仍可重新打开）">完成写作</button>
        )}
      </div>

      {/* tabs */}
      <div className="flex items-center gap-2 border-b border-mk-border bg-mk-surface px-8 py-2.5">
        <Tab active={tab === "outline"} onClick={() => setTab("outline")} icon="plan">大纲</Tab>
        <Tab active={tab === "snippets"} onClick={() => setTab("snippets")} icon="spark">片段</Tab>
        <Tab active={tab === "draft"} onClick={() => setTab("draft")} icon="writing">正文</Tab>
      </div>

      <div className="relative grid min-h-0 flex-1 grid-cols-[1fr,320px]">
        {tab === "outline" ? (
          <OutlinePane projectId={projectId} title={title} />
        ) : tab === "snippets" ? (
          <SnippetsPane snip={snip} projectId={projectId} importedSections={importedSections} />
        ) : (
          <DraftPane
            projectId={projectId}
            title={title}
            locked={locked}
            onFocusPart={setFocusPart}
            registerInsert={(fn) => { draftInsertRef.current = fn; }}
            pendingReview={pendingReview}
            onPendingReviewHandled={() => setPendingReview(null)}
          />
        )}
        <CoachRail
          projectId={projectId}
          focusPart={focusPart}
          onClearFocus={() => setFocusPart(null)}
          locked={locked}
          onCardArtifact={(text, section) => snip.add(text, section)}
          sectionOptions={knownSectionLabels}
          onRunReview={requestReview}
        />
        {/* #23/#9 · draggable materials sidebar — browses 材料/大纲/片段 and places a
            fragment where you're working: into the draft at the caret on 正文,
            else appended as a new snippet. */}
        <MaterialsSidebar
          projectId={projectId}
          activeTab={tab}
          locked={locked}
          snippets={snip.snippets}
          onAddSnippet={(text) => snip.add(text)}
          onInsertToDraft={(text) => draftInsertRef.current?.(text)}
          onImportOutlineAsGroups={importOutlineAsGroups}
          importedSections={importedSections}
        />
      </div>

      {/* #5/#20 · 完成写作 confirm — the first guarded moment. Confirming LOCKS the
          draft read-only and unlocks 回顾. Reversible via 重新打开写作 until you 归档
          there, at which point 正文与回顾都会锁定、不能再改，并生成过程评估。 */}
      {showFinishModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-mk-ink/40 px-6">
          <div className="w-full max-w-md rounded-mk-lg border border-mk-border bg-mk-surface p-7 shadow-[0_20px_60px_rgba(28,35,51,0.25)]">
            <h2 className="font-sans text-[18px] font-bold text-mk-ink">完成写作？</h2>
            <p className="mt-3 text-[14px] leading-relaxed text-mk-muted">
              点「完成写作」会<span className="font-bold text-mk-ink">锁定初稿</span>、解锁<span className="font-bold text-mk-ink">回顾</span>。之后<span className="font-bold text-mk-accent">仍可重新打开写作</span>继续改；只有在回顾里<span className="font-bold text-mk-accent">定稿评估</span>后才真正锁定。
            </p>
            {finishWritingError && (
              <p className="mt-3 text-[12.5px] font-semibold text-mk-accent">{finishWritingError}</p>
            )}
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setShowFinishModal(false)}
                className="rounded-mk border border-mk-border px-4 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-ink"
              >
                再改改
              </button>
              <button
                type="button"
                disabled={finishingWriting}
                onClick={() => { setShowFinishModal(false); void doFinishWriting(); }}
                className="rounded-mk bg-mk-primary px-5 py-2 text-[13px] font-bold text-white transition hover:bg-mk-primary-hover disabled:opacity-50"
              >
                {finishingWriting ? "锁定中……" : "完成写作，去回顾 →"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/* ---------- #23 · snippets (片段) ---------- */

// #6 · outline→snippet-groups is opt-in ONLY (imported once via an explicit
// 「把大纲导入为片段分组」action in the materials sidebar) — the live outline is
// never auto-rendered as snippet board sections (the student must choose to
// bring it in). The imported heading set is a client-side per-project memory,
// mirroring ReadingBlock's viewModeMemo: in-memory only (lost on a hard
// refresh), no DB migration for this UI-only grouping.
const importedSectionsMemo = new Map<string, string[]>();

export type Snip = { id: string; text: string; section: string | null };

// useSnippets owns the 片段 board's load + debounced whole-set save (mirrors
// OutlinePane's persistence), lifted so both SnippetsPane and the materials
// sidebar mutate one source of truth. section (#5) files a snippet under an
// outline heading / 线索 label (null = 未归类).
export type SnippetsHandle = {
  snippets: Snip[];
  // Returns the new snippet's (temp, stable-until-save) id, so a caller that
  // just created a blank row (「+ 在此加片段」) can immediately focus it into
  // edit mode without guessing which row is new.
  add: (text: string, section?: string | null) => string;
  update: (id: string, text: string) => void;
  remove: (id: string) => void;
  setSection: (id: string, section: string | null) => void;
};
function useSnippets(projectId: string): SnippetsHandle {
  const [snippets, setSnippets] = useState<Snip[]>([]);
  const ref = useRef<Snip[]>([]);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  async function save(rows: Snip[]) {
    try {
      // Whole-set replace ignores incoming ids (the server mints fresh ones), so
      // the client id is purely a local React key — keep it STABLE across saves.
      // Adopting the server id here would change the key and remount the
      // textarea mid-edit, dropping focus/caret (review MEDIUM). So don't swap.
      await putSnippets(projectId, rows.map((s) => ({ text: s.text, section: s.section })));
    } catch {
      /* keep local; the next debounced save retries */
    }
  }
  function commit(next: Snip[]) {
    ref.current = next;
    setSnippets(next);
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => save(next), 700);
  }

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const loaded = await getSnippets(projectId);
        if (cancelled) return;
        const rows = loaded.map((s) => ({ id: s.id, text: s.text, section: s.section }));
        ref.current = rows;
        setSnippets(rows);
      } catch {
        /* leave empty */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  useEffect(
    () => () => {
      if (saveTimer.current) {
        clearTimeout(saveTimer.current);
        void save(ref.current);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  return {
    snippets,
    add: (text, section = null) => {
      const id = tempId();
      commit([...ref.current, { id, text, section }]);
      return id;
    },
    update: (id, text) => commit(ref.current.map((s) => (s.id === id ? { ...s, text } : s))),
    remove: (id) => commit(ref.current.filter((s) => s.id !== id)),
    setSection: (id, section) => commit(ref.current.map((s) => (s.id === id ? { ...s, section } : s))),
  };
}

// UNFILED is the sentinel value for the 未归类 option in the 归到 <select>
// (an empty option value maps to section=null).
const UNFILED = "__unfiled__";
const dedupe = (xs: string[]) => Array.from(new Set(xs));

// #5/#6 · the 片段 board, organized into foldable sections. A section is either
// an outline heading the student explicitly IMPORTED (see importedSectionsMemo
// — the live outline is never auto-rendered here) or an open 探索 线索 (both
// plain string labels); a snippet is filed under one via drag (a ⠿ handle onto
// a section header) or the 归到 <select>. The draft itself stays a plain
// textarea — this is organizing thinking material, not a structured document
// editor (铁律②).
function SnippetsPane({ snip, projectId, importedSections }: { snip: SnippetsHandle; projectId: string; importedSections: string[] }) {
  // 探索 线索 sections still auto-populate from the live exploration graph — only
  // the OUTLINE side was the unwanted auto-render (item #6); leads are unaffected.
  const [leadLabels, setLeadLabels] = useState<string[]>([]);
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [dragId, setDragId] = useState<string | null>(null);
  const [dropLabel, setDropLabel] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    void getExploration(projectId)
      .then((exploration) => {
        if (alive) setLeadLabels(dedupe(exploration.leads.filter((l) => l.status !== "pruned" && l.text.trim()).map((l) => l.text.trim())));
      })
      .catch(() => { /* leave last-good */ });
    return () => { alive = false; };
  }, [projectId]);

  const knownLabels = useMemo(() => dedupe([...importedSections, ...leadLabels]), [importedSections, leadLabels]);

  // Build ordered groups: imported outline headings, then leads, then any
  // orphaned section label still present on a snippet (renamed/deleted —
  // never vanish), then 未归类 last.
  const { groups, unfiled } = useMemo(() => {
    const bySection = new Map<string, Snip[]>();
    const un: Snip[] = [];
    for (const s of snip.snippets) {
      if (s.section == null) un.push(s);
      else { const arr = bySection.get(s.section) ?? []; arr.push(s); bySection.set(s.section, arr); }
    }
    const ordered: { label: string; kind: "outline" | "lead" | "orphan"; snips: Snip[] }[] = [];
    for (const l of importedSections) ordered.push({ label: l, kind: "outline", snips: bySection.get(l) ?? [] });
    for (const l of leadLabels) if (!importedSections.includes(l)) ordered.push({ label: l, kind: "lead", snips: bySection.get(l) ?? [] });
    for (const [label, snips] of bySection) if (!knownLabels.includes(label)) ordered.push({ label, kind: "orphan", snips });
    return { groups: ordered, unfiled: un };
  }, [snip.snippets, importedSections, leadLabels, knownLabels]);

  function dropOnto(label: string | null) {
    if (dragId) snip.setSection(dragId, label);
    setDragId(null);
    setDropLabel(null);
  }

  return (
    <div className="min-h-0 overflow-y-auto px-8 py-6">
      <div className="mx-auto max-w-2xl">
        <div className="mb-4">
          <h2 className="font-sans text-[18px] font-bold text-mk-ink">片段</h2>
          <p className="mt-1 text-[13px] text-mk-muted">攒下引文、笔记、灵光一现的句子——把它们归到大纲的章节或探索的线索下（拖 ⠿ 或用「归到」），写作时一目了然。从右侧「材料」也能一键收进来。</p>
        </div>
        <div className="flex flex-col gap-4">
          {groups.map((g) => (
            <SnippetSection
              key={`${g.kind}:${g.label}`}
              label={g.label}
              kind={g.kind}
              snips={g.snips}
              collapsed={collapsed.has(g.label)}
              isDropTarget={dropLabel === g.label}
              sectionOptions={knownLabels}
              onToggle={() => setCollapsed((c) => { const n = new Set(c); n.has(g.label) ? n.delete(g.label) : n.add(g.label); return n; })}
              onAdd={() => snip.add("", g.label)}
              onDragOverHead={() => setDropLabel(g.label)}
              onDropHead={() => dropOnto(g.label)}
              snip={snip}
              onDragStart={setDragId}
            />
          ))}
          {/* 未归类 — also the drop target for un-filing */}
          <SnippetSection
            label="未归类"
            kind="unfiled"
            snips={unfiled}
            collapsed={collapsed.has(UNFILED)}
            isDropTarget={dropLabel === UNFILED}
            sectionOptions={knownLabels}
            onToggle={() => setCollapsed((c) => { const n = new Set(c); n.has(UNFILED) ? n.delete(UNFILED) : n.add(UNFILED); return n; })}
            onAdd={() => snip.add("", null)}
            onDragOverHead={() => setDropLabel(UNFILED)}
            onDropHead={() => dropOnto(null)}
            snip={snip}
            onDragStart={setDragId}
          />
        </div>
      </div>
    </div>
  );
}

function SnippetSection({
  label, kind, snips, collapsed, isDropTarget, sectionOptions, onToggle, onAdd, onDragOverHead, onDropHead, snip, onDragStart,
}: {
  label: string;
  kind: "outline" | "lead" | "orphan" | "unfiled";
  snips: Snip[];
  collapsed: boolean;
  isDropTarget: boolean;
  sectionOptions: string[];
  onToggle: () => void;
  // Returns the newly-created snippet's id so the caller can focus it into
  // edit mode immediately (a student-initiated "+加片段" isn't the
  // programmatic-artifact case #9 wants defaulted to read mode).
  onAdd: () => string;
  onDragOverHead: () => void;
  onDropHead: () => void;
  snip: SnippetsHandle;
  onDragStart: (id: string | null) => void;
}) {
  const tag = kind === "outline" ? "章节" : kind === "lead" ? "线索" : kind === "orphan" ? "旧标签" : "";
  const tone = kind === "lead" ? "text-mk-green" : kind === "orphan" ? "text-mk-muted-2" : "text-mk-primary";
  // #9 · a snippet defaults to a READ view; double-click enters edit mode (a
  // taller textarea + a ✓ to leave it). Local to this section instance — a
  // freshly-programmatic snippet (AI rail / card compile / materials) is never
  // in this set, so it lands in READ mode as required.
  const [editingIds, setEditingIds] = useState<Set<string>>(new Set());
  function startEditing(id: string) {
    setEditingIds((ids) => { const n = new Set(ids); n.add(id); return n; });
  }
  function stopEditing(id: string) {
    setEditingIds((ids) => { if (!ids.has(id)) return ids; const n = new Set(ids); n.delete(id); return n; });
  }
  return (
    <section className={`rounded-mk-lg border ${isDropTarget ? "border-mk-primary bg-mk-primary-tint/30" : "border-mk-border-2 bg-mk-bg/30"}`}>
      <div
        onDragOver={(e) => { e.preventDefault(); onDragOverHead(); }}
        onDrop={(e) => { e.preventDefault(); onDropHead(); }}
        className="flex items-center gap-2 px-3 py-2"
      >
        <button type="button" onClick={onToggle} className="flex min-w-0 flex-1 items-center gap-1.5 text-left">
          <span className={`text-mk-muted-2 transition ${collapsed ? "" : "rotate-90"}`}>▸</span>
          {tag && <span className={`flex-none rounded-full bg-mk-surface px-1.5 py-0.5 text-[10px] font-bold ${tone}`}>{tag}</span>}
          <span className="min-w-0 flex-1 truncate text-[13.5px] font-bold text-mk-ink">{label}</span>
          <span className="flex-none text-[11.5px] font-semibold text-mk-muted-2">{snips.length}</span>
        </button>
      </div>
      {!collapsed && (
        <div className="flex flex-col gap-2 px-3 pb-3">
          {snips.map((s) => {
            const editing = editingIds.has(s.id);
            return (
            <div
              key={s.id}
              className="group rounded-mk border border-mk-border bg-mk-surface p-2.5 shadow-[0_1px_2px_rgba(28,35,51,0.04)]"
            >
              <div className="flex items-start gap-1.5">
                <span
                  draggable
                  onDragStart={() => onDragStart(s.id)}
                  onDragEnd={() => onDragStart(null)}
                  title="拖到某个章节/线索下"
                  className="mt-1 flex-none cursor-grab text-[13px] leading-none text-mk-muted-2 active:cursor-grabbing"
                >
                  ⠿
                </span>
                {editing ? (
                  <textarea
                    autoFocus
                    value={s.text}
                    onChange={(e) => snip.update(s.id, e.target.value)}
                    onBlur={() => stopEditing(s.id)}
                    rows={6}
                    placeholder="写下或粘贴一个片段……"
                    className="min-h-[7rem] w-full resize-y bg-transparent text-[13.5px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted-2"
                  />
                ) : (
                  <button
                    type="button"
                    onDoubleClick={() => startEditing(s.id)}
                    title="双击编辑"
                    className="w-full flex-1 cursor-text whitespace-pre-wrap break-words text-left text-[13.5px] leading-relaxed text-mk-ink"
                  >
                    {s.text.trim() ? s.text : <span className="text-mk-muted-2">写下或粘贴一个片段……（双击编辑）</span>}
                  </button>
                )}
                {editing && (
                  <button
                    type="button"
                    title="完成编辑"
                    onClick={() => stopEditing(s.id)}
                    className="mt-0.5 flex-none rounded px-1 text-[14px] font-bold leading-none text-mk-primary hover:text-mk-primary-hover"
                  >
                    ✓
                  </button>
                )}
              </div>
              <div className="mt-1 flex items-center justify-end gap-2">
                <label className="flex items-center gap-1 text-[11px] text-mk-muted-2">
                  归到
                  <select
                    value={s.section ?? UNFILED}
                    onChange={(e) => snip.setSection(s.id, e.target.value === UNFILED ? null : e.target.value)}
                    aria-label="把片段归到"
                    className="max-w-[10rem] rounded border border-mk-border bg-mk-surface px-1.5 py-0.5 text-[11.5px] text-mk-ink outline-none focus:border-mk-primary"
                  >
                    <option value={UNFILED}>未归类</option>
                    {/* keep a stale/orphan section selectable so its value shows */}
                    {s.section && !sectionOptions.includes(s.section) && <option value={s.section}>{s.section}</option>}
                    {sectionOptions.map((o) => <option key={o} value={o}>{o}</option>)}
                  </select>
                </label>
                <button type="button" onClick={() => snip.remove(s.id)} className="text-[12px] font-semibold text-mk-muted-2 opacity-0 transition hover:text-mk-accent group-hover:opacity-100">删除</button>
              </div>
            </div>
            );
          })}
          <button
            type="button"
            onClick={() => startEditing(onAdd())}
            className="rounded-mk border border-dashed border-mk-border py-2 text-[12.5px] font-semibold text-mk-muted-2 hover:border-mk-primary hover:text-mk-primary"
          >
            + 在此加片段
          </button>
        </div>
      )}
    </section>
  );
}

function Tab({ active, onClick, icon, children }: { active: boolean; onClick: () => void; icon: string; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex items-center gap-1.5 rounded-mk px-3.5 py-1.5 text-[13.5px] font-bold transition ${active ? "bg-mk-primary-tint text-mk-primary" : "text-mk-muted-2 hover:text-mk-ink"}`}
    >
      <Icon name={icon} size={15} /> {children}
    </button>
  );
}

/* ---------- 提纲 · outline ---------- */

function OutlinePane({ projectId, title }: { projectId: string; title: string }) {
  const [nodes, setNodes] = useState<Row[]>([]);
  const [view, setView] = useState<"list" | "map">("list");
  // nodesRef mirrors the latest committed rows so the debounced save (and the
  // id-reconcile after it resolves) reads current state without stale closures.
  const nodesRef = useRef<Row[]>([]);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Focus plumbing so typing flows like a real outliner: each row registers its
  // input by id, and a key action parks a pending focus target that the effect
  // applies once the new/changed rows have rendered.
  const inputRefs = useRef(new Map<string, HTMLInputElement>());
  const pendingFocus = useRef<{ id: string; atEnd: boolean } | null>(null);
  const registerInput = (id: string, el: HTMLInputElement | null) => {
    if (el) inputRefs.current.set(id, el);
    else inputRefs.current.delete(id);
  };

  // Persist the whole set. The server re-mints ids on every PUT, but the client
  // id is only a local React key — keep it STABLE across saves. Adopting the
  // server id here (the old behavior) changed key={n.id} on the mounted rows,
  // remounting the focused <input> ~700ms after every keystroke and dropping
  // focus/caret — the "Enter/Tab/mouse-move drops me out of editing" bug. So we
  // deliberately DON'T swap ids (mirrors useSnippets above). On failure keep
  // local; the next debounce retries.
  async function save(rows: Row[]) {
    try {
      await putOutline(projectId, rows.map((r) => ({ id: r.id, text: r.text, depth: r.depth })));
    } catch {
      /* keep local; the next debounced save retries */
    }
  }

  function scheduleSave(rows: Row[]) {
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => save(rows), 700);
  }

  // Single mutation entry point: update state + ref, then debounce a save. Every
  // edit (list or mind-map) flows through here, so the two views stay in sync.
  function commit(next: Row[]) {
    nodesRef.current = next;
    setNodes(next);
    scheduleSave(next);
  }

  // Load on mount. Empty outline → a single blank editable row (not yet saved;
  // it persists once the student types).
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const loaded = await getOutline(projectId);
        if (cancelled) return;
        const rows: Row[] = loaded.length
          ? loaded.map((n) => ({ id: n.id, text: n.text, depth: n.depth }))
          : [{ id: tempId(), text: "", depth: 0 }];
        nodesRef.current = rows;
        setNodes(rows);
      } catch {
        if (cancelled) return;
        const rows: Row[] = [{ id: tempId(), text: "", depth: 0 }];
        nodesRef.current = rows;
        setNodes(rows);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  // Flush a pending save on unmount so a last edit inside the debounce window
  // isn't lost when the student leaves the room.
  useEffect(
    () => () => {
      if (saveTimer.current) {
        clearTimeout(saveTimer.current);
        void save(nodesRef.current);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  const edit = (id: string, text: string) => commit(nodesRef.current.map((n) => (n.id === id ? { ...n, text } : n)));
  const bump = (id: string, dir: 1 | -1) => commit(nodesRef.current.map((n) => (n.id === id ? { ...n, depth: Math.max(0, Math.min(2, n.depth + dir)) } : n)));
  const remove = (id: string) => {
    const next = nodesRef.current.filter((n) => n.id !== id);
    // Never leave the outline with zero rows — keep one blank editable row.
    commit(next.length ? next : [{ id: tempId(), text: "", depth: 0 }]);
  };
  const addAfter = (id: string) => {
    const xs = nodesRef.current;
    const i = xs.findIndex((n) => n.id === id);
    const depth = xs[i]?.depth ?? 0;
    const nid = tempId();
    const next = [...xs];
    next.splice(i < 0 ? next.length : i + 1, 0, { id: nid, text: "", depth });
    pendingFocus.current = { id: nid, atEnd: false };
    commit(next);
  };

  // Add a child under a map node (or, for the synthetic root, a new top-level
  // row). We insert right after the parent with depth+1 so the flat→tree build
  // adopts it as that parent's child. Writes to the same `nodes` state, so the
  // list view sees it too.
  const addChild = (mapNodeId: string) => {
    const xs = nodesRef.current;
    const nid = tempId();
    if (mapNodeId === "root") {
      pendingFocus.current = { id: nid, atEnd: false };
      commit([{ id: nid, text: "", depth: 0 }, ...xs]);
      return;
    }
    const i = xs.findIndex((n) => n.id === mapNodeId);
    if (i < 0) return;
    const depth = Math.min(MAX_DEPTH, xs[i]!.depth + 1);
    const next = [...xs];
    next.splice(i + 1, 0, { id: nid, text: "", depth });
    pendingFocus.current = { id: nid, atEnd: false };
    commit(next);
  };

  // Translate a keydown on a row into an outline mutation. Returns true when it
  // handled the key (so the caller can preventDefault); false lets the browser
  // do its normal thing (typing, caret backspace inside text).
  const onRowKey = (id: string, e: React.KeyboardEvent<HTMLInputElement>): void => {
    let type: OutlineKeyType | null = null;
    if (e.key === "Enter") type = "enter";
    else if (e.key === "Tab") type = e.shiftKey ? "outdent" : "indent";
    else if (e.key === "Backspace") {
      const row = nodesRef.current.find((n) => n.id === id);
      // Only intercept backspace on an already-empty row; inside text the
      // default deletes a character.
      if (!row || row.text !== "") return;
      type = "backspace";
    }
    if (!type) return;
    e.preventDefault();
    const res = outlineKey(nodesRef.current, type, id);
    if (res.focus) pendingFocus.current = res.focus;
    commit(res.rows);
  };

  // Mind-map keyboard parity (#11): the map's inputs carried no keydown, so a
  // student couldn't grow the map without the mouse. XMind convention — Enter
  // adds a sibling after this node, Tab adds a child (depth+1). Both reuse the
  // add* helpers, which park pendingFocus so the new node's input takes focus
  // once rendered (the map inputs register into inputRefs too).
  const onNodeKey = (id: string, e: React.KeyboardEvent<HTMLInputElement>): void => {
    if (e.key === "Enter") { e.preventDefault(); addAfter(id); }
    else if (e.key === "Tab" && !e.shiftKey) { e.preventDefault(); addChild(id); }
  };

  // Apply a parked focus target after the rows it references have rendered.
  useEffect(() => {
    const pf = pendingFocus.current;
    if (!pf) return;
    const el = inputRefs.current.get(pf.id);
    if (!el) return;
    el.focus();
    if (pf.atEnd) {
      const len = el.value.length;
      el.setSelectionRange(len, len);
    }
    pendingFocus.current = null;
  }, [nodes]);

  return (
    <div className="flex min-h-0 flex-col">
      <div className="flex items-center justify-between px-8 pt-6 pb-3">
        <div>
          <h2 className="font-sans text-[19px] font-bold text-mk-ink">提纲</h2>
          <p className="mt-0.5 text-[13px] text-mk-muted">先把骨架搭出来。和印记聊聊哪里还站不住。</p>
        </div>
        <div className="flex rounded-mk border border-mk-border bg-mk-surface p-0.5">
          <ModeTab active={view === "list"} onClick={() => setView("list")}>大纲</ModeTab>
          <ModeTab active={view === "map"} onClick={() => setView("map")}>思维导图</ModeTab>
        </div>
      </div>

      {view === "list" ? (
        <div className="min-h-0 flex-1 overflow-y-auto px-8 pb-7">
          <div className="mx-auto max-w-2xl">
            <div className="flex flex-col">
              {nodes.map((n) => (
                <OutlineRow
                  key={n.id}
                  node={n}
                  registerInput={(el) => registerInput(n.id, el)}
                  onKey={(e) => onRowKey(n.id, e)}
                  onEdit={(t) => edit(n.id, t)}
                  onIndent={() => bump(n.id, 1)}
                  onOutdent={() => bump(n.id, -1)}
                  onAdd={() => addAfter(n.id)}
                  onRemove={() => remove(n.id)}
                />
              ))}
            </div>
            <button type="button" onClick={() => addAfter(nodes[nodes.length - 1]?.id ?? "")} className="mt-2 rounded-mk border border-dashed border-mk-border px-3 py-2 text-[13px] font-semibold text-mk-muted-2 hover:border-mk-primary hover:text-mk-primary">
              + 新增一条
            </button>
          </div>
        </div>
      ) : (
        <MindMap nodes={nodes} title={title} onEdit={edit} onAddChild={addChild} registerInput={registerInput} onNodeKey={onNodeKey} />
      )}
    </div>
  );
}

/* ----- mind map view (same outline data, laid out as a tree) ----- */

type MapNode = { id: string; text: string; depth: number; children: MapNode[]; row: number; cx: number; cy: number };

const COL = 250;
const ROW = 56;
const NODE_W = 200;

function MindMap({ nodes, title, onEdit, onAddChild, registerInput, onNodeKey }: { nodes: Row[]; title: string; onEdit: (id: string, t: string) => void; onAddChild: (id: string) => void; registerInput: (id: string, el: HTMLInputElement | null) => void; onNodeKey: (id: string, e: React.KeyboardEvent<HTMLInputElement>) => void }) {
  // Build a tree from the flat depth list, with the project title as the root.
  const root: MapNode = { id: "root", text: title || "未命名项目", depth: -1, children: [], row: 0, cx: 0, cy: 0 };
  const lastAtDepth: Record<number, MapNode> = { [-1]: root };
  for (const n of nodes) {
    const node: MapNode = { id: n.id, text: n.text, depth: n.depth, children: [], row: 0, cx: 0, cy: 0 };
    const parent = lastAtDepth[n.depth - 1] ?? root;
    parent.children.push(node);
    lastAtDepth[n.depth] = node;
    Object.keys(lastAtDepth).forEach((k) => { if (Number(k) > n.depth) delete lastAtDepth[Number(k)]; });
  }

  // Assign a row to every leaf; internal nodes centre on their children.
  let slot = 0;
  let maxDepth = 0;
  const layout = (node: MapNode) => {
    maxDepth = Math.max(maxDepth, node.depth);
    if (node.children.length === 0) node.row = slot++;
    else { node.children.forEach(layout); node.row = (node.children[0]!.row + node.children[node.children.length - 1]!.row) / 2; }
    node.cx = (node.depth + 1) * COL;
    node.cy = node.row * ROW + ROW / 2 + 20;
  };
  layout(root);

  const flat: MapNode[] = [];
  const collect = (n: MapNode) => { flat.push(n); n.children.forEach(collect); };
  collect(root);

  const width = (maxDepth + 2) * COL + 40;
  const height = slot * ROW + 60;

  const tone = (d: number) =>
    d < 0 ? "bg-mk-primary text-white border-mk-primary"
      : d === 0 ? "bg-mk-primary-tint text-mk-primary border-mk-primary/30"
        : d === 1 ? "bg-mk-accent-tint text-mk-accent border-mk-accent/30"
          : "bg-mk-green-tint text-mk-green border-mk-green/30";

  return (
    <div className="min-h-0 flex-1 overflow-auto px-8 pb-8">
      <div className="relative" style={{ width, height }}>
        <svg className="absolute inset-0" width={width} height={height} style={{ pointerEvents: "none" }}>
          {flat.flatMap((p) =>
            p.children.map((c) => {
              const x1 = p.cx + NODE_W, y1 = p.cy, x2 = c.cx, y2 = c.cy;
              return <path key={`${p.id}-${c.id}`} d={`M ${x1} ${y1} C ${x1 + 46} ${y1}, ${x2 - 46} ${y2}, ${x2} ${y2}`} fill="none" stroke="#D4D9E6" strokeWidth={1.6} />;
            }),
          )}
        </svg>
        {flat.map((n) => (
          <div
            key={n.id}
            className={`group absolute flex items-center rounded-mk border px-3 shadow-[0_1px_3px_rgba(28,35,51,0.06)] ${tone(n.depth)}`}
            style={{ left: n.cx, top: n.cy - 18, width: NODE_W, height: 36 }}
          >
            {n.depth < 0 ? (
              <span className="truncate text-[13px] font-bold">{n.text}</span>
            ) : (
              <input
                ref={(el) => registerInput(n.id, el)}
                value={n.text}
                onChange={(e) => onEdit(n.id, e.target.value)}
                onKeyDown={(e) => onNodeKey(n.id, e)}
                placeholder="写一条……（回车加同级、Tab 加子节点）"
                className={`w-full truncate bg-transparent text-[12.5px] outline-none placeholder:opacity-60 ${n.depth === 0 ? "font-bold" : "font-semibold"}`}
              />
            )}
            {/* Add a child under this node (depth clamps ≤ MAX_DEPTH). Hidden
                until hover so the map stays calm; sits just off the right edge. */}
            {n.depth < MAX_DEPTH && (
              <button
                type="button"
                title="加一个子节点"
                onClick={() => onAddChild(n.id)}
                className="absolute -right-3 top-1/2 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-full border border-mk-border bg-mk-surface text-[15px] font-bold leading-none text-mk-muted-2 opacity-0 shadow-sm transition hover:border-mk-primary hover:text-mk-primary group-hover:opacity-100"
              >
                +
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function OutlineRow({ node, registerInput, onKey, onEdit, onIndent, onOutdent, onAdd, onRemove }: { node: Row; registerInput: (el: HTMLInputElement | null) => void; onKey: (e: React.KeyboardEvent<HTMLInputElement>) => void; onEdit: (t: string) => void; onIndent: () => void; onOutdent: () => void; onAdd: () => void; onRemove: () => void }) {
  const dot = node.depth === 0 ? "bg-mk-primary" : node.depth === 1 ? "bg-mk-accent" : "bg-mk-green";
  return (
    <div className="group flex items-center gap-2 rounded-mk py-1 hover:bg-mk-bg/60" style={{ paddingLeft: node.depth * 26 }}>
      <span className={`h-1.5 w-1.5 flex-none rounded-full ${dot}`} />
      <input
        ref={registerInput}
        value={node.text}
        onChange={(e) => onEdit(e.target.value)}
        onKeyDown={onKey}
        placeholder="写一条……（回车换行、Tab 缩进）"
        className={`min-w-0 flex-1 rounded bg-transparent px-1.5 py-1 text-mk-ink outline-none transition placeholder:text-mk-muted-2 focus:bg-mk-input-bg ${node.depth === 0 ? "text-[14.5px] font-bold" : "text-[13.5px]"}`}
      />
      <div className="flex flex-none items-center gap-0.5 opacity-0 transition group-hover:opacity-100">
        <IconBtn onClick={onOutdent} title="升级">←</IconBtn>
        <IconBtn onClick={onIndent} title="缩进">→</IconBtn>
        <IconBtn onClick={onAdd} title="下面加一条">+</IconBtn>
        <IconBtn onClick={onRemove} title="删除">×</IconBtn>
      </div>
    </div>
  );
}

function IconBtn({ onClick, title, children }: { onClick: () => void; title: string; children: React.ReactNode }) {
  return (
    <button type="button" onClick={onClick} title={title} className="flex h-6 w-6 items-center justify-center rounded text-[13px] font-bold text-mk-muted-2 hover:bg-mk-surface hover:text-mk-primary">
      {children}
    </button>
  );
}

/* ---------- 写作 · single draft panel ---------- */

// The extensions we can read client-side as plain text. Binary formats
// (.docx/.pdf) are accepted but parked with a note — real parsing is later.
const TEXT_EXT = [".md", ".txt", ".markdown"];

// #6 · the 整稿体检 lenses, each with a one-line plain-language description so a
// student knows what a voice does before picking it (shown inline in the
// dropdown, and echoed under the selector). The voice only changes the coach's
// tone/lens on the server — never the rubric or the "never rewrite" rule.
const VOICE_META: Record<ReviewVoice, { label: string; desc: string }> = {
  board: { label: "评审团", desc: "像考官那样全面权衡" },
  sceptic: { label: "质疑者", desc: "专挑论证漏洞与反例" },
  layperson: { label: "门外汉", desc: "用常识追问你没交代的" },
  executioner: { label: "审判者", desc: "只看能不能站得住" },
};
const VOICE_ORDER: ReviewVoice[] = ["board", "sceptic", "layperson", "executioner"];

// #10 · classify a failed 体检 so the student sees an honest, distinguishing
// message instead of one blanket "体检没跑完" string. `review_rejected` is the
// server telling us it ran the review and threw the output away (enforcement
// / unparseable model reply) — a DIFFERENT situation from a transient network
// hiccup, so it gets its own copy. Everything else (network drop, timeout,
// unexpected HTTP failure) reads as transient.
function classifyReviewError(err: unknown): string {
  if (err instanceof ApiError && err.code === "review_rejected") {
    return "这次体检没通过内部校验，请再试一次。";
  }
  return "体检没跑完（可能是网络不稳定），稍后再试一次。";
}

// paragraphAtCaret returns the blank-line-separated paragraph the caret sits in
// (trimmed). Bounds are computed from the ACTUAL separators (a blank-line gap is
// 2..n chars), so it never drifts; a caret in a gap attaches to the following
// block; empty text → "". Exported for direct unit testing (WC · M1).
export function paragraphAtCaret(src: string, caret: number): string {
  const bounds: Array<[number, number]> = [];
  const re = /\n{2,}/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(src)) !== null) {
    bounds.push([last, m.index]);
    last = m.index + m[0].length;
  }
  bounds.push([last, src.length]);
  for (const [s, e] of bounds) {
    if (caret <= e) return src.slice(s, e).trim();
  }
  const [s] = bounds[bounds.length - 1]!;
  return src.slice(s).trim();
}

function DraftPane({
  projectId,
  title,
  locked,
  onFocusPart,
  registerInsert,
  pendingReview,
  onPendingReviewHandled,
}: {
  projectId: string;
  title: string;
  locked: boolean;
  onFocusPart: (part: string) => void;
  registerInsert: (fn: ((t: string) => void) | null) => void;
  // #8-second · a voice-scoped 体检 requested from the persistent rail shelf
  // (a tab-sibling of this pane) — queued here rather than called directly so
  // it's never lost to the mount race when the request also switches the tab
  // to 正文 (this pane's own mount-effect below flushes it once mounted).
  pendingReview: { scope?: string; voice: ReviewVoice } | null;
  onPendingReviewHandled: () => void;
}) {
  const [mode, setMode] = useState<"write" | "upload">("write");
  const [pane, setPane] = useState<"edit" | "preview">("edit");
  // #5-follow-on · 自由 (one free textarea) vs 分节 (write under outline-driven
  // headings). A toggle — free-form is always available (never a cage).
  const [layout, setLayout] = useState<"free" | "sections">("free");
  // When 分节 is active, the sections editor registers its own insert here so
  // the materials sidebar drops a fragment into the focused section.
  const sectionInsertRef = useRef<((t: string) => void) | null>(null);
  const [text, setText] = useState("");
  const [uploadNote, setUploadNote] = useState<string | null>(null);
  // #7 · a floating "问印记" chip that appears next to a text selection. selPop
  // carries the selected text + where to float the chip (coords relative to the
  // editor pane). Clicking it pins that part into the coach composer.
  const [selPop, setSelPop] = useState<{ x: number; y: number; text: string } | null>(null);
  const fileInput = useRef<HTMLInputElement | null>(null);
  const draftRef = useRef<HTMLTextAreaElement | null>(null);
  const paneRef = useRef<HTMLDivElement | null>(null);
  // #9 · textRef mirrors the latest draft so the sidebar's insert (registered
  // once, below) reads current content without a stale closure. hasFocusedRef
  // tracks whether the caret is meaningful yet (review L3).
  const textRef = useRef("");
  const hasFocusedRef = useRef(false);

  // #7 · on mouse-up in the draft, if there's a highlighted range, float the
  // "问印记" chip just above the pointer. No selection (or a locked draft) hides
  // it. Coordinates are relative to the editor pane so the chip tracks the text.
  function onDraftMouseUp(e: React.MouseEvent<HTMLTextAreaElement>) {
    if (locked) return;
    const ta = e.currentTarget;
    const part = ta.value.slice(ta.selectionStart, ta.selectionEnd).trim();
    const host = paneRef.current;
    if (part && host) {
      const rect = host.getBoundingClientRect();
      setSelPop({ x: e.clientX - rect.left, y: e.clientY - rect.top, text: part });
    } else {
      setSelPop(null);
    }
  }
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const words = text.replace(/\s+/g, "").length;

  // WA · 整稿体检: save a version + run the whole-draft review, render its advice
  // read-only. Never edits the draft — 印记 checks argument/structure, you revise.
  const [voice, setVoice] = useState<ReviewVoice>("board");
  const [reviewing, setReviewing] = useState(false);
  const [review, setReview] = useState<DraftReviewResult | null>(null);
  const [reviewError, setReviewError] = useState<string | null>(null);
  // #8 · the 体检 voices work on the whole draft OR just a selected paragraph —
  // scope drives the panel's wording so it's clear what got checked.
  const [reviewScope, setReviewScope] = useState<"draft" | "part">("draft");

  // Run the chosen voice's review. No arg → whole draft; a scopeText → just that
  // paragraph (from the selection chip, or the shelf's 这段 button). Never
  // rewrites — 印记 checks argument and structure, the student revises in her
  // own words. `voiceOverride` lets the persistent rail shelf pick a voice
  // directly (its per-voice buttons) without going through the toolbar <select>
  // first — it also updates that <select> so the two stay in sync.
  async function runReview(scopeText?: string, voiceOverride?: ReviewVoice) {
    const useVoice = voiceOverride ?? voice;
    if (voiceOverride && voiceOverride !== voice) setVoice(voiceOverride);
    const target = scopeText ?? text;
    if (reviewing || locked || target.trim() === "") return;
    setReviewing(true);
    setReviewError(null);
    setReviewScope(scopeText ? "part" : "draft");
    // flush any pending autosave so the persisted buffer matches what's on screen
    if (saveTimer.current) clearTimeout(saveTimer.current);
    try {
      await putBuffer(projectId, text);
      try {
        setReview(await runDraftReview(projectId, target, useVoice));
      } catch (err) {
        // #10 · one retry, but only for a TRANSIENT failure. `review_rejected`
        // means the server already ran the review and threw the output away
        // (its own retry included, see agent.ProposeReview) — retrying the
        // identical content client-side would just resend the same rejected
        // request, so that one surfaces immediately instead.
        if (err instanceof ApiError && err.code === "review_rejected") throw err;
        setReview(await runDraftReview(projectId, target, useVoice));
      }
    } catch (err) {
      setReviewError(classifyReviewError(err));
    } finally {
      setReviewing(false);
    }
  }

  // #8-second · flush a review requested from the persistent rail shelf. A
  // whole-draft request needs `text` loaded first — the request may have also
  // just switched the tab here, remounting this pane fresh, so the effect
  // below hasn't populated `text` yet. Wait for draftReady in that case; a
  // paragraph-scoped request carries its own text and can run immediately.
  const [draftReady, setDraftReady] = useState(false);
  useEffect(() => {
    if (!pendingReview) return;
    if (!pendingReview.scope && !draftReady) return;
    void runReview(pendingReview.scope, pendingReview.voice);
    onPendingReviewHandled();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingReview, draftReady]);

  // Load the persisted draft on mount ("" when there's no buffer yet).
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const content = await getDraft(projectId);
        if (!cancelled) { setText(content); textRef.current = content; }
      } catch {
        /* leave empty; the placeholder shows */
      } finally {
        if (!cancelled) setDraftReady(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  // Flush a pending autosave on unmount so a last keystroke isn't lost.
  useEffect(
    () => () => {
      if (saveTimer.current) clearTimeout(saveTimer.current);
    },
    [],
  );

  function onChange(next: string) {
    setText(next);
    textRef.current = next;
    setSelPop(null); // any edit invalidates the floating selection chip
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      void putBuffer(projectId, next).catch(() => {/* retries on next keystroke */});
    }, 800);
  }

  async function handleFile(file: File) {
    const name = file.name.toLowerCase();
    if (TEXT_EXT.some((ext) => name.endsWith(ext))) {
      const content = await file.text();
      setText(content);
      textRef.current = content;
      setUploadNote(null);
      setMode("write");
      void putBuffer(projectId, content).catch(() => {/* retries via next edit */});
    } else {
      // .docx / .pdf and friends — accepted but not parsed yet. Don't crash;
      // just tell the student we've noted it.
      setUploadNote(`已上传「${file.name}」，正文解析稍后支持。`);
    }
  }

  // #9 · insert a fragment (from the materials sidebar) into the draft at the
  // caret — 印记 never authors, the STUDENT places her own material. Reads the
  // live text/caret from refs; separates with blank lines; no-op when locked.
  function insertAtCaret(t: string) {
    if (locked) return;
    const frag = t.trim();
    if (!frag) return;
    setMode("write");
    setPane("edit");
    // #5 · in 分节 mode the sections editor owns placement (into the focused
    // section); the free textarea path below only runs in 自由 mode.
    if (layout === "sections" && sectionInsertRef.current) {
      sectionInsertRef.current(frag);
      return;
    }
    const cur = textRef.current;
    const ta = draftRef.current;
    // honor the caret only once the textarea has been focused — a never-focused
    // textarea reports selectionStart 0, which would insert above the intro; in
    // that case append at the end instead (review L3).
    const start = ta && hasFocusedRef.current ? ta.selectionStart : cur.length;
    const end = ta && hasFocusedRef.current ? ta.selectionEnd : cur.length;
    const before = cur.slice(0, start);
    const after = cur.slice(end);
    const lead = before && !before.endsWith("\n") ? "\n\n" : "";
    const tail = after && !after.startsWith("\n") ? "\n\n" : "";
    const next = before + lead + frag + tail + after;
    onChange(next);
    const caret = (before + lead + frag).length;
    requestAnimationFrame(() => {
      const el = draftRef.current;
      if (el) { el.focus(); el.setSelectionRange(caret, caret); }
    });
  }
  // Register the inserter once; a ref holds the latest closure so the stable
  // registered fn always sees current state. Unregister on unmount so the
  // sidebar's insert no-ops when the draft tab isn't mounted.
  const insertRef = useRef<(t: string) => void>(() => {});
  insertRef.current = insertAtCaret;
  useEffect(() => {
    registerInsert((t) => insertRef.current(t));
    return () => registerInsert(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="flex min-h-0 flex-col px-8 py-6">
      <div ref={paneRef} className="relative mx-auto flex min-h-0 w-full max-w-2xl flex-1 flex-col">
        <div className="mb-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            {!locked && (
              <div className="flex rounded-mk border border-mk-border bg-mk-surface p-0.5">
                <ModeTab active={mode === "write"} onClick={() => setMode("write")}>写在这里</ModeTab>
                <ModeTab active={mode === "upload"} onClick={() => setMode("upload")}>我在别处写了</ModeTab>
              </div>
            )}
            {mode === "write" && (
              <div className="flex rounded-mk border border-mk-border bg-mk-surface p-0.5">
                <SubTab active={pane === "edit"} onClick={() => setPane("edit")}>写</SubTab>
                <SubTab active={pane === "preview"} onClick={() => { setSelPop(null); setPane("preview"); }}>预览</SubTab>
              </div>
            )}
            {mode === "write" && pane === "edit" && (
              <div className="flex rounded-mk border border-mk-border bg-mk-surface p-0.5">
                <SubTab active={layout === "free"} onClick={() => setLayout("free")}>自由</SubTab>
                <SubTab active={layout === "sections"} onClick={() => setLayout("sections")}>分节</SubTab>
              </div>
            )}
          </div>
          {mode === "write" && (
            <div className="flex items-center gap-2">
              <span className="text-[12px] font-semibold text-mk-muted-2">{words} 字</span>
              {!locked && (
                <>
                  {/* #6 · 体检视角: each option carries a plain-language description so
                      the lens is legible before picking; the closed control shows the
                      chosen lens + what it does. */}
                  <span className="text-[12px] font-semibold text-mk-muted-2">视角</span>
                  <select
                    value={voice}
                    onChange={(e) => setVoice(e.target.value as ReviewVoice)}
                    aria-label="体检视角"
                    title="换个视角，印记体检整稿的侧重就不同"
                    className="max-w-[13rem] rounded-mk border border-mk-border bg-mk-surface px-2 py-1 text-[12px] text-mk-ink outline-none focus:border-mk-primary"
                  >
                    {VOICE_ORDER.map((v) => (
                      <option key={v} value={v}>{VOICE_META[v].label} · {VOICE_META[v].desc}</option>
                    ))}
                  </select>
                  <button
                    type="button"
                    onClick={() => void runReview()}
                    disabled={reviewing || text.trim() === ""}
                    className="rounded-mk bg-mk-accent px-3 py-1.5 text-[12.5px] font-bold text-white transition hover:bg-mk-accent-hover disabled:opacity-50"
                  >
                    {reviewing ? "体检中…" : "让印记体检整稿"}
                  </button>
                </>
              )}
              <button
                type="button"
                onClick={() => { void exportDraftDocx(text, { title }).catch(() => {/* never crash the room */}); }}
                disabled={text.trim() === ""}
                className="rounded-mk border border-mk-border px-3 py-1.5 text-[12.5px] font-semibold text-mk-muted hover:text-mk-primary disabled:opacity-50"
              >
                导出成品 .docx
              </button>
            </div>
          )}
        </div>
        {locked && (
          <p className="mb-3 rounded-mk border border-mk-border bg-mk-bg px-3 py-2 text-[12.5px] font-semibold text-mk-muted-2">这篇已归档，正文只读——你仍可预览与导出。</p>
        )}

        {mode === "write" ? (
          pane === "edit" ? (
            layout === "sections" ? (
              <SectionedDraft
                projectId={projectId}
                text={text}
                onChange={onChange}
                locked={locked}
                registerInsert={(fn) => { sectionInsertRef.current = fn; }}
              />
            ) : (
              <textarea
                ref={draftRef}
                value={text}
                onChange={(e) => onChange(e.target.value)}
                onFocus={() => { hasFocusedRef.current = true; }}
                onMouseUp={onDraftMouseUp}
                onScroll={() => setSelPop(null)}
                readOnly={locked}
                placeholder="在这里写你的草稿……（支持 Markdown）"
                className={`min-h-0 flex-1 resize-none rounded-mk-lg border border-mk-border p-5 font-sans text-[14.5px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted-2 ${locked ? "bg-mk-bg/60 cursor-default" : "bg-mk-surface focus:border-mk-primary"}`}
              />
            )
          ) : (
            <MarkdownPreview text={text} />
          )
        ) : (
          <div
            className="flex min-h-0 flex-1 flex-col items-center justify-center rounded-mk-lg border-2 border-dashed border-mk-input bg-mk-input-bg/50 px-6 text-center"
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
              e.preventDefault();
              const f = e.dataTransfer.files[0];
              if (f) void handleFile(f);
            }}
          >
            <span className="text-mk-primary"><Icon name="writing" size={28} /></span>
            <p className="mt-3 text-[15px] font-bold text-mk-ink">把你写好的文档拖进来</p>
            <p className="mt-1 text-[13px] text-mk-muted-2">Word / PDF / Markdown——印记读进来后，也能和你聊这一稿</p>
            <input
              ref={fileInput}
              type="file"
              accept=".md,.markdown,.txt,.docx,.pdf"
              className="hidden"
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) void handleFile(f);
                e.target.value = "";
              }}
            />
            <button type="button" onClick={() => fileInput.current?.click()} className="mt-4 rounded-mk bg-mk-primary px-4 py-2 text-[13px] font-bold text-white hover:bg-mk-primary-hover">选择文件</button>
            {uploadNote && <p className="mt-3 text-[12.5px] font-semibold text-mk-accent">{uploadNote}</p>}
          </div>
        )}
        {/* #7 · floating "问印记" chip — appears next to a text selection; clicking
            it pins that part into the coach composer (replaces the old top button).
            onMouseDown preventDefault keeps the textarea selection alive through the
            click, so we still have the pinned text. */}
        {selPop && mode === "write" && pane === "edit" && !locked && (
          <div
            // sit above the pointer, but flip below when the selection is near the
            // pane top so the chip never clips over the toolbar (review L3).
            style={{ left: selPop.x, top: selPop.y, transform: selPop.y < 44 ? "translate(-50%, 45%)" : "translate(-50%, -130%)" }}
            className="absolute z-20 flex items-center gap-1 whitespace-nowrap rounded-full bg-mk-primary p-1 shadow-[0_4px_14px_rgba(28,35,51,0.25)]"
            onMouseDown={(e) => e.preventDefault()}
          >
            {/* #7 · chat about this part */}
            <button
              type="button"
              onClick={() => { onFocusPart(selPop.text); setSelPop(null); }}
              className="flex items-center gap-1 rounded-full px-2.5 py-1 text-[12px] font-bold text-white hover:bg-white/15"
            >
              <Icon name="spark" size={13} /> 问印记
            </button>
            <span className="h-3.5 w-px bg-white/30" />
            {/* #8 · run the chosen voice's 体检 on just this paragraph */}
            <button
              type="button"
              onClick={() => { const t = selPop.text; setSelPop(null); void runReview(t); }}
              className="rounded-full px-2.5 py-1 text-[12px] font-bold text-white hover:bg-white/15"
            >
              体检这段
            </button>
          </div>
        )}
        {mode === "write" && (reviewError || reviewing || review) && (
          <DraftReviewPanel scope={reviewScope} reviewing={reviewing} error={reviewError} review={review} onClose={() => { setReview(null); setReviewError(null); }} />
        )}
        <p className="mt-2 text-center text-[11.5px] text-mk-muted-2">你写，印记只在一旁陪你想——它不替你写正文。</p>
      </div>
    </div>
  );
}

// #5 · the 分节 draft editor: write under outline-driven headings. A VIEW over
// the same Markdown draft string (parses in, serializes out via onChange), so
// 体检 / 导出 / 预览 all consume the draft unchanged. Free-form text is one
// toggle away — this organizes the student's structure, it never authors or
// lays out her deliverable (铁律②).
function SectionedDraft({
  projectId,
  text,
  onChange,
  locked,
  registerInsert,
}: {
  projectId: string;
  text: string;
  onChange: (next: string) => void;
  locked: boolean;
  registerInsert: (fn: ((t: string) => void) | null) => void;
}) {
  const [sections, setSections] = useState<DraftSection[]>(() => parseSections(text));
  const [outlineHeads, setOutlineHeads] = useState<string[]>([]);
  const [focusId, setFocusId] = useState<string | null>(null);
  const sectionsRef = useRef(sections);
  sectionsRef.current = sections;

  useEffect(() => {
    let alive = true;
    getOutline(projectId)
      .then((nodes) => { if (alive) setOutlineHeads(nodes.filter((n) => n.depth === 0 && n.text.trim()).map((n) => n.text.trim())); })
      .catch(() => {/* leave empty — the generate button just won't show */});
    return () => { alive = false; };
  }, [projectId]);

  function commit(next: DraftSection[]) {
    setSections(next);
    onChange(serializeSections(next));
  }
  const setHeading = (id: string, heading: string) => commit(sectionsRef.current.map((s) => (s.id === id ? { ...s, heading } : s)));
  const setBody = (id: string, body: string) => commit(sectionsRef.current.map((s) => (s.id === id ? { ...s, body } : s)));
  const removeSection = (id: string) => commit(sectionsRef.current.filter((s) => s.id !== id));
  const addSection = () => { const s = newSection(); commit([...sectionsRef.current, s]); setFocusId(s.id); };
  const generate = () => { const gen = sectionsFromOutline(outlineHeads, sectionsRef.current); if (gen.length) commit([...sectionsRef.current, ...gen]); };

  // Insert a fragment (materials sidebar) into the focused section's body — else
  // the last section, else a new intro when there are none yet.
  function insert(t: string) {
    if (locked) return;
    const frag = t.trim();
    if (!frag) return;
    const cur = sectionsRef.current;
    const target = (focusId && cur.some((s) => s.id === focusId) ? focusId : cur[cur.length - 1]?.id) ?? null;
    if (!target) { commit([{ ...newSection(0), body: frag }]); return; }
    commit(cur.map((s) => (s.id === target ? { ...s, body: s.body ? `${s.body}\n\n${frag}` : frag } : s)));
  }
  const insertRef = useRef(insert);
  insertRef.current = insert;
  useEffect(() => {
    registerInsert((t) => insertRef.current(t));
    return () => registerInsert(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="min-h-0 flex-1 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface p-4">
      {!locked && (
        <div className="mb-3 flex flex-wrap items-center gap-2">
          {outlineHeads.length > 0 && (
            <button type="button" onClick={generate} className="rounded-mk border border-mk-primary/40 px-2.5 py-1 text-[12px] font-bold text-mk-primary hover:bg-mk-primary-tint">＋ 从大纲生成章节</button>
          )}
          <span className="text-[11.5px] text-mk-muted-2">在小标题下写；从右侧「材料」插入会落到你正在写的这一节。</span>
        </div>
      )}
      {sections.length === 0 ? (
        <div className="flex flex-col items-center gap-2 py-10 text-center">
          <p className="text-[13px] text-mk-muted-2">还没有章节。{outlineHeads.length > 0 ? "用大纲生成，或" : ""}加一节，在标题下写。</p>
          {!locked && <button type="button" onClick={addSection} className="rounded-mk bg-mk-primary px-3 py-1.5 text-[12.5px] font-bold text-white hover:bg-mk-primary-hover">＋ 加一节</button>}
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {sections.map((s) => (
            <section key={s.id} className="group rounded-mk-lg border border-mk-border-2 bg-mk-bg/20 p-3">
              {s.level > 0 ? (
                <input
                  value={s.heading}
                  onChange={(e) => setHeading(s.id, e.target.value)}
                  onFocus={() => setFocusId(s.id)}
                  readOnly={locked}
                  placeholder="小标题……"
                  className={`w-full bg-transparent font-sans font-bold text-mk-ink outline-none placeholder:text-mk-muted-2 ${s.level === 1 ? "text-[16px]" : "text-[14px]"}`}
                />
              ) : (
                <div className="mb-1 text-[11px] font-bold uppercase tracking-wide text-mk-muted-2">开头（无标题）</div>
              )}
              <textarea
                value={s.body}
                onChange={(e) => setBody(s.id, e.target.value)}
                onFocus={() => setFocusId(s.id)}
                readOnly={locked}
                rows={4}
                placeholder="在这一节写……"
                className="mt-1.5 w-full resize-y bg-transparent font-sans text-[14px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted-2"
              />
              {!locked && (
                <div className="mt-1 flex justify-end">
                  <button type="button" onClick={() => removeSection(s.id)} className="text-[11.5px] font-semibold text-mk-muted-2 opacity-0 transition hover:text-mk-accent group-hover:opacity-100">删除本节</button>
                </div>
              )}
            </section>
          ))}
          {!locked && (
            <button type="button" onClick={addSection} className="rounded-mk border border-dashed border-mk-border py-2 text-[12.5px] font-semibold text-mk-muted-2 hover:border-mk-primary hover:text-mk-primary">＋ 加一节</button>
          )}
        </div>
      )}
    </div>
  );
}

// DraftReviewPanel — WA: renders the 整稿体检 advice read-only. Each row names a
// criterion, which descriptor band the draft evidences, what's still missing,
// and a suggested DIRECTION (not a rewrite). The student revises in her own words.
function DraftReviewPanel({
  scope,
  reviewing,
  error,
  review,
  onClose,
}: {
  scope: "draft" | "part";
  reviewing: boolean;
  error: string | null;
  review: DraftReviewResult | null;
  onClose: () => void;
}) {
  return (
    <div className="mt-3 max-h-72 overflow-y-auto rounded-mk-lg border border-mk-accent/40 bg-mk-accent-tint/30 p-4">
      <div className="mb-2 flex items-center justify-between">
        <p className="text-[13px] font-bold text-mk-ink">{scope === "part" ? "印记体检了你选中的这一段" : "印记的整稿体检"} · 供你参考，不替你改字</p>
        <button type="button" onClick={onClose} className="text-[12px] font-semibold text-mk-muted-2 hover:text-mk-muted">收起</button>
      </div>
      {reviewing ? (
        <p className="text-[12.5px] text-mk-muted-2">印记正在逐段体检你的论证与结构……</p>
      ) : error ? (
        <p className="text-[12.5px] font-semibold text-mk-accent">{error}</p>
      ) : review ? (
        review.items.length === 0 ? (
          <p className="text-[12.5px] text-mk-muted-2">这一稿没跑出具体条目——可能正文还太短，先多写一点再体检。</p>
        ) : (
          <>
            <p className="mb-2 text-[11.5px] text-mk-muted-2">已存一版（{review.wordCount} 字{review.inBand ? " · 在字数区间内" : " · 字数偏离区间"}）。</p>
            <ul className="flex flex-col gap-2">
              {review.items.map((it: ReviewItem, i: number) => (
                <li key={i} className="rounded-mk border border-mk-border bg-mk-surface p-3">
                  <div className="flex items-baseline gap-2">
                    <span className="text-[12.5px] font-bold text-mk-primary">{it.criterion_name || it.criterion_code}</span>
                    {it.band && <span className="rounded-full bg-mk-primary-tint px-2 py-0.5 text-[10.5px] font-bold text-mk-primary">{it.band}</span>}
                  </div>
                  {it.evidence && <p className="mt-1 text-[12.5px] text-mk-ink"><span className="font-semibold">现在做到：</span>{it.evidence}</p>}
                  {it.missing && <p className="mt-1 text-[12.5px] text-mk-muted"><span className="font-semibold">还差：</span>{it.missing}</p>}
                  {it.fix && <p className="mt-1 text-[12.5px] text-mk-accent"><span className="font-semibold">可以往哪想：</span>{it.fix}</p>}
                </li>
              ))}
            </ul>
          </>
        )
      ) : null}
    </div>
  );
}

function ModeTab({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button type="button" onClick={onClick} className={`rounded-[10px] px-3 py-1.5 text-[12.5px] font-bold transition ${active ? "bg-mk-primary text-white" : "text-mk-muted-2 hover:text-mk-muted"}`}>
      {children}
    </button>
  );
}

// A lighter segmented control for the 写/预览 switch — a tinted active state so
// it reads as secondary to the write/upload tabs beside it.
function SubTab({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button type="button" onClick={onClick} className={`rounded-[10px] px-3 py-1.5 text-[12.5px] font-bold transition ${active ? "bg-mk-primary-tint text-mk-primary" : "text-mk-muted-2 hover:text-mk-muted"}`}>
      {children}
    </button>
  );
}

/* ---------- right · AI rail ---------- */

const RAIL_GREETING: ChatMsg = {
  role: "ai",
  text: "把你正在纠结的那一段贴过来，或者告诉我它想让读者信什么——我们从这个目的倒推它够不够。",
};

// WC · the writing thinking-cards a student can summon in the Write room (all
// persist through /cards/persist's writing-deck allowlist). #17 adds the two
// knowledge cards that are writing/checking tools (not source-finding):
// 确定度光谱 (match hedging to certainty) and 事实/观点/价值判断 (sort a passage).
const WRITING_DECK = ["toulmin", "argument-map", "pee", "concession", "fact-opinion-value"];

function CoachRail({
  projectId,
  focusPart,
  onClearFocus,
  locked,
  onCardArtifact,
  sectionOptions,
  onRunReview,
}: {
  projectId: string;
  focusPart: string | null;
  onClearFocus: () => void;
  locked: boolean;
  // #8 · adds the compiled card paragraph as a snippet filed under `section`
  // (null = 未归类) — only called when the student explicitly taps 收进片段.
  onCardArtifact: (text: string, section: string | null) => void;
  // Known 片段 board section labels the student can file the artifact under.
  sectionOptions: string[];
  // #8-second · request a voice-scoped 体检 from the persistent shelf below.
  // No scope arg → whole draft; a scope string → just that paragraph.
  onRunReview: (scope: string | undefined, voice: ReviewVoice) => void;
}) {
  const [chat, setChat] = useState<ChatMsg[]>([RAIL_GREETING]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  // S4 · cross-phase card proposing. `proposal` is the coach's latest OFFER (a
  // dismissable chip); `openCardId` is the card the student CHOSE to open — the
  // only path to a card sheet, so triggering stays automatic while opening is
  // the student's tap (铁律).
  const [proposal, setProposal] = useState<CardProposalWire | null>(null);
  const [openCardId, setOpenCardId] = useState<string | null>(null);
  // #8 · a finished card's compiled paragraph, offered (never auto-added) as a
  // 片段 once the coach has responded to it. cardName is kept for the offer's
  // copy; cleared once collected or dismissed.
  const [pendingArtifact, setPendingArtifact] = useState<{ cardName: string; text: string } | null>(null);
  const [artifactSection, setArtifactSection] = useState<string>(UNFILED);

  // S1 · one continuous session: load this room's slice of the project thread
  // once on open, appended after the greeting. Empty → greeting only.
  useEffect(() => {
    let alive = true;
    getCoachHistory(projectId, "writing")
      .then((msgs) => {
        if (alive && msgs.length) setChat((c) => [...c, ...msgs]);
      })
      .catch(() => {
        /* keep greeting-only; the next turn still persists */
      });
    return () => {
      alive = false;
    };
  }, [projectId]);

  async function send() {
    const text = draft.trim();
    if (!text || sending || locked) return;
    // WC · if a draft part is pinned, scope this turn to it so 印记 checks THAT
    // part's argument/function — never rewriting it.
    const turnText = focusPart ? `就这一段想（帮我看它的论证与功能，别替我改写）：\n「${focusPart}」\n\n${text}` : text;
    // #9-second · the referenced paragraph rides as its own field (quotedPart),
    // rendered as a styled quote block above the bubble — never baked into the
    // message string as a literal 【就这一段】 token.
    setChat((c) => [...c, { role: "student", text, quotedPart: focusPart ?? undefined }]);
    setDraft("");
    setSending(true);
    try {
      const { reply, proposal: p } = await coach(projectId, "writing", turnText);
      onClearFocus(); // clear the pinned part only on success — a failed turn keeps it so she needn't re-pin
      setChat((c) => [...c, { role: "ai", text: reply }]);
      // S4 · the coach may OFFER a thinking-card (克制 summon rung). It's a
      // dismissable chip; opening it (below) is the student's tap, never auto.
      setProposal(p);
    } catch {
      setChat((c) => [...c, { role: "ai", text: "刚才没接上，稍等再问我一次。" }]);
    } finally {
      setSending(false);
    }
  }

  // Opening a proposed card is the student's explicit choice (铁律). On submit
  // the completed envelope persists (过程即数据 — recorded, not discarded), and
  // the rail acknowledges it.
  function openProposedCard(cardId: string) {
    setProposal(null);
    setOpenCardId(cardId);
  }
  // Dismiss is an explicit "no": record it so the coach stops offering this card
  // (铁律 · 不操纵). Best-effort — the chip clears regardless.
  function dismissProposedCard(cardId: string) {
    setProposal(null);
    void dismissProposal(projectId, cardId).catch(() => {});
  }
  // #8 · finishing a card now runs a coach turn that responds to what the
  // student wrote FIRST (mirrors the forming flow's reflect path) — it no
  // longer silently dumps a labeled-value block into 片段. Adding it as a
  // snippet is then a separate, explicit 收进片段 offer below.
  async function submitProposedCard(fieldValues: Record<string, unknown>, eventTrace: unknown[]) {
    const cardId = openCardId;
    setOpenCardId(null);
    if (!cardId) return;
    const spec = CARD_REGISTRY[cardId];
    const studentText = spec ? compileCardForCoach(spec, fieldValues) : "";
    try {
      const { reply } = await reflectProjectCard(projectId, cardId, fieldValues, eventTrace, "writing");
      if (studentText) setChat((c) => [...c, { role: "student", text: studentText }]);
      if (reply) {
        setChat((c) => [...c, { role: "ai", text: reply }]);
      } else if (!studentText) {
        setChat((c) => [...c, { role: "ai", text: "这张卡还没填内容，先留着，想清楚了再来。" }]);
      }
      // #7/#8 · offer (never silently add) the compiled paragraph as a 片段.
      const artifact = spec ? compileCardEnvelope(spec, fieldValues) : "";
      if (artifact) {
        setArtifactSection(UNFILED);
        setPendingArtifact({ cardName: spec!.name, text: artifact });
      }
    } catch {
      if (studentText) setChat((c) => [...c, { role: "student", text: studentText }]);
      setChat((c) => [...c, { role: "ai", text: "刚才没接住这张卡，等下再试一次。" }]);
    }
  }

  function collectArtifact() {
    if (!pendingArtifact) return;
    onCardArtifact(pendingArtifact.text, artifactSection === UNFILED ? null : artifactSection);
    setChat((c) => [...c, { role: "ai", text: "收进了「片段」——去那儿看看、改改，随时能插进正文。" }]);
    setPendingArtifact(null);
    setArtifactSection(UNFILED);
  }

  return (
    <aside className="flex min-h-0 flex-col border-l border-mk-border bg-mk-surface">
      <header className="border-b border-mk-border px-4 py-3">
        <div className="flex items-center gap-2 text-mk-primary">
          <Icon name="spark" size={16} />
          <h2 className="font-sans text-[14px] font-bold">印记 · 陪你写</h2>
        </div>
        <p className="mt-1 text-[11.5px] text-mk-muted-2">聊提纲、挑逻辑、撞反例——但不替你写正文。</p>
      </header>
      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-4 py-4">
        {chat.map((m, i) => (
          <div key={i} className={`flex ${m.role === "ai" ? "justify-start" : "justify-end"}`}>
            <div className="max-w-[88%]">
              {/* #9-second · the referenced paragraph is a styled quote callout
                  above the question — never a 【就这一段】 token baked into the
                  bubble text. */}
              {m.quotedPart && (
                <blockquote className="mb-1 rounded-mk border-l-[3px] border-mk-accent bg-mk-accent-tint/40 px-2.5 py-1.5 text-[12px] italic leading-snug text-mk-muted">
                  {m.quotedPart}
                </blockquote>
              )}
              <div className={`rounded-mk-lg px-3.5 py-2.5 text-[13px] leading-relaxed ${m.role === "ai" ? "bg-mk-bg text-mk-ink" : "bg-mk-primary text-white"}`}>{m.text}</div>
            </div>
          </div>
        ))}
        {proposal && !openCardId ? (
          <CoachProposal proposal={proposal} onOpen={openProposedCard} onDismiss={() => dismissProposedCard(proposal.cardId)} />
        ) : null}
        {/* #8 · 收进片段 is an explicit offer, never a silent add — the coach has
            already responded to the card's content above; this just asks
            whether the compiled paragraph should also become a 片段. */}
        {pendingArtifact && !openCardId && (
          <div className="rounded-mk-lg border border-mk-border-2 bg-mk-bg/60 p-3">
            <p className="text-[12px] font-semibold text-mk-muted-2">要不要把《{pendingArtifact.cardName}》里写的收进「片段」？</p>
            <p className="mt-1.5 max-h-28 overflow-y-auto whitespace-pre-wrap text-[12.5px] leading-relaxed text-mk-ink">{pendingArtifact.text}</p>
            <div className="mt-2 flex flex-wrap items-center justify-end gap-2">
              <label className="flex items-center gap-1 text-[11px] text-mk-muted-2">
                归到
                <select
                  value={artifactSection}
                  onChange={(e) => setArtifactSection(e.target.value)}
                  aria-label="把片段归到"
                  className="max-w-[9rem] rounded border border-mk-border bg-mk-surface px-1.5 py-0.5 text-[11.5px] text-mk-ink outline-none focus:border-mk-primary"
                >
                  <option value={UNFILED}>未归类</option>
                  {sectionOptions.map((o) => <option key={o} value={o}>{o}</option>)}
                </select>
              </label>
              <button type="button" onClick={() => setPendingArtifact(null)} className="text-[11px] font-semibold text-mk-muted-2 hover:text-mk-ink">先不收</button>
              <button type="button" onClick={collectArtifact} className="rounded-full bg-mk-primary px-3 py-1 text-[11px] font-bold text-white hover:bg-mk-primary-hover">收进片段</button>
            </div>
          </div>
        )}
        {sending && (
          <div className="flex justify-start">
            <div className="max-w-[88%] rounded-mk-lg bg-mk-bg px-3.5 py-2.5 text-[13px] leading-relaxed text-mk-muted-2">印记在想……</div>
          </div>
        )}
      </div>
      {/* #8-second · the persistent tool shelf — always visible (no ＋ to hide
          it), grouped by what it targets: 正文 (the writing thinking-cards,
          summoned onto a paragraph) and 正文·检查 (the four examiner voices,
          each choosable against the WHOLE draft or — once a paragraph is
          pinned via 问印记 — just 这段). Triggering a card/voice is automatic
          UI; OPENING the card sheet or SEEING the check result still needs her
          tap/click (铁律 · 不操纵). */}
      {!locked && (
        <div className="border-t border-mk-border bg-mk-bg px-3 py-2">
          <p className="mb-1.5 text-[11px] font-bold text-mk-muted-2">正文 · 挑一张写作卡，想清楚这一段的论证——你填，印记不替你写</p>
          <div className="flex flex-wrap gap-1.5">
            {WRITING_DECK.map((id) => CARD_REGISTRY[id] && (
              <button
                key={id}
                type="button"
                onClick={() => openProposedCard(id)}
                title={CARD_REGISTRY[id]!.purpose}
                className="rounded-mk border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-semibold text-mk-ink hover:border-mk-primary hover:text-mk-primary"
              >
                {CARD_REGISTRY[id]!.name}
              </button>
            ))}
          </div>
          <p className="mb-1.5 mt-2.5 text-[11px] font-bold text-mk-muted-2">正文·检查 · 换个视角体检{focusPart ? "（整稿，或只查你选中的这段）" : "（整稿）"}</p>
          <div className="flex flex-col gap-1">
            {VOICE_ORDER.map((v) => (
              <div key={v} className="flex items-center gap-1.5">
                <button
                  type="button"
                  onClick={() => onRunReview(undefined, v)}
                  title={`${VOICE_META[v].desc} · 体检整稿`}
                  className="rounded-mk border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-semibold text-mk-ink hover:border-mk-primary hover:text-mk-primary"
                >
                  {VOICE_META[v].label}
                </button>
                {focusPart && (
                  <button
                    type="button"
                    onClick={() => onRunReview(focusPart, v)}
                    title="只体检你目前选中的这一段"
                    className="rounded-full border border-mk-accent/40 px-2 py-0.5 text-[11px] font-semibold text-mk-accent hover:bg-mk-accent-tint"
                  >
                    这段
                  </button>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
      {focusPart && (
        <div className="flex items-center gap-2 border-t border-mk-accent/30 bg-mk-accent-tint/30 px-3 py-2">
          <span className="flex-none text-[11px] font-bold text-mk-accent">就这一段</span>
          <span className="min-w-0 flex-1 truncate text-[12px] text-mk-muted">{focusPart}</span>
          <button type="button" onClick={onClearFocus} className="flex-none text-[12px] font-semibold text-mk-muted-2 hover:text-mk-muted">✕</button>
        </div>
      )}
      {locked ? (
        // #5 (review L2) · an archived project's process is sealed — the writing
        // coach takes no new turns/cards so 过程即数据 stays true to the record.
        <div className="border-t border-mk-border p-3 text-center text-[12px] font-semibold text-mk-muted-2">这篇已归档——过程已封存，印记不再新增这里的思考。</div>
      ) : (
        <div className="flex items-end gap-2 border-t border-mk-border p-3">
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); void send(); } }}
            rows={1}
            placeholder={focusPart ? "就这一段，你想问什么？" : "问问这段逻辑、这个结构……"}
            className="max-h-24 flex-1 resize-none rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2 text-[13px] text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
          />
          <button
            type="button"
            onClick={() => void send()}
            disabled={sending}
            className="flex h-9 w-9 flex-none items-center justify-center rounded-mk bg-mk-primary text-white hover:bg-mk-primary-hover disabled:opacity-50"
          >
            <Icon name="send" size={16} />
          </button>
        </div>
      )}

      {/* #3 · the opened tool card is a centered modal over the whole room, not a
          card buried in the right rail. Triggering stays automatic (the proposal
          chip / deck); OPENING is the student's tap, and the sheet then fills the
          modal. Backdrop / 收起 closes it. */}
      {openCardId && CARD_REGISTRY[openCardId] && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-mk-ink/40 p-4" onClick={() => setOpenCardId(null)}>
          <div className="max-h-[88vh] w-full max-w-lg overflow-y-auto rounded-mk-lg bg-mk-surface shadow-[0_20px_60px_rgba(28,35,51,0.3)]" onClick={(e) => e.stopPropagation()}>
            <StudioCardSheet
              spec={CARD_REGISTRY[openCardId]}
              onSubmit={(env) => submitProposedCard(env.field_values, env.event_trace)}
              onSkip={() => setOpenCardId(null)}
            />
          </div>
        </div>
      )}
    </aside>
  );
}
