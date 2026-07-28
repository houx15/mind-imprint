import { useEffect, useState } from "react";
import { api } from "../api";
import type { ProjectListItem } from "../api/projects";
import { Icon } from "./Icon";

// The all-projects home: the student's list of workspaces + a create form.
// Opening a row (or a fresh create) hands the id up to WorkspaceContainer,
// which swaps the directory for the four-room shell. Create needs only a
// title + qualification — no prompt gate (the old studio's seed step is gone).
// Real per-room wiring lands in slices 2–5; this slice reuses the existing
// listProjects/createProject client as-is.
export function Directory({ onOpen }: { onOpen: (id: string) => void }) {
  const [projects, setProjects] = useState<ProjectListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [title, setTitle] = useState("");
  const [qualification, setQualification] = useState("");
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await api.listProjects();
        if (!cancelled) setProjects(list);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleCreate() {
    const t = title.trim();
    if (!t || creating) return;
    setCreating(true);
    setError(null);
    try {
      // The redesign's create body is just title + qualification. The existing
      // client still carries a `prompt` seed (retired for real in a later
      // slice) — pass the title so the backend has a non-empty seed. The
      // qualification is captured here for the coming wiring.
      const { id } = await api.createProject({ title: t, prompt: qualification.trim() || t });
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
        <div className="flex flex-col gap-3 sm:flex-row">
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") handleCreate(); }}
            placeholder="给项目起个名字，比如「中国是否让地球更可持续？」"
            className="flex-1 rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2.5 text-[14px] text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
          />
          <input
            value={qualification}
            onChange={(e) => setQualification(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") handleCreate(); }}
            placeholder="类型（如 EPQ / TOK）"
            className="w-full rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2.5 text-[14px] text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary sm:w-48"
          />
          <button
            type="button"
            onClick={handleCreate}
            disabled={!title.trim() || creating}
            className="flex flex-none items-center justify-center gap-2 rounded-mk bg-mk-primary px-5 py-2.5 text-[14px] font-bold text-white transition enabled:hover:bg-mk-primary-hover disabled:cursor-not-allowed disabled:bg-mk-input disabled:text-mk-muted-2"
          >
            {creating ? "创建中…" : "新建"}
            {!creating && <Icon name="arrow" size={16} />}
          </button>
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
            <p className="mt-1.5 text-[13px] text-mk-muted-2">在上面起个名字，开始你的第一个项目。</p>
          </div>
        ) : (
          <div className="flex flex-col gap-2.5">
            {projects.map((p) => (
              <button
                key={p.id}
                type="button"
                onClick={() => onOpen(p.id)}
                className="group flex items-center gap-3 rounded-mk-lg border border-mk-border bg-mk-surface px-5 py-4 text-left shadow-[0_1px_2px_rgba(28,35,51,0.04)] transition hover:border-mk-primary/40 hover:shadow-[0_2px_8px_rgba(28,35,51,0.07)]"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[15px] font-bold text-mk-ink group-hover:text-mk-primary">{p.title || "未命名项目"}</p>
                  <span className="mt-1.5 inline-block rounded-full bg-mk-primary-tint px-2.5 py-0.5 text-[11px] font-bold text-mk-primary">{p.qualLabel || "项目"}</span>
                </div>
                <span className="flex-none text-mk-muted-2 transition group-hover:text-mk-primary">
                  <Icon name="arrow" size={18} />
                </span>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
