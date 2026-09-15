import { ClassPreview } from "./ClassPreview";
import "./teacher-studio.css";
import { useBackground } from "@/ui/background";
import { StudentArtwork, studentArtwork } from "../learning/StudentArtwork";
import { CompanionAppearanceProvider } from "@/ui/CompanionAppearance";
import { bookmark } from "../home/LearningHome";
import { useEffect, useLayoutEffect, useState, useRef, type CSSProperties } from "react";
import { LayoutGrid, Users, GraduationCap, UploadCloud } from "lucide-react";
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
import { resolveTeacherRoute, teacherRoutePath, type TeacherRoute } from "./teacherRouting";
import { ClassPage } from "./ClassPage";
import { StudentPage } from "./StudentPage";
import { ItemPage } from "./ItemPage";

/** Lite teaching studio. Routing and teacher data remain independent of student views. */

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

  const mainRef = useRef<HTMLElement>(null);
  useEffect(() => { mainRef.current?.scrollTo({ top: 0 }); }, [route]);

  const go = (r: TeacherRoute) => navigate(teacherRoutePath(r));

  const railItems = user.role === "admin" ? ADMIN_ITEMS : TEACHER_ITEMS;

  return (
    <StudentArtwork><CompanionAppearanceProvider image={bookmark}>
    <div className="lite-teacher teacher-studio" style={themeStyle} data-background={background}>
      <nav className="teacher-nav" aria-label="主导航">
        <button className="teacher-brand" onClick={() => go({ view: user.role === "admin" ? "overview" : "classes" })}>
          <img src={bookmark} alt="" /><span>思维印记<small>教学工作室</small></span>
        </button>
        <p className="teacher-nav-label">TEACHER STUDIO</p>
        {railItems.map(({ key, label, icon }) => {
          const active = route.view === key || (key === "classes" && ["class", "student", "item"].includes(route.view));
          return <button key={key} className="teacher-nav-item" aria-current={active ? "page" : undefined}
            onClick={() => go({ view: key } as TeacherRoute)}><Icon icon={icon} size={19} /><span>{label}</span><span className="teacher-nav-arrow" aria-hidden="true">↗</span></button>;
        })}
        <div className="teacher-nav-footer">
          <div className="teacher-account"><span className="teacher-avatar">{Array.from(user.display_name)[0]}</span><div>{user.display_name}<small>{user.school.name}</small></div></div>
          <button className="teacher-nav-item" onClick={() => go({ view: "settings" })} aria-current={route.view === "settings" ? "page" : undefined}><Icon icon={Settings} size={19} />设置</button>
        </div>
      </nav>
      <main className="teacher-main" ref={mainRef}>
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
          <SettingsView teachingStudio session={unusedSettingsSession} user={user} onLogout={onLogout} />
        )}
      </main>
    </div>
    </CompanionAppearanceProvider></StudentArtwork>
  );
}
