import { useEffect, useState } from "react";
import { BookOpen, PenLine } from "lucide-react";
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
import { ReadingsLanding } from "./readings/ReadingsLanding";
import { ReadingRoomHost } from "./readings/ReadingRoomHost";
import { WritingsLanding } from "./writings/WritingsLanding";
import { WritingRoomHost } from "./writings/WritingRoomHost";

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
 * `/readings/:id` mounts `ReadingRoomHost` (Task 12), which mounts the REAL
 * `ReadingRoom` from apps/web under `LITE_READING_CAPABILITIES`.
 * `/writings/:id` mounts `WritingRoomHost` (P3 Task 8) — writing has no
 * analogous standalone pro room to host (`WritingBlock`/`WorkspaceContainer`
 * are module-private and cannot mount independently — see the task brief's
 * 复用边界), so the writing room is assembled fresh out of the standalone
 * primitives (`StudioCardSheet`, the chat log/composer) rather than hosted.
 */

type LiteTab = "readings" | "writings";

const TABS: { key: LiteTab; label: string; icon: LucideIcon }[] = [
  { key: "readings", label: "阅读", icon: BookOpen },
  { key: "writings", label: "写作", icon: PenLine },
];

function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

function tabPath(tab: LiteTab): string {
  return liteRoutePath(tab === "readings" ? { tab: "readings" } : { tab: "writings" });
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
        <LiteShell user={user} onLogout={onLogout} />
      </BackgroundProvider>
    </AccentProvider>
  );
}

function LiteShell({ user, onLogout }: { user: MeUser; onLogout: () => void }) {
  const [route, setRoute] = useState<LiteRoute>(() => parseLiteRoute(window.location.pathname));

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

  return (
    <div className="flex h-full w-full overflow-hidden bg-mk-paper text-mk-ink">
      <div className="relative z-30 w-[64px] shrink-0">
        <nav
          className={cx(
            "group/nav absolute inset-y-0 left-0 flex w-[64px] flex-col gap-1 overflow-hidden p-3",
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
            const active = route.tab === key;
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

          {/* 设置 sits at the FOOT of the rail, not among the tabs: it is where
              the account lives, not a third place to work. `mt-auto` pins it
              below whatever tabs exist above. */}
          <button
            type="button"
            onClick={() => navigate(settingsPath())}
            aria-current={route.tab === "settings" ? "page" : undefined}
            className={cx(
              "mt-auto flex items-center gap-3 rounded-mk-md px-1.5 py-2 transition-colors duration-[120ms] ease-mk",
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

      <main className="min-w-0 flex-1 overflow-y-auto">
        {route.tab === "settings" ? (
          <SettingsView session={unusedSettingsSession} user={user} onLogout={onLogout} />
        ) : route.tab === "writings" ? (
          route.writingId ? (
            <WritingRoomHost key={route.writingId} writingId={route.writingId} />
          ) : (
            <WritingsLanding />
          )
        ) : route.tab === "readings" && route.readingId ? (
          <ReadingRoomHost key={route.readingId} readingId={route.readingId} />
        ) : (
          // Also covers `route.tab === "share"`: the share route is meant to
          // be caught by `LiteApp`'s own pre-auth check and never reach this
          // shell at all (see the comment there). This branch is only a safe
          // fallback should `LiteShell`'s popstate listener ever pick one up
          // mid-session — the readings landing, same as any other unknown
          // path (`parseLiteRoute`'s own default).
          <ReadingsLanding />
        )}
      </main>
    </div>
  );
}
