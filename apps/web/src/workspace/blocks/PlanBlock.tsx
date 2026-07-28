import { useEffect, useRef, useState } from "react";
import type { LogEntry, PlanColumn, PlanItem, PlanTag, Proposal } from "@mind-imprint/contracts";
import { Icon } from "../Icon";
import {
  formingChat,
  formingChatEN,
  PROPOSAL_DIMS,
  TAG_LABEL,
  COLUMN_LABEL,
  TIMELINE_DAYS,
  STAGES,
  STAGE_1,
  type BlockKey,
  type ChatMsg,
} from "./mockData";
import {
  putProposal,
  getPlan,
  createPlanItem,
  patchPlanItem,
  deletePlanItem,
  getLog,
  addLog,
  coach,
  type PlanItemPatch,
} from "../api/workspace";

const TAG_STYLE: Record<PlanTag, string> = {
  read: "bg-mk-primary-tint text-mk-primary",
  write: "bg-mk-accent-tint text-mk-accent",
  review: "bg-mk-green-tint text-mk-green",
};
const TAG_BAR: Record<PlanTag, string> = {
  read: "bg-mk-primary",
  write: "bg-mk-accent",
  review: "bg-mk-green",
};

// A plan card's tag is a doorway: it routes to the room that owns that kind of
// work (读→阅读, 写→写作, 省→回顾).
const roomForTag = (tag: PlanTag): BlockKey =>
  tag === "read" ? "reading" : tag === "write" ? "writing" : "reflection";

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
  onOpenRoom,
  refreshWorkspace,
}: {
  projectId: string;
  title: string;
  qualification: string;
  proposal: Proposal;
  onOpenRoom: (room: BlockKey) => void;
  refreshWorkspace: () => void;
}) {
  const [phase, setPhase] = useState<"forming" | "working">("working");
  // Local proposal state seeded from the projection; the component is keyed on
  // projectId upstream, so this initialises once per opened project.
  const [prop, setProp] = useState<Proposal>(proposal);
  const [chat, setChat] = useState<ChatMsg[]>(formingChat);
  const [lang, setLang] = useState<"zh" | "en">("zh");
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);

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
    saveTimer.current = setTimeout(() => persistProposal(next), 600);
  }
  useEffect(() => () => { if (saveTimer.current) clearTimeout(saveTimer.current); }, []);

  function onGenerate() {
    // Flush any pending debounced save so the working board opens on a stored
    // proposal (persist nothing destructive — just the dims as typed).
    if (saveTimer.current) clearTimeout(saveTimer.current);
    persistProposal(prop);
    setPhase("working");
  }

  async function onSend() {
    const text = draft.trim();
    if (!text || sending) return;
    setChat((c) => [...c, { role: "student", text }]);
    setDraft("");
    setSending(true);
    // In EN mode nudge the model to reply in English; the scope stays the same.
    const userInput = lang === "en" ? `${text}\n\n(reply in English)` : text;
    try {
      const reply = await coach(projectId, "forming", userInput);
      setChat((c) => [...c, { role: "ai", text: reply }]);
    } catch {
      setChat((c) => [...c, { role: "ai", text: "（网络好像有点卡，我没接住这句——再说一次？）" }]);
    } finally {
      setSending(false);
    }
  }

  if (phase === "forming") {
    return (
      <FormingPhase
        title={title}
        proposal={prop}
        setDim={setDim}
        chat={chat}
        lang={lang}
        onToggleLang={() => {
          const next = lang === "zh" ? "en" : "zh";
          setLang(next);
          setChat(next === "en" ? formingChatEN : formingChat);
        }}
        draft={draft}
        setDraft={setDraft}
        sending={sending}
        onSend={onSend}
        onGenerate={onGenerate}
      />
    );
  }

  return (
    <WorkingPhase
      projectId={projectId}
      title={title}
      qualification={qualification}
      proposal={prop}
      onReopen={() => setPhase("forming")}
      onOpenItem={(item) => onOpenRoom(roomForTag(item.tag))}
    />
  );
}

const clamp = (n: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, n));

/* ---------- Phase A · forming ---------- */

