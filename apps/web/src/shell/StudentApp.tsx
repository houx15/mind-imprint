import { useState } from "react";
import type { SessionStore } from "./session";
import { Nav, type NavTab } from "./Nav";
import { HomePage } from "./home/HomePage";
import { CoursesContainer } from "./courses/CoursesContainer";
import { WorkspaceContainer } from "../workspace/WorkspaceContainer";
import { GrowthReport } from "./growth/GrowthReport";
import { ToolkitCards } from "./growth/ToolkitCards";
import { SettingsView } from "./settings/SettingsView";
import { AccentProvider, ACCENT_PRESETS, type AccentId } from "../ui/accent";
import { Segmented } from "../ui/feedback";
import { api } from "../api";

// StudentApp — platform shell rebuild (Task 4). Four top-level surfaces
// reachable from the 52px `Nav` rail: 首页 (home, new default landing) /
// 项目 (projects — the Directory → four-room studio, unchanged internally) /
// 图鉴 (gallery — the tool-card catalog, promoted to top level) / 我 (a small
// hub for 成长报告 + 设置). Course play and a project's growth report are
// reached via deep-link state rather than their own nav entries.
//
// The 聊天/ChatContainer tab was dropped from the nav in an earlier slice;
// this rewrite drops its now-dead branch from the shell too. The component
// and its backend endpoints are untouched — only the nav-less route is gone.

type Tab = NavTab;

/** Only a known 8-preset id seeds the accent provider; anything else (unset,
 * legacy free-hex `avatar_color`, etc) falls back to accent.tsx's own
 * localStorage → vermilion default. */
function coerceAccent(value: string | null | undefined): AccentId | undefined {
  return ACCENT_PRESETS.some((p) => p.id === value) ? (value as AccentId) : undefined;
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

  // When the student finishes a project (Directory's "完成" flow), we
  // deep-link 成长报告 to that entry. Cleared when 我/成长报告 is opened
  // directly via the nav rather than via that flow.
  const [growthFocus, setGrowthFocus] = useState<string | null>(null);
  // Course play is reachable from home, the gallery, or a growth-report
  // card's "去学这张卡的课程" link — an overlay independent of the active
  // tab, so it survives (and is cleared by) nav navigation regardless of tab.
  const [courseFocus, setCourseFocus] = useState<string | null>(null);
  const [meSection, setMeSection] = useState<"growth" | "settings">("growth");
  // Open-from-home deep-links into the 项目 tab (Task 6): a specific project
  // (home's recent-project tiles) or the create drawer (home's "新建
  // 项目"/新建 tiles). Each is a one-shot signal — WorkspaceContainer calls
  // the matching `on...Consumed` callback right after acting on it, so
  // revisiting 项目 via the nav rail afterwards just shows the plain
  // directory rather than re-triggering the same open/create.
  const [pendingProjectId, setPendingProjectId] = useState<string | null>(null);
  const [pendingCreate, setPendingCreate] = useState(false);
  // Immersive studio (spec §17, 外壳 B): while a project is OPEN inside the
  // 项目 tab, the platform nav rail is hidden so the studio is full-bleed —
  // the only way out is the studio top bar's 「← 主页」capsule. On the project
  // directory (no project open) the nav returns.
  const [inProject, setInProject] = useState(false);
  const showNav = !(tab === "projects" && inProject);

  function selectTab(t: Tab) {
    setCourseFocus(null);
    if (t === "me") setGrowthFocus(null);
    setTab(t);
  }

  function openCourse(slug: string) {
    setCourseFocus(slug);
  }

  function openProjectFromHome(id: string) {
    setPendingCreate(false);
    setPendingProjectId(id);
    selectTab("projects");
  }

  function openCreateFromHome() {
    setPendingProjectId(null);
    setPendingCreate(true);
    selectTab("projects");
  }

  let body;
  if (courseFocus) {
    body = <CoursesContainer initialCourseId={courseFocus} onGoPortal={() => setCourseFocus(null)} />;
  } else if (tab === "home") {
    body = (
      <HomePage
        user={user}
        onOpenProject={openProjectFromHome}
        onOpenCourse={openCourse}
        onCreateProject={openCreateFromHome}
        onGoProjects={() => selectTab("projects")}
        onGoGallery={() => selectTab("gallery")}
      />
    );
  } else if (tab === "projects") {
    body = (
      <WorkspaceContainer
        onFinished={(projectId?: string) => {
          setInProject(false);
          setGrowthFocus(projectId ?? null);
          setMeSection("growth");
          setTab("me");
        }}
        initialProjectId={pendingProjectId}
        onInitialProjectIdConsumed={() => setPendingProjectId(null)}
        autoOpenCreate={pendingCreate}
        onAutoOpenCreateConsumed={() => setPendingCreate(false)}
        onInProjectChange={setInProject}
        onExitToHome={() => {
          setInProject(false);
          setTab("home");
        }}
      />
    );
  } else if (tab === "gallery") {
    body = <ToolkitCards onOpenCourse={openCourse} />;
  } else {
    body = (
      <div className="flex h-full w-full flex-col overflow-hidden bg-mk-paper">
        <div className="flex shrink-0 items-center justify-center border-b border-mk-border bg-mk-surface p-3">
          <Segmented
            value={meSection}
            onChange={(v) => setMeSection(v === "settings" ? "settings" : "growth")}
            options={[
              { value: "growth", label: "成长报告" },
              { value: "settings", label: "设置" },
            ]}
          />
        </div>
        <div className="min-h-0 flex-1 overflow-hidden">
          {meSection === "growth" ? (
            <GrowthReport initialScopeId={growthFocus} onOpenCourse={openCourse} />
          ) : (
            <SettingsView session={session} user={user} onLogout={onLogout} />
          )}
        </div>
      </div>
    );
  }

  return (
    <AccentProvider
      initialAccent={coerceAccent(user?.avatar_color)}
      onPersist={(id) => {
        void api.setAccent(id);
      }}
    >
      <div className="flex h-full w-full overflow-hidden bg-mk-paper">
        {showNav && <Nav tab={tab} onTab={selectTab} user={user} />}
        <div className="relative flex-1 overflow-hidden">{body}</div>
      </div>
    </AccentProvider>
  );
}
