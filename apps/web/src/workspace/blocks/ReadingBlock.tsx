import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { Collection, MaterialSource, PhaseTag, Reference } from "@mind-imprint/contracts";
import { Icon } from "../Icon";
import {
  getLibrary,
  createCollection,
  createReference,
  patchReference,
  enterReading,
  pasteContent,
  NoReadableContentError,
  type ReferencePatch,
  type SourceMeta,
  type ReferenceBib,
} from "../api/workspace";
import { exportAnnotatedBib as buildAnnotatedBib } from "../export";
import { putReadingBrief } from "../../api/reading";
import { ExplorationView } from "./exploration/ExplorationView";
import { getExploration } from "../../api/exploration";
import { useStudioAiSlot } from "@/studio/ai/StudioAiSlot";
import { useStudioChat } from "@/studio/ai/StudioChatContext";
import { StudioCoachChat } from "@/studio/ai/StudioCoachChat";

// #4 (review M1) · remembers the chosen 列表/探索图谱 view per project for the
// life of the session, so a manual toggle survives the room unmounting (switching
// rooms / opening a source). Absence of a key means "not yet auto-defaulted" —
// the first load then picks graph-if-sources-exist, else list.
const viewModeMemo = new Map<string, "list" | "graph">();

// Display labels — pure enum→label maps (kept local so the room owns no mock
// seed data). Values mirror the contract's Credibility / UseDecision enums.
const CRED_LABEL: Record<NonNullable<Reference["credibility"]>, string> = {
  strong: "可信度高",
  mixed: "需交叉核实",
  weak: "存疑",
};
const DECISION_LABEL: Record<"use" | "maybe" | "drop", string> = {
  use: "该用",
  maybe: "待定",
  drop: "不用",
};

const CRED_STYLE: Record<NonNullable<Reference["credibility"]>, string> = {
  strong: "bg-mk-success-bg text-mk-success",
  mixed: "bg-mk-accent-50 text-mk-accent",
  weak: "bg-mk-paper text-mk-muted",
};

// A7 · 待读/在读/读完 shelf status — a single explicit field on every reference
// (set automatically to "reading" when a paper is adopted from 探索; editable
// by hand everywhere else) shown as a small badge/control in 图书馆.
const READING_STATUS_LABEL: Record<Reference["readingStatus"], string> = {
  to_read: "待读",
  reading: "在读",
  done: "读完",
};
const READING_STATUS_STYLE: Record<Reference["readingStatus"], string> = {
  to_read: "bg-mk-paper text-mk-faint",
  reading: "bg-mk-accent-50 text-mk-accent",
  done: "bg-mk-success-bg text-mk-success",
};

