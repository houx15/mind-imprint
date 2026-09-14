import { useEffect, useState } from "react";
import { BookOpen, Compass, GraduationCap, Globe, Hammer, PenLine } from "lucide-react";
import { Icon, Pebble, Settings, ACCENT_PRESETS, AccentProvider, type AccentId, type LucideIcon } from "@/ui";
// `ui/background` is not re-exported from the ui barrel (only `ui/accent` is),
// so it is imported from its module directly — the same way pro's StudentApp
// reaches it.
import { BACKGROUND_PRESETS, BackgroundProvider, type BackgroundId } from "@/ui/background";
import { AuthScreen } from "@/shell/auth/AuthScreen";
import { SettingsView } from "@/shell/settings/SettingsView";
import { createSession, makeMemoryStorage } from "@/shell/session";
import { resolveEditionDecision } from "@/shell/edition/editionRouting";
import { EditionRedirectNotice } from "@/shell/edition/EditionRedirectNotice";
import { liteRoutePath, navigate, parseLiteRoute, settingsPath, type LiteRoute } from "./routing";
import { getMe, signin, signout, signup, setAccent, setBackground, type MeUser } from "./api/auth";
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
import { AwakeningQuiz } from "./tree/quiz/AwakeningQuiz";
import { LiteTeacherShell } from "./teacher/LiteTeacherShell";
import { InboxButton } from "./inbox/InboxButton";

/**
 * LiteApp — the lite edition's shell: a left icon-rail with two tabs (阅读 /
 * 写作) that AUTO-FOLDS TO ICONS ONLY, matching the shape of the existing
 * frontend's `Nav` (apps/web/src/shell/Nav.tsx) rather than a Cowork-style
 * "switcher on top, session list below".
 *
 * Why (2026-08-26 product-owner decision, see AGENTS.md/task brief): Cowork's
 * session list serves PARALLEL work — many threads alive at once, so the
 * list itself is the workspace. Reading one article or drafting one piece is
 * a FOCUSED task — one thing in hand at a time — so the sidebar is
 * navigation, not workspace, and history stays on each tab's own landing
 * page (`ReadingsLanding`'s 过往的阅读), never in the sidebar.
 *
 * The fold is not cosmetic: the reading room's article, coach chat, and
 * paragraph-anchored cards all compete for horizontal space, so a folded
 * 64px icon rail gives that space back. It rests folded by default (matching
 * `Nav`'s collapsed-64px/hover-to-208px behavior via CSS only — no JS toggle
 * state) and only opens on hover/keyboard-focus, i.e. while the student is
 * choosing what to do.
 *
 * Routing: no router library. `parseLiteRoute`/`liteRoutePath` (./routing)
 * parse/format `window.location.pathname`; a `popstate` listener re-derives
 * the route on Back/Forward and on `navigate`'s synthetic dispatch.
 *
 * `/readings/:id` mounts `ReadingRoomHost` (Task 12), which mounts lite's OWN
 * `ReadingRoom` (`./readings/ReadingRoom`). It used to host pro's room under
 * `LITE_READING_CAPABILITIES`; that room forked into lite on 2026-08-29, so
 * the pro-only surfaces are no longer switched off by a capability object —
 * they are not in lite's file at all.
 * `/writings/:id` mounts `WritingRoomHost` (P3 Task 8) — writing has no
 * analogous standalone pro room to host (`WritingBlock`/`WorkspaceContainer`
 * are module-private and cannot mount independently — see the task brief's
 * 复用边界), so the writing room is assembled fresh out of the standalone
 * primitives (`StudioCardSheet`, the chat log/composer) rather than hosted.
 */

type LiteTab = "explore" | "readings" | "writings" | "projects" | "courses" | "mysite";

// 顺序本身在说一句话：**探索（找到）→ 读 → 写 → 做 → 课程 → 主页（东西放在
// 那里）。**
//
// 2026-09-05：我的树不再单独占一格。它和探索地图是同一件事的两半 —— 树是她已经
// 有的，地图是她还没走过的 —— 合进「探索」那一格，由顶部切换器换
// （`explore/SkyTab.tsx`）。腾出来的一格给了「我的主页」：主页发布之后她随时能
// 回来改，而在这之前，回到那一页的路只有「项目室 → 主页项目 → 侧栏那一行」。
const TABS: { key: LiteTab; label: string; icon: LucideIcon }[] = [
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
  return ACCENT_PRESETS.some((p) => p.id === value) ? (value as AccentId) : undefined;
}

