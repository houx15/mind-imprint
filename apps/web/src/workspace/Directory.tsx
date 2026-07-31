import { useEffect, useRef, useState } from "react";
import type { ProjectStatus } from "@mind-imprint/contracts";
import { api } from "../api";
import type { ProjectListItem } from "../api/projects";
import { Icon } from "./Icon";

// The status pill shown on each project row. The lifecycle is derived server-
// side (forming → working → evaluating → done); the tints read at a glance.
const STATUS_META: Record<ProjectStatus, { label: string; cls: string }> = {
  forming: { label: "立题中", cls: "bg-mk-bg text-mk-muted" },
  working: { label: "进行中", cls: "bg-mk-primary-tint text-mk-primary" },
  evaluating: { label: "评估中", cls: "bg-mk-amber/15 text-mk-amber animate-pulse" },
  done: { label: "已完成", cls: "bg-mk-green-tint text-mk-green" },
};

// #1: the project TYPE is picked from a selector (not typed). The value is a
// display label shown as the project pill; it does not change the board.
const PROJECT_TYPES = ["拓展论文 EE", "TOK 论文", "内部评估 IA", "EPQ", "个人项目", "其他"];

// #4: the essay's target writing language. Students on the international track
// ultimately write in English, so it leads and is the default.
const WRITING_LANGS: { value: "en" | "zh" | "bilingual"; label: string }[] = [
  { value: "en", label: "English" },
  { value: "zh", label: "中文" },
  { value: "bilingual", label: "双语" },
];

// Derive a readable project title from the first line of the assignment prompt
// — the refined research question is sharpened later, in forming.
function titleFromPrompt(prompt: string): string {
  const firstLine = prompt.split("\n").map((s) => s.trim()).find((s) => s.length > 0) ?? "";
  return [...firstLine].slice(0, 60).join("");
}

