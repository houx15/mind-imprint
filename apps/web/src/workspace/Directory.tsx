import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import type { ProjectListItem } from "../api/projects";
import { CreateProjectDrawer } from "./CreateProjectDrawer";
import { SkeletonCard, EmptyState } from "../ui";
import { ProjectCard, NewProjectTile } from "@/catalog/ProjectCard";

// The all-projects home: a gradient-cover card grid of the student's
// workspaces, led by a dashed "新建" tile that opens the create drawer.
// Opening a card (or a fresh create) hands the id up to WorkspaceContainer,
// which swaps the directory for the four-room shell. While any project is
// "评估中" the list polls so it flips to "已完成" (查看评估报告 in the
// card's overflow menu) without a manual refresh.
export function Directory({
  onOpen,
  onViewReport,
  autoOpenCreate,
  onAutoOpenCreateHandled,
}: {
  onOpen: (id: string) => void;
  onViewReport?: (id: string) => void;
  /** Open the create drawer as soon as this mounts (home's "新建" deep-link,
   * Task 6) — read once, on mount, not tracked as a live toggle. */
  autoOpenCreate?: boolean;
  onAutoOpenCreateHandled?: () => void;
}) {
  const [projects, setProjects] = useState<ProjectListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Mount-only: Directory itself unmounts/remounts each time the workspace
  // swaps between the directory and an open project, so "on mount" already
  // means "this specific open-request", with no live-toggle re-trigger risk.
  useEffect(() => {
    if (autoOpenCreate) {
      setDrawerOpen(true);
      onAutoOpenCreateHandled?.();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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

  function handleCreated(id: string) {
    setDrawerOpen(false);
    onOpen(id);
  }

  async function handleRename(id: string, title: string) {
    const { title: saved } = await api.renameProject(id, title);
    setProjects((prev) => prev.map((p) => (p.id === id ? { ...p, title: saved } : p)));
  }

  return (
    <div className="flex h-full w-full flex-col px-10 pb-14 pt-11 font-sans text-mk-ink">
      <header className="mb-6 text-[28px] font-extrabold tracking-[-0.01em] text-mk-ink">
        <span className="text-mk-muted">项目：</span>探究性写作空间
      </header>

      {error && <div className="mb-4 rounded-mk-sm border border-mk-danger bg-mk-danger-bg px-4 py-2.5 text-mk-small font-semibold text-mk-danger">{error}</div>}

      <div className="min-h-0 flex-1 overflow-y-auto">
        {loading ? (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-5">
            {Array.from({ length: 4 }, (_, i) => (
              <SkeletonCard key={i} />
            ))}
          </div>
        ) : projects.length === 0 ? (
          <EmptyState
            illustration="emptyProjects"
            title="还没有项目"
            body="在下面新建一个，贴上作业题目，开始你的第一个项目。"
            action={{ label: "新建项目", onClick: () => setDrawerOpen(true) }}
          />
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-5">
            <NewProjectTile onClick={() => setDrawerOpen(true)} className="min-h-[240px]" />
            {projects.map((p) => (
              <ProjectCard key={p.id} project={p} onOpen={() => onOpen(p.id)} onViewReport={onViewReport} onRename={handleRename} />
            ))}
          </div>
        )}
      </div>

      <CreateProjectDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} onCreated={handleCreated} />
    </div>
  );
}
