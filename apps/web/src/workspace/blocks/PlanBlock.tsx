import { useEffect, useRef, useState } from "react";
import type { LogEntry, PlanColumn, PlanItem, PlanTag, Proposal } from "@mind-imprint/contracts";
import { Icon } from "../Icon";
import {
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
  getCoachHistory,
  generatePlan,
  type PlanItemPatch,
} from "../api/workspace";
import { ApiError } from "../../api/client";
import { exportTimescale, exportActivityLog, exportProposalDocx } from "../export";

// The forming chat opens with a scripted guiding intro (NOT an LLM call). It
// names the four things worth thinking through and offers a fork: be walked
// through them one part at a time, or fill the panel directly. **bold** markers
// are rendered by ChatBubble.
const INTRO_ZH =
  "要做好一个研究项目，先想清楚四件事：**目标**（想回答什么）、**缘由**（为什么做）、**活动与时间**（打算怎么做）、**资源**（需要什么）。想让我一部分一部分带你想，还是你已经有想法、想直接填右边？";
const INTRO_EN =
  "To set up a research project well, get four things clear first: **objective** (what you want to answer), **reason** (why do it), **activities & timeline** (how you'll do it), and **resources** (what you'll need). Want me to walk you through them one part at a time, or do you already have ideas and want to fill in the panel on the right?";
