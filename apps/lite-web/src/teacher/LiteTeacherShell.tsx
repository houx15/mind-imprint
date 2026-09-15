import { ClassPreview } from "./ClassPreview";
import "./teacher-studio.css";
import { useBackground } from "@/ui/background";
import { StudentArtwork, studentArtwork } from "../learning/StudentArtwork";
import { CompanionAppearanceProvider } from "@/ui/CompanionAppearance";
import { useEffect, useLayoutEffect, useState, useRef, type CSSProperties } from "react";
import { LayoutGrid, Users, GraduationCap, UploadCloud, ClipboardList, FileText } from "lucide-react";
import { Icon, Settings, useAccent, type LucideIcon } from "@/ui";
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
import { resolveTeacherRoute, teacherRoutePath, type TeacherRoute } from "./teacherRouting";
import { isTeacherRailActive, teacherRailItems, type TeacherRailKey } from "./teacherRail";
import { ClassPage } from "./ClassPage";
import { ClassWeeklyPage } from "./ClassWeeklyPage";
import { StudentPage } from "./StudentPage";
import { ItemPage } from "./ItemPage";
import { AssignmentsPage } from "./AssignmentsPage";
import { AssignmentForm } from "./AssignmentForm";
import { AssignmentDetailPage } from "./AssignmentDetailPage";
import { GradingPage } from "./GradingPage";
import { ParentReportsPage } from "./ParentReportsPage";
import { ParentReportEditor } from "./ParentReportEditor";
import { writeLastClassId } from "./assignmentLogic";
import { LEAVE_UNSAVED_CONFIRM } from "./gradingLogic";
import "./teacher.css";