function FormingPhase(props: {
  title: string;
  proposal: Proposal;
  setDim: (key: keyof Proposal, v: string) => void;
  chat: ChatMsg[];
  lang: "zh" | "en";
  onToggleLang: () => void;
  draft: string;
  setDraft: (s: string) => void;
  sending: boolean;
  onSend: () => void;
  onGenerate: () => void;
}) {
  const { title, proposal, setDim, chat, lang, onToggleLang, draft, setDraft, sending, onSend, onGenerate } = props;
  const [writing, setWriting] = useState(false);
  const covered = PROPOSAL_DIMS.filter((d) => proposal[d.key].trim().length > 0).length;
  const ready = covered >= 1;
  return (
    <div className="relative mx-auto grid h-full w-full max-w-6xl grid-cols-[1fr,380px] gap-8 px-10 py-9">
      {/* Chat column */}
      <div className="flex min-h-0 flex-col">
        <header className="mb-5 flex items-start justify-between">
          <div>
            <p className="text-[12px] font-semibold uppercase tracking-[0.18em] text-mk-muted-2">先想清楚，再动手</p>
            <h1 className="mt-1 font-sans text-[26px] font-bold leading-tight text-mk-ink">你想弄清楚的，到底是什么？</h1>
            <p className="mt-1.5 text-[14px] text-mk-muted">不用急着列提纲。先把念头说出来，计划会自己长出来。</p>
          </div>
          <button type="button" onClick={onToggleLang} className="mt-1 flex flex-none items-center gap-1 rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-muted hover:text-mk-primary" title="印记可用中文或英文引导">
            <span className={lang === "zh" ? "text-mk-primary" : ""}>中</span>
            <span className="text-mk-muted-2">/</span>
            <span className={lang === "en" ? "text-mk-primary" : ""}>EN</span>
          </button>
        </header>

        <div className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto pr-1">
          {chat.map((m, i) => (
            <ChatBubble key={i} msg={m} />
          ))}
          {sending && (
            <div className="flex justify-start">
              <div className="max-w-[82%] rounded-mk-lg bg-mk-surface px-4 py-2.5 text-[14px] leading-relaxed text-mk-muted-2 shadow-[0_1px_2px_rgba(28,35,51,0.05)]">
                <span className="mb-0.5 block text-[11px] font-bold text-mk-primary">印记</span>
                在想……
              </div>
            </div>
          )}
        </div>

        <div className="mt-4 flex items-end gap-2 rounded-mk-lg border border-mk-border bg-mk-surface p-2.5 shadow-[0_1px_2px_rgba(28,35,51,0.04)]">
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); onSend(); } }}
            rows={1}
            placeholder="说说你的想法……"
            className="max-h-28 flex-1 resize-none bg-transparent px-2 py-1.5 text-[14px] text-mk-ink outline-none placeholder:text-mk-muted-2"
          />
          <button type="button" onClick={onSend} disabled={sending} className="flex h-9 w-9 items-center justify-center rounded-mk bg-mk-primary text-white transition hover:bg-mk-primary-hover disabled:cursor-not-allowed disabled:opacity-50">
            <Icon name="send" size={17} />
          </button>
        </div>
      </div>

      {/* Live 开题 panel — the proposal's four dimensions, coached not required */}
      <aside className="flex min-h-0 flex-col">
        <div className="flex min-h-0 flex-1 flex-col rounded-mk-lg border border-mk-border bg-mk-surface p-5 shadow-[0_1px_3px_rgba(28,35,51,0.05)]">
          <div className="mb-1 flex items-center justify-between">
            <div className="flex items-center gap-2 text-mk-primary">
              <Icon name="spark" size={16} />
              <span className="text-[13px] font-bold tracking-wide">开题 · 想清楚这几件事</span>
            </div>
            <span className="text-[11px] font-bold text-mk-muted-2">{covered}/4 已聊到</span>
          </div>
          <p className="mb-4 text-[11.5px] text-mk-muted-2">不用写正式开题报告——把这几件事聊清楚就行。</p>
          <div className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto">
            {PROPOSAL_DIMS.map((d) => (
              <DimField key={d.key} label={d.label} hint={d.hint} filled={proposal[d.key].trim().length > 0} value={proposal[d.key]} onChange={(v) => setDim(d.key, v)} />
            ))}
          </div>
        </div>
        <button
          type="button"
          disabled={!ready}
          onClick={onGenerate}
          className="mt-3 flex items-center justify-center gap-2 rounded-mk bg-mk-accent py-3 text-[14px] font-bold text-white transition enabled:hover:bg-mk-accent-hover disabled:cursor-not-allowed disabled:bg-mk-input disabled:text-mk-muted-2"
        >
          生成项目计划
          <Icon name="arrow" size={16} />
        </button>
        <button
          type="button"
          onClick={() => setWriting(true)}
          className="mt-2 flex items-center justify-center gap-2 rounded-mk border-2 border-mk-primary bg-mk-surface py-2.5 text-[14px] font-bold text-mk-primary transition hover:bg-mk-primary-tint"
        >
          <Icon name="writing" size={16} /> 写开题报告（可选）
        </button>
      </aside>

      {writing && <ProposalWriter proposal={proposal} setDim={setDim} title={title} onClose={() => setWriting(false)} />}
    </div>
  );
}