// The Reading block = a Zotero-shaped Library: collections + tags (left) for
// categorization, a reference table (center) that scales to many sources with
// multi-select batch export, and a thin preview (right). Task 3 (P2a): the
// room used to carry its own floating/docked 印记 coach (context-isolated
// "find_sources" thread); it now portals nothing of its own — the ONE
// constant 印记 rail (shared studio thread) shows beside it, same as
// plan/writing/reflection.
//
// All state is now persisted through the workspace API (slice 3): the library
// loads on mount; edits patch optimistically; 进入阅读室 mints/loads a real
// MaterialSource and hands it to the container's reading-room swap slot.
export function ReadingBlock({
  projectId,
  title,
  setReadingSource,
  refreshNonce,
}: {
  projectId: string;
  title: string;
  // Task 8 (P2b) · WorkspaceContainer's `explorationRefreshNonce`, forwarded
  // straight through to ExplorationView so a 印记-confirmed question (the chat
  // chip, Task 7) re-fetches the graph without a manual reload.
  refreshNonce?: number;
  // referenceId is the Library row this material was opened from — the S2
  // reading-brief/takeaway endpoints are keyed by reference id, not material
  // id, so the room needs it threaded through. suggestedReason is the
  // deterministic seed enter-reading computes (empty on the paste-body path,
  // which has no proposal to template from). phaseTag/readingReason/
  // readingFocus (Task 9 fix) are the reference's PERSISTED brief — passed
  // through so the room seeds its banner from her true last-saved values
  // instead of always re-deriving/blanking them, which used to let editing
  // one field wipe the other on the room's next full-replace save.
  setReadingSource: (
    m: MaterialSource,
    referenceId: string,
    suggestedReason?: string,
    phaseTag?: PhaseTag | null,
    readingReason?: string | null,
    readingFocus?: string | null,
    readingNote?: string | null,
    // #4: the reference's persisted bib (abstract/journal/author/year/url) so the
    // Reading Room header can show the abstract + metadata + a 打开原文 link.
    bib?: ReferenceBib,
  ) => void;
}) {
  const [refs, setRefs] = useState<Reference[]>([]);
  const [collections, setCollections] = useState<Collection[]>([]);
  const [loading, setLoading] = useState(true);
  const [collId, setCollId] = useState<string>("all");
  const [activeTag, setActiveTag] = useState<string | null>(null);
  const [selId, setSelId] = useState<string>("");
  const [checked, setChecked] = useState<Set<string>>(new Set());
  const [adding, setAdding] = useState(false);
  const [railOpen, setRailOpen] = useState(true);
  // 列表 ⇄ 探索图谱 (Task 9): 列表 is today's Zotero-shaped table, unchanged;
  // 探索图谱 is the S3 rabbit-hole branch view over the same references.
  // #4: default to 探索图谱 once the project HAS sources (auto-default runs once
  // per project, below); a brand-new/empty library opens on 列表 where the add
  // affordances live. The student's manual toggle then sticks across room
  // re-entry — ReadingBlock unmounts when switching rooms / opening a source to
  // read, so the choice is remembered in a module-level per-project memo
  // (viewModeMemo) rather than lost on remount (review M1).
  const [viewMode, setViewModeRaw] = useState<"list" | "graph">(() => viewModeMemo.get(projectId) ?? "list");
  const chooseViewMode = (m: "list" | "graph") => {
    viewModeMemo.set(projectId, m);
    setViewModeRaw(m);
  };
  // Task 3 (P2a): the room→panel contract (spec §17) — this room's WORK
  // renders directly below, in <main>; its COACH portals into the constant
  // AiPanel via `useStudioAiSlot`, same as plan/writing/reflection. `slot` is
  // null when the panel is collapsed or this component renders outside a
  // studio shell (e.g. some tests) — in either case the coach content simply
  // doesn't render, never crashes. A rabbit-hole (or other exploration deck)
  // card reflected inside ExplorationView used to bridge into a
  // FloatingCoach-owned local thread; it now appends straight to the ONE
  // shared studio thread via `useStudioChat().setMessages`, so it shows up in
  // the SAME 印记 rail every other card's feedback uses.
  const slot = useStudioAiSlot();
  const { setMessages: setStudioMessages } = useStudioChat();

  // Debounce timers for free-text metadata edits, keyed by ref+field so each
  // field coalesces independently.
  const debounceTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  // EB · a discoverability signal for the rabbit-hole: how many leads still need
  // following + how many read-but-unconnected (dangling) sources. Shown as a
  // badge on the 探索图谱 toggle so the graph advertises itself — the student
  // still chooses to open it (不操纵; no auto-switch).
  const [explorationSignal, setExplorationSignal] = useState(0);
  const loadExplorationSignal = useMemo(
    () => async () => {
      try {
        const v = await getExploration(projectId);
        setExplorationSignal(v.leads.filter((l) => l.status === "open").length + v.danglingSourceIds.length);
      } catch {
        /* the badge is a nicety; never block the room */
      }
    },
    [projectId],
  );

  const reload = useMemo(
    () => async () => {
      try {
        const lib = await getLibrary(projectId);
        setRefs(lib.references);
        setCollections(lib.collections);
      } catch {
        /* keep the last-good library; the room stays usable */
      }
      void loadExplorationSignal();
    },
    [projectId, loadExplorationSignal],
  );

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    (async () => {
      try {
        const lib = await getLibrary(projectId);
        if (cancelled) return;
        setRefs(lib.references);
        setCollections(lib.collections);
        setSelId(lib.references[0]?.id ?? "");
        // #18 · always open on the exploration graph, even for a brand-new
        // empty library — 探索图谱 shows its own empty/线索 state and offers
        // ＋添加来源 right there, so there's no need to force 列表 first. Auto-
        // default runs ONCE per project (guarded by the memo) so a later manual
        // toggle isn't overridden on the next mount (review M1). Set before
        // loading clears → no flash.
        if (!viewModeMemo.has(projectId)) {
          viewModeMemo.set(projectId, "graph");
          setViewModeRaw("graph");
        }
        void loadExplorationSignal();
      } catch {
        /* an empty library reads as the empty state */
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  // Snapshot & clear all pending debounce timers on unmount.
  useEffect(() => {
    const timers = debounceTimers.current;
    return () => {
      for (const t of timers.values()) clearTimeout(t);
      timers.clear();
    };
  }, []);

  const descendants = useMemo(() => descendantMap(collections), [collections]);
  const allTags = useMemo(() => [...new Set(refs.flatMap((r) => r.tags))], [refs]);

  // Optimistic local write; the server patch is fire-and-forget (reload on
  // failure). The client already holds the authoritative new value, so we never
  // clobber an in-flight edit to a sibling field with a stale server row.
  function applyLocal(refId: string, p: ReferencePatch) {
    setRefs((xs) => xs.map((r) => (r.id === refId ? { ...r, ...p } : r)));
  }
  function persist(refId: string, p: ReferencePatch) {
    patchReference(projectId, refId, p).catch(() => reload());
  }
  // Immediate persistence — for selectors / tag add-remove.
  function patchNow(refId: string, p: ReferencePatch) {
    applyLocal(refId, p);
    persist(refId, p);
  }
  // Debounced persistence (~500ms) — for free-text fields.
  function patchDebounced(refId: string, p: ReferencePatch) {
    applyLocal(refId, p);
    const key = `${refId}:${Object.keys(p).join(",")}`;
    const timers = debounceTimers.current;
    const existing = timers.get(key);
    if (existing) clearTimeout(existing);
    timers.set(
      key,
      setTimeout(() => {
        timers.delete(key);
        persist(refId, p);
      }, 500),
    );
  }

  function addTag(refId: string, tag: string) {
    const t = tag.trim();
    if (!t) return;
    const r = refs.find((x) => x.id === refId);
    if (!r || r.tags.includes(t)) return;
    patchNow(refId, { tags: [...r.tags, t] });
  }
  function removeTag(refId: string, tag: string) {
    const r = refs.find((x) => x.id === refId);
    if (!r) return;
    patchNow(refId, { tags: r.tags.filter((x) => x !== tag) });
  }

  // #2 · tag which argument-stage a source served, editable inline from the list
  // so older/untagged readings can be labelled. phaseTag lives on the reading
  // brief (a full-replace), so carry the source's existing reason/focus through.
  function setPhaseTag(refId: string, phase: PhaseTag | "") {
    const r = refs.find((x) => x.id === refId);
    setRefs((xs) => xs.map((x) => (x.id === refId ? { ...x, phaseTag: phase === "" ? null : phase } : x)));
    putReadingBrief(projectId, refId, {
      readingReason: r?.readingReason ?? "",
      readingFocus: r?.readingFocus ?? "",
      phaseTag: phase,
    }).catch(() => reload());
  }

  async function addSource(src: { title: string; url: string; classification: string; collectionId: string | null }) {
    try {
      const created = await createReference(projectId, {
        title: src.title || undefined,
        url: src.url || undefined,
        classification: src.classification || undefined,
        collectionId: src.collectionId,
      });
      setRefs((xs) => [created, ...xs]);
      setSelId(created.id);
    } catch {
      /* leave the modal's job to the reload path */
      reload();
    } finally {
      setAdding(false);
    }
  }

  // Paste-source path: create the reference, immediately paste the body as its
  // material, and open the Reading Room — the modal twin of the preview's
  // paste fallback.
  async function addPastedSource(src: { title: string; text: string; collectionId: string | null }) {
    try {
      const created = await createReference(projectId, {
        title: src.title || undefined,
        classification: "粘贴正文",
        collectionId: src.collectionId,
      });
      setRefs((xs) => [created, ...xs]);
      setSelId(created.id);
      const source = await pasteContent(projectId, created.id, src.text, src.title || undefined);
      setReadingSource(source, created.id);
    } catch {
      reload();
    } finally {
      setAdding(false);
    }
  }

  // Item A #2 (exploration graph) · register a brand-new untracked source
  // without leaving 探索图谱 — reuses the SAME createReference call + updates
  // the SAME refs state the 列表 add-source modal does, so the library never
  // holds two divergent copies of "what references exist".
  async function createUntrackedSource(input: { title: string; url?: string }): Promise<Reference> {
    const created = await createReference(projectId, { title: input.title || undefined, url: input.url || undefined });
    setRefs((xs) => [created, ...xs]);
    return created;
  }

  async function addCollection(name: string, parentId: string | null) {
    const n = name.trim();
    if (!n) return;
    try {
      const created = await createCollection(projectId, { name: n, parentId });
      setCollections((xs) => [...xs, created]);
    } catch {
      reload();
    }
  }

  // The annotated bibliography — the submittable .xlsx, straight from the
  // library. Columns mirror the school's 资源评估表 form (slice 6). The batch
  // path passes the checked subset; the toolbar path exports everything. The
  // xlsx lib loads lazily; a busy flag guards the round-trip and a caught error
  // keeps the room alive.
  const [exportingBib, setExportingBib] = useState(false);
  async function exportAnnotatedBib(ids?: Set<string>) {
    if (exportingBib) return;
    const rows = refs.filter((r) => !r.pending && (!ids || ids.has(r.id)));
    setExportingBib(true);
    try {
      await buildAnnotatedBib(rows, { title });
    } catch {
      /* a failed export must never crash the room */
    } finally {
      setExportingBib(false);
    }
  }

  const rows = useMemo(() => {
    let list = refs;
    if (collId !== "all") {
      const ok = new Set([collId, ...(descendants.get(collId) ?? [])]);
      list = list.filter((r) => r.collectionId != null && ok.has(r.collectionId));
    }
    if (activeTag) list = list.filter((r) => r.tags.includes(activeTag));
    return list;
  }, [refs, collId, activeTag, descendants]);

  const selected = refs.find((r) => r.id === selId) ?? rows[0] ?? refs[0];

  function toggleCheck(id: string) {
    setChecked((s) => {
      const n = new Set(s);
      n.has(id) ? n.delete(id) : n.add(id);
      return n;
    });
  }

  const modal = adding && (
    <AddSourceModal
      collections={collections}
      defaultCollection={collId === "all" ? collections[0]?.id ?? "" : collId}
      onClose={() => setAdding(false)}
      onSubmit={addSource}
      onPaste={addPastedSource}
    />
  );

  // Still loading the library.
  if (loading) {
    return <div className="flex h-full items-center justify-center text-[14px] text-mk-faint">加载中…</div>;
  }

  // #3 · the 列表 empty state names the project topic (no model call). #18 ·
  // this no longer gates on a special empty-library layout — 探索图谱 is now
  // the default even when refs.length === 0 (its own view renders a
  // graph-flavored empty state), so 列表's EmptyLibrary is just that view's
  // empty content, reached via the toggle like any other view.
  const topic = title?.trim();

  return (
    <div className="relative flex h-full flex-col">
      <div className="flex items-center justify-start border-b border-mk-border bg-mk-surface px-4 py-1.5">
        <ViewModeToggle mode={viewMode} onChange={chooseViewMode} signal={explorationSignal} />
      </div>

      <div className="min-h-0 flex-1">
        {viewMode === "list" ? (
          refs.length === 0 ? (
            <EmptyLibrary onAdd={() => setAdding(true)} topic={topic} />
          ) : (
            <div className="grid h-full" style={{ gridTemplateColumns: `${railOpen ? "220px" : "48px"} 1fr 300px` }}>
              <CollectionsRail
                open={railOpen}
                onToggle={() => setRailOpen((o) => !o)}
                collections={collections}
                collId={collId}
                onPick={(id) => { setCollId(id); setActiveTag(null); }}
                tags={allTags}
                activeTag={activeTag}
                onTag={setActiveTag}
                total={refs.length}
                countFor={(id) => (id === "all" ? refs.length : refs.filter((r) => r.collectionId != null && new Set([id, ...(descendants.get(id) ?? [])]).has(r.collectionId)).length)}
                onDropRef={(collId2, refId) => patchNow(refId, { collectionId: collId2 })}
                onCreateCollection={(name) => addCollection(name, null)}
              />

              <RefTable
                rows={rows}
                selId={selected?.id ?? ""}
                onSelect={setSelId}
                checked={checked}
                onCheck={toggleCheck}
                onClearChecks={() => setChecked(new Set())}
                onExportBib={exportAnnotatedBib}
                collName={collId === "all" ? "全部文献" : collections.find((c) => c.id === collId)?.name ?? ""}
                activeTag={activeTag}
                onAdd={() => setAdding(true)}
                onSetPhase={setPhaseTag}
                onSetStatus={(id, status) => patchNow(id, { readingStatus: status })}
              />

              {selected ? (
                <Preview
                  key={selected.id}
                  projectId={projectId}
                  item={selected}
                  allTags={allTags}
                  onAddTag={(t) => addTag(selected.id, t)}
                  onRemoveTag={(t) => removeTag(selected.id, t)}
                  onPatchNow={(p) => patchNow(selected.id, p)}
                  onPatchDebounced={(p) => patchDebounced(selected.id, p)}
                  onEnterReading={setReadingSource}
                />
              ) : (
                <div className="border-l border-mk-border bg-mk-surface" />
              )}
            </div>
          )
        ) : (
          // Task 3 (P2a): the unified right sidebar's DEFAULT ('ai') state used
          // to dock the room's own FloatingCoach; that coach is gone, so no
          // `coach` slot is passed — ExplorationSidebar's existing fallback
          // (coach ? … : blank aside) renders instead, and the ONE 印记 lives
          // in the constant rail beside this room (portaled below).
          <div className="relative h-full min-h-0">
            <ExplorationView
              projectId={projectId}
              references={refs}
              // GVf · no dedicated research-question field is reachable here —
              // the project title (already threaded in as `title`/`topic`) is
              // the driving-question seed's fallback source.
              projectTitle={title}
              refreshNonce={refreshNonce}
              onEnterReading={setReadingSource}
              onCreateReference={createUntrackedSource}
              // 采纳 in 探索 creates a new library reference — reload so its bib
              // shows on the new paper node (else every metadata field is 「—」).
              onLibraryChanged={reload}
              // Task 3 (P2a): a rabbit-hole (or other exploration deck) card's
              // reflect result used to bridge into the room-owned FloatingCoach;
              // it now appends straight to the ONE shared studio thread, so the
              // feedback lands in the same constant 印记 rail every other card
              // uses.
              onCardReflected={(studentText, reply, card) =>
                setStudioMessages((c) => [
                  ...c,
                  ...(card
                    ? [{ role: "student" as const, text: studentText, card }]
                    : studentText
                      ? [{ role: "student" as const, text: studentText }]
                      : []),
                  ...(reply ? [{ role: "ai" as const, text: reply }] : []),
                ])
              }
            />
          </div>
        )}
      </div>

      {/* COACH — portaled into the constant AiPanel (Task 3, P2a), same
          contract as plan/writing/reflection. Reading has no room-specific
          chrome to add, so it reuses the shared `StudioCoachChat` body as-is. */}
      {slot && createPortal(<StudioCoachChat />, slot)}

      {modal}
    </div>
  );
}

// 图书馆 ⇄ 探索 segmented toggle — Task 9 (renamed + iconed, A7). 图书馆 (list
// keys unchanged) is the default; opening 探索 (graph keys unchanged) is always
// an explicit click (铁律 2 不操纵 applies to surfaces too).
export function ViewModeToggle({ mode, onChange, signal }: { mode: "list" | "graph"; onChange: (m: "list" | "graph") => void; signal: number }) {
  return (
    <div className="flex rounded-mk border border-mk-border bg-mk-paper p-0.5 text-[12px] font-bold">
      {(["list", "graph"] as const).map((m) => (
        <button
          key={m}
          type="button"
          onClick={() => onChange(m)}
          className={`flex items-center gap-1.5 rounded-[8px] px-3 py-1 transition ${mode === m ? "bg-mk-surface text-mk-accent shadow-sm" : "text-mk-faint hover:text-mk-ink"}`}
        >
          <Icon name={m === "list" ? "library" : "explore"} size={14} />
          {m === "list" ? "图书馆" : "探索"}
          {/* EB · advertise pending leads/dangling so the graph is discoverable */}
          {m === "graph" && signal > 0 && (
            <span
              title={`${signal} 条线索待追 / 悬空来源`}
              className="inline-flex min-w-[16px] items-center justify-center rounded-full bg-mk-accent px-1 text-[10px] font-bold leading-4 text-white"
            >
              {signal}
            </span>
          )}
        </button>
      ))}
    </div>
  );
}

function descendantMap(cols: Collection[]): Map<string, string[]> {
  const m = new Map<string, string[]>();
  for (const c of cols) if (c.parentId) m.set(c.parentId, [...(m.get(c.parentId) ?? []), c.id]);
  return m;
}

/* ---------- left · collections + tags ---------- */

function CollectionsRail(props: {
  open: boolean;
  onToggle: () => void;
  collections: Collection[];
  collId: string;
  onPick: (id: string) => void;
  tags: string[];
  activeTag: string | null;
  onTag: (t: string | null) => void;
  total: number;
  countFor: (id: string) => number;
  onDropRef: (collectionId: string, refId: string) => void;
  onCreateCollection: (name: string) => void;
}) {
  const { open, onToggle, collections, collId, onPick, tags, activeTag, onTag, total, countFor, onDropRef, onCreateCollection } = props;
  const roots = collections.filter((c) => !c.parentId);
  const [folded, setFolded] = useState<Set<string>>(new Set());
  const [dropId, setDropId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");

  // Collapsed: a thin strip with an expand affordance.
  if (!open) {
    return (
      <aside className="flex min-h-0 flex-col items-center border-r border-mk-border bg-mk-surface py-3">
        <button type="button" onClick={onToggle} title="展开合集" className="flex h-8 w-8 items-center justify-center rounded-mk text-mk-faint hover:bg-mk-paper hover:text-mk-accent">
          <Chevron dir="right" />
        </button>
        <div className="mt-2 text-mk-faint"><FolderGlyph /></div>
      </aside>
    );
  }

  function dropProps(id: string) {
    return {
      onDragOver: (e: React.DragEvent) => { e.preventDefault(); setDropId(id); },
      onDragLeave: () => setDropId((d) => (d === id ? null : d)),
      onDrop: (e: React.DragEvent) => { const rid = e.dataTransfer.getData("text/ref"); if (rid) onDropRef(id, rid); setDropId(null); },
      isDrop: dropId === id,
    };
  }

  function commitNew() {
    if (newName.trim()) onCreateCollection(newName);
    setNewName("");
    setCreating(false);
  }

  return (
    <aside className="flex min-h-0 flex-col border-r border-mk-border bg-mk-surface">
      <div className="flex items-center justify-between px-3 pt-3">
        <span className="text-[11px] font-bold uppercase tracking-wider text-mk-faint">文献库</span>
        <button type="button" onClick={onToggle} title="收起合集" className="flex h-6 w-6 items-center justify-center rounded text-mk-faint hover:bg-mk-paper hover:text-mk-accent">
          <Chevron dir="left" />
        </button>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-2.5 py-2">
        <CollRow label="全部文献" count={total} active={collId === "all"} onClick={() => onPick("all")} icon="reading" {...dropProps("all")} />
        <div className="mt-3 mb-1.5 flex items-center justify-between px-2">
          <span className="text-[11px] font-bold uppercase tracking-wider text-mk-faint">我的合集</span>
          <button type="button" onClick={() => setCreating(true)} className="text-[15px] leading-none text-mk-faint hover:text-mk-accent">+</button>
        </div>
        {creating && (
          <div className="mb-1 px-1">
            <input
              autoFocus
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") commitNew(); if (e.key === "Escape") { setNewName(""); setCreating(false); } }}
              onBlur={commitNew}
              placeholder="合集名称"
              className="w-full rounded-mk border border-mk-accent/40 bg-mk-surface px-2 py-1 text-[12.5px] text-mk-ink outline-none placeholder:text-mk-faint"
            />
          </div>
        )}
        {roots.map((root) => {
          const kids = collections.filter((c) => c.parentId === root.id);
          const isFolded = folded.has(root.id);
          return (
            <div key={root.id}>
              <CollRow
                label={root.name}
                count={countFor(root.id)}
                active={collId === root.id}
                onClick={() => onPick(root.id)}
                folder
                caret={kids.length > 0 ? (isFolded ? "closed" : "open") : undefined}
                onCaret={() => setFolded((s) => { const n = new Set(s); n.has(root.id) ? n.delete(root.id) : n.add(root.id); return n; })}
                {...dropProps(root.id)}
              />
              {!isFolded && kids.map((child) => (
                <CollRow key={child.id} label={child.name} count={countFor(child.id)} active={collId === child.id} onClick={() => onPick(child.id)} folder indent {...dropProps(child.id)} />
              ))}
            </div>
          );
        })}

        <div className="mt-4 mb-1.5 px-2 text-[11px] font-bold uppercase tracking-wider text-mk-faint">标签</div>
        <div className="flex flex-wrap gap-1.5 px-1.5">
          {tags.map((t) => (
            <button
              key={t}
              type="button"
              onClick={() => onTag(activeTag === t ? null : t)}
              className={`rounded-full px-2 py-0.5 text-[11.5px] font-semibold transition ${activeTag === t ? "bg-mk-accent text-white" : "bg-mk-paper text-mk-muted hover:text-mk-accent"}`}
            >
              {t}
            </button>
          ))}
        </div>
      </div>
      <div className="border-t border-mk-border px-4 py-2.5 text-[11px] text-mk-faint">把文献拖到合集上归类</div>
    </aside>
  );
}

function CollRow({ label, count, active, onClick, folder, indent, icon, caret, onCaret, onDragOver, onDragLeave, onDrop, isDrop }: {
  label: string; count: number; active: boolean; onClick: () => void; folder?: boolean; indent?: boolean; icon?: string;
  caret?: "open" | "closed"; onCaret?: () => void;
  onDragOver?: (e: React.DragEvent) => void; onDragLeave?: () => void; onDrop?: (e: React.DragEvent) => void; isDrop?: boolean;
}) {
  return (
    <div
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
      className={`flex w-full items-center gap-1 rounded-mk pr-2 transition ${indent ? "pl-4" : "pl-1"} ${isDrop ? "bg-mk-accent-50 ring-1 ring-mk-accent/40" : active ? "bg-mk-accent-50" : "hover:bg-mk-paper"}`}
    >
      {caret ? (
        <button type="button" onClick={onCaret} className="flex h-5 w-4 flex-none items-center justify-center text-mk-faint hover:text-mk-accent">
          <Chevron dir={caret === "open" ? "down" : "right"} small />
        </button>
      ) : (
        <span className="w-4 flex-none" />
      )}
      <button type="button" onClick={onClick} className="flex flex-1 items-center gap-2 py-1.5 text-left">
        <span className={active ? "text-mk-accent" : "text-mk-faint"}>{icon ? <Icon name={icon} size={15} /> : <FolderGlyph />}</span>
        <span className={`flex-1 truncate text-[13px] font-semibold ${active ? "text-mk-accent" : "text-mk-ink"}`}>{label}</span>
        <span className="text-[11px] font-semibold text-mk-faint">{count}</span>
      </button>
    </div>
  );
}

function Chevron({ dir, small }: { dir: "left" | "right" | "down"; small?: boolean }) {
  const s = small ? 12 : 16;
  const d = dir === "left" ? "M15 6l-6 6 6 6" : dir === "right" ? "M9 6l6 6-6 6" : "M6 9l6 6 6-6";
  return (
    <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d={d} /></svg>
  );
}

function FolderGlyph() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
    </svg>
  );
}

/* ---------- center · reference table ---------- */

function RefTable(props: {
  rows: Reference[];
  selId: string;
  onSelect: (id: string) => void;
  checked: Set<string>;
  onCheck: (id: string) => void;
  onClearChecks: () => void;
  onExportBib: (ids?: Set<string>) => void;
  collName: string;
  activeTag: string | null;
  onAdd: () => void;
  onSetPhase: (id: string, phase: PhaseTag | "") => void;
  onSetStatus: (id: string, status: Reference["readingStatus"]) => void;
}) {
  const { rows, selId, onSelect, checked, onCheck, onClearChecks, onExportBib, collName, activeTag, onAdd, onSetPhase, onSetStatus } = props;
  const nChecked = checked.size;
  return (
    <div className="flex min-h-0 flex-col bg-mk-surface">
      {/* toolbar */}
      <header className="flex items-center justify-between border-b border-mk-border px-5 py-3">
        <div className="flex items-baseline gap-2">
          <h2 className="font-sans text-[16px] font-bold text-mk-ink">{collName}</h2>
          <span className="text-[12px] font-semibold text-mk-faint">{rows.length} 篇</span>
          {activeTag && <span className="rounded-full bg-mk-accent-50 px-2 py-0.5 text-[11px] font-bold text-mk-accent">#{activeTag}</span>}
        </div>
        <div className="flex items-center gap-2">
          <button type="button" onClick={() => onExportBib()} className="rounded-full border border-mk-border px-3 py-1.5 text-[12.5px] font-bold text-mk-muted hover:text-mk-accent">导出注释书目</button>
          <button type="button" onClick={onAdd} className="rounded-full bg-mk-accent px-3 py-1.5 text-[12.5px] font-bold text-white hover:bg-mk-accent-600">+ 添加来源</button>
        </div>
      </header>

      {/* batch bar */}
      {nChecked > 0 && (
        <div className="flex items-center gap-3 border-b border-mk-border bg-mk-accent-50 px-5 py-2">
          <span className="text-[13px] font-bold text-mk-accent">已选 {nChecked} 篇</span>
          <button type="button" onClick={() => onExportBib(checked)} className="rounded-mk bg-mk-accent px-3 py-1.5 text-[12.5px] font-bold text-white">导出注释书目</button>
          <button type="button" className="rounded-mk border border-mk-accent/40 bg-mk-surface px-3 py-1.5 text-[12.5px] font-bold text-mk-accent">导出参考文献</button>
          <button type="button" className="rounded-mk border border-mk-accent/40 bg-mk-surface px-3 py-1.5 text-[12.5px] font-bold text-mk-accent">加入合集</button>
          <button type="button" onClick={onClearChecks} className="ml-auto text-[12.5px] font-semibold text-mk-muted hover:text-mk-accent">取消</button>
        </div>
      )}

      {/* Body scrolls both ways: vertical for the list, and horizontal so a
          narrow library never crushes the title column — the columns keep a
          min width and a scrollbar appears instead. */}
      <div className="min-h-0 flex-1 overflow-auto">
        <div className="min-w-[640px]">
          {/* column header — sticky so it stays put while the list scrolls */}
          <div className="sticky top-0 z-10 grid grid-cols-[32px,1fr,140px,64px,88px] items-center gap-2 border-b border-mk-border bg-mk-surface px-5 py-2 text-[11px] font-bold uppercase tracking-wider text-mk-faint">
            <span />
            <span>标题</span>
            <span>来源 · 日期</span>
            <span className="text-center">笔记</span>
            <span>可信度</span>
          </div>
          {rows.map((r) => (
            <Row key={r.id} r={r} active={r.id === selId} checked={props.checked.has(r.id)} onSelect={() => onSelect(r.id)} onCheck={() => onCheck(r.id)} onSetPhase={(p) => onSetPhase(r.id, p)} onSetStatus={(s) => onSetStatus(r.id, s)} />
          ))}
        </div>
      </div>
    </div>
  );
}

const PHASE_OPTIONS: PhaseTag[] = ["立题探索", "背景理解", "支持论点", "反例检验", "方法参考"];

// #2 · a compact inline stage picker. Unset shows a subtle "＋阶段" prompt; set
// shows the stage in an accent pill. Editing persists via the reading brief so
// even older/untagged readings can be labelled from the list.
function PhaseTagPicker({ value, onChange }: { value: PhaseTag | null; onChange: (p: PhaseTag | "") => void }) {
  return (
    <select
      value={value ?? ""}
      onChange={(e) => onChange(e.target.value as PhaseTag | "")}
      onClick={(e) => e.stopPropagation()}
      onMouseDown={(e) => e.stopPropagation()}
      draggable={false}
      aria-label="用于哪个阶段"
      title="标注：这条来源用在哪个阶段"
      className={`flex-none cursor-pointer rounded px-1.5 py-0.5 text-[10px] font-bold outline-none ${value ? "bg-mk-accent-50 text-mk-accent" : "bg-mk-paper text-mk-faint"}`}
    >
      <option value="">＋阶段</option>
      {PHASE_OPTIONS.map((p) => (<option key={p} value={p}>{p}</option>))}
    </select>
  );
}

// A7 · the 待读/在读/读完 shelf-status control — always set (defaults to
// to_read), so unlike PhaseTagPicker there's no empty option. Adopting a paper
// from 探索 sets this to "reading" server-side; this select is how she can move
// it along the shelf (or back) by hand from 图书馆 too.
function ReadingStatusPicker({ value, onChange }: { value: Reference["readingStatus"]; onChange: (s: Reference["readingStatus"]) => void }) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value as Reference["readingStatus"])}
      onClick={(e) => e.stopPropagation()}
      onMouseDown={(e) => e.stopPropagation()}
      draggable={false}
      aria-label="阅读状态"
      title="标注：这条来源读到哪了"
      className={`flex-none cursor-pointer rounded px-1.5 py-0.5 text-[10px] font-bold outline-none ${READING_STATUS_STYLE[value]}`}
    >
      {(["to_read", "reading", "done"] as const).map((s) => (
        <option key={s} value={s}>{READING_STATUS_LABEL[s]}</option>
      ))}
    </select>
  );
}

