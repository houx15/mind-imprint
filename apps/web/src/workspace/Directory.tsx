import { useEffect, useRef, useState } from "react";
import type { ProjectStatus } from "@mind-imprint/contracts";
import { api } from "../api";
import type { ProjectListItem } from "../api/projects";
import { CreateProjectDrawer } from "./CreateProjectDrawer";
import {
  Card,
  Badge,
  type BadgeTone,
  Menu,
  IconButton,
  Icon,
  Plus,
  MoreHorizontal,
  SkeletonCard,
  EmptyState,
  ProjectCover,
} from "../ui";

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

// The status badge shown on each project card. The lifecycle is derived
// server-side (forming → working → evaluating → done); labels mirror
// shell/home/HomePage.tsx's STATUS_LABEL/STATUS_TONE so the wording and tint
// never drift between the home snapshot and this full list.
const STATUS_LABEL: Record<ProjectStatus, string> = {
  forming: "立题中",
  working: "进行中",
  evaluating: "评估中",
  done: "已完成",
};

const STATUS_TONE: Record<ProjectStatus, BadgeTone> = {
  forming: "draft",
  working: "progress",
  evaluating: "pending",
  done: "done",
};

function NewProjectTile({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cx(
        "flex min-h-[240px] flex-col items-center justify-center gap-2 rounded-mk-md border border-dashed border-mk-border text-mk-muted",
        "transition-colors duration-[120ms] ease-mk hover:border-mk-accent hover:text-mk-accent-600",
        "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-mk-accent/15",
      )}
    >
      <span className="flex h-9 w-9 items-center justify-center rounded-mk-full bg-mk-accent-50 text-mk-accent-600">
        <Icon icon={Plus} size={18} />
      </span>
      <span className="text-mk-body font-medium">新建项目</span>
    </button>
  );
}

function ProjectCard({
  project,
  onOpen,
  onViewReport,
}: {
  project: ProjectListItem;
  onOpen: () => void;
  onViewReport?: (id: string) => void;
}) {
  // The only action that exists today on a project card is the done-status
  // "查看评估报告" deep-link — no rename/archive/delete backend exists yet,
  // so the overflow menu simply doesn't render for any other status rather
  // than offering actions that would silently do nothing.
  const showMenu = project.status === "done" && Boolean(onViewReport);
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => {
        // Only act when the card itself is the focused/keydown target, not a
        // descendant (the ⋯ menu's own trigger button, an <input> inside a
        // future field, etc.) — otherwise Enter/Space on the menu trigger
        // both bubbles up into "open the project" AND has its own native
        // button activation suppressed by this handler's preventDefault.
        if (e.target !== e.currentTarget) return;
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onOpen();
        }
      }}
      className="group h-full cursor-pointer"
    >
      <Card className="relative flex h-full flex-col gap-2 p-4">
        <div className="relative -mx-4 -mt-4 h-[120px] w-[calc(100%+2rem)] shrink-0 overflow-hidden rounded-t-mk-sm">
          <ProjectCover project={project} />
          <Badge tone={STATUS_TONE[project.status]} className="absolute left-2 top-2">
            {STATUS_LABEL[project.status]}
          </Badge>
        </div>

        {showMenu && (
          <div
            className="absolute right-2 top-2 opacity-0 transition-opacity duration-[120ms] ease-mk group-hover:opacity-100 group-focus-within:opacity-100"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <Menu
              trigger={<IconButton icon={MoreHorizontal} label="更多操作" size="sm" variant="secondary" />}
              items={[
                {
                  key: "report",
                  label: "查看评估报告",
                  onSelect: () => onViewReport?.(project.id),
                },
              ]}
            />
          </div>
        )}

        <div className="line-clamp-2 flex-1 text-mk-h3 text-mk-ink">{project.title || "未命名项目"}</div>
        <div className="truncate text-mk-small text-mk-muted">
          {project.qualLabel || "项目"}
          {project.activeStation ? ` · ${project.activeStation}` : ""}
        </div>
      </Card>
    </div>
  );
}

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

  return (
    <div className="mx-auto flex h-full w-full max-w-5xl flex-col px-8 py-10 font-sans text-mk-ink">
      <header className="mb-6">
        <p className="text-mk-small font-semibold uppercase tracking-[0.18em] text-mk-faint">你的项目</p>
        <h1 className="mt-1 text-mk-h1 text-mk-ink">全部项目</h1>
        <p className="mt-1.5 text-mk-body text-mk-muted">选一个继续，或开一个新项目——带着你真实的任务进来。</p>
      </header>

      {error && <div className="mb-4 rounded-mk-sm border border-mk-danger bg-mk-danger-bg px-4 py-2.5 text-mk-small font-semibold text-mk-danger">{error}</div>}

      <div className="min-h-0 flex-1 overflow-y-auto">
        {loading ? (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-4">
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
          <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-4">
            <NewProjectTile onClick={() => setDrawerOpen(true)} />
            {projects.map((p) => (
              <ProjectCard key={p.id} project={p} onOpen={() => onOpen(p.id)} onViewReport={onViewReport} />
            ))}
          </div>
        )}
      </div>

      <CreateProjectDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} onCreated={handleCreated} />
    </div>
  );
}