const introChat = (l: "zh" | "en"): ChatMsg[] => [{ role: "ai", text: l === "en" ? INTRO_EN : INTRO_ZH }];

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
  // #11 — a brand-new project (all four dims blank) opens in the calm forming
  // coach; anything already thought through opens straight on the working board.
  const proposalEmpty = PROPOSAL_DIMS.every((d) => proposal[d.key].trim().length === 0);
  const [phase, setPhase] = useState<"forming" | "working">(proposalEmpty ? "forming" : "working");
  // Local proposal state seeded from the projection; the component is keyed on
  // projectId upstream, so this initialises once per opened project.
  const [prop, setProp] = useState<Proposal>(proposal);
  const [chat, setChat] = useState<ChatMsg[]>(() => introChat("zh"));
  const [lang, setLang] = useState<"zh" | "en">("zh");
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  // The two quick-reply chips live only under the scripted intro; any turn
  // (chip, typed message, or a language reset) dismisses them.
  const [chipsDismissed, setChipsDismissed] = useState(false);
  // 生成项目计划 round-trip state + the items it returns (seeded into the board).
  const [generating, setGenerating] = useState(false);
  const [genError, setGenError] = useState<string | null>(null);
  const [seedBoard, setSeedBoard] = useState<PlanItem[] | undefined>(undefined);

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

  // S1 · one continuous session: load THIS room's slice of the project's thread
  // once on open, appended after the scripted intro so re-entry shows the
  // conversation so far. Empty (a fresh project) → intro only, unchanged.
  useEffect(() => {
    let alive = true;
    getCoachHistory(projectId, "forming")
      .then((msgs) => {
        if (!alive || msgs.length === 0) return;
        setChat((c) => [...c, ...msgs]);
        setChipsDismissed(true);
      })
      .catch(() => {
        /* keep the intro-only view; the next turn still persists */
      });
    return () => {
      alive = false;
    };
  }, [projectId]);

  async function onGenerate() {
    if (generating) return;
    // Flush any pending debounced save so the plan is generated from the stored
    // proposal (persist nothing destructive — just the dims as typed).
    if (saveTimer.current) clearTimeout(saveTimer.current);
    persistProposal(prop);
    setGenerating(true);
    setGenError(null);
    try {
      const items = await generatePlan(projectId);
      setSeedBoard(items);
      setPhase("working");
    } catch (e) {
      // 422 proposal_empty → nudge; anything else → a gentle retry hint.
      if (e instanceof ApiError && e.code === "proposal_empty") {
        setGenError("先聊几句开题再生成");
      } else {
        setGenError("生成没成功，稍后再试一次");
      }
    } finally {
      setGenerating(false);
    }
  }

  // One restrained coaching turn appended to the chat, with a busy state.
  async function runCoachTurn(scope: "forming" | "proposal_review", userInput: string, studentEcho: string) {
    if (sending) return;
    setChat((c) => [...c, { role: "student", text: studentEcho }]);
    setSending(true);
    try {
      const reply = await coach(projectId, scope, userInput);
      setChat((c) => [...c, { role: "ai", text: reply }]);
    } catch {
      setChat((c) => [...c, { role: "ai", text: "（网络好像有点卡，我没接住——再试一次？）" }]);
    } finally {
      setSending(false);
    }
  }

  async function onSend() {
    const text = draft.trim();
    if (!text || sending) return;
    setChipsDismissed(true);
    setDraft("");
    // In EN mode nudge the model to reply in English; the scope stays the same.
    const userInput = lang === "en" ? `${text}\n\n(reply in English)` : text;
    await runCoachTurn("forming", userInput, text);
  }

  // Chip 1 — kick off the guided walk-through, starting with 目标.
  async function onGuideMe() {
    setChipsDismissed(true);
    const prompt =
      lang === "en"
        ? "Walk me through it one part at a time. Start with the first thing — my objective: help me get clear on what question I'm actually trying to answer.\n\n(reply in English)"
        : "请一部分一部分带我想。先从第一件事「目标」开始：帮我想清楚我到底想回答什么问题。";
    await runCoachTurn("forming", prompt, lang === "en" ? "Walk me through it, part by part." : "带我一部分一部分想");
  }

  // #2 — 让印记看看我的开题：hand the four dims to a review-scoped turn. The AI
  // critiques; it never writes into the dim fields (the student still types).
  async function onReview() {
    const labelled = PROPOSAL_DIMS.map((d) => `${d.label}：${prop[d.key].trim() || "（空）"}`).join("\n");
    const userInput = lang === "en" ? `${labelled}\n\n(reply in English)` : labelled;
    await runCoachTurn(
      "proposal_review",
      userInput,
      lang === "en" ? "Take a look at my kickoff — what still needs thinking through?" : "帮我看看我的开题——哪里还要再想清楚？",
    );
  }

  if (phase === "forming") {
    return (
      <FormingPhase
        title={title}
        qualification={qualification}
        proposal={prop}
        setDim={setDim}
        chat={chat}
        lang={lang}
        onToggleLang={() => {
          const next = lang === "zh" ? "en" : "zh";
          setLang(next);
          setChat(introChat(next));
          setChipsDismissed(false);
        }}
        draft={draft}
        setDraft={setDraft}
        sending={sending}
        onSend={onSend}
        showChips={!chipsDismissed && chat.length === 1}
        onGuideMe={onGuideMe}
        onSelfFill={() => setChipsDismissed(true)}
        onReview={onReview}
        onGenerate={onGenerate}
        generating={generating}
        genError={genError}
      />
    );
  }

  return (
    <WorkingPhase
      projectId={projectId}
      title={title}
      qualification={qualification}
      proposal={prop}
      seedBoard={seedBoard}
      onReopen={() => setPhase("forming")}
      onOpenItem={(item) => onOpenRoom(roomForTag(item.tag))}
    />
  );
}

const clamp = (n: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, n));

/* ---------- Phase A · forming ---------- */

