import { useState } from "react";
import { Icon } from "../Icon";
import {
  project,
  outline as seedOutline,
  draftMarkdown,
  writingCoachChat,
  type ChatMsg,
  type OutlineNode,
} from "./mockData";

// The Write block: two gears — 提纲 (outline) and 写作 (a single draft panel) —
// with a slim goal strip up top so you write against your thesis, and an AI
// rail that talks about your outline/draft (never writes it). The proposal
// stays in Project Management; here we only link back to it.
export function WritingBlock() {
  const [tab, setTab] = useState<"outline" | "draft">("outline");
  return (
    <div className="flex h-full flex-col">
      {/* goal strip */}
      <div className="flex items-center gap-3 border-b border-mk-border bg-mk-surface px-8 py-2.5">
        <span className="flex-none rounded-full bg-mk-accent-tint px-2 py-0.5 text-[11px] font-bold text-mk-accent">论点</span>
        <p className="min-w-0 flex-1 truncate text-[13px] text-mk-ink">{project.proposal.objective}</p>
        <button type="button" className="flex-none text-[12px] font-semibold text-mk-muted-2 hover:text-mk-primary">看开题 →</button>
      </div>

      {/* tabs */}
      <div className="flex items-center gap-2 border-b border-mk-border bg-mk-surface px-8 py-2.5">
        <Tab active={tab === "outline"} onClick={() => setTab("outline")} icon="plan">提纲</Tab>
        <Tab active={tab === "draft"} onClick={() => setTab("draft")} icon="writing">写作</Tab>
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-[1fr,320px]">
        {tab === "outline" ? <OutlinePane /> : <DraftPane />}
        <CoachRail />
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

function OutlinePane() {
  const [nodes, setNodes] = useState<OutlineNode[]>(seedOutline);
  const [view, setView] = useState<"list" | "map">("list");
  const edit = (id: string, text: string) => setNodes((xs) => xs.map((n) => (n.id === id ? { ...n, text } : n)));
  const bump = (id: string, dir: 1 | -1) => setNodes((xs) => xs.map((n) => (n.id === id ? { ...n, depth: Math.max(0, Math.min(2, n.depth + dir)) } : n)));
  const remove = (id: string) => setNodes((xs) => xs.filter((n) => n.id !== id));
  const addAfter = (id: string) =>
    setNodes((xs) => {
      const i = xs.findIndex((n) => n.id === id);
      const depth = xs[i]?.depth ?? 0;
      const next = [...xs];
      next.splice(i + 1, 0, { id: `o-${Date.now() % 100000}`, text: "", depth });
      return next;
    });

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
                <OutlineRow key={n.id} node={n} onEdit={(t) => edit(n.id, t)} onIndent={() => bump(n.id, 1)} onOutdent={() => bump(n.id, -1)} onAdd={() => addAfter(n.id)} onRemove={() => remove(n.id)} />
              ))}
            </div>
            <button type="button" onClick={() => addAfter(nodes[nodes.length - 1]?.id ?? "")} className="mt-2 rounded-mk border border-dashed border-mk-border px-3 py-2 text-[13px] font-semibold text-mk-muted-2 hover:border-mk-primary hover:text-mk-primary">
              + 新增一条
            </button>
          </div>
        </div>
      ) : (
        <MindMap nodes={nodes} onEdit={edit} />
      )}
    </div>
  );
}

/* ----- mind map view (same outline data, laid out as a tree) ----- */

type MapNode = { id: string; text: string; depth: number; children: MapNode[]; row: number; cx: number; cy: number };

const COL = 250;
const ROW = 56;
const NODE_W = 200;