// The formal proposal is WRITTEN by the student (not generated). This is a
// larger writing surface over the same four (persisted) dimensions, headed as
// EPQ §1–§4, with a structured export (real .docx lands in slice 6). Optional —
// the dimensions are already valued from the chat.
function ProposalWriter({ proposal, setDim, title, onClose }: { proposal: Proposal; setDim: (k: keyof Proposal, v: string) => void; title: string; onClose: () => void }) {
  const SECTIONS: { key: keyof Proposal; n: string; title: string; hint: string }[] = [
    { key: "objective", n: "§1", title: "题目、目标与职责", hint: "你想回答什么问题？想学会做什么？想发现什么？" },
    { key: "reason", n: "§2", title: "选题理由", hint: "与你所学学科的关联、个人兴趣、未来规划、想提升的知识/技能、为什么这个题目重要" },
    { key: "activities", n: "§3", title: "活动与时间安排", hint: "研究、想法的发展与分析、写作、数据收集、排练、成果产出、评估、准备展示等" },
    { key: "resources", n: "§4", title: "资源", hint: "图书馆、书籍、期刊、设备、场地、技术、经费等" },
  ];
  function exportReport() {
    const lines = [`# 开题报告 · ${title || "未命名项目"}`, ""];
    for (const s of SECTIONS) {
      lines.push(`## ${s.n} ${s.title}`, "", proposal[s.key].trim() || "（未填写）", "");
    }
    downloadFile("开题报告.md", lines.join("\n"), "text/markdown");
  }
  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-mk-ink/30 px-8" onClick={onClose}>
      <div className="flex max-h-[86%] w-[640px] flex-col rounded-mk-lg border border-mk-border bg-mk-surface shadow-[0_20px_60px_rgba(28,35,51,0.25)]" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b border-mk-border px-6 py-4">
          <div>
            <h3 className="font-sans text-[18px] font-bold text-mk-ink">开题报告</h3>
            <p className="mt-0.5 text-[12px] text-mk-muted-2">你自己写——印记只在一旁陪你想，不替你写。</p>
          </div>
          <button type="button" onClick={onClose} className="text-[20px] leading-none text-mk-muted-2 hover:text-mk-ink">×</button>
        </div>
        <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-6 py-5">
          {SECTIONS.map((s) => (
            <label key={s.key} className="block">
              <span className="flex items-baseline gap-2">
                <span className="text-[12px] font-bold text-mk-accent">{s.n}</span>
                <span className="text-[14px] font-bold text-mk-ink">{s.title}</span>
              </span>
              <span className="mt-0.5 block text-[11.5px] text-mk-muted-2">{s.hint}</span>
              <textarea
                value={proposal[s.key]}
                onChange={(e) => setDim(s.key, e.target.value)}
                rows={4}
                placeholder="在这里写……"
                className="mt-2 w-full resize-none rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2.5 text-[13.5px] leading-relaxed text-mk-ink outline-none focus:border-mk-primary"
              />
            </label>
          ))}
        </div>
        <div className="flex items-center justify-between border-t border-mk-border px-6 py-3.5">
          <span className="text-[12px] text-mk-muted-2">随时保存 · 你写的每一段都算数</span>
          <div className="flex gap-2">
            <button type="button" onClick={exportReport} className="rounded-mk border border-mk-border px-4 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-primary">导出</button>
            <button type="button" onClick={onClose} className="rounded-mk bg-mk-primary px-4 py-2 text-[13px] font-bold text-white hover:bg-mk-primary-hover">完成</button>
          </div>
        </div>
      </div>
    </div>
  );
}