function FormingPhase(props: {
  title: string;
  qualification: string;
  proposal: Proposal;
  setDim: (key: keyof Proposal, v: string) => void;
  chat: ChatMsg[];
  lang: "zh" | "en";
  onToggleLang: () => void;
  draft: string;
  setDraft: (s: string) => void;
  sending: boolean;
  onSend: () => void;
  showChips: boolean;
  onGuideMe: () => void;
  onSelfFill: () => void;
  onReview: () => void;
  onGenerate: () => void;
  generating: boolean;
  genError: string | null;
}) {
  const {
    title, qualification, proposal, setDim, chat, lang, onToggleLang, draft, setDraft, sending, onSend,
    showChips, onGuideMe, onSelfFill, onReview, onGenerate, generating, genError,
  } = props;
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
          {showChips && !sending && (
            <div className="flex flex-wrap gap-2 pl-1">
              <button
                type="button"
                onClick={onGuideMe}
                className="rounded-full border border-mk-primary bg-mk-primary-tint px-3.5 py-1.5 text-[13px] font-bold text-mk-primary transition hover:bg-mk-primary hover:text-white"
              >
                {lang === "en" ? "Walk me through it" : "带我一部分一部分想"}
              </button>
              <button
                type="button"
                onClick={onSelfFill}
                className="rounded-full border border-mk-border bg-mk-surface px-3.5 py-1.5 text-[13px] font-semibold text-mk-muted transition hover:text-mk-primary"
              >
                {lang === "en" ? "I'll fill it in myself" : "我自己填"}
              </button>
            </div>
          )}
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
          <button
            type="button"
            disabled={!ready || sending}
            onClick={onReview}
            className="mt-3.5 flex items-center justify-center gap-1.5 rounded-mk border border-mk-primary/50 bg-mk-primary-tint py-2 text-[12.5px] font-bold text-mk-primary transition enabled:hover:bg-mk-primary enabled:hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Icon name="spark" size={14} /> 让印记看看我的开题
          </button>
        </div>
        <button
          type="button"
          disabled={!ready || generating}
          onClick={onGenerate}
          className="mt-3 flex items-center justify-center gap-2 rounded-mk bg-mk-accent py-3 text-[14px] font-bold text-white transition enabled:hover:bg-mk-accent-hover disabled:cursor-not-allowed disabled:bg-mk-input disabled:text-mk-muted-2"
        >
          {generating ? (
            "印记正在排计划…"
          ) : (
            <>
              生成项目计划
              <Icon name="arrow" size={16} />
            </>
          )}
        </button>
        {genError && <p className="mt-1.5 text-center text-[12px] font-semibold text-mk-accent">{genError}</p>}
        <button
          type="button"
          onClick={() => setWriting(true)}
          className="mt-2 flex items-center justify-center gap-2 rounded-mk border-2 border-mk-primary bg-mk-surface py-2.5 text-[14px] font-bold text-mk-primary transition hover:bg-mk-primary-tint"
        >
          <Icon name="writing" size={16} /> 写开题报告（可选）
        </button>
      </aside>

      {writing && <ProposalWriter proposal={proposal} setDim={setDim} title={title} qualification={qualification} onClose={() => setWriting(false)} />}
    </div>
  );
}