function MindMap({ nodes, onEdit }: { nodes: OutlineNode[]; onEdit: (id: string, t: string) => void }) {
  // Build a tree from the flat depth list, with the project title as the root.
  const root: MapNode = { id: "root", text: project.title, depth: -1, children: [], row: 0, cx: 0, cy: 0 };
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
            className={`absolute flex items-center rounded-mk border px-3 shadow-[0_1px_3px_rgba(28,35,51,0.06)] ${tone(n.depth)}`}
            style={{ left: n.cx, top: n.cy - 18, width: NODE_W, height: 36 }}
          >
            {n.depth < 0 ? (
              <span className="truncate text-[13px] font-bold">{n.text}</span>
            ) : (
              <input
                value={n.text}
                onChange={(e) => onEdit(n.id, e.target.value)}
                className={`w-full truncate bg-transparent text-[12.5px] outline-none ${n.depth === 0 ? "font-bold" : "font-semibold"}`}
              />
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function OutlineRow({ node, onEdit, onIndent, onOutdent, onAdd, onRemove }: { node: OutlineNode; onEdit: (t: string) => void; onIndent: () => void; onOutdent: () => void; onAdd: () => void; onRemove: () => void }) {
  const dot = node.depth === 0 ? "bg-mk-primary" : node.depth === 1 ? "bg-mk-accent" : "bg-mk-green";
  return (
    <div className="group flex items-center gap-2 rounded-mk py-1 hover:bg-mk-bg/60" style={{ paddingLeft: node.depth * 26 }}>
      <span className={`h-1.5 w-1.5 flex-none rounded-full ${dot}`} />
      <input
        value={node.text}
        onChange={(e) => onEdit(e.target.value)}
        placeholder="写一条……"
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

function DraftPane() {
  const [mode, setMode] = useState<"write" | "upload">("write");
  const [text, setText] = useState(draftMarkdown);
  const words = text.replace(/\s+/g, "").length;
  return (
    <div className="flex min-h-0 flex-col px-8 py-6">
      <div className="mx-auto flex min-h-0 w-full max-w-2xl flex-1 flex-col">
        <div className="mb-3 flex items-center justify-between">
          <div className="flex rounded-mk border border-mk-border bg-mk-surface p-0.5">
            <ModeTab active={mode === "write"} onClick={() => setMode("write")}>写在这里</ModeTab>
            <ModeTab active={mode === "upload"} onClick={() => setMode("upload")}>我在别处写了</ModeTab>
          </div>
          {mode === "write" && <span className="text-[12px] font-semibold text-mk-muted-2">{words} 字</span>}
        </div>

        {mode === "write" ? (
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="在这里写你的草稿……（支持 Markdown）"
            className="min-h-0 flex-1 resize-none rounded-mk-lg border border-mk-border bg-mk-surface p-5 font-sans text-[14.5px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
          />
        ) : (
          <div className="flex min-h-0 flex-1 flex-col items-center justify-center rounded-mk-lg border-2 border-dashed border-mk-input bg-mk-input-bg/50 px-6 text-center">
            <span className="text-mk-primary"><Icon name="writing" size={28} /></span>
            <p className="mt-3 text-[15px] font-bold text-mk-ink">把你写好的文档拖进来</p>
            <p className="mt-1 text-[13px] text-mk-muted-2">Word / PDF / Markdown——印记读进来后，也能和你聊这一稿</p>
            <button type="button" className="mt-4 rounded-mk bg-mk-primary px-4 py-2 text-[13px] font-bold text-white hover:bg-mk-primary-hover">选择文件</button>
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

/* ---------- right · AI rail ---------- */

function CoachRail() {
  const [chat, setChat] = useState<ChatMsg[]>(writingCoachChat);
  const [draft, setDraft] = useState("");
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
      </div>
      <div className="flex items-end gap-2 border-t border-mk-border p-3">
        <textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          rows={1}
          placeholder="问问这段逻辑、这个结构……"
          className="max-h-24 flex-1 resize-none rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2 text-[13px] text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
        />
        <button
          type="button"
          onClick={() => { if (!draft.trim()) return; setChat((c) => [...c, { role: "student", text: draft.trim() }, { role: "ai", text: "先说说你这一段想让读者信什么？我们从这个目的倒推它够不够。" }]); setDraft(""); }}
          className="flex h-9 w-9 flex-none items-center justify-center rounded-mk bg-mk-primary text-white hover:bg-mk-primary-hover"
        >
          <Icon name="send" size={16} />
        </button>
      </div>
    </aside>
  );
}