function DimField({ label, hint, value, filled, onChange }: { label: string; hint: string; value: string; filled: boolean; onChange: (v: string) => void }) {
  return (
    <label className="block">
      <span className="mb-1 flex items-center gap-1.5">
        <span className={`h-1.5 w-1.5 rounded-full ${filled ? "bg-mk-green" : "border border-mk-muted-2"}`} />
        <span className="text-[11.5px] font-bold text-mk-ink">{label}</span>
        <span className="text-[10.5px] font-normal text-mk-muted-2">· {hint}</span>
      </span>
      <textarea
        value={value}
        placeholder="跟印记聊几句，这里会慢慢填上"
        onChange={(e) => onChange(e.target.value)}
        rows={2}
        className="w-full resize-none rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2 text-[13px] leading-relaxed text-mk-ink outline-none transition placeholder:text-mk-muted-2 focus:border-mk-primary"
      />
    </label>
  );
}

function ChatBubble({ msg }: { msg: ChatMsg }) {
  const isAi = msg.role === "ai";
  return (
    <div className={`flex ${isAi ? "justify-start" : "justify-end"}`}>
      <div className={`max-w-[82%] rounded-mk-lg px-4 py-2.5 text-[14px] leading-relaxed ${isAi ? "bg-mk-surface text-mk-ink shadow-[0_1px_2px_rgba(28,35,51,0.05)]" : "bg-mk-primary text-white"}`}>
        {isAi && <span className="mb-0.5 block text-[11px] font-bold text-mk-primary">印记</span>}
        {msg.text}
      </div>
    </div>
  );
}

/* ---------- download helper (structured .md for now; .docx in slice 6) ---------- */

function downloadFile(name: string, body: string, mime = "text/markdown") {
  const blob = new Blob([body], { type: `${mime};charset=utf-8` });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = name;
  a.click();
  URL.revokeObjectURL(a.href);
}

/* ---------- Phase B · working (kanban / gantt / log) ---------- */

const COLUMNS: PlanColumn[] = ["todo", "doing", "done"];
type PlanView = "kanban" | "gantt" | "log";

