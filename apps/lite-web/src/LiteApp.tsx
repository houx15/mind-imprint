import { useEffect, useState } from "react";
import { BookOpen, PenLine } from "lucide-react";
import { Icon, Pebble, type LucideIcon } from "@/ui";
import { liteRoutePath, navigate, parseLiteRoute, type LiteRoute } from "./routing";
import { ReadingsLanding } from "./readings/ReadingsLanding";
import { ReadingRoomHost } from "./readings/ReadingRoomHost";

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
 * 写作 is not built until P3 — its tab is an honest "写作即将上线" line, not a
 * fake composer.
 */

type LiteTab = LiteRoute["tab"];

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

function WritingsComingSoon() {
  return (
    <div className="flex h-full items-center justify-center p-8">
      <p className="text-mk-body text-mk-muted">写作即将上线。</p>
    </div>
  );
}

export function LiteApp() {
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
        </nav>
      </div>

      <main className="min-w-0 flex-1 overflow-y-auto">
        {route.tab === "writings" ? (
          <WritingsComingSoon />
        ) : route.readingId ? (
          <ReadingRoomHost key={route.readingId} readingId={route.readingId} />
        ) : (
          <ReadingsLanding />
        )}
      </main>
    </div>
  );
}