// The all-projects home: the student's list of workspaces + a create form.
// Opening a row (or a fresh create) hands the id up to WorkspaceContainer,
// which swaps the directory for the four-room shell. Each row shows its
// lifecycle status; while any project is "评估中" the list polls so it flips to
// "已完成 · 查看评估报告" without a manual refresh.
export function Directory({ onOpen, onViewReport }: { onOpen: (id: string) => void; onViewReport?: (id: string) => void }) {
  const [projects, setProjects] = useState<ProjectListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [prompt, setPrompt] = useState("");
  const [projectType, setProjectType] = useState(PROJECT_TYPES[0]);
  const [writingLang, setWritingLang] = useState<"en" | "zh" | "bilingual">("en");
  const [creating, setCreating] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const list = await api.listProjects();
        if (cancelled) return;
        setProjects(list);
        // Poll only while something is generating its evaluation.
        const anyEvaluating = list.some((p) => p.status === "evaluating");
        if (anyEvaluating && !pollRef.current) {
          pollRef.current = setInterval(load, 15000);
        } else if (!anyEvaluating && pollRef.current) {
          clearInterval(pollRef.current);
          pollRef.current = null;
        }
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
    };
  }, []);

  async function handleCreate() {
    const p = prompt.trim();
    if (!p || creating) return;
    setCreating(true);
    setError(null);
    try {
      // Primary captured field is the assignment prompt (#1); the title is
      // derived from it for the list, and the research question is sharpened
      // later in forming. Type + writing language travel too (#1, #4).
      const { id } = await api.createProject({
        title: titleFromPrompt(p),
        prompt: p,
        projectType,
        writingLanguage: writingLang,
      });
      onOpen(id);
    } catch {
      setError("创建失败，请重试");
      setCreating(false);
    }
  }

  return (
    <div className="mx-auto flex h-full w-full max-w-3xl flex-col px-8 py-10 font-sans text-mk-ink">
      <header className="mb-6">
        <p className="text-[12px] font-semibold uppercase tracking-[0.18em] text-mk-muted-2">你的项目</p>
        <h1 className="mt-1 font-sans text-[26px] font-bold leading-tight text-mk-ink">全部项目</h1>
        <p className="mt-1.5 text-[14px] text-mk-muted">选一个继续，或开一个新项目——带着你真实的任务进来。</p>
      </header>

      {/* create form */}
      <div className="mb-7 rounded-mk-lg border border-mk-border bg-mk-surface p-5 shadow-[0_1px_3px_rgba(28,35,51,0.05)]">
        <div className="mb-3 flex items-center gap-2 text-mk-primary">
          <Icon name="spark" size={16} />
          <span className="text-[13px] font-bold tracking-wide">新建项目</span>
        </div>
        <div className="flex flex-col gap-3">
          {/* Row 1: project type + writing language selectors */}
          <div className="flex flex-col gap-3 sm:flex-row">
            <label className="flex flex-1 flex-col gap-1">
              <span className="text-[12px] font-semibold text-mk-muted-2">项目类型</span>
              <select
                value={projectType}
                onChange={(e) => setProjectType(e.target.value)}
                className="rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2.5 text-[14px] text-mk-ink outline-none focus:border-mk-primary"
              >
                {PROJECT_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </label>
            <label className="flex flex-1 flex-col gap-1">
              <span className="text-[12px] font-semibold text-mk-muted-2">写作语言</span>
              <select
                value={writingLang}
                onChange={(e) => setWritingLang(e.target.value as "en" | "zh" | "bilingual")}
                className="rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2.5 text-[14px] text-mk-ink outline-none focus:border-mk-primary"
              >
                {WRITING_LANGS.map((l) => <option key={l.value} value={l.value}>{l.label}</option>)}
              </select>
            </label>
          </div>
          {/* Row 2: the assignment prompt — the primary field (#1) */}
          <label className="flex flex-col gap-1">
            <span className="text-[12px] font-semibold text-mk-muted-2">作业题目 / 提示</span>
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              // Enter+Cmd/Ctrl submits; plain Enter is a newline so long, multi-
              // line prompts paste cleanly. IME-safe.
              onKeyDown={(e) => { if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && !e.nativeEvent.isComposing) { e.preventDefault(); handleCreate(); } }}
              rows={3}
              placeholder="贴上你的作业题目或提示，比如「Can we only understand something to the extent that we understand its context? Discuss with reference to two areas of knowledge.」"
              className="min-h-[76px] resize-y rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2.5 text-[14px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
            />
          </label>
          <div className="flex items-center justify-between gap-3">
            <span className="text-[12px] text-mk-muted-2">研究问题不用现在就想好——进立题房间我陪你一部分一部分磨。</span>
            <button
              type="button"
              onClick={handleCreate}
              disabled={!prompt.trim() || creating}
              className="flex flex-none items-center justify-center gap-2 rounded-mk bg-mk-primary px-5 py-2.5 text-[14px] font-bold text-white transition enabled:hover:bg-mk-primary-hover disabled:cursor-not-allowed disabled:bg-mk-input disabled:text-mk-muted-2"
            >
              {creating ? "创建中…" : "开始"}
              {!creating && <Icon name="arrow" size={16} />}
            </button>
          </div>
        </div>
      </div>

      {error && <div className="mb-4 rounded-mk border border-mk-border bg-mk-accent-tint/40 px-4 py-2.5 text-[13px] font-semibold text-mk-accent">{error}</div>}

      {/* list */}
      <div className="min-h-0 flex-1 overflow-y-auto">
        {loading ? (
          <p className="py-10 text-center text-[13px] text-mk-muted-2">正在加载…</p>
        ) : projects.length === 0 ? (
          <div className="rounded-mk-lg border border-dashed border-mk-border px-6 py-12 text-center">
            <p className="text-[14px] font-bold text-mk-ink">还没有项目</p>
            <p className="mt-1.5 text-[13px] text-mk-muted-2">在上面贴上作业题目，开始你的第一个项目。</p>
          </div>
        ) : (
          <div className="flex flex-col gap-2.5">
            {projects.map((p) => {
              const status = STATUS_META[p.status] ?? STATUS_META.working;
              return (
                <div
                  key={p.id}
                  role="button"
                  tabIndex={0}
                  onClick={() => onOpen(p.id)}
                  onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpen(p.id); } }}
                  className="group flex cursor-pointer items-center gap-3 rounded-mk-lg border border-mk-border bg-mk-surface px-5 py-4 text-left shadow-[0_1px_2px_rgba(28,35,51,0.04)] transition hover:border-mk-primary/40 hover:shadow-[0_2px_8px_rgba(28,35,51,0.07)]"
                >
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-[15px] font-bold text-mk-ink group-hover:text-mk-primary">{p.title || "未命名项目"}</p>
                    <div className="mt-1.5 flex items-center gap-2">
                      <span className="inline-block rounded-full bg-mk-primary-tint px-2.5 py-0.5 text-[11px] font-bold text-mk-primary">{p.qualLabel || "项目"}</span>
                      <span className={`inline-block rounded-full px-2.5 py-0.5 text-[11px] font-bold ${status.cls}`}>{status.label}</span>
                    </div>
                  </div>
                  {p.status === "done" && onViewReport && (
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); onViewReport(p.id); }}
                      className="flex-none rounded-mk border border-mk-green/40 bg-mk-green-tint px-3 py-1.5 text-[12.5px] font-bold text-mk-green transition hover:bg-mk-green/20"
                    >
                      查看评估报告 →
                    </button>
                  )}
                  <span className="flex-none text-mk-muted-2 transition group-hover:text-mk-primary">
                    <Icon name="arrow" size={18} />
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
