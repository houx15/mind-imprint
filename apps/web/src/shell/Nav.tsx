import type { ReactNode } from "react";
import { Home, FolderKanban, GraduationCap } from "lucide-react";
import { Icon } from "@/ui/Icon";
import { Pebble } from "@/ui/Pebble";
import type { MeUser } from "@/api";

/**
 * Nav — an accent-colored vertical rail. Four entries top-to-bottom: 首页 /
 * 项目 / 课程 / 我 (the last renders the user's initial in a chip instead of a
 * Lucide icon). A small brand `Pebble` sits above the entries in a white chip
 * so it stays legible on the accent ground.
 *
 * Collapsed the rail is a 64px icon-only strip; on hover (or keyboard focus
 * within) it slides open to 208px and reveals the Chinese labels beside each
 * icon. It reserves only the collapsed 64px in layout — the expanded panel is
 * absolutely positioned and floats over the content, so opening it never
 * reflows the page. Icons stay pinned at the same x while the panel grows.
 *
 * 项目 hosts the project directory/studio AND the 评估报告 timeline (a
 * Segmented inside the tab). 课程 hosts the course list, 学习记录, and 图鉴.
 * The old top-level 评估 tab is gone — its 成长报告 moved under 项目 and its
 * 图鉴 moved under 课程.
 */

export type NavTab = "home" | "projects" | "courses" | "me";

function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

function initialOf(name: string | null | undefined): string {
  const trimmed = (name ?? "").trim();
  return trimmed ? trimmed.charAt(0).toUpperCase() : "?";
}

interface NavEntry {
  key: NavTab;
  label: string;
  render: (active: boolean) => ReactNode;
}

export function Nav({
  tab,
  onTab,
  user,
}: {
  tab: NavTab;
  onTab: (t: NavTab) => void;
  user?: Pick<MeUser, "display_name"> | null;
}) {
  const initial = initialOf(user?.display_name);

  const items: NavEntry[] = [
    {
      key: "home",
      label: "首页",
      render: (active) => (
        <Icon icon={Home} size={22} className={active ? "text-white" : "text-white/70"} />
      ),
    },
    {
      key: "projects",
      label: "项目",
      render: (active) => (
        <Icon icon={FolderKanban} size={22} className={active ? "text-white" : "text-white/70"} />
      ),
    },
    {
      key: "courses",
      label: "课程",
      render: (active) => (
        <Icon icon={GraduationCap} size={22} className={active ? "text-white" : "text-white/70"} />
      ),
    },
    {
      key: "me",
      label: "我",
      render: (active) => (
        <span
          className={cx(
            "flex h-7 w-7 items-center justify-center rounded-mk-full text-[13px] font-bold",
            active ? "bg-white text-mk-accent-700" : "bg-white/20 text-white",
          )}
        >
          {initial}
        </span>
      ),
    },
  ];

  // Labels are always in the DOM (opacity toggled, not conditionally rendered)
  // so they stay queryable and the reveal is a pure CSS transition.
  const labelCls =
    "whitespace-nowrap text-mk-body opacity-0 transition-opacity duration-200 ease-mk " +
    "group-hover/nav:opacity-100 group-focus-within/nav:opacity-100 motion-reduce:transition-none";

  return (
    <div className="relative z-30 w-[64px] shrink-0">
      <nav
        className={cx(
          "group/nav absolute inset-y-0 left-0 flex w-[64px] flex-col gap-1 overflow-hidden p-3",
          "transition-[width] duration-200 ease-mk hover:w-[208px] focus-within:w-[208px]",
          "hover:shadow-mk-lg focus-within:shadow-mk-lg motion-reduce:transition-none",
        )}
        style={{ background: "linear-gradient(180deg, var(--mk-accent-500), var(--mk-accent-600))" }}
        aria-label="主导航"
      >
        <div className="mb-3 flex items-center gap-3 px-1.5">
          <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-mk-full bg-white shadow-mk-xs">
            <Pebble size={18} />
          </span>
          <span className={cx(labelCls, "font-semibold text-white")}>思维印记</span>
        </div>

        {items.map(({ key, label, render }) => {
          const active = tab === key;
          return (
            <button
              key={key}
              type="button"
              role="tab"
              aria-selected={active}
              onClick={() => onTab(key)}
              className={cx(
                "flex items-center gap-3 rounded-mk-md px-1.5 py-2 transition-colors duration-[120ms] ease-mk",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60",
                active ? "bg-white/15" : "hover:bg-white/10",
              )}
            >
              <span className="flex h-7 w-7 shrink-0 items-center justify-center">{render(active)}</span>
              <span className={cx(labelCls, active ? "font-semibold text-white" : "text-white/80")}>
                {label}
              </span>
            </button>
          );
        })}
      </nav>
    </div>
  );
}
