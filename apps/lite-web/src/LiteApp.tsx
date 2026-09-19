import { StudentArtwork } from "./learning/StudentArtwork";
import { ApiError } from "./api/client";
import { apiErrorText } from "./api/errorText";
import learningTogether from "./home/assets/learning-together-v3.webp";
import { GuestTheme } from "./learning/GuestTheme";
import { CompanionAppearanceProvider } from "../../web/src/ui/CompanionAppearance";
import "./learning/student-surfaces.css";
import { LITE_ACCENT_PRESETS } from "../../web/src/ui/themes/lite";
import "../../web/src/ui/themes/lite.css";
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
import { AccentProvider, type LucideIcon } from "@/ui";
// `ui/background` is not re-exported from the ui barrel (only `ui/accent` is),
// so it is imported from its module directly — the same way pro's StudentApp
// reaches it.
import {
  BACKGROUND_PRESETS,
  BackgroundProvider,
  type BackgroundId,
} from "@/ui/background";
import { initialLiteAccent } from "./shared/liteAccent";
import { useLiteTheme } from "./shared/useLiteTheme";
import { LearningAccountButton, LearningRail } from "./shared/LearningRail";
import { AuthScreen } from "@/shell/auth/AuthScreen";
import { SettingsView } from "@/shell/settings/SettingsView";
import { createSession, makeMemoryStorage } from "@/shell/session";
import { resolveEditionDecision } from "@/shell/edition/editionRouting";
import { EditionRedirectNotice } from "@/shell/edition/EditionRedirectNotice";
import {
  liteRoutePath,
  navigate,
  listenForNavigation,
  parseLiteRoute,
  settingsPath,
  type LiteRoute,
  readingPath,
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
import { AwakeningRoom } from "./awakening/AwakeningRoom";
import { startLibraryReading } from "./api/library";
import { LiteTeacherShell } from "./teacher/LiteTeacherShell";
import { InboxButton } from "./inbox/InboxButton";

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
  const [bootError, setBootError] = useState<string | null>(null);
  const [bootAttempt, setBootAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    async function boot() {
      setBooted(false);
      setBootError(null);
      try {
        const me = await getMe();
        if (!cancelled) setUser(me);
      } catch (err) {
        if (!cancelled) {
          if (err instanceof ApiError && err.status === 401) setUser(null);
          else setBootError(apiErrorText(err));
        }
      } finally {
        if (!cancelled) setBooted(true);
      }
    }
    void boot();
    return () => {
      cancelled = true;
    };
  }, [bootAttempt]);

  if (!booted) return <GuestTheme><div className="h-full w-full bg-mk-paper" /></GuestTheme>;

  if (bootError) {
    return <GuestTheme><div className="flex min-h-screen flex-col items-center justify-center gap-4 p-6 text-mk-ink">
      <p role="alert">读取账号状态失败：{bootError}</p>
      <button type="button" className="rounded-mk-full bg-mk-accent-500 px-5 py-2.5 text-white" onClick={() => setBootAttempt((n) => n + 1)}>重新加载</button>
    </div></GuestTheme>;
  }

  if (user === null) {
    return <GuestTheme><AuthScreen illustration={learningTogether} onAuthed={setUser} client={{ signin, signup }} /></GuestTheme>;
  }

  const editionDecision = resolveEditionDecision("lite", user.school?.edition);
  if (editionDecision.kind !== "stay") {
    return (
      <GuestTheme><EditionRedirectNotice decision={editionDecision} appEdition="lite" /></GuestTheme>
    );
  }

  function onLogout() {
    void signout().finally(() => setUser(null));
  }

  return (
    // Students and teachers share the lite presets: both shells mount in the
    // lite theme scope (`useLiteTheme`), and SettingsView offers `presets`.
    <AccentProvider
      presets={LITE_ACCENT_PRESETS}
      initialAccent={initialLiteAccent(user.avatar_color, () => localStorage.getItem("mk-accent"))}
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
        {user.role === "teacher" || user.role === "admin" ? (
          <LiteTeacherShell user={user} onLogout={onLogout} />
        ) : (
          <StudentArtwork><CompanionAppearanceProvider image={bookmark}>
            <LiteShell user={user} onLogout={onLogout} />
          </CompanionAppearanceProvider></StudentArtwork>
        )}
      </BackgroundProvider>
    </AccentProvider>
  );
}

