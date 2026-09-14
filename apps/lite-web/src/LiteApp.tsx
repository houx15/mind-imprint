import { useEffect, useState } from "react";
import {
  House,
  BookOpen,
  Compass,
  GraduationCap,
  Globe,
  Hammer,
  PenLine,
} from "lucide-react";
import {
  Settings,
  useAccent,
  ACCENT_PRESETS,
  AccentProvider,
  type AccentId,
  type LucideIcon,
} from "@/ui";
// `ui/background` is not re-exported from the ui barrel (only `ui/accent` is),
// so it is imported from its module directly — the same way pro's StudentApp
// reaches it.
import {
  BACKGROUND_PRESETS,
  BackgroundProvider,
  useBackground,
  type BackgroundId,
} from "@/ui/background";
import { AuthScreen } from "@/shell/auth/AuthScreen";
import { SettingsView } from "@/shell/settings/SettingsView";
import { createSession, makeMemoryStorage } from "@/shell/session";
import { resolveEditionDecision } from "@/shell/edition/editionRouting";
import { EditionRedirectNotice } from "@/shell/edition/EditionRedirectNotice";
import {
  liteRoutePath,
  navigate,
  parseLiteRoute,
  settingsPath,
  type LiteRoute,
} from "./routing";
import {
  getMe,
  signin,
  signout,
  signup,
  setAccent,
  setBackground,
  type MeUser,
} from "./api/auth";
import { ProjectRoom } from "./projects/ProjectRoom";
import { ProjectsLanding } from "./projects/ProjectsLanding";
import { ReadingsLanding } from "./readings/ReadingsLanding";
import { ReadingLibraryPage } from "./readings/ReadingLibraryPage";
import { ReadingRoomHost } from "./readings/ReadingRoomHost";
import { WritingsLanding } from "./writings/WritingsLanding";
import { WritingRoomHost } from "./writings/WritingRoomHost";
import { SkyTab } from "./explore/SkyTab";
import { MySitePage } from "./mysite/MySitePage";
import { CoursesHost } from "./courses/CoursesHost";
import { LearningHome, bookmark } from "./home/LearningHome";
import "./home/learning.css";
import { AwakeningQuiz } from "./tree/quiz/AwakeningQuiz";

/** Lite student shell. The learning home uses expanded navigation on wide
 * screens; workrooms retain a compact rail to preserve reading/writing space.
 * Auth, edition checks, immersive courses and room routes stay unchanged.
 */

type LiteTab =
  | "home"
  | "explore"
  | "readings"
  | "writings"
  | "projects"
  | "courses"
  | "mysite";

// 顺序本身在说一句话：**探索（找到）→ 读 → 写 → 做 → 课程 → 主页（东西放在
// 那里）。**
//
// 2026-09-05：我的树不再单独占一格。它和探索地图是同一件事的两半 —— 树是她已经
// 有的，地图是她还没走过的 —— 合进「探索」那一格，由顶部切换器换
// （`explore/SkyTab.tsx`）。腾出来的一格给了「我的主页」：主页发布之后她随时能
// 回来改，而在这之前，回到那一页的路只有「项目室 → 主页项目 → 侧栏那一行」。
const TABS: { key: LiteTab; label: string; icon: LucideIcon }[] = [
  { key: "home", label: "首页", icon: House },
  { key: "explore", label: "探索", icon: Compass },
  { key: "readings", label: "阅读", icon: BookOpen },
  { key: "writings", label: "写作", icon: PenLine },
  { key: "projects", label: "项目", icon: Hammer },
  // 课程 sits right after 项目 because that is where it is reached from: 印记
  // hands her a course when the project needs a skill she has not learned yet
  // (see projects/tools/surfaces/Course.tsx).
  { key: "courses", label: "课程", icon: GraduationCap },
  { key: "mysite", label: "我的主页", icon: Globe },
];

function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

function tabPath(tab: LiteTab): string {
  // Explicit per tab rather than a cast: `LiteRoute` carries optional id
  // fields per tab, so a blanket `{ tab }` would not narrow.
  switch (tab) {
    case "home":
      return "/";
    case "readings":
      return liteRoutePath({ tab: "readings" });
    case "writings":
      return liteRoutePath({ tab: "writings" });
    case "projects":
      return liteRoutePath({ tab: "projects" });
    case "courses":
      return liteRoutePath({ tab: "courses" });
    case "explore":
      return liteRoutePath({ tab: "explore" });
    case "mysite":
      return liteRoutePath({ tab: "mysite" });
  }
}

/** Coerce the server's avatar_color into a known accent preset id (else
 * undefined, so AccentProvider falls back to its own default). Mirrors the
 * pro shell's identical helper — the presets are the shared source of truth,
 * so an unknown value degrades to the default rather than being applied raw. */
function coerceAccent(value: string | null | undefined): AccentId | undefined {
  return ACCENT_PRESETS.some((p) => p.id === value)
    ? (value as AccentId)
    : undefined;
}

