import { LayoutGrid, Users, GraduationCap, UploadCloud } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Icon, Pebble, Settings } from "@/ui";

export type ConsoleTab = "overview" | "classes" | "teachers" | "import" | "settings";

function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

interface NavItem {
  key: ConsoleTab;
  label: string;
  icon: LucideIcon;
}

const NAV_ITEMS: NavItem[] = [
  { key: "overview", label: "概览", icon: LayoutGrid },
  { key: "classes", label: "班级", icon: Users },
  { key: "teachers", label: "教师", icon: GraduationCap },
  { key: "import", label: "导入", icon: UploadCloud },
  { key: "settings", label: "设置", icon: Settings },
];

const TEACHER_TABS: ConsoleTab[] = ["classes", "settings"];

export function ConsoleRail({ role, tab, onTab }: { role: string; tab: ConsoleTab; onTab: (t: ConsoleTab) => void }) {
  const items = role === "admin" ? NAV_ITEMS : NAV_ITEMS.filter((i) => TEACHER_TABS.includes(i.key));
  return (
    <div className="flex w-[74px] shrink-0 flex-col items-center gap-1 border-r border-mk-border bg-mk-surface py-4">
      <div className="mb-4">
        <Pebble size={38} />
      </div>

      {items.map(({ key, label, icon }) => {
        const isActive = tab === key;
        return (
          <div
            key={key}
            role="tab"
            aria-selected={isActive}
            onClick={() => onTab(key)}
            className="flex cursor-pointer flex-col items-center gap-[5px] py-1"
          >
            <div className={cx("flex h-[42px] w-[42px] items-center justify-center rounded-mk-lg", isActive && "bg-mk-accent-50")}>
              <Icon icon={icon} size={20} className={isActive ? "text-mk-accent-600" : "text-mk-faint"} />
            </div>
            <span className={cx("text-[10px]", isActive ? "font-bold text-mk-accent-600" : "text-mk-faint")}>{label}</span>
          </div>
        );
      })}
    </div>
  );
}