/** Lite teaching studio. Routing and teacher data remain independent of student views. */

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
  const { id: accent, presets } = useAccent();
  const palette = presets.find(p => p.id === accent) ?? presets[0]!;
  const themeStyle = Object.fromEntries(Object.entries(palette.scale).map(([step, value]) => [`--mk-theme-accent-${step}`, value])) as CSSProperties;
  const { id: background } = useBackground();
  // Portals inherit the same Lite theme; restore the host on unmount.
  useEffect(() => {
    const body = document.body;
    const hadClass = body.classList.contains("lite-teacher-theme");
    body.classList.add("lite-teacher-theme");
    return () => { if (!hadClass) body.classList.remove("lite-teacher-theme"); };
  }, []);
  useEffect(() => {
    const body = document.body;
    const previousBackground = body.getAttribute("data-background");
    const previous = Object.keys(palette.scale).map(step => {
      const key = `--mk-theme-accent-${step}`;
      return [key, body.style.getPropertyValue(key)] as const;
    });
    body.dataset.background = background;
    for (const [step, value] of Object.entries(palette.scale)) body.style.setProperty(`--mk-theme-accent-${step}`, value);
    return () => {
      if (previousBackground === null) body.removeAttribute("data-background");
      else body.setAttribute("data-background", previousBackground);
      for (const [key, value] of previous) {
        if (value) body.style.setProperty(key, value);
        else body.style.removeProperty(key);
      }
    };
  }, [palette, background]);
  // `resolveTeacherRoute` (not `parseTeacherRoute`) is what decides what she
  // is looking at: it folds an unrecognised path (root, a leftover student
  // path, a typo) onto her role's landing tab, and sends a teacher away from
  // an admin-only view. See its doc comment in `teacherRouting.ts` for why
  // this has to be a distinct function from the raw parser. The assignment
  // route's `?tab=grading` hint lives in `window.location.search`, so every
  // read of the address bar below carries pathname + search together, not
  // pathname alone.
  const fullPath = () => window.location.pathname + window.location.search;
  const [route, setRoute] = useState<TeacherRoute>(() => resolveTeacherRoute(fullPath(), user.role));

  // Mirror `route` into a ref for the popstate handler and `go` below: both
  // read the CURRENT route synchronously from an event handler, not a
  // value captured when their effect/closure was created (the popstate
  // effect only re-runs on `[user.role]`, so `route` inside it would
  // otherwise be frozen at whatever it was on mount). Same idiom as
  // `AssignmentDetailPage`'s `aidRef`.
  const routeRef = useRef(route);
  routeRef.current = route;

  // Whether `GradingPage` currently has unsaved edits — set via its
  // `onDirtyChange` prop below. A ref, not state: `go`/the popstate handler
  // need to read it synchronously inside a click/pop handler, and a state
  // update here would not itself need to trigger a re-render of this shell.
  const dirtyGradingRef = useRef(false);

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
    if (fullPath() !== path) {
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
    //
    // Unsaved-edits guard: if she is leaving the grading view (to a
    // different view, or to a different `gradingId`) with `dirtyGradingRef`
    // set, confirm first. The browser has already changed the URL by the
    // time `popstate` fires, so a cancel pushes the grading URL straight
    // back onto the stack (undoing the Back) instead of trying to prevent
    // the pop itself, which the History API has no hook for.
    const onPop = () => {
      const resolved = resolveTeacherRoute(fullPath(), user.role);
      const cur = routeRef.current;
      if (
        dirtyGradingRef.current &&
        cur.view === "grading" &&
        (resolved.view !== "grading" || resolved.gradingId !== cur.gradingId)
      ) {
        if (!window.confirm(LEAVE_UNSAVED_CONFIRM)) {
          window.history.pushState(null, "", teacherRoutePath(cur));
          return;
        }
        dirtyGradingRef.current = false;
      }
      const path = teacherRoutePath(resolved);
      if (fullPath() !== path) {
        window.history.replaceState(null, "", path);
      }
      setRoute(resolved);
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, [user.role]);

  const mainRef = useRef<HTMLElement>(null);
  useEffect(() => { mainRef.current?.scrollTo({ top: 0 }); }, [route]);

  // Cross-component navigation guard (rail links, the brand link, any other
  // `go()` call): `GradingPage`'s own 返回 button has its own inline confirm
  // (the lite teacher UI's existing pattern — see `confirmRegrade`/
  // `confirmArchive` elsewhere) for leaving through ITS OWN button; this is
  // the one for leaving through something GradingPage does not own. A
  // second, styled confirm surface for an arbitrary navigation initiated
  // outside the page is more machinery than this warrants, so `window.
  // confirm` here — acceptable per the ruling when nothing else already
  // reaches this case.
  const go = (r: TeacherRoute) => {
    if (dirtyGradingRef.current && route.view === "grading") {
      if (!window.confirm(LEAVE_UNSAVED_CONFIRM)) return;
      dirtyGradingRef.current = false;
    }
    navigate(teacherRoutePath(r));
  };

  const railItems = teacherRailItems(user.role).map(item => ({ ...item, icon: RAIL_ICONS[item.key] }));

  return (
    <StudentArtwork><CompanionAppearanceProvider image={bookmark}>
    <div className="lite-teacher teacher-studio" style={themeStyle} data-background={background}>
      <nav className="teacher-nav" aria-label="主导航">
        <button className="teacher-brand" onClick={() => go({ view: user.role === "admin" ? "overview" : "classes" })}>
          <img src={bookmark} alt="" /><span>思维印记<small>教学工作室</small></span>
        </button>
        <p className="teacher-nav-label">TEACHER STUDIO</p>
        {railItems.map(({ key, label, icon }) => {
          const active = isTeacherRailActive(key, route);
          return <button key={key} className="teacher-nav-item" aria-current={active ? "page" : undefined}
            onClick={() => go({ view: key } as TeacherRoute)}><Icon icon={icon} size={19} /><span>{label}</span><span className="teacher-nav-arrow" aria-hidden="true">↗</span></button>;
        })}
        <div className="teacher-nav-footer">
          <div className="teacher-account"><span className="teacher-avatar">{Array.from(user.display_name)[0]}</span><div>{user.display_name}<small>{user.school.name}</small></div></div>
          <button className="teacher-nav-item" onClick={() => go({ view: "settings" })} aria-current={route.view === "settings" ? "page" : undefined}><Icon icon={Settings} size={19} />设置</button>
        </div>
      </nav>
      <main className="teacher-main" data-teacher-page={route.view} ref={mainRef}>
        {route.view === "classes" && (
          <ClassesView renderClassPreview={classId => <ClassPreview classId={classId} />} studioArtwork={studentArtwork.writing} client={api} role={user.role} onOpenClass={(classId) => go({ view: "class", classId })} />
        )}
        {route.view === "class" && (
          <ClassPage
            key={route.classId}
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
            initialTab={route.tab}
            onBack={() => go({ view: "assignments" })}
            onOpenItem={(classId, userId, atomId) => go({ view: "item", classId, userId, atomId })}
            onOpenGrading={(gradingId) => go({ view: "grading", gradingId })}
          />
        )}
        {route.view === "grading" && (
          <GradingPage
            key={route.gradingId}
            gradingId={route.gradingId}
            onDirtyChange={(dirty) => {
              dirtyGradingRef.current = dirty;
            }}
            onBack={(g) =>
              g?.assignmentId
                ? go({ view: "assignment", assignmentId: g.assignmentId, tab: "grading" })
                : g
                  ? go({ view: "item", classId: g.classId, userId: g.userId, atomId: g.atomId })
                  : go({ view: "assignments" })
            }
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
          <SettingsView teachingStudio session={unusedSettingsSession} user={user} onLogout={onLogout} />
        )}
      </main>
    </div>
    </CompanionAppearanceProvider></StudentArtwork>
  );
}
