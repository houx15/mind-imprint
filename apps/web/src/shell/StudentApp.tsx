import { useEffect, useRef, useState, type MutableRefObject, type ReactNode } from "react";
import type { SessionStore } from "./session";
import { Nav, type NavTab } from "./Nav";
import { parsePath, routePath, type AppRoute } from "./routing";
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
import { fullJourney, journeyStarting } from "@/tour/journey";
import type { TourNavContext, StudioRoom, TourWritingView, TourRefPanelTab, TourPlanView } from "@/tour/types";
import { DEMO_PROJECT_ID, DEMO_READING_MATERIAL_ID, DEMO_READING_REFERENCE_ID, DEMO_READING_NOTE } from "@/tour/types";
import { exampleCourseReport } from "@/tour/fixtures/exampleCourseReport";
import { getMaterialSource } from "@/workspace/api/workspace";
import type { DigCandidate, MaterialSource } from "@mind-imprint/contracts";

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

// The canonical route for the current shell state — the source of truth the URL
// is kept in sync with. Only the open project id / course slug enrich the top
// tab; sub-tabs (评估报告 / 学习记录 / 图鉴) are intentionally not addressed.
function deriveRoute(tab: Tab, openProjectId: string | null, openCourseSlug: string | null): AppRoute {
  if (tab === "projects") return openProjectId ? { tab: "projects", projectId: openProjectId } : { tab: "projects" };
  if (tab === "courses") return openCourseSlug ? { tab: "courses", slug: openCourseSlug } : { tab: "courses" };
  if (tab === "me") return { tab: "me" };
  return { tab: "home" };
}

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
  // Seed the initial tab + deep-links from the browser URL so a refresh or a
  // pasted link lands on the same page (and, for 项目/课程, the same open
  // project/course — reusing the existing `pending*` deep-link machinery).
  const [initialRoute] = useState<AppRoute>(() => parsePath(window.location.pathname));
  const [tab, setTab] = useState<Tab>(initialRoute.tab);
  const user = session.getUser();

  // Open-from-home deep-links into the 项目 tab: a specific project (recent
  // tiles) or the create drawer (新建 tiles). Each is a one-shot signal —
  // ProjectsTab / WorkspaceContainer consume it right after acting on it.
  const [pendingProjectId, setPendingProjectId] = useState<string | null>(
    initialRoute.tab === "projects" ? initialRoute.projectId ?? null : null,
  );
  const [pendingCreate, setPendingCreate] = useState(false);
  // Open-from-home deep-link into the 项目 tab's 评估报告 sub: a finished
  // project's report (home project card's ⋯ menu → 查看评估报告). One-shot,
  // consumed by ProjectsTab on entry.
  const [pendingReportId, setPendingReportId] = useState<string | null>(null);
  // Open-from-anywhere deep-link into the 课程 tab: a course to play (home
  // course cards, a 图鉴 card's "去学这张卡的课程"). One-shot, consumed by
  // CoursesTab on entry.
  const [pendingCourseId, setPendingCourseId] = useState<string | null>(
    initialRoute.tab === "courses" ? initialRoute.slug ?? null : null,
  );
  // One-shot deep-link into the 课程 tab's sub-tab, driven by the guided tour
  // (TourNavContext.setCoursesSub). Consumed by CoursesTab on entry.
  const [pendingCoursesSub, setPendingCoursesSub] = useState<CoursesSub | null>(null);
  // One-shot deep-link to drive the open project's studio to a specific room,
  // driven by the guided tour (TourNavContext.setStudioRoom, P3 Task 1).
  // Consumed by ProjectsTab/WorkspaceContainer on entry.
  const [pendingRoom, setPendingRoom] = useState<StudioRoom | null>(null);
  // One-shot deep-link to drive the open reading room's inner 列表/探索图谱 view,
  // driven by the guided tour (TourNavContext.setReadingView, P5 Task 4).
  // Consumed by ProjectsTab/WorkspaceContainer on entry.
  const [pendingReadingView, setPendingReadingView] = useState<"list" | "graph" | null>(null);
  // One-shot deep-link to drive the open writing room to a specific document +
  // tab (提案 片段 → where the 片段引导/写作卡 lives), driven by the guided tour
  // (TourNavContext.setWritingView, P6 Task 9). Consumed by ProjectsTab/
  // WorkspaceContainer on entry.
  const [pendingWritingView, setPendingWritingView] = useState<TourWritingView | null>(null);
  // One-shot deep-link to select a tab on the open writing room's left
  // ReferencePanel (阅读笔记/AI批注/…), driven by the guided tour
  // (TourNavContext.selectRefPanelTab, P7). Consumed by ProjectsTab/
  // WorkspaceContainer on entry.
  const [pendingRefPanelTab, setPendingRefPanelTab] = useState<TourRefPanelTab | null>(null);
  // One-shot deep-link to force the open project's 管理 room to a specific
  // view (看板/甘特图/活动日志), driven by the guided tour
  // (TourNavContext.setPlanView, P7). Consumed by ProjectsTab/
  // WorkspaceContainer on entry.
  const [pendingPlanView, setPendingPlanView] = useState<TourPlanView | null>(null);
  // One-shot signal (a bumped nonce — the action carries no payload) to open
  // the 检索卡 teaching modal in the open project's exploration graph, driven by
  // the guided tour (TourNavContext.openSearchCard, P7). Consumed by
  // ProjectsTab/WorkspaceContainer on entry.
  const [pendingOpenSearchCard, setPendingOpenSearchCard] = useState<number | null>(null);
  // One-shot signal (a bumped nonce — the action carries no payload) to exit
  // the open project's exploration graph "hole" zoom back to the Level-1 root
  // map, driven by the guided tour (TourNavContext.resetExplorationZoom, P8).
  // Consumed by ProjectsTab/WorkspaceContainer/ReadingBlock on entry. Mirrors
  // pendingOpenSearchCard's nonce shape exactly.
  const [pendingResetExplorationZoom, setPendingResetExplorationZoom] = useState<number | null>(null);
  // One-shot demo-badge-as-已读 root-lead id, driven by the guided tour
  // (TourNavContext.markDemoNodeRead, P7 Task 4b) when it returns from the
  // read-only demo reading room. Consumed by ProjectsTab/WorkspaceContainer,
  // which ACCUMULATES it (unlike every sibling `pending*` above) rather than
  // replacing prior state.
  const [pendingMarkNodeRead, setPendingMarkNodeRead] = useState<string | null>(null);
  // One-shot demo-adopt signal — a fresh {candidate, parentLeadId} object each
  // call — driven by the guided tour (TourNavContext.markDemoNodeAdopted, P8
  // Task 6). Consumed by ProjectsTab/WorkspaceContainer, which ACCUMULATES it
  // (mirrors pendingMarkNodeRead) into a synthetic node list rather than
  // replacing prior state. ExplorationView itself also reaches the same
  // WorkspaceContainer accumulator directly (a live 采纳 click, not a tour
  // onEnter) — this pending slot only carries the TOUR-driven entry point.
  const [pendingDemoAdopt, setPendingDemoAdopt] = useState<{ candidate: DigCandidate; parentLeadId: string } | null>(
    null,
  );
  // One-shot deep-link to open an already-fetched demo `MaterialSource` into
  // the real immersive 精读 reading room, driven by the guided tour
  // (TourNavContext.openDemoReadingRoom, P6 Task 5). Consumed by
  // ProjectsTab/WorkspaceContainer once opened.
  const [pendingDemoReading, setPendingDemoReading] = useState<{
    source: MaterialSource;
    referenceId: string;
    readingNote?: string;
  } | null>(null);
  // Task 9: the demo project's guard modal's 好，带我逛一遍 needs `tour.play(...)`,
  // which only exists inside `TourProvider`'s subtree (`useTour()` in
  // StudentAppInner below) — but `body` (ProjectsTab included) is built here,
  // one level above the provider. A ref bridge (same pattern as
  // TourProvider's own `navRef`/`doneRef`) lets StudentAppInner keep this
  // pointed at the current `tour.play` call without threading `tour` itself
  // down through `body`.
  const demoTourRef = useRef<() => void>(() => {});

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

  // ── URL sync ────────────────────────────────────────────────────────────
  // The shell mirrors its top tab + open project/course into the browser URL
  // (`/projects/:id`, `/courses/:slug`, `/me`, `/` — see routing.ts), so a
  // refresh, a copied link, and the Back/Forward buttons all resolve. The open
  // ids are lifted from ProjectsTab/CoursesTab; opening from a URL reuses the
  // `pending*` deep-links, closing from a URL bumps a close-signal nonce. No
  // domain is ever referenced — everything is root-relative + the History API.
  const [openProjectId, setOpenProjectId] = useState<string | null>(null);
  const [openCourseSlug, setOpenCourseSlug] = useState<string | null>(null);
  const [projectCloseSignal, setProjectCloseSignal] = useState<number | null>(null);
  const [courseCloseSignal, setCourseCloseSignal] = useState<number | null>(null);
  // Refs so the once-registered popstate handler reads current values (no stale
  // closure) without re-subscribing on every open/close.
  const openProjectIdRef = useRef<string | null>(null);
  openProjectIdRef.current = openProjectId;
  const openCourseSlugRef = useRef<string | null>(null);
  openCourseSlugRef.current = openCourseSlug;
  // While non-null, we're reconciling toward a URL the browser already shows
  // (initial load or a Back/Forward pop) — the state→URL effect must NOT push
  // during that window, or an async open (pending* → open completes a tick
  // later) would get clobbered by a premature `/projects` push. Cleared the
  // moment the derived route actually matches the address bar. Initialised to
  // the canonical initial path so the first commit doesn't self-push.
  const pendingUrlSync = useRef<string | null>(routePath(initialRoute));

  // Normalise the address bar once on mount (e.g. `/index.html`, a trailing
  // slash, or `/home` → `/`) so the canonical path is what gets bookmarked.
  useEffect(() => {
    const canonical = routePath(initialRoute);
    if (canonical !== window.location.pathname) {
      window.history.replaceState(null, "", canonical);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // State → URL. Pushes a new history entry whenever the derived route diverges
  // from the address bar, except while reconciling FROM the URL (see above).
  useEffect(() => {
    const desired = routePath(deriveRoute(tab, openProjectId, openCourseSlug));
    if (pendingUrlSync.current !== null) {
      if (desired === window.location.pathname) pendingUrlSync.current = null;
      return;
    }
    if (desired !== window.location.pathname) {
      window.history.pushState(null, "", desired);
    }
  }, [tab, openProjectId, openCourseSlug]);

  // URL → State on Back/Forward. Re-parses the popped path and reconciles: open
  // via the `pending*` deep-links when the URL names something not open; bump a
  // close-signal when it drops back to a list. Switching tabs unmounts the old
  // tab's surface, which closes its open project/course on its own.
  useEffect(() => {
    function onPop() {
      const route = parsePath(window.location.pathname);
      const canonical = routePath(route);
      if (canonical !== window.location.pathname) {
        window.history.replaceState(null, "", canonical);
      }
      pendingUrlSync.current = canonical; // suppress the echo push while reconciling
      setShowExampleReport(false);
      setTab(route.tab);
      if (route.tab === "projects") {
        if (route.projectId) {
          if (route.projectId !== openProjectIdRef.current) setPendingProjectId(route.projectId);
        } else if (openProjectIdRef.current) {
          setProjectCloseSignal((n) => (n ?? 0) + 1);
        }
      }
      if (route.tab === "courses") {
        if (route.slug) {
          if (route.slug !== openCourseSlugRef.current) setPendingCourseId(route.slug);
        } else if (openCourseSlugRef.current) {
          setCourseCloseSignal((n) => (n ?? 0) + 1);
        }
      }
    }
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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
  // Open the demo's seeded reading material into the real, immersive 精读 room
  // as a read-only replay (demoMode + canned transcript + seeded outcome). Used
  // BOTH by the tour nav hook (`openDemoReadingRoom`) and — via the
  // `onDemoEnterReading` prop threaded down to the reading room's real
  // 进入阅读室 buttons — as the demo project's honest, non-403 entry: the live
  // enter-reading POST 403s on the read-only demo, so the button routes here to
  // the GET-only replay instead. A fetch failure falls back to the 列表 view.
  function enterDemoReadingRoom() {
    setPendingRoom("reading");
    void getMaterialSource(DEMO_PROJECT_ID, DEMO_READING_MATERIAL_ID)
      .then((source) =>
        setPendingDemoReading({ source, referenceId: DEMO_READING_REFERENCE_ID, readingNote: DEMO_READING_NOTE }),
      )
      .catch(() => setPendingReadingView("list"));
  }

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
    openDemoReport: () => openReportFromHome(DEMO_PROJECT_ID),
    setReadingView: (view) => setPendingReadingView(view),
    // P6 (Task 9): drive the open writing room to a document + tab so the tour
    // can land on the PROPOSAL 片段 tab (where the 片段引导/写作卡 lives). The room
    // itself is opened by the step's own setStudioRoom("writing").
    setWritingView: (view) => setPendingWritingView(view),
    // P7: select a tab on the open writing room's left ReferencePanel — used
    // to explicitly switch to AI批注 now that the demo defaults to 阅读笔记.
    selectRefPanelTab: (tab) => setPendingRefPanelTab(tab),
    // P7: force the open project's 管理 room to a view — used to land on
    // 活动日志.
    setPlanView: (view) => setPendingPlanView(view),
    // P7: open the 检索卡 modal in the open project's exploration graph. Void
    // action → bump a nonce so a repeat call (same step re-entered) still
    // fires the one-shot downstream.
    openSearchCard: () => setPendingOpenSearchCard((n) => (n ?? 0) + 1),
    // P8: exit the open project's exploration graph "hole" zoom back to the
    // Level-1 root map — used before spotlighting the root map's 已读 badge
    // (`warren-node-read`, which only renders `!inHole`). Void action → bump a
    // nonce so a repeat call (same step re-entered) still fires the one-shot
    // downstream.
    resetExplorationZoom: () => setPendingResetExplorationZoom((n) => (n ?? 0) + 1),
    // P7 Task 4b: demo-badge a warren-map root as 已读 — fired when the tour
    // returns from the read-only demo reading room. Client-only override
    // (the demo project is write-blocked); accumulated downstream, never
    // retracted.
    markDemoNodeRead: (rootLeadId) => setPendingMarkNodeRead(rootLeadId),
    // P8 Task 6: demo-simulate adopting a searched candidate into the open
    // project's exploration graph. Client-only override (the demo project is
    // write-blocked); accumulated downstream, never retracted. A fresh object
    // each call so WorkspaceContainer's ref-guard always sees a new value.
    markDemoNodeAdopted: (candidate, parentLeadId) => setPendingDemoAdopt({ candidate, parentLeadId }),
    // P6 (Task 5): switch the open project's studio to the reading room, then
    // fetch the demo's seeded material (read-only GET, no enter-reading side
    // effects) and open it into the real, immersive 精读 room via
    // WorkspaceContainer's `pendingDemoReading` path. A fetch failure (offline,
    // a flaky demo backend, …) falls back to forcing the 列表 view instead of
    // dead-ending the step — the room switch above already landed somewhere
    // visible for that fallback to show up in.
    openDemoReadingRoom: enterDemoReadingRoom,
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
        pendingReadingView={pendingReadingView}
        onPendingReadingViewConsumed={() => setPendingReadingView(null)}
        pendingWritingView={pendingWritingView}
        onPendingWritingViewConsumed={() => setPendingWritingView(null)}
        pendingRefPanelTab={pendingRefPanelTab}
        onPendingRefPanelTabConsumed={() => setPendingRefPanelTab(null)}
        pendingPlanView={pendingPlanView}
        onPendingPlanViewConsumed={() => setPendingPlanView(null)}
        pendingOpenSearchCard={pendingOpenSearchCard}
        onPendingOpenSearchCardConsumed={() => setPendingOpenSearchCard(null)}
        pendingResetExplorationZoom={pendingResetExplorationZoom}
        onPendingResetExplorationZoomConsumed={() => setPendingResetExplorationZoom(null)}
        pendingMarkNodeRead={pendingMarkNodeRead}
        onPendingMarkNodeReadConsumed={() => setPendingMarkNodeRead(null)}
        pendingDemoAdopt={pendingDemoAdopt}
        onPendingDemoAdoptConsumed={() => setPendingDemoAdopt(null)}
        pendingDemoReading={pendingDemoReading}
        onPendingDemoReadingConsumed={() => setPendingDemoReading(null)}
        onDemoEnterReading={enterDemoReadingRoom}
        onImmersiveChange={setProjectsImmersive}
        onActiveProjectChange={setOpenProjectId}
        closeSignal={projectCloseSignal}
        onCloseSignalConsumed={() => setProjectCloseSignal(null)}
        onRequestDemoTour={() => demoTourRef.current()}
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
        onActiveCourseChange={setOpenCourseSlug}
        closeSignal={courseCloseSignal}
        onCloseSignalConsumed={() => setCourseCloseSignal(null)}
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
            demoTourRef={demoTourRef}
          />
        </TourProvider>
      </BackgroundProvider>
    </AccentProvider>
  );
}

// StudentAppInner — the shell body rendered inside TourProvider so it can call
// useTour(). It owns the tour triggers: the Nav footer's 重新开始引导 (always
// courses-first, via `fullJourney`), the welcome modal's 课程/项目 pick (via
// `journeyStarting(start)`, which reorders the two groups and always ends
// with the settings/accent finale), and the example-report overlay the tour
// deep-links into.
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
  demoTourRef,
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
  demoTourRef: MutableRefObject<() => void>;
}) {
  const tour = useTour();
  const [feedbackOpen, setFeedbackOpen] = useState(false);
  // Keep the bridge pointed at the current `tour.play` call every render —
  // same "assign a ref directly in the render body" pattern TourProvider
  // itself uses for `navRef`/`doneRef` (no effect needed: this never
  // triggers a re-render, it just keeps a plain callback ref fresh).
  demoTourRef.current = () => tour.play(journeyStarting("projects"));

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
        onPick={(start) => {
          setWelcomeOpen(false);
          tour.play(journeyStarting(start));
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
