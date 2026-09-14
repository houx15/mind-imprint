import { useEffect, useState } from "react";
import { LayoutGrid, Users, GraduationCap, UploadCloud } from "lucide-react";
import { Icon, Pebble, Settings, type LucideIcon } from "@/ui";
import { api } from "@/api";
import { ClassesView } from "@/console/ClassesView";
import { OverviewView } from "@/console/OverviewView";
import { TeachersView } from "@/console/TeachersView";
import { ImportView } from "@/console/ImportView";
import { SettingsView } from "@/shell/settings/SettingsView";
import { createSession, makeMemoryStorage } from "@/shell/session";
import { navigate } from "../routing";
import type { MeUser } from "../api/auth";
import { parseTeacherRoute, teacherRoutePath, type TeacherRoute } from "./teacherRouting";
import { ClassPage } from "./ClassPage";
import { StudentPage } from "./StudentPage";
import { ItemPage } from "./ItemPage";

/**
 * LiteTeacherShell — the lite edition's teacher/admin shell. Same pattern as
 * pro's `AppShell` → `ConsoleShell`: a role branch in `LiteApp.tsx` sends
 * teachers and admins here instead of into the student-facing `LiteShell`.
 *
 * Rail markup (`mk-lite-navslot`/`mk-lite-nav`, `labelCls`, the 设置 button
 * pinned with `mt-auto`) is copied from `LiteShell` in `LiteApp.tsx` so the
 * two shells look identical — this is deliberate duplication, not a shared
 * component, for the same reason `LiteShell` itself does not import from
 * pro's `Nav`: the two rails evolve on their own schedules.
 */

// SettingsView requires a pro SessionStore; lite's auth state lives in
// LiteApp, so this one is never read. Same construction as LiteApp.tsx's
// unusedSettingsSession.
const unusedSettingsSession = createSession({ storage: makeMemoryStorage() });

type RailItem = { key: TeacherRoute["view"]; label: string; icon: LucideIcon };

const TEACHER_ITEMS: RailItem[] = [{ key: "classes", label: "班级", icon: Users }];

const ADMIN_ITEMS: RailItem[] = [
  { key: "overview", label: "概览", icon: LayoutGrid },
  { key: "classes", label: "班级", icon: Users },
  { key: "teachers", label: "教师", icon: GraduationCap },
  { key: "import", label: "导入", icon: UploadCloud },
];

/** The role-appropriate landing view — same split as pro `ConsoleShell`
 * (`role === "admin" ? "overview" : "classes"`). Only used for the BOOT
 * path (an unset/root pathname); an explicit path she navigated to (e.g.
 * `/classes`) is always honoured as typed. */
function landingRoute(role: string): TeacherRoute {
  return role === "admin" ? { view: "overview" } : { view: "classes" };
}

const ADMIN_ONLY_VIEWS: TeacherRoute["view"][] = ["overview", "teachers", "import"];

