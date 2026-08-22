import { useState, type ReactNode } from "react";
import type { SessionStore } from "./session";
import { Nav, type NavTab } from "./Nav";
import { HomePage } from "./home/HomePage";
import { ProjectsTab } from "./ProjectsTab";
import { CoursesTab } from "./CoursesTab";
import { SettingsView } from "./settings/SettingsView";
import { CourseReport } from "./courses/CourseReport";
import { AccentProvider, ACCENT_PRESETS, type AccentId } from "../ui/accent";
import { BackgroundProvider, BACKGROUND_PRESETS, type BackgroundId } from "../ui/background";
import { api, type MeUser } from "../api";
import { TourProvider, useTour } from "@/tour/TourProvider";
import { TourRunner } from "@/tour/TourRunner";
import { WelcomeModal } from "@/tour/WelcomeModal";
import { FeedbackModal } from "@/tour/FeedbackModal";
import { fullJourney } from "@/tour/journey";
import type { TourNavContext, StudioRoom } from "@/tour/types";
import { DEMO_PROJECT_ID } from "@/tour/types";
import { exampleCourseReport } from "@/tour/fixtures/exampleCourseReport";

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
//
// The guided tour (印记 onboarding) is mounted here too: TourProvider wraps the
// shell so `useTour()` is available to the inner bridge (StudentAppInner), which
// owns the Nav footer's 重新开始引导, the welcome modal's 课程/项目 picks, and
// the example-report overlay the tour deep-links into via openCourse("__example__").

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

type CoursesSub = "courses" | "history" | "gallery";

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
  // One-shot deep-link into the 课程 tab's sub-tab, driven by the guided tour
  // (TourNavContext.setCoursesSub). Consumed by CoursesTab on entry.
  const [pendingCoursesSub, setPendingCoursesSub] = useState<CoursesSub | null>(null);
  // One-shot deep-link to drive the open project's studio to a specific room,
  // driven by the guided tour (TourNavContext.setStudioRoom, P3 Task 1).
  // Consumed by ProjectsTab/WorkspaceContainer on entry.
  const [pendingRoom, setPendingRoom] = useState<StudioRoom | null>(null);

  // Welcome modal: opens on first login (never-onboarded user). The tour's
  // example-report overlay renders above the body when the tour deep-links into
  // openCourse("__example__").
  const [welcomeOpen, setWelcomeOpen] = useState(user?.onboarded_at == null);
  const [showExampleReport, setShowExampleReport] = useState(false);

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

  // The tour opens the fixture report via the sentinel slug "__example__";
  // everything else is a real course slug that CoursesTab consumes.
  function openCourse(slug: string) {
    if (slug === "__example__") {
      setTab("courses");
      setShowExampleReport(true);
      return;
    }
    // Any real-course navigation leaves the example-report overlay behind.
    setShowExampleReport(false);
    setPendingCourseId(slug);
    setTab("courses");
  }

  // Setters the guided tour uses to drive the shell before each step renders.
  // The example-report overlay is opaque and full-bleed, so every tour nav that
  // moves off the report segment (setTab / setCoursesSub) must dismiss it — the
  // report segment's own steps carry no onEnter, so this never closes it early.
  const tourNav: TourNavContext = {
    setTab: (t) => {
      setShowExampleReport(false);
      setTab(t);
    },
    openCourse,
    setCoursesSub: (s) => {
      setShowExampleReport(false);
      setTab("courses");
      setPendingCoursesSub(s);
    },
    openDemoProject: () => openProjectFromHome(DEMO_PROJECT_ID),
    setStudioRoom: (room) => setPendingRoom(room),
  };

  // Stamp onboarding so the welcome modal never fires again. Called on tour
  // completion (TourProvider.onComplete) and on 稍后再说 / dismiss. Also clears
  // any lingering example-report overlay so 结束 leaves nothing behind.
  function completeOnboarding() {
    setShowExampleReport(false);
    void api.putOnboarding();
  }

  let body: ReactNode;
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
        pendingRoom={pendingRoom}
        onPendingRoomConsumed={() => setPendingRoom(null)}
        onImmersiveChange={setProjectsImmersive}
      />
    );
  } else if (tab === "courses") {
    body = (
      <CoursesTab
        pendingCourseId={pendingCourseId}
        onPendingCourseConsumed={() => setPendingCourseId(null)}
        pendingSub={pendingCoursesSub}
        onPendingSubConsumed={() => setPendingCoursesSub(null)}
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
        <TourProvider nav={tourNav} onComplete={completeOnboarding}>
          <StudentAppInner
            tab={tab}
            onTab={setTab}
            user={user}
            body={body}
            showNav={showNav}
            onLogout={onLogout}
            welcomeOpen={welcomeOpen}
            setWelcomeOpen={setWelcomeOpen}
            completeOnboarding={completeOnboarding}
            showExampleReport={showExampleReport}
            setShowExampleReport={setShowExampleReport}
          />
        </TourProvider>
      </BackgroundProvider>
    </AccentProvider>
  );
}

// StudentAppInner — the shell body rendered inside TourProvider so it can call
// useTour(). It owns the tour triggers: the Nav footer's 重新开始引导, the
// welcome modal's 课程/项目 picks (both play `fullJourney` from the start —
// segments are sequential-only, so there are no per-segment entry points), and
// the example-report overlay the tour deep-links into.
function StudentAppInner({
  tab,
  onTab,
  user,
  body,
  showNav,
  onLogout,
  welcomeOpen,
  setWelcomeOpen,
  completeOnboarding,
  showExampleReport,
  setShowExampleReport,
}: {
  tab: NavTab;
  onTab: (t: NavTab) => void;
  user: MeUser | null;
  body: ReactNode;
  showNav: boolean;
  onLogout: () => void;
  welcomeOpen: boolean;
  setWelcomeOpen: (open: boolean) => void;
  completeOnboarding: () => void;
  showExampleReport: boolean;
  setShowExampleReport: (show: boolean) => void;
}) {
  const tour = useTour();
  const [feedbackOpen, setFeedbackOpen] = useState(false);

  return (
    <>
      <div className="flex h-full w-full overflow-hidden bg-mk-paper">
        {showNav && (
          <Nav
            tab={tab}
            onTab={onTab}
            user={user}
            onRestartTour={() => tour.play(fullJourney)}
            onFeedback={() => setFeedbackOpen(true)}
            onLogout={onLogout}
          />
        )}
        <div className="relative flex-1 overflow-hidden">{body}</div>
      </div>
      <TourRunner />
      <FeedbackModal open={feedbackOpen} onClose={() => setFeedbackOpen(false)} />
      <WelcomeModal
        open={welcomeOpen}
        displayName={user?.display_name ?? ""}
        onPick={(_start) => {
          // P1: fullJourney = courses group only, so the courses/projects order
          // has no effect yet — the param is kept for P3 when the groups swap.
          setWelcomeOpen(false);
          tour.play(fullJourney);
        }}
        onDismiss={() => {
          setWelcomeOpen(false);
          completeOnboarding();
        }}
      />
      {showExampleReport && (
        <div className="absolute inset-0 z-40 overflow-auto bg-mk-paper">
          <CourseReport
            courseId="__example__"
            exampleReport={exampleCourseReport}
            onBackToCourses={() => setShowExampleReport(false)}
            onGoPortal={() => setShowExampleReport(false)}
          />
        </div>
      )}
    </>
  );
}