function Row({ r, active, checked, onSelect, onCheck, onSetPhase, onSetStatus }: { r: Reference; active: boolean; checked: boolean; onSelect: () => void; onCheck: () => void; onSetPhase: (phase: PhaseTag | "") => void; onSetStatus: (status: Reference["readingStatus"]) => void }) {
  const hasRead = r.notes.length > 0;
  // 已归纳/在读/未读 badge — derived, never a separate flag: a finalized
  // takeaway means 已归纳; a linked material with no takeaway yet means she's
  // opened it but hasn't wrapped it up (在读); a saved source with no material
  // yet is 未读 (body never fetched — honest so an added-but-unopened link never
  // looks read). A pending lead keeps its own 待找 badge, not 未读.
  const readingBadge = r.takeaway != null ? "已归纳" : r.materialId != null ? "在读" : r.pending ? null : "未读";
  return (
    <div
      draggable
      onDragStart={(e) => { e.dataTransfer.setData("text/ref", r.id); e.dataTransfer.effectAllowed = "move"; }}
      className={`grid cursor-grab grid-cols-[32px,1fr,140px,64px,88px] items-center gap-2 border-b border-mk-border px-5 py-2.5 transition active:cursor-grabbing ${active ? "bg-mk-accent-50" : "hover:bg-mk-paper"}`}
    >
      <button type="button" onClick={onCheck} className={`flex h-4 w-4 items-center justify-center rounded border ${checked ? "border-mk-accent bg-mk-accent text-white" : "border-mk-input-border bg-mk-surface"}`}>
        {checked && <span className="text-[10px] leading-none">✓</span>}
      </button>
      <div className="min-w-0">
        <div className="flex items-center gap-1.5">
          <button type="button" onClick={onSelect} className="flex min-w-0 flex-1 items-center gap-1.5 text-left">
            <span className={`h-1.5 w-1.5 flex-none rounded-full ${r.pending ? "bg-mk-accent" : hasRead ? "bg-mk-success" : "border border-mk-faint"}`} />
            <span className={`truncate text-[13.5px] font-semibold ${active ? "text-mk-accent" : "text-mk-ink"}`}>{r.title}</span>
          </button>
          {r.pending && <span className="flex-none rounded bg-mk-accent-50 px-1.5 py-0.5 text-[10px] font-bold text-mk-accent">待找</span>}
          {readingBadge && (
            <span
              className={`flex-none rounded px-1.5 py-0.5 text-[10px] font-bold ${
                readingBadge === "已归纳"
                  ? "bg-mk-success-bg text-mk-success"
                  : readingBadge === "在读"
                    ? "bg-mk-accent-50 text-mk-accent"
                    : "bg-mk-paper text-mk-faint"
              }`}
            >
              {readingBadge}
            </span>
          )}
          {/* #2 · which argument-stage this source served — inline-editable */}
          <PhaseTagPicker value={r.phaseTag ?? null} onChange={onSetPhase} />
          {/* A7 · 待读/在读/读完 shelf status — inline-editable, set to 在读
              automatically when adopted from 探索 */}
          <ReadingStatusPicker value={r.readingStatus} onChange={onSetStatus} />
        </div>
        <button type="button" onClick={onSelect} className="mt-0.5 flex gap-1 pl-3 text-left">
          {r.tags.map((t) => (<span key={t} className="text-[10.5px] text-mk-faint">#{t}</span>))}
        </button>
      </div>
      <button type="button" onClick={onSelect} className="truncate text-left text-[12px] text-mk-muted">{r.classification || "—"}{r.year ? ` · ${r.year}` : ""}</button>
      <button type="button" onClick={onSelect} className="text-center text-[12px] font-semibold text-mk-faint">{r.notes.length > 0 ? `✎ ${r.notes.length}` : "—"}</button>
      <button type="button" onClick={onSelect} className="text-left">
        {r.credibility ? <span className={`rounded px-1.5 py-0.5 text-[10.5px] font-bold ${CRED_STYLE[r.credibility]}`}>{CRED_LABEL[r.credibility]}</span> : <span className="text-[11px] text-mk-faint">—</span>}
      </button>
    </div>
  );
}

/* ---------- right · thin preview ---------- */

// refBib projects a reference row's persisted bibliographic metadata (#4) into
// the ReferenceBib the Reading Room header consumes — abstract/journal fall back
// to "" via the contract default, so a source with no DOI simply shows no
// abstract block.
function refBib(r: Reference): ReferenceBib {
  return { title: r.title, author: r.author, year: r.year, journal: r.journal, abstract: r.abstract, url: r.url };
}

function Preview({ projectId, item: r, allTags, onAddTag, onRemoveTag, onPatchNow, onPatchDebounced, onEnterReading }: {
  projectId: string;
  item: Reference;
  allTags: string[];
  onAddTag: (t: string) => void;
  onRemoveTag: (t: string) => void;
  onPatchNow: (p: ReferencePatch) => void;
  onPatchDebounced: (p: ReferencePatch) => void;
  onEnterReading: (
    m: MaterialSource,
    referenceId: string,
    suggestedReason?: string,
    phaseTag?: PhaseTag | null,
    readingReason?: string | null,
    readingFocus?: string | null,
    readingNote?: string | null,
    bib?: ReferenceBib,
  ) => void;
}) {
  const [entering, setEntering] = useState(false);
  const [enterNote, setEnterNote] = useState<string | null>(null);
  const [pendingUrl, setPendingUrl] = useState("");
  // Paste-body fallback (#1): shown when enter-reading can't fetch the source.
  const [showPaste, setShowPaste] = useState(false);
  const [pasteText, setPasteText] = useState("");
  // #4 · DOI metadata recovered when the full text couldn't be fetched.
  const [pasteMeta, setPasteMeta] = useState<SourceMeta | null>(null);
  const [pasteBusy, setPasteBusy] = useState(false);
  const [pasteError, setPasteError] = useState<string | null>(null);

  async function enter() {
    if (entering) return;
    setEnterNote(null);
    setEntering(true);
    try {
      const { source, suggestedReason } = await enterReading(projectId, r.id);
      onEnterReading(source, r.id, suggestedReason, r.phaseTag, r.readingReason, r.readingFocus, r.readingNote, refBib(r));
    } catch (e) {
      if (e instanceof NoReadableContentError) {
        // Fetch failed / no content → let the student paste the body in,
        // showing any DOI metadata + abstract we recovered (#4).
        setEnterNote(e.message);
        setPasteMeta(e.meta ?? null);
        setShowPaste(true);
      } else {
        setEnterNote("打开阅读室失败，请重试");
      }
    } finally {
      setEntering(false);
    }
  }

  async function startPaste() {
    const text = pasteText.trim();
    if (!text || pasteBusy) return;
    setPasteBusy(true);
    setPasteError(null);
    try {
      const source = await pasteContent(projectId, r.id, text);
      onEnterReading(source, r.id, undefined, r.phaseTag, r.readingReason, r.readingFocus, r.readingNote, refBib(r));
    } catch {
      setPasteError("粘贴失败了，再试一次？");
    } finally {
      setPasteBusy(false);
    }
  }

  if (r.pending) {
    return (
      <aside className="flex min-h-0 flex-col overflow-y-auto border-l border-mk-border bg-mk-surface px-5 py-5">
        <span className="w-fit rounded-full bg-mk-accent-50 px-2.5 py-1 text-[11px] font-bold text-mk-accent">还没找到 · 待补充</span>
        <h1 className="mt-3 font-sans text-[16px] font-bold leading-snug text-mk-ink">{r.title}</h1>
        <p className="mt-2 text-[12.5px] leading-relaxed text-mk-muted">印记不替你搜，但能帮你搜得更准：</p>
        <ul className="mt-3 flex flex-col gap-2">
          {r.searchHints?.map((h, i) => (
            <li key={i} className="flex gap-2 text-[12.5px] leading-relaxed text-mk-ink">
              <span className="mt-0.5 flex h-4 w-4 flex-none items-center justify-center rounded-full bg-mk-accent-50 text-[10px] font-bold text-mk-accent">{i + 1}</span>
              {h}
            </li>
          ))}
        </ul>
        <div className="mt-4 flex items-center gap-2 rounded-mk border border-mk-border bg-mk-surface px-2.5 py-2">
          <input
            value={pendingUrl}
            onChange={(e) => setPendingUrl(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter" && pendingUrl.trim()) { onPatchNow({ url: pendingUrl.trim(), pending: false }); setPendingUrl(""); } }}
            placeholder="找到了？粘链接……"
            className="flex-1 bg-transparent text-[12.5px] text-mk-ink outline-none placeholder:text-mk-faint"
          />
          <button type="button" onClick={() => { if (pendingUrl.trim()) { onPatchNow({ url: pendingUrl.trim(), pending: false }); setPendingUrl(""); } }} className="rounded bg-mk-accent px-2.5 py-1 text-[11.5px] font-bold text-white">添加</button>
        </div>
      </aside>
    );
  }
  return (
    <aside className="flex min-h-0 flex-col overflow-y-auto border-l border-mk-border bg-mk-surface px-5 py-5">
      {/* Editable title */}
      <input
        value={r.title}
        onChange={(e) => onPatchDebounced({ title: e.target.value })}
        placeholder="来源标题"
        className="-mx-1 rounded px-1 py-0.5 font-sans text-[16px] font-bold leading-snug text-mk-ink outline-none transition focus:bg-mk-surface"
      />

      {/* Editable metadata */}
      <div className="mt-2 space-y-0.5">
        <MetaEdit k="作者" v={r.author} onChange={(v) => onPatchDebounced({ author: v })} placeholder="作者" />
        <MetaEdit k="分类" v={r.classification} onChange={(v) => onPatchDebounced({ classification: v })} placeholder="期刊/报告/网页…" />
        <MetaEdit k="日期" v={r.year} onChange={(v) => onPatchDebounced({ year: v })} placeholder="年份" />
        <MetaEdit k="链接" v={r.url} onChange={(v) => onPatchDebounced({ url: v })} placeholder="https://…" />
      </div>

      {/* A7 · 待读/在读/读完 shelf status — adopting a paper from 探索 sets this
          to 在读 automatically server-side; editable here too. */}
      <div className="mt-3">
        <p className="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-mk-faint">阅读状态</p>
        <div className="flex gap-1.5">
          {(["to_read", "reading", "done"] as const).map((s) => {
            const on = r.readingStatus === s;
            return (
              <button
                key={s}
                type="button"
                onClick={() => onPatchNow({ readingStatus: s })}
                className={`flex-1 rounded-mk border py-1.5 text-[12.5px] font-bold transition ${on ? `${READING_STATUS_STYLE[s]} border-transparent` : "border-mk-border bg-mk-surface text-mk-faint hover:text-mk-ink"}`}
              >
                {READING_STATUS_LABEL[s]}
              </button>
            );
          })}
        </div>
      </div>

      {/* #5 · 打开原文: an honest external link to the source (readable extraction
          already "opens" the content on 进入阅读室; this opens the live page). */}
      {r.url?.trim() && (
        <a
          href={r.url}
          target="_blank"
          rel="noopener noreferrer"
          className="mt-2 inline-flex items-center gap-1 text-[12px] font-bold text-mk-accent hover:underline"
        >
          打开原文 ↗
        </a>
      )}

      {/* #4 · recovered abstract (from a DOI via Crossref) — collapsible context. */}
      {r.abstract?.trim() && (
        <details className="mt-3 rounded-mk border border-mk-border bg-mk-paper p-3">
          <summary className="cursor-pointer text-[11px] font-bold uppercase tracking-wider text-mk-faint">摘要</summary>
          <p className="mt-2 text-[12.5px] leading-relaxed text-mk-ink">{r.abstract}</p>
        </details>
      )}

      {/* Author credentials — annotated-bib field */}
      <div className="mt-3">
        <p className="mb-1 text-[11px] font-bold uppercase tracking-wider text-mk-faint">作者资历</p>
        <textarea
          value={r.credentials}
          onChange={(e) => onPatchDebounced({ credentials: e.target.value })}
          rows={2}
          placeholder="作者是谁、有什么资历？（注释书目要用）"
          className="w-full resize-none rounded-mk border border-mk-border bg-mk-surface px-2.5 py-1.5 text-[12.5px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent"
        />
      </div>

      <TagEditor tags={r.tags} allTags={allTags} onAdd={onAddTag} onRemove={onRemoveTag} />

      {/* Should I use this resource? — annotated-bib verdict */}
      <div className="mt-4">
        <p className="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-mk-faint">是否采用</p>
        <div className="flex gap-1.5">
          {(["use", "maybe", "drop"] as const).map((d) => {
            const on = r.decision === d;
            const tone = d === "use" ? "bg-mk-success text-white border-mk-success" : d === "maybe" ? "bg-mk-accent text-white border-mk-accent" : "bg-mk-muted text-white border-mk-muted";
            return (
              <button key={d} type="button" onClick={() => onPatchNow({ decision: on ? null : d })} className={`flex-1 rounded-mk border py-1.5 text-[12.5px] font-bold transition ${on ? tone : "border-mk-border bg-mk-surface text-mk-faint hover:text-mk-ink"}`}>
                {DECISION_LABEL[d]}
              </button>
            );
          })}
        </div>
      </div>

      {/* Reliability evaluation — credibility selector + editable notes */}
      <div className="mt-4 rounded-mk border border-mk-border bg-mk-paper p-3">
        <p className="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-mk-faint">可信度评估</p>
        <div className="mb-2 flex gap-1.5">
          {(["strong", "mixed", "weak"] as const).map((c) => {
            const on = r.credibility === c;
            return (
              <button key={c} type="button" onClick={() => onPatchNow({ credibility: on ? null : c })} className={`flex-1 rounded px-1.5 py-1 text-[11px] font-bold transition ${on ? CRED_STYLE[c] : "bg-mk-surface text-mk-faint hover:text-mk-ink"}`}>
                {CRED_LABEL[c]}
              </button>
            );
          })}
        </div>
        <textarea
          value={r.evaluation}
          onChange={(e) => onPatchDebounced({ evaluation: e.target.value })}
          rows={3}
          placeholder="这篇能回答什么 / 不能回答什么？"
          className="w-full resize-none rounded-mk border border-mk-border bg-mk-surface px-2.5 py-1.5 text-[12px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent"
        />
      </div>

      {/* S2 (Task 9): once finalized (完成这篇), the structured takeaway
          supersedes the raw notes list as the reading record for this
          source — findings/credibility/keyQuotes are her already-confirmed
          work, newLeads/proposalImpact are her authored synthesis. */}
      {r.takeaway ? (
        <div className="mt-4 rounded-mk border border-mk-success/40 bg-mk-success-bg p-3">
          <p className="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-mk-success">已归纳 · 阅读成果</p>
          {r.takeaway.findings.length > 0 && (
            <ul className="mb-2 list-disc space-y-1 pl-4 text-[12px] leading-relaxed text-mk-ink">
              {r.takeaway.findings.map((f, i) => (<li key={i}>{f}</li>))}
            </ul>
          )}
          {r.takeaway.credibility.verdict && (
            <p className="mb-2 text-[12px] leading-relaxed text-mk-ink">
              <span className="font-bold">可信度 · {r.takeaway.credibility.verdict}</span>
              {r.takeaway.credibility.why ? ` — ${r.takeaway.credibility.why}` : ""}
            </p>
          )}
          {r.takeaway.keyQuotes.length > 0 && (
            <div className="mb-2 flex flex-col gap-1.5">
              {r.takeaway.keyQuotes.map((q, i) => (
                <p key={i} className="border-l-2 border-mk-success pl-2 text-[12px] leading-relaxed text-mk-ink">“{q.quote}”{q.why ? ` — ${q.why}` : ""}</p>
              ))}
            </div>
          )}
          {r.takeaway.newLeads.length > 0 && (
            <p className="mb-1 text-[12px] leading-relaxed text-mk-ink"><span className="font-bold">新的线索 · </span>{r.takeaway.newLeads.join("；")}</p>
          )}
          {r.takeaway.proposalImpact && (
            <p className="text-[12px] leading-relaxed text-mk-ink"><span className="font-bold">对论点的影响 · </span>{r.takeaway.proposalImpact}</p>
          )}
        </div>
      ) : (
        r.notes.length > 0 && (
          <div className="mt-4">
            <p className="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-mk-faint">阅读笔记 · {r.notes.length}</p>
            <div className="flex flex-col gap-2">
              {r.notes.map((n, i) => (
                <p key={i} className="border-l-2 border-mk-accent pl-2 text-[12px] leading-relaxed text-mk-ink">{n.finding}</p>
              ))}
            </div>
          </div>
        )
      )}

      <div className="mt-5 flex flex-col gap-2">
        <button type="button" onClick={enter} disabled={entering} className="flex items-center justify-center gap-2 rounded-mk bg-mk-accent py-2.5 text-[13.5px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-60">
          {entering ? "打开中…" : <>进入阅读室 <Icon name="arrow" size={15} /></>}
        </button>
        {enterNote ? (
          <p className="text-center text-[11px] font-semibold text-mk-accent">{enterNote}</p>
        ) : (
          <p className="text-center text-[11px] text-mk-faint">和印记逐句共读（已上线的阅读室）</p>
        )}

        {showPaste && (
          <div className="mt-1 rounded-mk border border-mk-border bg-mk-paper p-3">
            <p className="text-[12.5px] font-bold text-mk-ink">取不到正文？把文章正文粘进来</p>
            {/* #4 · what Crossref recovered for a DOI, even without the full text */}
            {pasteMeta && (pasteMeta.title || pasteMeta.author || pasteMeta.abstract) ? (
              <div className="mt-2 rounded-mk border border-mk-accent/25 bg-mk-accent-50 p-2.5">
                <p className="text-[11px] font-bold text-mk-accent">已从 DOI 取到这篇的信息（正文仍需你粘贴）</p>
                {pasteMeta.title && <p className="mt-1 text-[12.5px] font-semibold leading-relaxed text-mk-ink">{pasteMeta.title}</p>}
                {(pasteMeta.author || pasteMeta.year || pasteMeta.journal) && (
                  <p className="mt-0.5 text-[11.5px] text-mk-faint">
                    {[pasteMeta.author, pasteMeta.year, pasteMeta.journal].filter(Boolean).join(" · ")}
                  </p>
                )}
                {pasteMeta.abstract && (
                  <details className="mt-1.5">
                    <summary className="cursor-pointer text-[11.5px] font-semibold text-mk-accent">摘要</summary>
                    <p className="mt-1 max-h-40 overflow-y-auto text-[12px] leading-relaxed text-mk-ink">{pasteMeta.abstract}</p>
                  </details>
                )}
              </div>
            ) : (
              <p className="mt-1 text-[11.5px] leading-relaxed text-mk-faint">有些链接抓不到正文（网站限制 / 网络问题）。把正文复制粘进来，就能和印记逐句共读。</p>
            )}
            <textarea
              value={pasteText}
              onChange={(e) => setPasteText(e.target.value)}
              rows={6}
              placeholder="把文章正文粘到这里……"
              className="mt-2 w-full resize-none rounded-mk border border-mk-border bg-mk-surface px-2.5 py-2 text-[12px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent"
            />
            <button
              type="button"
              onClick={startPaste}
              disabled={pasteBusy || !pasteText.trim()}
              className="mt-2 w-full rounded-mk bg-mk-accent py-2 text-[13px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-60"
            >
              {pasteBusy ? "开始中…" : "开始共读"}
            </button>
            {pasteError && <p className="mt-1.5 text-center text-[11px] font-semibold text-mk-accent">{pasteError}</p>}
          </div>
        )}
      </div>
    </aside>
  );
}

// Tags are added right here in the preview — type a new one or click a
// suggestion drawn from tags already used elsewhere in the library.
function TagEditor({ tags, allTags, onAdd, onRemove }: { tags: string[]; allTags: string[]; onAdd: (t: string) => void; onRemove: (t: string) => void }) {
  const [editing, setEditing] = useState(false);
  const [val, setVal] = useState("");
  const suggestions = allTags.filter((t) => !tags.includes(t) && t.includes(val)).slice(0, 6);
  return (
    <div className="mt-3">
      <p className="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-mk-faint">标签</p>
      <div className="flex flex-wrap items-center gap-1.5">
        {tags.map((t) => (
          <span key={t} className="group flex items-center gap-1 rounded-full bg-mk-accent-50 px-2 py-0.5 text-[11px] font-semibold text-mk-accent">
            #{t}
            <button type="button" onClick={() => onRemove(t)} className="text-mk-accent/50 hover:text-mk-accent">×</button>
          </span>
        ))}
        {editing ? (
          <span className="relative">
            <input
              autoFocus
              value={val}
              onChange={(e) => setVal(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && val.trim()) { onAdd(val); setVal(""); } if (e.key === "Escape") { setEditing(false); setVal(""); } }}
              onBlur={() => { if (val.trim()) onAdd(val); setEditing(false); setVal(""); }}
              placeholder="输入后回车"
              className="w-24 rounded-full border border-mk-accent/40 bg-mk-surface px-2 py-0.5 text-[11px] text-mk-ink outline-none"
            />
            {val && suggestions.length > 0 && (
              <div className="absolute left-0 top-7 z-10 w-40 rounded-mk border border-mk-border bg-mk-surface p-1 shadow-lg">
                {suggestions.map((s) => (
                  <button key={s} type="button" onMouseDown={(e) => { e.preventDefault(); onAdd(s); setVal(""); }} className="block w-full rounded px-2 py-1 text-left text-[12px] text-mk-ink hover:bg-mk-accent-50">#{s}</button>
                ))}
              </div>
            )}
          </span>
        ) : (
          <button type="button" onClick={() => setEditing(true)} className="rounded-full border border-dashed border-mk-input-border px-2 py-0.5 text-[11px] font-semibold text-mk-faint hover:border-mk-accent hover:text-mk-accent">+ 标签</button>
        )}
      </div>
    </div>
  );
}

// Add a source: paste a link / DOI (印记 fills in the metadata) or upload a
// file — and drop it into a collection. No auto-fetching of the source's
// *content*; this only registers the reference.
function AddSourceModal({ collections, defaultCollection, onClose, onSubmit, onPaste }: { collections: Collection[]; defaultCollection: string; onClose: () => void; onSubmit: (s: { title: string; url: string; classification: string; collectionId: string | null }) => void; onPaste: (s: { title: string; text: string; collectionId: string | null }) => void }) {
  const [tab, setTab] = useState<"link" | "paste" | "upload" | "manual">("link");
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const [pasteBody, setPasteBody] = useState("");
  const [fileName, setFileName] = useState("");
  const [coll, setColl] = useState(defaultCollection);
  const [pasting, setPasting] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  function submit() {
    const collectionId = coll || null;
    if (tab === "paste") {
      if (!pasteBody.trim() || pasting) return;
      setPasting(true);
      onPaste({ title: title.trim() || "粘贴正文", text: pasteBody.trim(), collectionId });
    } else if (tab === "upload") {
      onSubmit({ title: fileName || "上传文档", url: "", classification: "上传文档", collectionId });
    } else if (tab === "manual") {
      onSubmit({ title: title || "新来源", url: "", classification: "", collectionId });
    } else {
      onSubmit({ title: url ? url.replace(/^https?:\/\//, "").slice(0, 32) : "新来源", url, classification: "网页", collectionId });
    }
  }

  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-black/40 px-6" onClick={onClose}>
      <div className="w-[440px] rounded-mk-lg border border-mk-border bg-mk-surface p-5 shadow-[0_20px_60px_rgba(28,35,51,0.25)]" onClick={(e) => e.stopPropagation()}>
        <div className="mb-3 flex items-center justify-between">
          <h3 className="font-sans text-[16px] font-bold text-mk-ink">添加来源</h3>
          <button type="button" onClick={onClose} className="text-[18px] leading-none text-mk-faint hover:text-mk-ink">×</button>
        </div>

        <div className="mb-4 flex rounded-mk border border-mk-border bg-mk-paper p-0.5 text-[12px] font-bold">
          {(["link", "paste", "upload", "manual"] as const).map((t) => (
            <button key={t} type="button" onClick={() => setTab(t)} className={`flex-1 rounded-[10px] py-1.5 transition ${tab === t ? "bg-mk-surface text-mk-accent shadow-sm" : "text-mk-faint"}`}>
              {t === "link" ? "链接 / DOI" : t === "paste" ? "粘贴正文" : t === "upload" ? "上传文件" : "手动填写"}
            </button>
          ))}
        </div>

        {tab === "link" && (
          <div>
            <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://…  或  10.1038/s41893-…" className="w-full rounded-mk border border-mk-border bg-mk-surface px-3 py-2.5 text-[13.5px] text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent" />
            <p className="mt-1.5 text-[12px] text-mk-faint">印记会抓取标题、作者、日期——你可以再改。</p>
          </div>
        )}
        {tab === "paste" && (
          <div className="space-y-2">
            <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="来源标题（可留空）" className="w-full rounded-mk border border-mk-border bg-mk-surface px-3 py-2 text-[13px] text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent" />
            <textarea value={pasteBody} onChange={(e) => setPasteBody(e.target.value)} rows={6} placeholder="把文章正文粘到这里，直接进阅读室和印记逐句共读……" className="w-full resize-none rounded-mk border border-mk-border bg-mk-surface px-3 py-2 text-[12.5px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent" />
            <p className="text-[12px] text-mk-faint">链接抓不到正文时用这个——粘完就打开阅读室。</p>
          </div>
        )}
        {tab === "upload" && (
          <div>
            <input ref={fileInput} type="file" className="hidden" onChange={(e) => setFileName(e.target.files?.[0]?.name ?? "")} />
            <button type="button" onClick={() => fileInput.current?.click()} className="flex w-full flex-col items-center gap-1.5 rounded-mk border border-dashed border-mk-input-border bg-mk-surface px-4 py-8 text-center hover:border-mk-accent">
              <span className="text-mk-accent"><Icon name="reading" size={22} /></span>
              <span className="text-[13px] font-bold text-mk-ink">{fileName || "把 PDF / 文档拖到这里"}</span>
              <span className="text-[12px] text-mk-faint">{fileName ? "点击重新选择" : "或点击选择文件"}</span>
            </button>
          </div>
        )}
        {tab === "manual" && (
          <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="来源标题" className="w-full rounded-mk border border-mk-border bg-mk-surface px-3 py-2.5 text-[13.5px] text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent" />
        )}

        <div className="mt-4">
          <label className="mb-1.5 block text-[11px] font-bold uppercase tracking-wider text-mk-faint">放进合集</label>
          <select value={coll} onChange={(e) => setColl(e.target.value)} className="w-full rounded-mk border border-mk-border bg-mk-surface px-3 py-2 text-[13.5px] text-mk-ink outline-none focus:border-mk-accent">
            <option value="">未归类</option>
            {collections.map((c) => (<option key={c.id} value={c.id}>{c.parentId ? "— " : ""}{c.name}</option>))}
          </select>
        </div>

        <div className="mt-5 flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-mk border border-mk-border px-4 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-accent">取消</button>
          <button type="button" onClick={submit} disabled={tab === "paste" && (pasting || !pasteBody.trim())} className="rounded-mk bg-mk-accent px-4 py-2 text-[13px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-60">{tab === "paste" ? (pasting ? "打开中…" : "开始共读") : "添加"}</button>
        </div>
      </div>
    </div>
  );
}

// An inline-editable metadata row — reads as text until you focus it.
function MetaEdit({ k, v, onChange, placeholder }: { k: string; v: string; onChange: (v: string) => void; placeholder?: string }) {
  return (
    <div className="flex items-center gap-2 text-[12.5px]">
      <span className="w-10 flex-none font-semibold text-mk-faint">{k}</span>
      <input
        value={v === "—" ? "" : v}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="min-w-0 flex-1 rounded bg-transparent px-1 py-0.5 text-mk-ink outline-none transition placeholder:text-mk-faint hover:bg-mk-paper focus:bg-mk-surface"
      />
    </div>
  );
}

/* ---------- empty state ---------- */

function EmptyLibrary({ onAdd, topic }: { onAdd: () => void; topic?: string }) {
  return (
    <div className="flex h-full flex-col items-center justify-center px-6 text-center">
      <div className="mb-5 flex h-16 w-16 items-center justify-center rounded-full bg-mk-accent-50 text-mk-accent">
        <Icon name="reading" size={30} />
      </div>
      <h2 className="font-sans text-[22px] font-bold text-mk-ink">你的文献库还是空的</h2>
      {/* #3 · when the topic is known, acknowledge it here too instead of the
          generic prompt — the platform already has it, no need to re-ask. */}
      <p className="mt-2 max-w-md text-[14px] leading-relaxed text-mk-muted">
        {topic
          ? `围绕「${topic}」，先加一篇来源——一个链接、一份 PDF，或手动填写都行。`
          : "先加一篇来源——一个链接、一份 PDF，或手动填写都行。"}
        <br />
        印记不替你搜，但你不知道去哪找、找到了不确定可不可信，随时问问旁边的印记。
      </p>
      <div className="mt-6 flex items-center gap-3">
        <button type="button" onClick={onAdd} className="rounded-mk bg-mk-accent px-5 py-2.5 text-[14px] font-bold text-white hover:bg-mk-accent-600">+ 添加第一篇来源</button>
        <span className="text-[13px] text-mk-faint">或从「项目管理」里点一个「读」任务进来</span>
      </div>
    </div>
  );
}

// Task 3 (P2a): the room's own FloatingCoach (a context-isolated "find_sources"
// thread, docked in 探索图谱 or a floating chip in 列表) is deleted — 印记 is now
// the ONE constant rail, portaled from the return above like every other room.
