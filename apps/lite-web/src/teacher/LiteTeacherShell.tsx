import { useEffect, useLayoutEffect, useState } from "react";
import { LayoutGrid, Users, GraduationCap, UploadCloud, ClipboardList, FileText } from "lucide-react";
import type { LucideIcon } from "@/ui";
import { api } from "@/api";
import { ClassesView } from "@/console/ClassesView";
import { OverviewView } from "@/console/OverviewView";
import { TeachersView } from "@/console/TeachersView";
import { ImportView } from "@/console/ImportView";
import { SettingsView } from "@/shell/settings/SettingsView";
import { createSession, makeMemoryStorage } from "@/shell/session";
import { navigate } from "../routing";
import type { MeUser } from "../api/auth";
import bookmark from "../home/assets/yinji-bookmark.webp";
import { LearningAccountButton, LearningRail, type RailLink } from "../shared/LearningRail";
import { useLiteTheme } from "../shared/useLiteTheme";
import { resolveTeacherRoute, teacherRoutePath, type TeacherRoute } from "./teacherRouting";
import { isTeacherRailActive, teacherRailItems, type TeacherRailKey } from "./teacherRail";
import { ClassPage } from "./ClassPage";
import { ClassWeeklyPage } from "./ClassWeeklyPage";
import { StudentPage } from "./StudentPage";
import { ItemPage } from "./ItemPage";
import { AssignmentsPage } from "./AssignmentsPage";
import { AssignmentForm } from "./AssignmentForm";
import { AssignmentDetailPage } from "./AssignmentDetailPage";
import { ParentReportsPage } from "./ParentReportsPage";
import { ParentReportEditor } from "./ParentReportEditor";
import { writeLastClassId } from "./assignmentLogic";
import "./teacher.css";

/**
 * LiteTeacherShell — the lite edition's teacher/admin shell. Same pattern as
 * pro's `AppShell` → `ConsoleShell`: a role branch in `LiteApp.tsx` sends
 * teachers and admins here instead of into the student-facing `LiteShell`.
 *
 * It mounts in the same lite theme scope as the student shell
 * (`lite-student` on the root and `lite-student-theme` on `body`, via
 * `useLiteTheme`), so teachers get the lite tokens, radii, font and dark mode.
 * `lite-teacher` marks the teacher shell for the few rules that are teacher
 * only (`teacher.css`). The rail is the student shell's `LearningRail`, with
 * the teacher's items (`teacherRail.ts`) and the account button at its foot.
 */

// SettingsView requires a pro SessionStore; lite's auth state lives in
// LiteApp, so this one is never read. Same construction as LiteApp.tsx's
// unusedSettingsSession.
const unusedSettingsSession = createSession({ storage: makeMemoryStorage() });

const RAIL_ICONS: Record<TeacherRailKey, LucideIcon> = {
  overview: LayoutGrid,
  classes: Users,
  assignments: ClipboardList,
  parentReports: FileText,
  teachers: GraduationCap,
  import: UploadCloud,
};

export function LiteTeacherShell({ user, onLogout }: { user: MeUser; onLogout: () => void }) {
  const { accent, background, themeStyle } = useLiteTheme();

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
    // first place (pop → push → pop → push …). The rail's brand link
    // navigates to `/`, which this folds onto her landing tab.
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

  const links: RailLink[] = teacherRailItems(user.role).map(({ key, label }) => ({
    key,
    label,
    icon: RAIL_ICONS[key],
    active: isTeacherRailActive(key, route),
    onSelect: () => go({ view: key } as TeacherRoute),
  }));

  return (
    <div
      className="lite-student lite-teacher flex h-full w-full overflow-hidden bg-mk-paper text-mk-ink"
      style={themeStyle}
      data-accent={accent}
      data-background={background}
    >
      <LearningRail
        links={links}
        footer={
          <LearningAccountButton
            name={user.display_name}
            image={bookmark}
            active={route.view === "settings"}
            onSelect={() => go({ view: "settings" })}
          />
        }
      />

      <main data-teacher-page={route.view} className="min-w-0 flex-1 overflow-y-auto">
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
            onOpenWeekly={() => go({ view: "classWeekly", classId: route.classId })}
          />
        )}
        {route.view === "classWeekly" && (
          <ClassWeeklyPage
            key={route.classId}
            classId={route.classId}
            onBack={() => go({ view: "class", classId: route.classId })}
            onOpenStudent={(userId) => go({ view: "student", classId: route.classId, userId })}
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
        {route.view === "parentReports" && (
          <ParentReportsPage onOpen={(reportId) => go({ view: "parentReport", reportId })} />
        )}
        {route.view === "parentReport" && (
          <ParentReportEditor
            key={route.reportId}
            reportId={route.reportId}
            onBack={(classId) => {
              // The list opens on the remembered class; make that the
              // report's class so 返回 lands next to the row she came from.
              if (classId) writeLastClassId(classId);
              go({ view: "parentReports" });
            }}
          />
        )}
        {route.view === "student" && (
          <StudentPage
            classId={route.classId}
            userId={route.userId}
            onBack={() => go({ view: "class", classId: route.classId })}
            onOpenItem={(atomId) => go({ view: "item", classId: route.classId, userId: route.userId, atomId })}
            onOpenParentReport={(reportId) => go({ view: "parentReport", reportId })}
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