// The formal proposal is WRITTEN by the student (not generated). This is a
// larger writing surface over the same four (persisted) dimensions, headed as
// EPQ §1–§4, with a structured export (real .docx lands in slice 6). Optional —
// the dimensions are already valued from the chat.
function ProposalWriter({ proposal, setDim, title, qualification, onClose }: { proposal: Proposal; setDim: (k: keyof Proposal, v: string) => void; title: string; qualification: string; onClose: () => void }) {
  const SECTIONS: { key: keyof Proposal; n: string; title: string; hint: string }[] = [
    { key: "objective", n: "§1", title: "题目、目标与职责", hint: "你想回答什么问题？想学会做什么？想发现什么？" },
    { key: "reason", n: "§2", title: "选题理由", hint: "与你所学学科的关联、个人兴趣、未来规划、想提升的知识/技能、为什么这个题目重要" },
    { key: "activities", n: "§3", title: "活动与时间安排", hint: "研究、想法的发展与分析、写作、数据收集、排练、成果产出、评估、准备展示等" },
    { key: "resources", n: "§4", title: "资源", hint: "图书馆、书籍、期刊、设备、场地、技术、经费等" },
  ];
  const [exporting, setExporting] = useState(false);
  async function exportReport() {
    if (exporting) return;
    setExporting(true);
    try {
      await exportProposalDocx(proposal, { title, qualification });
    } catch {
      /* a failed export must never crash the room */
    } finally {
      setExporting(false);
    }
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
            <button type="button" onClick={exportReport} disabled={exporting} className="rounded-mk border border-mk-border px-4 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-primary disabled:opacity-60">{exporting ? "导出中…" : "导出"}</button>
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

// Render **bold** spans inline; everything else is plain text.
function renderRich(text: string) {
  return text.split(/(\*\*[^*]+\*\*)/g).map((part, i) =>
    part.startsWith("**") && part.endsWith("**") ? (
      <strong key={i} className="font-bold">{part.slice(2, -2)}</strong>
    ) : (
      <span key={i}>{part}</span>
    ),
  );
}

function ChatBubble({ msg }: { msg: ChatMsg }) {
  const isAi = msg.role === "ai";
  return (
    <div className={`flex ${isAi ? "justify-start" : "justify-end"}`}>
      <div className={`max-w-[82%] rounded-mk-lg px-4 py-2.5 text-[14px] leading-relaxed ${isAi ? "bg-mk-surface text-mk-ink shadow-[0_1px_2px_rgba(28,35,51,0.05)]" : "bg-mk-primary text-white"}`}>
        {isAi && <span className="mb-0.5 block text-[11px] font-bold text-mk-primary">印记</span>}
        {renderRich(msg.text)}
      </div>
    </div>
  );
}

/* ---------- Phase B · working (kanban / gantt / log) ---------- */

const COLUMNS: PlanColumn[] = ["todo", "doing", "done"];
type PlanView = "kanban" | "gantt" | "log";

function WorkingPhase(props: {
  projectId: string;
  title: string;
  qualification: string;
  proposal: Proposal;
  seedBoard?: PlanItem[];
  onReopen: () => void;
  onOpenItem: (item: PlanItem) => void;
}) {
  const { projectId, title, qualification, proposal, seedBoard, onReopen, onOpenItem } = props;
  const [view, setView] = useState<PlanView>("kanban");
  const [open, setOpen] = useState(false);

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
    <div className="relative flex h-full flex-col px-10 py-8">
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
          <p className="mt-0.5 text-[13px] text-mk-muted">{view === "log" ? "项目一路上发生了什么——大多自动记下，你也能补一笔。" : "拖动来编辑：看板换列、甘特图挪动/拉长。点任务卡查看或修改，点「进入 →」去对应房间。"}</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex rounded-mk border border-mk-border bg-mk-surface p-0.5">
            <ViewTab active={view === "kanban"} onClick={() => setView("kanban")}>看板</ViewTab>
            <ViewTab active={view === "gantt"} onClick={() => setView("gantt")}>甘特图</ViewTab>
            <ViewTab active={view === "log"} onClick={() => setView("log")}>活动日志</ViewTab>
          </div>
          <button type="button" onClick={view === "log" ? exportLog : exportPlan} disabled={exporting} className="rounded-mk border border-mk-border bg-mk-surface px-3.5 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-primary disabled:opacity-60">
            {exporting ? "导出中…" : "导出"}
          </button>
          <button type="button" onClick={onReopen} className="flex items-center gap-1.5 rounded-mk border border-mk-border bg-mk-surface px-3.5 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-primary">
            <Icon name="spark" size={15} /> 聊聊计划
          </button>
        </div>
      </div>

      {view === "kanban" && <KanbanView board={board} loading={loadingPlan} onMove={(id, column) => patchItem(id, { column })} onAddTask={addTask} onEditItem={(i) => setEditingId(i.id)} onJumpItem={onOpenItem} />}
      {view === "gantt" && <GanttView board={board} onReschedule={(id, start) => patchItem(id, { start })} onResize={(id, days) => patchItem(id, { days })} onAddTask={addTaskGantt} onEditItem={(i) => setEditingId(i.id)} onJumpItem={onOpenItem} />}
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

function KanbanView({ board, loading, onMove, onAddTask, onEditItem, onJumpItem }: { board: PlanItem[]; loading: boolean; onMove: (id: string, c: PlanColumn) => void; onAddTask: (stage: string) => void; onEditItem: (i: PlanItem) => void; onJumpItem: (i: PlanItem) => void }) {
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
                <PlanCard key={item.id} item={item} dragging={dragId === item.id} onEdit={() => onEditItem(item)} onJump={() => onJumpItem(item)} onDragStart={() => setDragId(item.id)} onDragEnd={() => { setDragId(null); setOver(null); }} />
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

// A plan card: the body (tag row + title) opens the edit popover; the single
// dedicated "进入 →" button is the doorway jump to the room. #4.
function PlanCard({ item, dragging, onEdit, onJump, onDragStart, onDragEnd }: { item: PlanItem; dragging: boolean; onEdit: () => void; onJump: () => void; onDragStart: () => void; onDragEnd: () => void }) {
  return (
    <div
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      className={`group relative cursor-grab rounded-mk border border-mk-border bg-mk-surface p-3 shadow-[0_1px_2px_rgba(28,35,51,0.04)] transition active:cursor-grabbing hover:border-mk-primary/40 hover:shadow-[0_2px_8px_rgba(28,35,51,0.07)] ${dragging ? "opacity-40" : ""}`}
    >
      <button type="button" onClick={onEdit} title="查看 / 修改任务" className="mb-2 flex w-full items-center gap-1.5 text-left">
        <span className={`rounded px-1.5 py-0.5 text-[11px] font-bold ${TAG_STYLE[item.tag]}`}>{TAG_LABEL[item.tag]}</span>
        <span className="ml-auto truncate text-[10.5px] text-mk-muted-2">{item.stage.split(" · ")[0]}</span>
      </button>
      <button type="button" onClick={onEdit} className="block w-full text-left text-[13.5px] font-medium leading-snug text-mk-ink hover:text-mk-primary">{item.title}</button>
      <button type="button" onClick={onJump} className="mt-2 flex items-center gap-0.5 text-[12px] font-semibold text-mk-primary opacity-0 transition hover:underline group-hover:opacity-100">进入 →</button>
    </div>
  );
}

/* ----- Gantt (drag to move, resize handle to change duration), by stage ----- */

function GanttView({ board, onReschedule, onResize, onAddTask, onEditItem, onJumpItem }: { board: PlanItem[]; onReschedule: (id: string, start: number) => void; onResize: (id: string, days: number) => void; onAddTask: () => void; onEditItem: (i: PlanItem) => void; onJumpItem: (i: PlanItem) => void }) {
  const days = Array.from({ length: TIMELINE_DAYS }, (_, i) => i);
  // Render every stage actually present on the board (a generated plan may use
  // stage names beyond the two canonical ones), keeping board order.
  const stages = Array.from(new Set(board.map((i) => i.stage)));
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
        {stages.map((stage) => {
          const items = board.filter((i) => i.stage === stage);
          if (items.length === 0) return null;
          return (
            <div key={stage}>
              <div className="grid grid-cols-[240px,1fr] border-b border-mk-border-2 bg-mk-bg/50">
                <div className="px-4 py-1.5 text-[11.5px] font-bold uppercase tracking-wider text-mk-muted">{stage}</div>
                <div />
              </div>
              {items.map((item) => (
                <div key={item.id} className="group grid grid-cols-[240px,1fr] items-center border-b border-mk-border-2 hover:bg-mk-bg/40">
                  <div className="flex items-center gap-1 px-4 py-3">
                    {/* label click = view/edit; the small arrow = doorway jump (#4) */}
                    <button type="button" onClick={() => onEditItem(item)} title="查看 / 修改任务" className="flex min-w-0 flex-1 items-center gap-2 text-left">
                      <span className={`flex-none rounded px-1.5 py-0.5 text-[11px] font-bold ${TAG_STYLE[item.tag]}`}>{TAG_LABEL[item.tag]}</span>
                      <span className="truncate text-[13px] font-medium text-mk-ink hover:text-mk-primary">{item.title.replace(/^[读写省]：/, "")}</span>
                    </button>
                    <button type="button" onClick={() => onJumpItem(item)} title="进入对应房间" className="flex-none rounded px-1 text-[13px] font-bold text-mk-primary opacity-0 transition hover:underline group-hover:opacity-100">→</button>
                  </div>
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
        {/* #3 — add a task straight from the Gantt (Kanban already has one) */}
        <div className="grid grid-cols-[240px,1fr] border-b border-mk-border-2">
          <button type="button" onClick={onAddTask} className="px-4 py-2.5 text-left text-[12.5px] font-semibold text-mk-muted-2 hover:text-mk-primary">+ 添加任务</button>
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
          // #5 — one box per date; each date's lines keep their 自动/我记的 tag.
          <div className="flex flex-col gap-4 p-4">
            {groupByDate(rows).map((g) => (
              <div key={g.date} className="rounded-mk border border-mk-border-2 bg-mk-bg/40">
                <div className="border-b border-mk-border-2 px-4 py-2 text-[12px] font-bold text-mk-muted-2">{g.date}</div>
                <div className="flex flex-col">
                  {g.entries.map((e, i) => (
                    <div key={e.id} className={`flex items-start gap-3 px-4 py-2.5 ${i < g.entries.length - 1 ? "border-b border-mk-border-2/60" : ""}`}>
                      <p className="flex-1 text-[13.5px] leading-relaxed text-mk-ink">{e.text}</p>
                      <span className={`flex-none self-start rounded-full px-2 py-0.5 text-[10.5px] font-bold ${e.source === "auto" ? "bg-mk-primary-tint text-mk-primary" : "bg-mk-green-tint text-mk-green"}`}>
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
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-mk-ink/30 px-8" onClick={onClose}>
      <div className="flex w-[460px] flex-col rounded-mk-lg border border-mk-border bg-mk-surface shadow-[0_20px_60px_rgba(28,35,51,0.25)]" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b border-mk-border px-5 py-3.5">
          <h3 className="font-sans text-[16px] font-bold text-mk-ink">任务详情</h3>
          <button type="button" onClick={onClose} className="text-[20px] leading-none text-mk-muted-2 hover:text-mk-ink">×</button>
        </div>
        <div className="flex flex-col gap-4 px-5 py-4">
          <label className="block">
            <span className="mb-1 block text-[11.5px] font-bold text-mk-muted-2">任务</span>
            <input value={title} onChange={(e) => setTitle(e.target.value)} className="w-full rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2 text-[13.5px] text-mk-ink outline-none focus:border-mk-primary" />
          </label>

          <div>
            <span className="mb-1 block text-[11.5px] font-bold text-mk-muted-2">类别</span>
            <div className="flex gap-2">
              {tags.map((t) => (
                <button key={t} type="button" onClick={() => setTag(t)} className={`rounded-mk px-3 py-1.5 text-[12.5px] font-bold transition ${tag === t ? TAG_STYLE[t] + " ring-2 ring-mk-primary/40" : "bg-mk-bg text-mk-muted-2 hover:text-mk-muted"}`}>{TAG_LABEL[t]}</button>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <label className="block">
              <span className="mb-1 block text-[11.5px] font-bold text-mk-muted-2">阶段</span>
              <select value={stage} onChange={(e) => setStage(e.target.value)} className="w-full rounded-mk border border-mk-border bg-mk-input-bg px-2 py-2 text-[13px] text-mk-ink outline-none focus:border-mk-primary">
                {(stageOptions.includes(stage) ? stageOptions : [stage, ...stageOptions]).map((s) => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-1 block text-[11.5px] font-bold text-mk-muted-2">状态</span>
              <select value={column} onChange={(e) => setColumn(e.target.value as PlanColumn)} className="w-full rounded-mk border border-mk-border bg-mk-input-bg px-2 py-2 text-[13px] text-mk-ink outline-none focus:border-mk-primary">
                {COLUMNS.map((c) => (<option key={c} value={c}>{COLUMN_LABEL[c]}</option>))}
              </select>
            </label>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <label className="block">
              <span className="mb-1 block text-[11.5px] font-bold text-mk-muted-2">开始（第几天）</span>
              <input type="number" min={1} max={TIMELINE_DAYS} value={start + 1} onChange={(e) => setStart((Number(e.target.value) || 1) - 1)} className="w-full rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2 text-[13.5px] text-mk-ink outline-none focus:border-mk-primary" />
            </label>
            <label className="block">
              <span className="mb-1 block text-[11.5px] font-bold text-mk-muted-2">持续（天）</span>
              <input type="number" min={1} max={TIMELINE_DAYS} value={days} onChange={(e) => setDays(Number(e.target.value) || 1)} className="w-full rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2 text-[13.5px] text-mk-ink outline-none focus:border-mk-primary" />
            </label>
          </div>
        </div>
        <div className="flex items-center justify-between border-t border-mk-border px-5 py-3.5">
          <button type="button" onClick={onDelete} className="rounded-mk px-3 py-2 text-[13px] font-semibold text-mk-accent hover:bg-mk-accent-tint">删除任务</button>
          <div className="flex gap-2">
            <button type="button" onClick={onClose} className="rounded-mk border border-mk-border px-4 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-primary">取消</button>
            <button type="button" onClick={save} className="rounded-mk bg-mk-primary px-4 py-2 text-[13px] font-bold text-white hover:bg-mk-primary-hover">保存</button>
          </div>
        </div>
      </div>
    </div>
  );
}