function WorkingPhase(props: {
  projectId: string;
  title: string;
  qualification: string;
  proposal: Proposal;
  onReopen: () => void;
  onOpenItem: (item: PlanItem) => void;
}) {
  const { projectId, title, qualification, proposal, onReopen, onOpenItem } = props;
  const [view, setView] = useState<PlanView>("kanban");
  const [open, setOpen] = useState(false);

  // The board — loaded on enter, mutated optimistically then reconciled.
  const [board, setBoard] = useState<PlanItem[]>([]);
  const [loadingPlan, setLoadingPlan] = useState(true);

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
  function removeItem(id: string) {
    const prev = board;
    setBoard((xs) => xs.filter((x) => x.id !== id));
    deletePlanItem(projectId, id).catch(() => setBoard(prev));
  }

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

  function exportPlan() {
    const lines = [`# ${title}`, `_${qualification}_`, "", "| 任务 | 阶段 | 类型 | 状态 | 第几天 | 时长(天) |", "| --- | --- | --- | --- | --- | --- |"];
    for (const i of board) lines.push(`| ${i.title} | ${i.stage} | ${TAG_LABEL[i.tag]} | ${COLUMN_LABEL[i.column]} | 第${i.start + 1}天 | ${i.days} |`);
    downloadFile("项目计划.md", lines.join("\n"));
  }
  function exportLog() {
    const rows = log ?? [];
    downloadFile("活动日志.md", `# 活动日志 · ${title}\n\n` + rows.map((e) => `- **${e.date}** ${e.text}`).join("\n"));
  }

  return (
    <div className="flex h-full flex-col px-10 py-8">
      {/* Slim goal header */}
      <div className="mb-6 rounded-mk-lg border border-mk-border bg-mk-surface px-5 py-3.5 shadow-[0_1px_2px_rgba(28,35,51,0.04)]">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <span className="rounded-full bg-mk-primary-tint px-2.5 py-1 text-[11px] font-bold text-mk-primary">{qualification}</span>
            <h1 className="font-sans text-[18px] font-bold text-mk-ink">{title}</h1>
          </div>
          <button type="button" onClick={() => setOpen((o) => !o)} className="text-[13px] font-semibold text-mk-muted hover:text-mk-primary">
            {open ? "收起" : "查看我的题目"}
          </button>
        </div>
        {open && (
          <div className="mt-3 grid grid-cols-3 gap-4 border-t border-mk-border pt-3 text-[13px] leading-relaxed text-mk-muted">
            <div><span className="font-bold text-mk-muted-2">缘由 · </span>{proposal.reason || "—"}</div>
            <div className="col-span-2"><span className="font-bold text-mk-accent">目标 · </span>{proposal.objective || "—"}</div>
          </div>
        )}
      </div>

      {/* Toolbar: title + view toggle + export */}
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h2 className="font-sans text-[20px] font-bold text-mk-ink">项目管理</h2>
          <p className="mt-0.5 text-[13px] text-mk-muted">{view === "log" ? "项目一路上发生了什么——大多自动记下，你也能补一笔。" : "拖动来编辑：看板换列、甘特图挪动/拉长。点任务名进入房间。"}</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex rounded-mk border border-mk-border bg-mk-surface p-0.5">
            <ViewTab active={view === "kanban"} onClick={() => setView("kanban")}>看板</ViewTab>
            <ViewTab active={view === "gantt"} onClick={() => setView("gantt")}>甘特图</ViewTab>
            <ViewTab active={view === "log"} onClick={() => setView("log")}>活动日志</ViewTab>
          </div>
          <button type="button" onClick={view === "log" ? exportLog : exportPlan} className="rounded-mk border border-mk-border bg-mk-surface px-3.5 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-primary">
            导出
          </button>
          <button type="button" onClick={onReopen} className="flex items-center gap-1.5 rounded-mk border border-mk-border bg-mk-surface px-3.5 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-primary">
            <Icon name="spark" size={15} /> 聊聊计划
          </button>
        </div>
      </div>

      {view === "kanban" && <KanbanView board={board} loading={loadingPlan} onMove={(id, column) => patchItem(id, { column })} onAddTask={addTask} onDelete={removeItem} onOpenItem={onOpenItem} />}
      {view === "gantt" && <GanttView board={board} onReschedule={(id, start) => patchItem(id, { start })} onResize={(id, days) => patchItem(id, { days })} onOpenItem={onOpenItem} />}
      {view === "log" && <ActivityLogView log={log} onAdd={addLogEntry} />}
    </div>
  );
}

function ViewTab({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button type="button" onClick={onClick} className={`rounded-[10px] px-3 py-1.5 text-[13px] font-bold transition ${active ? "bg-mk-primary text-white" : "text-mk-muted-2 hover:text-mk-muted"}`}>
      {children}
    </button>
  );
}

/* ----- Kanban (HTML5 drag between columns) ----- */