// Preserve a saved choice; only accounts without a preference get the new default.
function initialLiteAccent(value: string | null | undefined): AccentId {
  const saved = coerceAccent(value);
  if (saved) return saved;
  try {
    return coerceAccent(localStorage.getItem("mk-accent")) ?? "teal";
  } catch {
    return "teal";
  }
}

function coerceBackground(
  value: string | null | undefined,
): BackgroundId | undefined {
  return BACKGROUND_PRESETS.some((p) => p.id === value)
    ? (value as BackgroundId)
    : undefined;
}

/** 地址栏当下是不是还停在课程页。用处见 `CoursesHost` 那两个回调上的注释。 */
function onCoursesPage(): boolean {
  return parseLiteRoute(window.location.pathname).tab === "courses";
}

/**
 * SettingsView takes a `SessionStore` it does not read (its parameter is
 * literally destructured as `session: _session`) — the prop is a leftover of
 * pro's shell wiring. Rather than change pro's signature to suit lite, lite
 * hands it a throwaway in-memory store: nothing is written, nothing is read,
 * and lite's own auth state stays where it belongs, in `LiteApp` below.
 */
const unusedSettingsSession = createSession({ storage: makeMemoryStorage() });

/**
 * LiteApp — auth boot, then the shell.
 *
 * Both editions show the SAME `AuthScreen` and share one session cookie
 * (host-only on the API, same-site for every *.uni-robot.cn frontend), so a
 * student signs in once no matter which host they landed on. Immediately
 * after, `resolveEditionDecision` checks the school's edition against this
 * app's — a pro student who arrived here is told what happened and sent to
 * the pro host, rather than left in an app where every request 404s.
 */
export function LiteApp() {
  const [booted, setBooted] = useState(false);
  const [user, setUser] = useState<MeUser | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function boot() {
      try {
        const me = await getMe();
        if (!cancelled) setUser(me);
      } catch {
        // No live session — fall through to the auth screen. Any other
        // failure looks the same from here and has the same right answer:
        // ask them to sign in.
        if (!cancelled) setUser(null);
      } finally {
        if (!cancelled) setBooted(true);
      }
    }
    void boot();
    return () => {
      cancelled = true;
    };
  }, []);

  if (!booted) return <div className="h-full w-full bg-mk-paper" />;

  if (user === null) {
    return <AuthScreen onAuthed={setUser} client={{ signin, signup }} />;
  }

  const editionDecision = resolveEditionDecision("lite", user.school?.edition);
  if (editionDecision.kind !== "stay") {
    return (
      <EditionRedirectNotice decision={editionDecision} appEdition="lite" />
    );
  }

  function onLogout() {
    void signout().finally(() => setUser(null));
  }

  return (
    <AccentProvider
      initialAccent={initialLiteAccent(user.avatar_color)}
      onPersist={(id) => {
        void setAccent(id);
      }}
    >
      <BackgroundProvider
        initialBackground={coerceBackground(user.page_background)}
        onPersist={(id) => {
          void setBackground(id);
        }}
      >
        <LiteShell user={user} onLogout={onLogout} />
      </BackgroundProvider>
    </AccentProvider>
  );
}

