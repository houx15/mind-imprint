import { useState } from "react";
import type { SessionStore } from "./session";
import { Nav, type NavTab } from "./Nav";
import { HomePage } from "./home/HomePage";
import { ProjectsTab } from "./ProjectsTab";
import { CoursesTab } from "./CoursesTab";
import { SettingsView } from "./settings/SettingsView";
import { AccentProvider, ACCENT_PRESETS, type AccentId } from "../ui/accent";
import { BackgroundProvider, BACKGROUND_PRESETS, type BackgroundId } from "../ui/background";
import { api } from "../api";

// StudentApp — the student platform shell. Four top-level surfaces reachable
// from the 64px `Nav` rail:
//   首页 (home)     — default landing.
//   项目 (projects) — ProjectsTab: 我的项目 (directory → four-room studio) +
//                     评估报告 (finished-project 过程评估报告 timeline).
//   课程 (courses)  — CoursesTab: 课程 (list → player → report) + 学习记录 +
//                     图鉴 (tool-card catalog).
//   我 (me)         — 设置.
//
// The retired 评估 tab's two halves moved here: 成长报告 → 项目/评估报告,
// 图鉴 → 课程/图鉴. Courses used to be an overlay with no nav entry; they are
// now a first-class tab, so "back from a course" lands on the 课程 page.

type Tab = NavTab;

/** Only a known 8-preset id seeds the accent provider; anything else (unset,
 * legacy free-hex `avatar_color`, etc) falls back to accent.tsx's own
 * localStorage → vermilion default. */
function coerceAccent(value: string | null | undefined): AccentId | undefined {
  return ACCENT_PRESETS.some((p) => p.id === value) ? (value as AccentId) : undefined;
}

/** Coerce the server's page_background into a known preset id (else undefined,
 * so BackgroundProvider falls back to localStorage → 'paper'). */
function coerceBackground(value: string | null | undefined): BackgroundId | undefined {
  return BACKGROUND_PRESETS.some((p) => p.id === value) ? (value as BackgroundId) : undefined;
}

export function StudentApp({
  session,
  onLogout,
}: {
  session: SessionStore;
  onLogout: () => void;
}) {
  const [tab, setTab] = useState<Tab>("home");
  const user = session.getUser();

  // Open-from-home deep-links into the 项目 tab: a specific project (recent
  // tiles) or the create drawer (新建 tiles). Each is a one-shot signal —
  // ProjectsTab / WorkspaceContainer consume it right after acting on it.
  const [pendingProjectId, setPendingProjectId] = useState<string | null>(null);
  const [pendingCreate, setPendingCreate] = useState(false);
  // Open-from-home deep-link into the 项目 tab's 评估报告 sub: a finished
  // project's report (home project card's ⋯ menu → 查看评估报告). One-shot,
  // consumed by ProjectsTab on entry.
  const [pendingReportId, setPendingReportId] = useState<string | null>(null);
  // Open-from-anywhere deep-link into the 课程 tab: a course to play (home
  // course cards, a 图鉴 card's "去学这张卡的课程"). One-shot, consumed by
  // CoursesTab on entry.
  const [pendingCourseId, setPendingCourseId] = useState<string | null>(null);

  // Immersive flags (per tab): while a project's studio or a course's player is
  // open the platform nav rail hides so the surface is full-bleed. Scoped by
  // `tab` in `showNav` so a stale flag from an inactive tab never hides the rail.
  const [projectsImmersive, setProjectsImmersive] = useState(false);
  const [coursesImmersive, setCoursesImmersive] = useState(false);
  const showNav = !((tab === "projects" && projectsImmersive) || (tab === "courses" && coursesImmersive));

  function openProjectFromHome(id: string) {
    setPendingCreate(false);
    setPendingReportId(null);
    setPendingProjectId(id);
    setTab("projects");
  }

  function openReportFromHome(id: string) {
    setPendingCreate(false);
    setPendingProjectId(null);
    setPendingReportId(id);
    setTab("projects");
  }

  function openCreateFromHome() {
    setPendingProjectId(null);
    setPendingCreate(true);
    setTab("projects");
  }

  function openCourse(slug: string) {
    setPendingCourseId(slug);
    setTab("courses");
  }

  let body;
  if (tab === "home") {
    body = (
      <HomePage
        user={user}
        onOpenProject={openProjectFromHome}
        onOpenCourse={openCourse}
        onCreateProject={openCreateFromHome}
        onGoProjects={() => setTab("projects")}
        onGoCourses={() => setTab("courses")}
        onViewReport={openReportFromHome}
      />
    );
  } else if (tab === "projects") {
    body = (
      <ProjectsTab
        pendingProjectId={pendingProjectId}
        onPendingProjectConsumed={() => setPendingProjectId(null)}
        autoOpenCreate={pendingCreate}
        onAutoOpenCreateConsumed={() => setPendingCreate(false)}
        pendingReportId={pendingReportId}
        onPendingReportConsumed={() => setPendingReportId(null)}
        onImmersiveChange={setProjectsImmersive}
      />
    );
  } else if (tab === "courses") {
    body = (
      <CoursesTab
        pendingCourseId={pendingCourseId}
        onPendingCourseConsumed={() => setPendingCourseId(null)}
        studentId={user?.id}
        onGoPortal={() => {
          setCoursesImmersive(false);
          setTab("projects");
        }}
        onImmersiveChange={setCoursesImmersive}
      />
    );
  } else {
    body = <SettingsView session={session} user={user} onLogout={onLogout} />;
  }

  return (
    <AccentProvider
      initialAccent={coerceAccent(user?.avatar_color)}
      onPersist={(id) => {
        void api.setAccent(id);
      }}
    >
      <BackgroundProvider
        initialBackground={coerceBackground(user?.page_background)}
        onPersist={(id) => {
          void api.setBackground(id);
        }}
      >
        <div className="flex h-full w-full overflow-hidden bg-mk-paper">
          {showNav && <Nav tab={tab} onTab={setTab} user={user} />}
          <div className="relative flex-1 overflow-hidden">{body}</div>
        </div>
      </BackgroundProvider>
    </AccentProvider>
  );
}