function KanbanView({ board, loading, onMove, onAddTask, onDelete, onOpenItem }: { board: PlanItem[]; loading: boolean; onMove: (id: string, c: PlanColumn) => void; onAddTask: (stage: string) => void; onDelete: (id: string) => void; onOpenItem: (i: PlanItem) => void }) {
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
            className={`flex min-h-0 flex-col rounded-mk-lg border-2 p-3 transition ${isOver ? "border-mk-primary/50 bg-mk-primary-tint/40" : "border-transparent bg-mk-surface/60"}`}
          >
            <header className="mb-3 flex items-center justify-between px-1">
              <span className="text-[13px] font-bold text-mk-ink">{COLUMN_LABEL[col]}</span>
              <span className="rounded-full bg-mk-bg px-2 py-0.5 text-[11px] font-semibold text-mk-muted-2">{colItems.length}</span>
            </header>
            <div className="flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto pr-0.5">
              {colItems.map((item) => (
                <PlanCard key={item.id} item={item} dragging={dragId === item.id} onOpen={() => onOpenItem(item)} onDelete={() => onDelete(item.id)} onDragStart={() => setDragId(item.id)} onDragEnd={() => { setDragId(null); setOver(null); }} />
              ))}
              {colItems.length === 0 && (
                <div className="rounded-mk border border-dashed border-mk-border px-3 py-6 text-center text-[12px] text-mk-muted-2">
                  {loading ? "加载中…" : "拖到这里"}
                </div>
              )}
            </div>
            {col === "todo" && (
              <button type="button" onClick={() => onAddTask(STAGE_1)} className="mt-2 rounded-mk border border-dashed border-mk-border py-2 text-[12.5px] font-semibold text-mk-muted-2 hover:border-mk-primary hover:text-mk-primary">+ 添加任务</button>
            )}
          </section>
        );
      })}
    </div>
  );
}

function PlanCard({ item, dragging, onOpen, onDelete, onDragStart, onDragEnd }: { item: PlanItem; dragging: boolean; onOpen: () => void; onDelete: () => void; onDragStart: () => void; onDragEnd: () => void }) {
  return (
    <div
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      className={`group relative cursor-grab rounded-mk border border-mk-border bg-mk-surface p-3 shadow-[0_1px_2px_rgba(28,35,51,0.04)] transition active:cursor-grabbing hover:border-mk-primary/40 hover:shadow-[0_2px_8px_rgba(28,35,51,0.07)] ${dragging ? "opacity-40" : ""}`}
    >
      <button
        type="button"
        onClick={onDelete}
        title="删除任务"
        className="absolute right-1.5 top-1.5 flex h-5 w-5 items-center justify-center rounded-full text-[13px] leading-none text-mk-muted-2 opacity-0 transition hover:bg-mk-accent-tint hover:text-mk-accent group-hover:opacity-100"
      >
        ×
      </button>
      <div className="mb-2 flex items-center gap-1.5">
        <span className={`rounded px-1.5 py-0.5 text-[11px] font-bold ${TAG_STYLE[item.tag]}`}>{TAG_LABEL[item.tag]}</span>
        <span className="ml-auto truncate pr-5 text-[10.5px] text-mk-muted-2">{item.stage.split(" · ")[0]}</span>
      </div>
      <button type="button" onClick={onOpen} className="block w-full text-left text-[13.5px] font-medium leading-snug text-mk-ink hover:text-mk-primary">{item.title}</button>
      <button type="button" onClick={onOpen} className="mt-2 text-[12px] font-semibold text-mk-primary opacity-0 transition hover:underline group-hover:opacity-100">进入 →</button>
    </div>
  );
}

/* ----- Gantt (drag to move, resize handle to change duration), by stage ----- */