export function LiteTeacherShell({ user, onLogout }: { user: MeUser; onLogout: () => void }) {
  const [route, setRoute] = useState<TeacherRoute>(() => {
    const path = window.location.pathname;
    if (path === "/" || path === "") return landingRoute(user.role);
    return parseTeacherRoute(path);
  });

  useEffect(() => {
    const onPop = () => setRoute(parseTeacherRoute(window.location.pathname));
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  const go = (r: TeacherRoute) => navigate(teacherRoutePath(r));

  // A teacher (not admin) who lands on an admin-only view — by typing the
  // URL, or a stale link — is sent to 班级 instead of shown a page she has
  // no data for.
  useEffect(() => {
    if (user.role !== "admin" && ADMIN_ONLY_VIEWS.includes(route.view)) {
      go({ view: "classes" });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [route.view, user.role]);

  const railItems = user.role === "admin" ? ADMIN_ITEMS : TEACHER_ITEMS;

  // Labels stay in the DOM always (opacity toggled, not conditionally
  // rendered) so the reveal is a pure CSS transition — same trick as `Nav`
  // / `LiteShell`.
  const labelCls =
    "whitespace-nowrap text-mk-body opacity-0 transition-opacity duration-200 ease-mk " +
    "group-hover/nav:opacity-100 group-focus-within/nav:opacity-100 motion-reduce:transition-none";

  function cx(...parts: Array<string | false | null | undefined>): string {
    return parts.filter(Boolean).join(" ");
  }

  return (
    <div className="flex h-full w-full overflow-hidden bg-mk-paper text-mk-ink">
      <div className="mk-lite-navslot relative z-30 w-[64px] shrink-0">
        <nav
          className={cx(
            "mk-lite-nav group/nav absolute inset-y-0 left-0 flex w-[64px] flex-col gap-1 overflow-hidden p-3",
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
            <span className={cx(labelCls, "font-semibold text-white")}>思维印记 · 轻量版</span>
          </div>

          {railItems.map(({ key, label, icon }) => {
            const active =
              route.view === key ||
              // 学生详情/单项详情在「班级」下面，班级那一格仍然是选中的。
              (key === "classes" &&
                (route.view === "class" || route.view === "student" || route.view === "item"));
            return (
              <button
                key={key}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => go(key === "classes" ? { view: "classes" } : ({ view: key } as TeacherRoute))}
                className={cx(
                  "flex items-center gap-3 rounded-mk-md px-1.5 py-2 transition-colors duration-[120ms] ease-mk",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60",
                  active ? "bg-white/15" : "hover:bg-white/10",
                )}
              >
                <span className="flex h-7 w-7 shrink-0 items-center justify-center">
                  <Icon icon={icon} size={22} className={active ? "text-white" : "text-white/70"} />
                </span>
                <span className={cx(labelCls, active ? "font-semibold text-white" : "text-white/80")}>
                  {label}
                </span>
              </button>
            );
          })}

          {/* 设置 sits at the FOOT of the rail, not among the tabs: it is where
              the account lives, not a third place to work. `mt-auto` pins it
              below whatever tabs exist above. */}
          <button
            type="button"
            onClick={() => go({ view: "settings" })}
            aria-current={route.view === "settings" ? "page" : undefined}
            className={cx(
              "mt-auto flex items-center gap-3 rounded-mk-md px-1.5 py-2 transition-colors duration-[120ms] ease-mk",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60",
              route.view === "settings" ? "bg-white/15" : "hover:bg-white/10",
            )}
          >
            <span className="flex h-7 w-7 shrink-0 items-center justify-center">
              <Icon icon={Settings} size={22} className={route.view === "settings" ? "text-white" : "text-white/70"} />
            </span>
            <span className={cx(labelCls, route.view === "settings" ? "font-semibold text-white" : "text-white/80")}>
              设置
            </span>
          </button>
        </nav>
      </div>

      <main className="min-w-0 flex-1 overflow-y-auto">
        {route.view === "classes" && (
          <ClassesView client={api} role={user.role} onOpenClass={(classId) => go({ view: "class", classId })} />
        )}
        {route.view === "class" && (
          <ClassPage
            classId={route.classId}
            role={user.role}
            onBack={() => go({ view: "classes" })}
            onOpenStudent={(userId) => go({ view: "student", classId: route.classId, userId })}
          />
        )}
        {route.view === "student" && (
          <StudentPage
            classId={route.classId}
            userId={route.userId}
            onBack={() => go({ view: "class", classId: route.classId })}
            onOpenItem={(atomId) => go({ view: "item", classId: route.classId, userId: route.userId, atomId })}
          />
        )}
        {route.view === "item" && (
          <ItemPage
            classId={route.classId}
            userId={route.userId}
            atomId={route.atomId}
            onBack={() => go({ view: "student", classId: route.classId, userId: route.userId })}
          />
        )}
        {route.view === "overview" && user.role === "admin" && <OverviewView client={api} />}
        {route.view === "teachers" && user.role === "admin" && <TeachersView client={api} />}
        {route.view === "import" && user.role === "admin" && <ImportView client={api} />}
        {route.view === "settings" && (
          <SettingsView session={unusedSettingsSession} user={user} onLogout={onLogout} />
        )}
      </main>
    </div>
  );
}
