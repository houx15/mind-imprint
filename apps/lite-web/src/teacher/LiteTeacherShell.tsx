import { useEffect, useLayoutEffect, useState } from "react";
import { LayoutGrid, Users, GraduationCap, UploadCloud, ClipboardList } from "lucide-react";
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
import { resolveTeacherRoute, teacherRoutePath, type TeacherRoute } from "./teacherRouting";
import { ClassPage } from "./ClassPage";
import { StudentPage } from "./StudentPage";
import { ItemPage } from "./ItemPage";
import { AssignmentsPage } from "./AssignmentsPage";
import { AssignmentForm } from "./AssignmentForm";
import { AssignmentDetailPage } from "./AssignmentDetailPage";
import { writeLastClassId } from "./assignmentLogic";

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

const TEACHER_ITEMS: RailItem[] = [
  { key: "classes", label: "班级", icon: Users },
  { key: "assignments", label: "布置", icon: ClipboardList },
];

const ADMIN_ITEMS: RailItem[] = [
  { key: "overview", label: "概览", icon: LayoutGrid },
  { key: "classes", label: "班级", icon: Users },
  { key: "assignments", label: "布置", icon: ClipboardList },
  { key: "teachers", label: "教师", icon: GraduationCap },
  { key: "import", label: "导入", icon: UploadCloud },
];

export function LiteTeacherShell({ user, onLogout }: { user: MeUser; onLogout: () => void }) {
  // `resolveTeacherRoute` (not `parseTeacherRoute`) is what decides what she
  // is looking at: it folds an unrecognised path (root, a leftover student
  // path, a typo) onto her role's landing tab, and sends a teacher away from
  // an admin-only view. See its doc comment in `teacherRouting.ts` for why
  // this has to be a distinct function from the raw parser.
  const [route, setRoute] = useState<TeacherRoute>(() =>
    resolveTeacherRoute(window.location.pathname, user.role),
  );

  // Reconcile the address bar with the BOOT resolution before the first
  // paint (a layout effect, not a regular one, so there is no blank frame
  // where the wrong view would have flashed). This runs once: if she lands
  // on `/` as admin, the bar becomes `/overview` immediately, and if she
  // signs in with a stale student path still showing (logout clears `user`
  // without navigating), it becomes her landing route instead of staying on
  // a path this shell can't render. Always `replaceState`, never
  // `pushState` — this is a correction of the entry she's already on, not a
  // new one, so it must not create a Back-button trap.
  useLayoutEffect(() => {
    const path = teacherRoutePath(route);
    if (window.location.pathname !== path) {
      window.history.replaceState(null, "", path);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    // Same resolver on Back/Forward: if popping lands her back on an
    // admin-only path she can't see (or anywhere unrecognised), reconcile
    // the URL with `replaceState` — synchronously, in the event handler
    // itself, before `setRoute` — rather than routing the correction
    // through `navigate`'s `pushState`, which is what trapped Back in the
    // first place (pop → push → pop → push …).
    const onPop = () => {
      const resolved = resolveTeacherRoute(window.location.pathname, user.role);
      const path = teacherRoutePath(resolved);
      if (window.location.pathname !== path) {
        window.history.replaceState(null, "", path);
      }
      setRoute(resolved);
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, [user.role]);

  const go = (r: TeacherRoute) => navigate(teacherRoutePath(r));

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
                (route.view === "class" || route.view === "student" || route.view === "item")) ||
              // 新建作业和作业详情在「布置」下面。
              (key === "assignments" && (route.view === "assignmentNew" || route.view === "assignment"));
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
            onNewAssignment={() => {
              // `navigate` fires popstate and the route is re-parsed from the
              // path, which has no class in it — so the form finds this class
              // through the remembered choice, not through the route object.
              writeLastClassId(route.classId);
              go({ view: "assignmentNew", classId: route.classId });
            }}
          />
        )}
        {route.view === "assignments" && (
          <AssignmentsPage
            onNew={(classId) => go({ view: "assignmentNew", classId })}
            onOpen={(assignmentId) => go({ view: "assignment", assignmentId })}
          />
        )}
        {route.view === "assignmentNew" && (
          <AssignmentForm
            initialClassId={route.classId}
            onBack={() => go({ view: "assignments" })}
            onCreated={(assignmentId) => go({ view: "assignment", assignmentId })}
          />
        )}
        {route.view === "assignment" && (
          <AssignmentDetailPage
            assignmentId={route.assignmentId}
            onBack={() => go({ view: "assignments" })}
            onOpenItem={(classId, userId, atomId) => go({ view: "item", classId, userId, atomId })}
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