function GanttView({ board, onReschedule, onResize, onOpenItem }: { board: PlanItem[]; onReschedule: (id: string, start: number) => void; onResize: (id: string, days: number) => void; onOpenItem: (i: PlanItem) => void }) {
  const days = Array.from({ length: TIMELINE_DAYS }, (_, i) => i);
  return (
    <div className="min-h-0 flex-1 overflow-auto rounded-mk-lg border border-mk-border bg-mk-surface">
      <div className="min-w-[820px]">
        {/* Day header */}
        <div className="sticky top-0 z-10 grid grid-cols-[240px,1fr] border-b border-mk-border bg-mk-surface">
          <div className="px-4 py-2.5 text-[12px] font-bold text-mk-muted-2">任务</div>
          <div className="grid" style={{ gridTemplateColumns: `repeat(${TIMELINE_DAYS}, 1fr)` }}>
            {days.map((d) => (
              <div key={d} className={`border-l border-mk-border-2 py-2.5 text-center text-[11px] font-semibold ${d % 7 >= 5 ? "text-mk-muted-2" : "text-mk-muted"}`}>{d + 1}</div>
            ))}
          </div>
        </div>
        {/* Stage groups */}
        {STAGES.map((stage) => {
          const items = board.filter((i) => i.stage === stage);
          if (items.length === 0) return null;
          return (
            <div key={stage}>
              <div className="grid grid-cols-[240px,1fr] border-b border-mk-border-2 bg-mk-bg/50">
                <div className="px-4 py-1.5 text-[11.5px] font-bold uppercase tracking-wider text-mk-muted">{stage}</div>
                <div />
              </div>
              {items.map((item) => (
                <div key={item.id} className="grid grid-cols-[240px,1fr] items-center border-b border-mk-border-2 hover:bg-mk-bg/40">
                  <button type="button" onClick={() => onOpenItem(item)} className="flex items-center gap-2 px-4 py-3 text-left">
                    <span className={`rounded px-1.5 py-0.5 text-[11px] font-bold ${TAG_STYLE[item.tag]}`}>{TAG_LABEL[item.tag]}</span>
                    <span className="truncate text-[13px] font-medium text-mk-ink hover:text-mk-primary">{item.title.replace(/^[读写省]：/, "")}</span>
                  </button>
                  <div data-track className="relative h-11">
                    <div className="absolute inset-0 grid" style={{ gridTemplateColumns: `repeat(${TIMELINE_DAYS}, 1fr)` }}>
                      {days.map((d) => (<div key={d} className={`border-l border-mk-border-2 ${d % 7 >= 5 ? "bg-mk-bg/60" : ""}`} />))}
                    </div>
                    <GanttBar item={item} onReschedule={onReschedule} onResize={onResize} />
                  </div>
                </div>
              ))}
            </div>
          );
        })}
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
      <span className="pointer-events-none flex-1 truncate px-2 text-[11px] font-bold leading-6 text-white">
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

function ActivityLogView({ log, onAdd }: { log: LogEntry[] | null; onAdd: (text: string) => void }) {
  const [draft, setDraft] = useState("");
  const rows = log ?? [];
  function submit() {
    if (!draft.trim()) return;
    onAdd(draft.trim());
    setDraft("");
  }
  return (
    <div className="mx-auto flex min-h-0 w-full max-w-3xl flex-1 flex-col">
      <div className="min-h-0 flex-1 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface">
        {log === null ? (
          <div className="flex h-full items-center justify-center py-16 text-[13px] text-mk-muted-2">加载中…</div>
        ) : rows.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center gap-1 py-16 text-center">
            <p className="text-[14px] font-semibold text-mk-ink">还没有记录</p>
            <p className="text-[12.5px] text-mk-muted-2">你在项目里做的事会自动记下——也可以现在补一笔。</p>
          </div>
        ) : (
          rows.map((e, i) => (
            <div key={e.id} className={`flex gap-4 px-5 py-3.5 ${i < rows.length - 1 ? "border-b border-mk-border-2" : ""}`}>
              <span className="w-12 flex-none pt-0.5 text-[13px] font-bold text-mk-muted-2">{e.date}</span>
              <p className="flex-1 text-[13.5px] leading-relaxed text-mk-ink">{e.text}</p>
              <span className={`flex-none self-start rounded-full px-2 py-0.5 text-[10.5px] font-bold ${e.source === "auto" ? "bg-mk-primary-tint text-mk-primary" : "bg-mk-green-tint text-mk-green"}`}>
                {e.source === "auto" ? "自动" : "我记的"}
              </span>
            </div>
          ))
        )}
      </div>
      <div className="mt-3 flex items-end gap-2 rounded-mk-lg border border-mk-border bg-mk-surface p-2.5">
        <input value={draft} onChange={(e) => setDraft(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); submit(); } }} placeholder="补一笔：今天做了什么、想到什么……" className="flex-1 bg-transparent px-2 py-1.5 text-[13.5px] text-mk-ink outline-none placeholder:text-mk-muted-2" />
        <button
          type="button"
          onClick={submit}
          className="rounded-mk bg-mk-primary px-3.5 py-2 text-[13px] font-bold text-white hover:bg-mk-primary-hover"
        >
          记一笔
        </button>
      </div>
    </div>
  );
}
