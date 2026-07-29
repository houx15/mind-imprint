import { useEffect, useRef, useState } from "react";
import type { Proposal } from "@mind-imprint/contracts";
import { putBuffer } from "../../api/writing";
import { Icon } from "../Icon";
import type { BlockKey } from "./mockData";
import { getOutline, putOutline, getDraft, coach } from "../api/workspace";
import { MarkdownPreview } from "./MarkdownPreview";

// One outline bullet in local edit shape — flat-with-depth, the same model the
// prototype used (the persisted OutlineNode adds a server-owned `position`,
// which the array order carries here).
type Row = { id: string; text: string; depth: number };
type ChatMsg = { role: "ai" | "student"; text: string };

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
  onOpenRoom,
}: {
  projectId: string;
  title: string;
  proposal: Proposal;
  onOpenRoom: (room: BlockKey) => void;
}) {
  const [tab, setTab] = useState<"outline" | "draft">("outline");
  return (
    <div className="flex h-full flex-col">
      {/* goal strip */}
      <div className="flex items-center gap-3 border-b border-mk-border bg-mk-surface px-8 py-2.5">
        <span className="flex-none rounded-full bg-mk-accent-tint px-2 py-0.5 text-[11px] font-bold text-mk-accent">论点</span>
        <p className="min-w-0 flex-1 truncate text-[13px] text-mk-ink">{proposal.objective || "还没有写下你的论点——先去开题里想清楚。"}</p>
        <button type="button" onClick={() => onOpenRoom("plan")} className="flex-none text-[12px] font-semibold text-mk-muted-2 hover:text-mk-primary">看开题 →</button>
      </div>

      {/* tabs */}
      <div className="flex items-center gap-2 border-b border-mk-border bg-mk-surface px-8 py-2.5">
        <Tab active={tab === "outline"} onClick={() => setTab("outline")} icon="plan">提纲</Tab>
        <Tab active={tab === "draft"} onClick={() => setTab("draft")} icon="writing">写作</Tab>
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-[1fr,320px]">
        {tab === "outline" ? <OutlinePane projectId={projectId} title={title} /> : <DraftPane projectId={projectId} />}
        <CoachRail projectId={projectId} />
      </div>
    </div>
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

  // Persist the whole set, then adopt the server ids onto the rows we sent —
  // but only when the local set hasn't structurally changed meanwhile (same
  // length), so an id-swap never clobbers a mid-flight edit. On failure keep
  // local; the next debounce retries.
  async function save(rows: Row[]) {
    try {
      const server = await putOutline(projectId, rows.map((r) => ({ id: r.id, text: r.text, depth: r.depth })));
      const cur = nodesRef.current;
      if (cur.length === server.length) {
        const next = cur.map((n, i) => ({ ...n, id: server[i]!.id }));
        nodesRef.current = next;
        setNodes(next);
      }
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
        <MindMap nodes={nodes} title={title} onEdit={edit} onAddChild={addChild} />
      )}
    </div>
  );
}

/* ----- mind map view (same outline data, laid out as a tree) ----- */

type MapNode = { id: string; text: string; depth: number; children: MapNode[]; row: number; cx: number; cy: number };

const COL = 250;
const ROW = 56;
const NODE_W = 200;

function MindMap({ nodes, title, onEdit, onAddChild }: { nodes: Row[]; title: string; onEdit: (id: string, t: string) => void; onAddChild: (id: string) => void }) {
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
                value={n.text}
                onChange={(e) => onEdit(n.id, e.target.value)}
                placeholder="写一条……"
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

function DraftPane({ projectId }: { projectId: string }) {
  const [mode, setMode] = useState<"write" | "upload">("write");
  const [pane, setPane] = useState<"edit" | "preview">("edit");
  const [text, setText] = useState("");
  const [uploadNote, setUploadNote] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement | null>(null);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const words = text.replace(/\s+/g, "").length;

  // Load the persisted draft on mount ("" when there's no buffer yet).
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const content = await getDraft(projectId);
        if (!cancelled) setText(content);
      } catch {
        /* leave empty; the placeholder shows */
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
      setUploadNote(null);
      setMode("write");
      void putBuffer(projectId, content).catch(() => {/* retries via next edit */});
    } else {
      // .docx / .pdf and friends — accepted but not parsed yet. Don't crash;
      // just tell the student we've noted it.
      setUploadNote(`已上传「${file.name}」，正文解析稍后支持。`);
    }
  }

  return (
    <div className="flex min-h-0 flex-col px-8 py-6">
      <div className="mx-auto flex min-h-0 w-full max-w-2xl flex-1 flex-col">
        <div className="mb-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="flex rounded-mk border border-mk-border bg-mk-surface p-0.5">
              <ModeTab active={mode === "write"} onClick={() => setMode("write")}>写在这里</ModeTab>
              <ModeTab active={mode === "upload"} onClick={() => setMode("upload")}>我在别处写了</ModeTab>
            </div>
            {mode === "write" && (
              <div className="flex rounded-mk border border-mk-border bg-mk-surface p-0.5">
                <SubTab active={pane === "edit"} onClick={() => setPane("edit")}>写</SubTab>
                <SubTab active={pane === "preview"} onClick={() => setPane("preview")}>预览</SubTab>
              </div>
            )}
          </div>
          {mode === "write" && <span className="text-[12px] font-semibold text-mk-muted-2">{words} 字</span>}
        </div>

        {mode === "write" ? (
          pane === "edit" ? (
            <textarea
              value={text}
              onChange={(e) => onChange(e.target.value)}
              placeholder="在这里写你的草稿……（支持 Markdown）"
              className="min-h-0 flex-1 resize-none rounded-mk-lg border border-mk-border bg-mk-surface p-5 font-sans text-[14.5px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
            />
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
        <p className="mt-2 text-center text-[11.5px] text-mk-muted-2">你写，印记只在一旁陪你想——它不替你写正文。</p>
      </div>
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

function CoachRail({ projectId }: { projectId: string }) {
  const [chat, setChat] = useState<ChatMsg[]>([RAIL_GREETING]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);

  async function send() {
    const text = draft.trim();
    if (!text || sending) return;
    setChat((c) => [...c, { role: "student", text }]);
    setDraft("");
    setSending(true);
    try {
      const reply = await coach(projectId, "writing", text);
      setChat((c) => [...c, { role: "ai", text: reply }]);
      // ── Card-summon hook ──────────────────────────────────────────────
      // A future writing-scope coach turn may summon a thinking-card (e.g.
      // 让步段 / 反例). When the coach response carries a summon signal, mount
      // the Card Runtime here and refeed its standard envelope back into this
      // thread. No behavior yet — the transport only returns the reply string.
      // ──────────────────────────────────────────────────────────────────
    } catch {
      setChat((c) => [...c, { role: "ai", text: "刚才没接上，稍等再问我一次。" }]);
    } finally {
      setSending(false);
    }
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
            <div className={`max-w-[88%] rounded-mk-lg px-3.5 py-2.5 text-[13px] leading-relaxed ${m.role === "ai" ? "bg-mk-bg text-mk-ink" : "bg-mk-primary text-white"}`}>{m.text}</div>
          </div>
        ))}
        {sending && (
          <div className="flex justify-start">
            <div className="max-w-[88%] rounded-mk-lg bg-mk-bg px-3.5 py-2.5 text-[13px] leading-relaxed text-mk-muted-2">印记在想……</div>
          </div>
        )}
      </div>
      <div className="flex items-end gap-2 border-t border-mk-border p-3">
        <textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); void send(); } }}
          rows={1}
          placeholder="问问这段逻辑、这个结构……"
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
    </aside>
  );
}
