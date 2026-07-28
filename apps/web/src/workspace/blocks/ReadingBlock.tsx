import { useEffect, useMemo, useRef, useState } from "react";
import type { Collection, MaterialSource, Reference } from "@mind-imprint/contracts";
import { Icon } from "../Icon";
import {
  getLibrary,
  createCollection,
  createReference,
  patchReference,
  enterReading,
  coach,
  NoReadableContentError,
  type ReferencePatch,
} from "../api/workspace";

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
  strong: "bg-mk-green-tint text-mk-green",
  mixed: "bg-mk-accent-tint text-mk-accent",
  weak: "bg-mk-bg text-mk-muted",
};

type ChatMsg = { role: "ai" | "student"; text: string };

// The Reading block = a Zotero-shaped Library: collections + tags (left) for
// categorization, a reference table (center) that scales to many sources with
// multi-select batch export, a thin preview (right), and a floating 印记 for
// coach-the-hunt help. The deep read-together AI lives in the shipped Reading
// Room, so the Library keeps AI on-tap rather than in a permanent column.
//
// All state is now persisted through the workspace API (slice 3): the library
// loads on mount; edits patch optimistically; 进入阅读室 mints/loads a real
// MaterialSource and hands it to the container's reading-room swap slot.
export function ReadingBlock({
  projectId,
  setReadingSource,
}: {
  projectId: string;
  setReadingSource: (m: MaterialSource) => void;
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

  // Debounce timers for free-text metadata edits, keyed by ref+field so each
  // field coalesces independently.
  const debounceTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  const reload = useMemo(
    () => async () => {
      try {
        const lib = await getLibrary(projectId);
        setRefs(lib.references);
        setCollections(lib.collections);
      } catch {
        /* keep the last-good library; the room stays usable */
      }
    },
    [projectId],
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

  // The annotated bibliography — the submittable table, straight from the
  // library. Columns mirror the school's 资源评估表 form.
  function exportAnnotatedBib(ids?: Set<string>) {
    const rows = refs.filter((r) => !r.pending && (!ids || ids.has(r.id)));
    const head = "| 资源 | 分类 | 作者 | 作者资历 | 期刊/网站 | 相关性（引用片段） | 可信度评估 | 是否采用 |";
    const sep = "| --- | --- | --- | --- | --- | --- | --- | --- |";
    const lines = rows.map((r) => {
      const rel = r.notes.map((n) => `「${n.quote}」→ ${n.finding}`).join("；") || "—";
      const dec = r.decision ? DECISION_LABEL[r.decision] : "未定";
      return `| ${r.title} | ${r.classification || "—"} | ${r.author || "—"} | ${r.credentials || "—"} | ${r.url || "—"} | ${rel} | ${r.evaluation || "—"} | ${dec} |`;
    });
    const body = ["# 注释书目 Annotated Bibliography", "", head, sep, ...lines].join("\n");
    const blob = new Blob([body], { type: "text/markdown;charset=utf-8" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "注释书目.md";
    a.click();
    URL.revokeObjectURL(a.href);
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
    />
  );

  // Still loading the library.
  if (loading) {
    return <div className="flex h-full items-center justify-center text-[14px] text-mk-muted-2">加载中…</div>;
  }

  // Empty library — a brand-new project with no sources yet.
  if (refs.length === 0) {
    return (
      <div className="relative h-full">
        <EmptyLibrary onAdd={() => setAdding(true)} />
        <FloatingCoach projectId={projectId} />
        {modal}
      </div>
    );
  }

  return (
    <div className="relative grid h-full" style={{ gridTemplateColumns: `${railOpen ? "220px" : "48px"} 1fr 300px` }}>
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

      <FloatingCoach projectId={projectId} />

      {modal}
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
        <button type="button" onClick={onToggle} title="展开合集" className="flex h-8 w-8 items-center justify-center rounded-mk text-mk-muted-2 hover:bg-mk-bg hover:text-mk-primary">
          <Chevron dir="right" />
        </button>
        <div className="mt-2 text-mk-muted-2"><FolderGlyph /></div>
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
        <span className="text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">文献库</span>
        <button type="button" onClick={onToggle} title="收起合集" className="flex h-6 w-6 items-center justify-center rounded text-mk-muted-2 hover:bg-mk-bg hover:text-mk-primary">
          <Chevron dir="left" />
        </button>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-2.5 py-2">
        <CollRow label="全部文献" count={total} active={collId === "all"} onClick={() => onPick("all")} icon="reading" {...dropProps("all")} />
        <div className="mt-3 mb-1.5 flex items-center justify-between px-2">
          <span className="text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">我的合集</span>
          <button type="button" onClick={() => setCreating(true)} className="text-[15px] leading-none text-mk-muted-2 hover:text-mk-primary">+</button>
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
              className="w-full rounded-mk border border-mk-primary/40 bg-mk-surface px-2 py-1 text-[12.5px] text-mk-ink outline-none placeholder:text-mk-muted-2"
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

        <div className="mt-4 mb-1.5 px-2 text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">标签</div>
        <div className="flex flex-wrap gap-1.5 px-1.5">
          {tags.map((t) => (
            <button
              key={t}
              type="button"
              onClick={() => onTag(activeTag === t ? null : t)}
              className={`rounded-full px-2 py-0.5 text-[11.5px] font-semibold transition ${activeTag === t ? "bg-mk-primary text-white" : "bg-mk-bg text-mk-muted hover:text-mk-primary"}`}
            >
              {t}
            </button>
          ))}
        </div>
      </div>
      <div className="border-t border-mk-border px-4 py-2.5 text-[11px] text-mk-muted-2">把文献拖到合集上归类</div>
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
      className={`flex w-full items-center gap-1 rounded-mk pr-2 transition ${indent ? "pl-4" : "pl-1"} ${isDrop ? "bg-mk-primary/15 ring-1 ring-mk-primary/40" : active ? "bg-mk-primary-tint" : "hover:bg-mk-bg"}`}
    >
      {caret ? (
        <button type="button" onClick={onCaret} className="flex h-5 w-4 flex-none items-center justify-center text-mk-muted-2 hover:text-mk-primary">
          <Chevron dir={caret === "open" ? "down" : "right"} small />
        </button>
      ) : (
        <span className="w-4 flex-none" />
      )}
      <button type="button" onClick={onClick} className="flex flex-1 items-center gap-2 py-1.5 text-left">
        <span className={active ? "text-mk-primary" : "text-mk-muted-2"}>{icon ? <Icon name={icon} size={15} /> : <FolderGlyph />}</span>
        <span className={`flex-1 truncate text-[13px] font-semibold ${active ? "text-mk-primary" : "text-mk-ink"}`}>{label}</span>
        <span className="text-[11px] font-semibold text-mk-muted-2">{count}</span>
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
}) {
  const { rows, selId, onSelect, checked, onCheck, onClearChecks, onExportBib, collName, activeTag, onAdd } = props;
  const nChecked = checked.size;
  return (
    <div className="flex min-h-0 flex-col bg-mk-surface">
      {/* toolbar */}
      <header className="flex items-center justify-between border-b border-mk-border px-5 py-3">
        <div className="flex items-baseline gap-2">
          <h2 className="font-sans text-[16px] font-bold text-mk-ink">{collName}</h2>
          <span className="text-[12px] font-semibold text-mk-muted-2">{rows.length} 篇</span>
          {activeTag && <span className="rounded-full bg-mk-primary-tint px-2 py-0.5 text-[11px] font-bold text-mk-primary">#{activeTag}</span>}
        </div>
        <div className="flex items-center gap-2">
          <button type="button" onClick={() => onExportBib()} className="rounded-full border border-mk-border px-3 py-1.5 text-[12.5px] font-bold text-mk-muted hover:text-mk-primary">导出注释书目</button>
          <button type="button" onClick={onAdd} className="rounded-full bg-mk-primary px-3 py-1.5 text-[12.5px] font-bold text-white hover:bg-mk-primary-hover">+ 添加来源</button>
        </div>
      </header>

      {/* batch bar */}
      {nChecked > 0 && (
        <div className="flex items-center gap-3 border-b border-mk-border bg-mk-primary-tint px-5 py-2">
          <span className="text-[13px] font-bold text-mk-primary">已选 {nChecked} 篇</span>
          <button type="button" onClick={() => onExportBib(checked)} className="rounded-mk bg-mk-primary px-3 py-1.5 text-[12.5px] font-bold text-white">导出注释书目</button>
          <button type="button" className="rounded-mk border border-mk-primary/40 bg-mk-surface px-3 py-1.5 text-[12.5px] font-bold text-mk-primary">导出参考文献</button>
          <button type="button" className="rounded-mk border border-mk-primary/40 bg-mk-surface px-3 py-1.5 text-[12.5px] font-bold text-mk-primary">加入合集</button>
          <button type="button" onClick={onClearChecks} className="ml-auto text-[12.5px] font-semibold text-mk-muted hover:text-mk-primary">取消</button>
        </div>
      )}

      {/* column header */}
      <div className="grid grid-cols-[32px,1fr,140px,64px,88px] items-center gap-2 border-b border-mk-border-2 px-5 py-2 text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">
        <span />
        <span>标题</span>
        <span>来源 · 日期</span>
        <span className="text-center">笔记</span>
        <span>可信度</span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {rows.map((r) => (
          <Row key={r.id} r={r} active={r.id === selId} checked={props.checked.has(r.id)} onSelect={() => onSelect(r.id)} onCheck={() => onCheck(r.id)} />
        ))}
      </div>
    </div>
  );
}

function Row({ r, active, checked, onSelect, onCheck }: { r: Reference; active: boolean; checked: boolean; onSelect: () => void; onCheck: () => void }) {
  const hasRead = r.notes.length > 0;
  return (
    <div
      draggable
      onDragStart={(e) => { e.dataTransfer.setData("text/ref", r.id); e.dataTransfer.effectAllowed = "move"; }}
      className={`grid cursor-grab grid-cols-[32px,1fr,140px,64px,88px] items-center gap-2 border-b border-mk-border-2 px-5 py-2.5 transition active:cursor-grabbing ${active ? "bg-mk-primary-tint/70" : "hover:bg-mk-primary-tint/30"}`}
    >
      <button type="button" onClick={onCheck} className={`flex h-4 w-4 items-center justify-center rounded border ${checked ? "border-mk-primary bg-mk-primary text-white" : "border-mk-input bg-mk-surface"}`}>
        {checked && <span className="text-[10px] leading-none">✓</span>}
      </button>
      <button type="button" onClick={onSelect} className="min-w-0 text-left">
        <div className="flex items-center gap-1.5">
          <span className={`h-1.5 w-1.5 flex-none rounded-full ${r.pending ? "bg-mk-accent" : hasRead ? "bg-mk-green" : "border border-mk-muted-2"}`} />
          <span className={`truncate text-[13.5px] font-semibold ${active ? "text-mk-primary" : "text-mk-ink"}`}>{r.title}</span>
          {r.pending && <span className="flex-none rounded bg-mk-accent-tint px-1.5 py-0.5 text-[10px] font-bold text-mk-accent">待找</span>}
        </div>
        <div className="mt-0.5 flex gap-1 pl-3">
          {r.tags.map((t) => (<span key={t} className="text-[10.5px] text-mk-muted-2">#{t}</span>))}
        </div>
      </button>
      <button type="button" onClick={onSelect} className="truncate text-left text-[12px] text-mk-muted">{r.classification || "—"}{r.year ? ` · ${r.year}` : ""}</button>
      <button type="button" onClick={onSelect} className="text-center text-[12px] font-semibold text-mk-muted-2">{r.notes.length > 0 ? `✎ ${r.notes.length}` : "—"}</button>
      <button type="button" onClick={onSelect} className="text-left">
        {r.credibility ? <span className={`rounded px-1.5 py-0.5 text-[10.5px] font-bold ${CRED_STYLE[r.credibility]}`}>{CRED_LABEL[r.credibility]}</span> : <span className="text-[11px] text-mk-muted-2">—</span>}
      </button>
    </div>
  );
}

/* ---------- right · thin preview ---------- */

function Preview({ projectId, item: r, allTags, onAddTag, onRemoveTag, onPatchNow, onPatchDebounced, onEnterReading }: {
  projectId: string;
  item: Reference;
  allTags: string[];
  onAddTag: (t: string) => void;
  onRemoveTag: (t: string) => void;
  onPatchNow: (p: ReferencePatch) => void;
  onPatchDebounced: (p: ReferencePatch) => void;
  onEnterReading: (m: MaterialSource) => void;
}) {
  const [entering, setEntering] = useState(false);
  const [enterNote, setEnterNote] = useState<string | null>(null);
  const [pendingUrl, setPendingUrl] = useState("");

  async function enter() {
    if (entering) return;
    setEnterNote(null);
    setEntering(true);
    try {
      const source = await enterReading(projectId, r.id);
      onEnterReading(source);
    } catch (e) {
      if (e instanceof NoReadableContentError) {
        setEnterNote(e.message);
      } else {
        setEnterNote("打开阅读室失败，请重试");
      }
    } finally {
      setEntering(false);
    }
  }

  if (r.pending) {
    return (
      <aside className="flex min-h-0 flex-col overflow-y-auto border-l border-mk-border bg-mk-surface px-5 py-5">
        <span className="w-fit rounded-full bg-mk-accent-tint px-2.5 py-1 text-[11px] font-bold text-mk-accent">还没找到 · 待补充</span>
        <h1 className="mt-3 font-sans text-[16px] font-bold leading-snug text-mk-ink">{r.title}</h1>
        <p className="mt-2 text-[12.5px] leading-relaxed text-mk-muted">印记不替你搜，但能帮你搜得更准：</p>
        <ul className="mt-3 flex flex-col gap-2">
          {r.searchHints?.map((h, i) => (
            <li key={i} className="flex gap-2 text-[12.5px] leading-relaxed text-mk-ink">
              <span className="mt-0.5 flex h-4 w-4 flex-none items-center justify-center rounded-full bg-mk-primary-tint text-[10px] font-bold text-mk-primary">{i + 1}</span>
              {h}
            </li>
          ))}
        </ul>
        <div className="mt-4 flex items-center gap-2 rounded-mk border border-mk-border bg-mk-input-bg px-2.5 py-2">
          <input
            value={pendingUrl}
            onChange={(e) => setPendingUrl(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter" && pendingUrl.trim()) { onPatchNow({ url: pendingUrl.trim(), pending: false }); setPendingUrl(""); } }}
            placeholder="找到了？粘链接……"
            className="flex-1 bg-transparent text-[12.5px] text-mk-ink outline-none placeholder:text-mk-muted-2"
          />
          <button type="button" onClick={() => { if (pendingUrl.trim()) { onPatchNow({ url: pendingUrl.trim(), pending: false }); setPendingUrl(""); } }} className="rounded bg-mk-primary px-2.5 py-1 text-[11.5px] font-bold text-white">添加</button>
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
        className="-mx-1 rounded px-1 py-0.5 font-sans text-[16px] font-bold leading-snug text-mk-ink outline-none transition focus:bg-mk-input-bg"
      />

      {/* Editable metadata */}
      <div className="mt-2 space-y-0.5">
        <MetaEdit k="作者" v={r.author} onChange={(v) => onPatchDebounced({ author: v })} placeholder="作者" />
        <MetaEdit k="分类" v={r.classification} onChange={(v) => onPatchDebounced({ classification: v })} placeholder="期刊/报告/网页…" />
        <MetaEdit k="日期" v={r.year} onChange={(v) => onPatchDebounced({ year: v })} placeholder="年份" />
        <MetaEdit k="链接" v={r.url} onChange={(v) => onPatchDebounced({ url: v })} placeholder="https://…" />
      </div>

      {/* Author credentials — annotated-bib field */}
      <div className="mt-3">
        <p className="mb-1 text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">作者资历</p>
        <textarea
          value={r.credentials}
          onChange={(e) => onPatchDebounced({ credentials: e.target.value })}
          rows={2}
          placeholder="作者是谁、有什么资历？（注释书目要用）"
          className="w-full resize-none rounded-mk border border-mk-border bg-mk-input-bg px-2.5 py-1.5 text-[12.5px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
        />
      </div>

      <TagEditor tags={r.tags} allTags={allTags} onAdd={onAddTag} onRemove={onRemoveTag} />

      {/* Should I use this resource? — annotated-bib verdict */}
      <div className="mt-4">
        <p className="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">是否采用</p>
        <div className="flex gap-1.5">
          {(["use", "maybe", "drop"] as const).map((d) => {
            const on = r.decision === d;
            const tone = d === "use" ? "bg-mk-green text-white border-mk-green" : d === "maybe" ? "bg-mk-accent text-white border-mk-accent" : "bg-mk-muted text-white border-mk-muted";
            return (
              <button key={d} type="button" onClick={() => onPatchNow({ decision: on ? null : d })} className={`flex-1 rounded-mk border py-1.5 text-[12.5px] font-bold transition ${on ? tone : "border-mk-border bg-mk-surface text-mk-muted-2 hover:text-mk-ink"}`}>
                {DECISION_LABEL[d]}
              </button>
            );
          })}
        </div>
      </div>

      {/* Reliability evaluation — credibility selector + editable notes */}
      <div className="mt-4 rounded-mk border border-mk-border bg-mk-bg/50 p-3">
        <p className="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">可信度评估</p>
        <div className="mb-2 flex gap-1.5">
          {(["strong", "mixed", "weak"] as const).map((c) => {
            const on = r.credibility === c;
            return (
              <button key={c} type="button" onClick={() => onPatchNow({ credibility: on ? null : c })} className={`flex-1 rounded px-1.5 py-1 text-[11px] font-bold transition ${on ? CRED_STYLE[c] : "bg-mk-surface text-mk-muted-2 hover:text-mk-ink"}`}>
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
          className="w-full resize-none rounded-mk border border-mk-border bg-mk-surface px-2.5 py-1.5 text-[12px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
        />
      </div>

      {r.notes.length > 0 && (
        <div className="mt-4">
          <p className="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">阅读笔记 · {r.notes.length}</p>
          <div className="flex flex-col gap-2">
            {r.notes.map((n, i) => (
              <p key={i} className="border-l-2 border-mk-accent pl-2 text-[12px] leading-relaxed text-mk-ink">{n.finding}</p>
            ))}
          </div>
        </div>
      )}

      <div className="mt-5 flex flex-col gap-2">
        <button type="button" onClick={enter} disabled={entering} className="flex items-center justify-center gap-2 rounded-mk bg-mk-primary py-2.5 text-[13.5px] font-bold text-white hover:bg-mk-primary-hover disabled:opacity-60">
          {entering ? "打开中…" : <>进入阅读室 <Icon name="arrow" size={15} /></>}
        </button>
        {enterNote ? (
          <p className="text-center text-[11px] font-semibold text-mk-accent">{enterNote}</p>
        ) : (
          <p className="text-center text-[11px] text-mk-muted-2">和印记逐句共读（已上线的阅读室）</p>
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
      <p className="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">标签</p>
      <div className="flex flex-wrap items-center gap-1.5">
        {tags.map((t) => (
          <span key={t} className="group flex items-center gap-1 rounded-full bg-mk-primary-tint px-2 py-0.5 text-[11px] font-semibold text-mk-primary">
            #{t}
            <button type="button" onClick={() => onRemove(t)} className="text-mk-primary/50 hover:text-mk-primary">×</button>
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
              className="w-24 rounded-full border border-mk-primary/40 bg-mk-surface px-2 py-0.5 text-[11px] text-mk-ink outline-none"
            />
            {val && suggestions.length > 0 && (
              <div className="absolute left-0 top-7 z-10 w-40 rounded-mk border border-mk-border bg-mk-surface p-1 shadow-lg">
                {suggestions.map((s) => (
                  <button key={s} type="button" onMouseDown={(e) => { e.preventDefault(); onAdd(s); setVal(""); }} className="block w-full rounded px-2 py-1 text-left text-[12px] text-mk-ink hover:bg-mk-primary-tint">#{s}</button>
                ))}
              </div>
            )}
          </span>
        ) : (
          <button type="button" onClick={() => setEditing(true)} className="rounded-full border border-dashed border-mk-input px-2 py-0.5 text-[11px] font-semibold text-mk-muted-2 hover:border-mk-primary hover:text-mk-primary">+ 标签</button>
        )}
      </div>
    </div>
  );
}

// Add a source: paste a link / DOI (印记 fills in the metadata) or upload a
// file — and drop it into a collection. No auto-fetching of the source's
// *content*; this only registers the reference.
function AddSourceModal({ collections, defaultCollection, onClose, onSubmit }: { collections: Collection[]; defaultCollection: string; onClose: () => void; onSubmit: (s: { title: string; url: string; classification: string; collectionId: string | null }) => void }) {
  const [tab, setTab] = useState<"link" | "upload" | "manual">("link");
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const [fileName, setFileName] = useState("");
  const [coll, setColl] = useState(defaultCollection);
  const fileInput = useRef<HTMLInputElement>(null);

  function submit() {
    const collectionId = coll || null;
    if (tab === "upload") {
      onSubmit({ title: fileName || "上传文档", url: "", classification: "上传文档", collectionId });
    } else if (tab === "manual") {
      onSubmit({ title: title || "新来源", url: "", classification: "", collectionId });
    } else {
      onSubmit({ title: url ? url.replace(/^https?:\/\//, "").slice(0, 32) : "新来源", url, classification: "网页", collectionId });
    }
  }

  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-mk-ink/30 px-6" onClick={onClose}>
      <div className="w-[440px] rounded-mk-lg border border-mk-border bg-mk-surface p-5 shadow-[0_20px_60px_rgba(28,35,51,0.25)]" onClick={(e) => e.stopPropagation()}>
        <div className="mb-3 flex items-center justify-between">
          <h3 className="font-sans text-[16px] font-bold text-mk-ink">添加来源</h3>
          <button type="button" onClick={onClose} className="text-[18px] leading-none text-mk-muted-2 hover:text-mk-ink">×</button>
        </div>

        <div className="mb-4 flex rounded-mk border border-mk-border bg-mk-bg p-0.5 text-[12.5px] font-bold">
          {(["link", "upload", "manual"] as const).map((t) => (
            <button key={t} type="button" onClick={() => setTab(t)} className={`flex-1 rounded-[10px] py-1.5 transition ${tab === t ? "bg-mk-surface text-mk-primary shadow-sm" : "text-mk-muted-2"}`}>
              {t === "link" ? "粘贴链接 / DOI" : t === "upload" ? "上传文件" : "手动填写"}
            </button>
          ))}
        </div>

        {tab === "link" && (
          <div>
            <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://…  或  10.1038/s41893-…" className="w-full rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2.5 text-[13.5px] text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary" />
            <p className="mt-1.5 text-[12px] text-mk-muted-2">印记会抓取标题、作者、日期——你可以再改。</p>
          </div>
        )}
        {tab === "upload" && (
          <div>
            <input ref={fileInput} type="file" className="hidden" onChange={(e) => setFileName(e.target.files?.[0]?.name ?? "")} />
            <button type="button" onClick={() => fileInput.current?.click()} className="flex w-full flex-col items-center gap-1.5 rounded-mk border border-dashed border-mk-input bg-mk-input-bg/60 px-4 py-8 text-center hover:border-mk-primary">
              <span className="text-mk-primary"><Icon name="reading" size={22} /></span>
              <span className="text-[13px] font-bold text-mk-ink">{fileName || "把 PDF / 文档拖到这里"}</span>
              <span className="text-[12px] text-mk-muted-2">{fileName ? "点击重新选择" : "或点击选择文件"}</span>
            </button>
          </div>
        )}
        {tab === "manual" && (
          <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="来源标题" className="w-full rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2.5 text-[13.5px] text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary" />
        )}

        <div className="mt-4">
          <label className="mb-1.5 block text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">放进合集</label>
          <select value={coll} onChange={(e) => setColl(e.target.value)} className="w-full rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2 text-[13.5px] text-mk-ink outline-none focus:border-mk-primary">
            <option value="">未归类</option>
            {collections.map((c) => (<option key={c.id} value={c.id}>{c.parentId ? "— " : ""}{c.name}</option>))}
          </select>
        </div>

        <div className="mt-5 flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-mk border border-mk-border px-4 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-primary">取消</button>
          <button type="button" onClick={submit} className="rounded-mk bg-mk-primary px-4 py-2 text-[13px] font-bold text-white hover:bg-mk-primary-hover">添加</button>
        </div>
      </div>
    </div>
  );
}

// An inline-editable metadata row — reads as text until you focus it.
function MetaEdit({ k, v, onChange, placeholder }: { k: string; v: string; onChange: (v: string) => void; placeholder?: string }) {
  return (
    <div className="flex items-center gap-2 text-[12.5px]">
      <span className="w-10 flex-none font-semibold text-mk-muted-2">{k}</span>
      <input
        value={v === "—" ? "" : v}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="min-w-0 flex-1 rounded bg-transparent px-1 py-0.5 text-mk-ink outline-none transition placeholder:text-mk-muted-2 hover:bg-mk-bg focus:bg-mk-input-bg"
      />
    </div>
  );
}

/* ---------- empty state ---------- */

function EmptyLibrary({ onAdd }: { onAdd: () => void }) {
  return (
    <div className="flex h-full flex-col items-center justify-center px-6 text-center">
      <div className="mb-5 flex h-16 w-16 items-center justify-center rounded-full bg-mk-primary-tint text-mk-primary">
        <Icon name="reading" size={30} />
      </div>
      <h2 className="font-sans text-[22px] font-bold text-mk-ink">你的文献库还是空的</h2>
      <p className="mt-2 max-w-md text-[14px] leading-relaxed text-mk-muted">
        先加一篇来源——一个链接、一份 PDF，或手动填写都行。<br />
        印记不替你搜，但你不知道去哪找、找到了不确定可不可信，随时右下角问它。
      </p>
      <div className="mt-6 flex items-center gap-3">
        <button type="button" onClick={onAdd} className="rounded-mk bg-mk-primary px-5 py-2.5 text-[14px] font-bold text-white hover:bg-mk-primary-hover">+ 添加第一篇来源</button>
        <span className="text-[13px] text-mk-muted-2">或从「项目管理」里点一个「读」任务进来</span>
      </div>
    </div>
  );
}

/* ---------- floating coach ---------- */

function FloatingCoach({ projectId }: { projectId: string }) {
  const [open, setOpen] = useState(false);
  const [chat, setChat] = useState<ChatMsg[]>([
    { role: "ai", text: "找资料卡住了？告诉我你想证明什么，我帮你想从哪找、怎么判断可不可信。" },
  ]);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);

  async function send() {
    const text = draft.trim();
    if (!text || busy) return;
    setChat((c) => [...c, { role: "student", text }]);
    setDraft("");
    setBusy(true);
    try {
      const reply = await coach(projectId, "find_sources", text);
      setChat((c) => [...c, { role: "ai", text: reply }]);
    } catch {
      setChat((c) => [...c, { role: "ai", text: "刚才没接上，再问我一次？" }]);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="absolute bottom-5 right-5 z-20 flex flex-col items-end">
      {open && (
        <div className="mb-3 flex h-[440px] w-[350px] flex-col overflow-hidden rounded-mk-lg border border-mk-border bg-mk-surface shadow-[0_12px_40px_rgba(28,35,51,0.18)]">
          <header className="flex items-center justify-between border-b border-mk-border px-4 py-3">
            <div className="flex items-center gap-2 text-mk-primary">
              <Icon name="spark" size={15} />
              <span className="text-[13.5px] font-bold">印记 · 找资料</span>
            </div>
            <button type="button" onClick={() => setOpen(false)} className="text-[16px] leading-none text-mk-muted-2 hover:text-mk-ink">×</button>
          </header>
          <div className="flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto px-3.5 py-3.5">
            {chat.map((m, i) => (
              <div key={i} className={`flex ${m.role === "ai" ? "justify-start" : "justify-end"}`}>
                <div className={`max-w-[88%] rounded-mk-lg px-3 py-2 text-[12.5px] leading-relaxed ${m.role === "ai" ? "bg-mk-bg text-mk-ink" : "bg-mk-primary text-white"}`}>{m.text}</div>
              </div>
            ))}
            {busy && (
              <div className="flex justify-start">
                <div className="max-w-[88%] rounded-mk-lg bg-mk-bg px-3 py-2 text-[12.5px] leading-relaxed text-mk-muted-2">印记在想……</div>
              </div>
            )}
          </div>
          <div className="flex items-end gap-2 border-t border-mk-border p-2.5">
            <textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); send(); } }}
              rows={1}
              placeholder="问从哪找、可不可信……"
              className="max-h-20 flex-1 resize-none rounded-mk border border-mk-border bg-mk-input-bg px-2.5 py-1.5 text-[12.5px] text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
            />
            <button type="button" onClick={send} disabled={busy} className="flex h-8 w-8 flex-none items-center justify-center rounded-mk bg-mk-primary text-white hover:bg-mk-primary-hover disabled:opacity-60">
              <Icon name="send" size={15} />
            </button>
          </div>
        </div>
      )}
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-2 rounded-full bg-mk-primary py-3 pl-4 pr-5 text-[13.5px] font-bold text-white shadow-[0_6px_20px_rgba(42,59,122,0.35)] transition hover:bg-mk-primary-hover"
      >
        <Icon name="spark" size={17} /> 问印记 · 找资料
      </button>
    </div>
  );
}