function LiteShell({ user, onLogout }: { user: MeUser; onLogout: () => void }) {
  const { id: accent } = useAccent();
  const { id: background } = useBackground();
  const [route, setRoute] = useState<LiteRoute>(() =>
    parseLiteRoute(window.location.pathname),
  );
  // 课程播放器占满整页时收起导航轨。一门课是一段连续的叙事，旁边留一条
  // 「阅读 / 写作 / 项目」的轨会把它降级成一个开着的面板 —— 和觉醒协议满屏
  // 渲染是同一条理由。pro 的 shell 在同一个位置做同一件事。
  const [immersive, setImmersive] = useState(false);

  useEffect(() => {
    function onPopState() {
      setRoute(parseLiteRoute(window.location.pathname));
    }
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  // 觉醒协议**满屏渲染，不带导航轨**。它是一个连续的七屏叙事，旁边杵着一条
  // 「阅读 / 写作 / 项目」的导航栏会把它降级成「一个开着的表单」——而这一屏的
  // 全部任务就是让一个还不知道自己喜欢什么的学生愿意花五分钟。
  //
  // 放在这里（所有 hook 之后）而不是 `rootElementFor`：它需要已登录的 `user`，
  // 而且做完之后要能原地回到树上，不该是一次整页跳转。
  if (route.tab === "tree" && route.quiz) {
    return (
      <div className="h-full w-full overflow-hidden">
        <AwakeningQuiz
          onExit={() => navigate(liteRoutePath({ tab: "tree" }))}
        />
      </div>
    );
  }

  return (
    <div
      className={cx(
        "lite-student flex h-full w-full overflow-hidden bg-mk-paper text-mk-ink",
        (route.tab === "home" ||
          (route.tab === "readings" && !route.readingId) ||
          (route.tab === "writings" && !route.writingId) ||
          (route.tab === "projects" && !route.projectId)) &&
          "lite-home-shell",
      )}
      data-accent={accent}
      data-background={background}
    >
      {!immersive && (
        <div className="learning-nav-slot">
          <nav className="learning-nav" aria-label="主导航">
            <a
              href="/"
              className="learning-brand"
              onClick={(e) => {
                e.preventDefault();
                navigate("/");
              }}
            >
              <BookOpen size={29} />
              <span>
                思维印记<small>MIND IMPRINT</small>
              </span>
            </a>
            <div className="learning-nav-links">
              {TABS.map(({ key, label, icon: NavIcon }) => {
                const active =
                  route.tab === key ||
                  (key === "explore" && route.tab === "tree");
                return (
                  <button
                    type="button"
                    key={key}
                    aria-label={label}
                    aria-current={active ? "page" : undefined}
                    onClick={() => navigate(tabPath(key))}
                  >
                    <NavIcon size={21} strokeWidth={1.7} />
                    <span>{label}</span>
                  </button>
                );
              })}
            </div>
            <div className="learning-nav-bottom">
              <button
                className="learning-account"
                type="button"
                aria-label="设置"
                aria-current={route.tab === "settings" ? "page" : undefined}
                onClick={() => navigate(settingsPath())}
              >
                <img src={bookmark} alt="" />
                <span>
                  {user.display_name || "我的账号"}
                  <small>账号与设置</small>
                </span>
                <Settings size={17} />
              </button>
            </div>
          </nav>
        </div>
      )}

      <main className="min-w-0 flex-1 overflow-y-auto">
        {route.tab === "home" ? (
          <LearningHome user={user} />
        ) : route.tab === "settings" ? (
          <SettingsView
            session={unusedSettingsSession}
            user={user}
            onLogout={onLogout}
          />
        ) : route.tab === "writings" ? (
          route.writingId ? (
            <WritingRoomHost
              key={route.writingId}
              writingId={route.writingId}
            />
          ) : (
            <WritingsLanding />
          )
        ) : route.tab === "courses" ? (
          // 🚨 只有当这一格还开着的时候，课程页才有权改 URL（2026-09-07）。
          //
          // `CoursesContainer` 在**卸载时**调 `onActiveCourseChange(null)`
          // （apps/web/.../CoursesContainer.tsx，那句 `return () => …(null)`），
          // 而这里把它接到了「回到课程目录」上。后果：她在课程页点任意一个别的
          // tab，导航先把 URL 推到 `/site`，课程页随即卸载、把 `/courses` 又推
          // 回来 —— 从她那边看是「课程页出不去了」。
          //
          // 判据是**地址栏当下的路径**，不是这个闭包里的 `route`：卸载时跑的
          // 是上一次提交留下的那个回调，它闭包里的 `route` 还停在 courses，
          // 判不出来。而 `window.location` 在那一刻已经是新的了。
          <CoursesHost
            slug={route.slug}
            onOpenCourse={(slug) => {
              if (onCoursesPage())
                navigate(liteRoutePath({ tab: "courses", slug }));
            }}
            onBackToList={() => {
              if (onCoursesPage()) navigate(liteRoutePath({ tab: "courses" }));
            }}
            onImmersiveChange={setImmersive}
          />
        ) : route.tab === "explore" || route.tab === "tree" ? (
          // 一格两屏。URL 仍然是两条（`/explore` / `/tree`），所以深链、后退、
          // 收藏都还是原来那样；切换器改的就是 URL，不是一个只活在内存里的状态。
          <SkyTab
            user={user}
            surface={route.tab === "tree" ? "tree" : "map"}
            onSwitch={(next) =>
              navigate(
                liteRoutePath(
                  next === "tree" ? { tab: "tree" } : { tab: "explore" },
                ),
              )
            }
          />
        ) : route.tab === "mysite" ? (
          <MySitePage />
        ) : route.tab === "projects" ? (
          route.projectId ? (
            <ProjectRoom key={route.projectId} projectId={route.projectId} />
          ) : (
            <ProjectsLanding />
          )
        ) : route.tab === "readings" && route.library ? (
          // 分级阅读库。同一条 tab 下的一屏（`/readings/library`），所以左侧
          // 导航栏仍然停在「阅读」上。
          <ReadingLibraryPage />
        ) : route.tab === "readings" && route.readingId ? (
          <ReadingRoomHost key={route.readingId} readingId={route.readingId} />
        ) : (
          // 这里只剩两种情况：`/readings`（没带 id），以及理论上不该走到这儿的
          // `share` / `page` —— 那两条链接由 `rootElementFor` 在登录之前就接走
          // 了，这个分支只是万一 `LiteShell` 的 popstate 中途捡到一条时的兜底。
          //
          // 🚨 注意它**不再是「未知路径的落点」**：落地页与未知路径现在都归探索
          // （见 `parseLiteRoute` 的注释），`readings` 只有从 `/readings` 进来
          // 才会出现。
          <ReadingsLanding />
        )}
      </main>
    </div>
  );
}