function LiteShell({ user, onLogout }: { user: MeUser; onLogout: () => void }) {
  // Portals inherit the same lite theme; see `useLiteTheme`.
  const { accent, background, themeStyle } = useLiteTheme();
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
    return listenForNavigation(onPopState);
  }, []);

  // 觉醒协议是**覆盖在树上面的一层**，不是替换整页的另一条路由。
  //
  // 这两者的差别就是 spec §7 那条门槛：树暗下去，让位给这个房间，一次动作。
  // 整页替换会让她感觉自己被送去了别的地方，而这个房间讲的恰恰是她自己那棵树。
  // 房间里不出现导航轨、页头和返回按钮 —— AwakeningRoom 自己铺满视口，
  // 下面这棵树仍然在，只是被盖住了。
  //
  // 它仍然有真实的 URL（`/tree/awakening`）：这一趟可能走十五分钟，刷新、
  // 误触返回键、第二天从历史记录点回来，都该落在同一个地方。
  const awakeningOpen = route.tab === "tree" && (route.awakening === true || !!route.reportRunId);
  // 走完一趟之后树要重拉一次。计数器住在这里而不是 SkyTab 里，因为触发它的
  // 是房间关门那一刻，而房间挂在这一层。
  const [treeNonce, setTreeNonce] = useState(0);

  return (
    <>
    <div
      className={cx(
        "lite-student flex h-full w-full overflow-hidden bg-mk-paper text-mk-ink",
        (route.tab === "home" ||
          (route.tab === "readings" && !route.readingId) ||
          (route.tab === "writings" && !route.writingId) ||
          (route.tab === "projects" && !route.projectId)) &&
          "lite-home-shell",
      )}
      style={themeStyle}
      data-accent={accent}
      data-background={background}
    >
      {!immersive && (
        <LearningRail
          links={TABS.map(({ key, label, icon }) => ({
            key,
            label,
            icon,
            active: route.tab === key || (key === "explore" && route.tab === "tree"),
            onSelect: () => navigate(tabPath(key)),
          }))}
          footer={
            <>
              {/* 收件箱 sits directly above 设置 at the foot of the rail:
                  teacher assignments arrive there, and neither is a place to
                  work, so neither belongs among the tabs. */}
              <InboxButton />
              <LearningAccountButton
                name={user.display_name}
                image={bookmark}
                active={route.tab === "settings"}
                onSelect={() => navigate(settingsPath())}
              />
            </>
          }
        />
      )}

      <main data-student-page={route.tab} className="min-w-0 flex-1 overflow-y-auto">
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
              const path = liteRoutePath({ tab: "courses", slug });
              if (onCoursesPage() && window.location.pathname !== path) navigate(path);
            }}
            onBackToList={() => {
              const path = liteRoutePath({ tab: "courses" });
              if (onCoursesPage() && window.location.pathname !== path) navigate(path);
            }}
            onImmersiveChange={setImmersive}
          />
        ) : route.tab === "explore" || route.tab === "tree" ? (
          // 一格两屏。URL 仍然是两条（`/explore` / `/tree`），所以深链、后退、
          // 收藏都还是原来那样；切换器改的就是 URL，不是一个只活在内存里的状态。
          <SkyTab
            user={user}
            surface={route.tab === "tree" ? "tree" : "map"}
            refreshNonce={treeNonce}
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

    {/* 觉醒协议的房间。盖在树上面，所以进门是一次动作，不是一次整页跳转。 */}
    <AwakeningRoom
      open={awakeningOpen}
      reportRunId={route.tab === "tree" ? route.reportRunId : undefined}
      onClose={() => {
        // **每次出门都换一个 nonce**，不看这一趟有没有长出词。
        //
        // 它同时驱动两件事：树重拉一次（否则她看到的还是走进来之前那一棵），
        // 以及觉醒协议那条入口重查一次（否则她刚走完，按钮上还写着「开始」，
        // 「查看兴趣印记」也不出现）。第二件事和长没长出词无关 —— 一趟一个词
        // 都没长出来，她仍然是走完过的人。
        setTreeNonce((n) => n + 1);
        navigate(liteRoutePath({ tab: "tree" }));
      }}
      onOpenReading={(slug, tier) => {
        // 报告里那几篇是**真的文章**，所以点它就该真的开始读，而不是把她丢回
        // 书架自己找。开一篇 = 建一条 reading（和书架那两处走同一个入口），
        // 失败时退到书架，她至少还看得见那一架书。
        void startLibraryReading(slug, tier)
          .then((id) => navigate(readingPath(id)))
          .catch(() => navigate(liteRoutePath({ tab: "readings", library: true })));
      }}
    />
    </>
  );
}
