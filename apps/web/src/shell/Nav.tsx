import type { ReactNode } from "react";
import { Home, FolderKanban, Sparkles } from "lucide-react";
import { Icon } from "@/ui/Icon";
import { Pebble } from "@/ui/Pebble";
import type { MeUser } from "@/api";

/**
 * Nav — 52px vertical icon rail (design-system rebuild, shell Task 4).
 * Replaces `LeftRail`. Four entries top-to-bottom: 首页 / 项目 / 图鉴 / 我
 * (the last renders the user's initial in an accent circle instead of a
 * Lucide icon). A small brand `Pebble` sits above the entries.
 */

export type NavTab = "home" | "projects" | "gallery" | "me";

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
        <Icon icon={Home} size={18} className={active ? "text-mk-accent-600" : "text-mk-muted"} />
      ),
    },
    {
      key: "projects",
      label: "项目",
      render: (active) => (
        <Icon icon={FolderKanban} size={18} className={active ? "text-mk-accent-600" : "text-mk-muted"} />
      ),
    },
    {
      key: "gallery",
      label: "图鉴",
      render: (active) => (
        <Icon icon={Sparkles} size={18} className={active ? "text-mk-accent-600" : "text-mk-muted"} />
      ),
    },
    {
      key: "me",
      label: "我",
      render: () => (
        <span className="flex h-5 w-5 items-center justify-center rounded-mk-full bg-mk-accent text-[10px] font-bold text-white">
          {initial}
        </span>
      ),
    },
  ];

  return (
    <nav
      className="flex w-[52px] shrink-0 flex-col items-center gap-1 border-r border-mk-border bg-mk-surface py-3"
      aria-label="主导航"
    >
      <div className="mb-3">
        <Pebble size={22} />
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
              "flex flex-col items-center gap-0.5 rounded-mk-sm px-1 py-1.5",
              active && "bg-mk-accent-50",
            )}
          >
            {render(active)}
            <span
              className={cx(
                "text-[9px] leading-none",
                active ? "font-semibold text-mk-accent-600" : "text-mk-muted",
              )}
            >
              {label}
            </span>
          </button>
        );
      })}
    </nav>
  );
}