function coerceBackground(value: string | null | undefined): BackgroundId | undefined {
  return BACKGROUND_PRESETS.some((p) => p.id === value) ? (value as BackgroundId) : undefined;
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
    return <EditionRedirectNotice decision={editionDecision} appEdition="lite" />;
  }

  function onLogout() {
    void signout().finally(() => setUser(null));
  }

  return (
    <AccentProvider
      initialAccent={coerceAccent(user.avatar_color)}
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
          <LiteShell user={user} onLogout={onLogout} />
        )}
      </BackgroundProvider>
    </AccentProvider>
  );
}

function LiteShell({ user, onLogout }: { user: MeUser; onLogout: () => void }) {
  const [route, setRoute] = useState<LiteRoute>(() => parseLiteRoute(window.location.pathname));
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

  // Labels stay in the DOM always (opacity toggled, not conditionally
  // rendered) so the reveal is a pure CSS transition — same trick as `Nav`.
  const labelCls =
    "whitespace-nowrap text-mk-body opacity-0 transition-opacity duration-200 ease-mk " +
    "group-hover/nav:opacity-100 group-focus-within/nav:opacity-100 motion-reduce:transition-none";

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
    <div className="flex h-full w-full overflow-hidden bg-mk-paper text-mk-ink">
      {/* `mk-lite-navslot` / `mk-lite-nav` exist only so `index.css` can fold
          the rail to 48px below 560px — 64px is 17% of a 375px screen, spent
          on two icons, on the same screen where the article and the coach
          column are already out of room. Tailwind can't express it: the width
          has to lose to `hover:w-[208px]`, and a media-query utility would sit
          at the same specificity. */}
      {!immersive && (
      <div className="mk-lite-navslot relative z-30 w-[64px] shrink-0">
        <nav
          className={cx(
            "mk-lite-nav group/nav absolute inset-y-0 left-0 flex w-[64px] flex-col gap-1 overflow-hidden p-3",
            // Folded (64px) is the resting state; hover/keyboard-focus within
            // the rail expands it to 208px and reveals labels. Purely CSS —
            // there is no JS "expanded" state to keep in sync.
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

          {TABS.map(({ key, label, icon }) => {
            // 探索那一格在 `/tree` 上也是选中的：树住在它下面（SkyTab 的第二屏），
            // 不高亮的话她切到树之后导航上没有任何一格是亮的。
            const active = route.tab === key || (key === "explore" && route.tab === "tree");
            return (
              <button
                key={key}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => navigate(tabPath(key))}
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

          {/* 收件箱 and 设置 sit at the FOOT of the rail, not among the tabs:
              设置 is where the account lives, and 收件箱 is where teacher
              assignments arrive — neither is a place to work. `mt-auto` on
              the inbox button pins both below whatever tabs exist above. */}
          <InboxButton labelCls={labelCls} />
          <button
            type="button"
            onClick={() => navigate(settingsPath())}
            aria-current={route.tab === "settings" ? "page" : undefined}
            className={cx(
              "flex items-center gap-3 rounded-mk-md px-1.5 py-2 transition-colors duration-[120ms] ease-mk",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60",
              route.tab === "settings" ? "bg-white/15" : "hover:bg-white/10",
            )}
          >
            <span className="flex h-7 w-7 shrink-0 items-center justify-center">
              <Icon icon={Settings} size={22} className={route.tab === "settings" ? "text-white" : "text-white/70"} />
            </span>
            <span
              className={cx(labelCls, route.tab === "settings" ? "font-semibold text-white" : "text-white/80")}
            >
              设置
            </span>
          </button>
        </nav>
      </div>
      )}

      <main className="min-w-0 flex-1 overflow-y-auto">
        {route.tab === "settings" ? (
          <SettingsView session={unusedSettingsSession} user={user} onLogout={onLogout} />
        ) : route.tab === "writings" ? (
          route.writingId ? (
            <WritingRoomHost key={route.writingId} writingId={route.writingId} />
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
              if (onCoursesPage()) navigate(liteRoutePath({ tab: "courses", slug }));
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
              navigate(liteRoutePath(next === "tree" ? { tab: "tree" } : { tab: "explore" }))
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
