import { useEffect, useRef, useState } from "react";
import { CalendarDays, Clock, Sparkles, Activity, type LucideIcon } from "lucide-react";
import type { ProjectStatus } from "@mind-imprint/contracts";
import type { ProjectListItem } from "@/api/projects";
import {
  Card,
  Badge,
  type BadgeTone,
  Menu,
  IconButton,
  Icon,
  Plus,
  MoreHorizontal,
  ProjectCover,
  MACARONS,
  type MacaronName,
} from "@/ui";

/**
 * ProjectCard / NewProjectTile — the single project catalog card, shared by the
 * 项目 directory (workspace/Directory) and the 首页 最近项目 strip (HomePage).
 * One rich look everywhere: a gradient/photo cover with the status badge
 * overlaid, the title, qualLabel, start date, and a hover-revealed ⋯ menu.
 *
 * The management affordances are opt-in via handlers: pass `onRename` to enable
 * inline rename (and the 重命名 menu item); pass `onViewReport` to add
 * 查看评估报告 on a finished project. With neither, the ⋯ menu is hidden and the
 * card is a plain openable tile.
 */

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

/** RFC3339 → MM-DD, the compact date shown inside the card's meta chips. Falls
 * back to the raw slice for anything that isn't a well-formed ISO string. */
function formatMonthDay(iso: string): string {
  return iso.length >= 10 ? iso.slice(5, 10) : iso;
}

/**
 * MetaChip — one label-style fact on the project card (start / last-active date,
 * AI-call count, activity count). Mirrors CourseCard's meta tags: a soft macaron
 * tint with its paired legible text, icon in `currentColor`. Colors come from the
 * concrete MACARONS hex pairs (never Tailwind `mk-*` alpha, which emits no CSS).
 */
function MetaChip({ tone, icon, children }: { tone: MacaronName; icon: LucideIcon; children: React.ReactNode }) {
  const m = MACARONS[tone];
  return (
    <span
      className="inline-flex items-center gap-1 rounded-mk-full px-2 py-0.5 text-mk-small font-semibold tabular-nums"
      style={{ background: m.bg, color: m.fg }}
    >
      <Icon icon={icon} size={12} />
      {children}
    </span>
  );
}

// The status badge shown on each project card. The lifecycle is derived
// server-side (forming → working → evaluating → done).
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

/** The dashed "新建项目" tile that leads both project card collections.
 * `className` sizes it for its context (fixed-width home strip vs grid cell). */
export function NewProjectTile({ onClick, className }: { onClick: () => void; className?: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cx(
        "flex flex-col items-center justify-center gap-2 rounded-mk-md border border-dashed border-mk-border text-mk-muted",
        "transition-colors duration-[120ms] ease-mk hover:border-mk-accent hover:text-mk-accent-600",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
        className,
      )}
    >
      <span className="flex h-9 w-9 items-center justify-center rounded-mk-full bg-mk-accent-50 text-mk-accent-600">
        <Icon icon={Plus} size={18} />
      </span>
      <span className="text-mk-body font-medium">新建项目</span>
    </button>
  );
}

export function ProjectCard({
  project,
  onOpen,
  onViewReport,
  onRename,
}: {
  project: ProjectListItem;
  onOpen: () => void;
  onViewReport?: (id: string) => void;
  onRename?: (id: string, title: string) => Promise<void>;
}) {
  // 重命名 (when onRename given) is always available; 查看评估报告 only once the
  // report exists (done).
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(project.title || "");
  const [saving, setSaving] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [editing]);

  function startRename() {
    setDraft(project.title || "");
    setEditing(true);
  }

  async function commitRename() {
    const next = draft.trim();
    if (saving || !onRename) return;
    if (next === "" || next === (project.title || "")) {
      setEditing(false);
      return;
    }
    setSaving(true);
    try {
      await onRename(project.id, next);
      setEditing(false);
    } finally {
      setSaving(false);
    }
  }

  const menuItems = [
    ...(onRename ? [{ key: "rename", label: "重命名", onSelect: startRename }] : []),
    ...(project.status === "done" && onViewReport
      ? [{ key: "report", label: "查看评估报告", onSelect: () => onViewReport(project.id) }]
      : []),
  ];

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => {
        if (editing) return;
        onOpen();
      }}
      onKeyDown={(e) => {
        // Only act when the card itself is the focused/keydown target, not a
        // descendant (the ⋯ menu's own trigger button, the rename <input>) —
        // otherwise Enter/Space on those both bubbles up into "open the
        // project" AND has its own native activation suppressed here.
        if (e.target !== e.currentTarget) return;
        if (editing) return;
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

        {menuItems.length > 0 && (
          <div
            className="absolute right-2 top-2 opacity-0 transition-opacity duration-[120ms] ease-mk group-hover:opacity-100 group-focus-within:opacity-100"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <Menu
              trigger={<IconButton icon={MoreHorizontal} label="更多操作" size="sm" variant="secondary" />}
              items={menuItems}
            />
          </div>
        )}

        {editing ? (
          <input
            ref={inputRef}
            value={draft}
            disabled={saving}
            onChange={(e) => setDraft(e.target.value)}
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => {
              e.stopPropagation();
              if (e.key === "Enter") {
                e.preventDefault();
                void commitRename();
              } else if (e.key === "Escape") {
                e.preventDefault();
                setEditing(false);
              }
            }}
            onBlur={() => void commitRename()}
            className="flex-1 rounded-mk-sm border border-mk-accent-200 bg-mk-surface px-2 py-1 text-mk-h3 text-mk-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
            aria-label="项目名称"
          />
        ) : (
          <div
            title={project.title || "未命名项目"}
            className="line-clamp-2 flex-1 text-mk-h3 text-mk-ink"
          >
            {project.title || "未命名项目"}
          </div>
        )}
        <div className="truncate text-mk-small text-mk-muted">
          {project.qualLabel || "项目"}
        </div>
        <div className="mt-0.5 flex flex-wrap items-center gap-1.5">
          {project.createdAt && (
            <MetaChip tone="lake" icon={CalendarDays}>开始 {formatMonthDay(project.createdAt)}</MetaChip>
          )}
          {project.lastActiveAt && (
            <MetaChip tone="mist" icon={Clock}>最近 {formatMonthDay(project.lastActiveAt)}</MetaChip>
          )}
          <MetaChip tone="peach" icon={Sparkles}>AI {project.aiCalls ?? 0}</MetaChip>
          <MetaChip tone="matcha" icon={Activity}>活动 {project.activityLog ?? 0}</MetaChip>
        </div>
      </Card>
    </div>
  );
}
