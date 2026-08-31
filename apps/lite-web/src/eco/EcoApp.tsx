import { useEffect, useState } from "react";
import { Compass, Hexagon, MessageCircle, User } from "lucide-react";
import { EcoProvider, useEco } from "./store";
import { ecoPath, go, parseEcoRoute, type EcoRoute } from "./route";
import { CoachDrawer } from "./coach/CoachDrawer";
import { WorldView } from "./world/WorldView";
import { TreeView } from "./home/TreeView";
import { ProjectsHub } from "./projects/ProjectsHub";
import { NewProject } from "./projects/NewProject";
import { Workbench } from "./projects/Workbench";
import { MyPage } from "./page/MyPage";
import { PersonalPage } from "./page/PersonalPage";
import { cx } from "./ui";
import "./eco.css";

/**
 * EcoApp — the ecosystem prototype's shell.
 *
 * Rail on the left, one surface in the middle, 印记 as a drawer that any
 * surface can call. The rail is DEEP INK rather than lite's accent gradient
 * because it has to sit over two very different grounds (the world's night
 * and the tree's paper) without fighting either; the accent is spent on the
 * active indicator instead, where it does more work.
 *
 * `/eco/p/:handle` renders WITHOUT the shell — a published personal page is
 * meant to be opened by a friend with no account, and a friend should not see
 * her navigation. Same reasoning as lite's `/s/:token`.
 */

const TABS = [
  { key: "home", label: "首页", sub: "世界 · 我的地图", icon: Compass },
  { key: "projects", label: "项目", sub: "做出来", icon: Hexagon },
  { key: "me", label: "我的主页", sub: "对外的那一页", icon: User },
] as const;

export function EcoRoot() {
  const [route, setRoute] = useState<EcoRoute>(() => parseEcoRoute(window.location.pathname));

  useEffect(() => {
    function onPop() {
      setRoute(parseEcoRoute(window.location.pathname));
    }
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  return (
    <EcoProvider>
      {route.name === "page" ? <PersonalPage handle={route.handle} /> : <Shell route={route} />}
    </EcoProvider>
  );
}

function Shell({ route }: { route: EcoRoute }) {
  const { state, openCoach } = useEco();
  // Both home views now paint their own dark ground, so neither wants the
  // paper texture behind it.
  const dark = route.name === "home";

  const activeTab: string =
    route.name === "home"
      ? "home"
      : route.name === "projects" || route.name === "project" || route.name === "project-new"
        ? "projects"
        : "me";

  function goTab(key: string) {
    switch (key) {
      case "home":
        go({ name: "home", view: "world" });
        break;
      case "projects":
        go({ name: "projects" });
        break;
      case "me":
        // 🚨 Always her site, inside the shell. The tab's own subtitle says
        // 对外的那一页, and a tab that sometimes opens a page and sometimes
        // opens a builder makes the student learn a rule instead of a place.
        // `MyPage` carries its own empty state; `/eco/p/:handle` is the same
        // page with her navigation taken away, for a visitor.
        go({ name: "homepage" });
        break;
    }
  }

  return (
    <div className="flex h-full w-full overflow-hidden bg-mk-paper text-mk-ink">
      <nav
        className="group/rail relative z-40 flex w-[68px] shrink-0 flex-col gap-1 overflow-hidden p-3
                   transition-[width] duration-200 ease-mk hover:w-[212px] focus-within:w-[212px]"
        style={{ background: "#1A1512", borderRight: "1px solid rgba(240,233,224,.1)" }}
        aria-label="主导航"
      >
        <div className="mb-4 flex items-center gap-3 px-1">
          <span
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-mk-full"
            style={{ background: "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))" }}
          >
            <span className="text-[13px] font-bold text-white">印</span>
          </span>
          <span className={cx(LABEL, "font-semibold text-[#F0E9E0]")}>思维印记</span>
        </div>

        {TABS.map(({ key, label, sub, icon: Ico }) => {
          const active = activeTab === key;
          return (
            <button
              key={key}
              type="button"
              onClick={() => goTab(key)}
              aria-current={active ? "page" : undefined}
              className={cx(
                "relative flex items-center gap-3 rounded-mk-md px-2 py-2.5 text-left transition-colors",
                "duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]",
                active ? "bg-[rgba(240,233,224,.1)]" : "hover:bg-[rgba(240,233,224,.06)]",
              )}
            >
              {active ? (
                <span
                  className="absolute left-0 top-1/2 h-5 w-[3px] -translate-y-1/2 rounded-r-mk-full"
                  style={{ background: "var(--mk-accent-400)" }}
                />
              ) : null}
              <span className="flex h-6 w-6 shrink-0 items-center justify-center">
                <Ico size={20} strokeWidth={1.7} color={active ? "#F5EFE7" : "#9A8E80"} />
              </span>
              <span className={cx(LABEL, "min-w-0")}>
                <span
                  className={cx(
                    "block truncate text-mk-body",
                    active ? "font-semibold text-[#F5EFE7]" : "text-[#C0B4A6]",
                  )}
                >
                  {label}
                </span>
                <span className="block truncate text-[11px] text-[#7C7166]">{sub}</span>
              </span>
            </button>
          );
        })}

        <button
          type="button"
          onClick={() => openCoach(surfaceFor(route))}
          className="mt-auto flex items-center gap-3 rounded-mk-md px-2 py-2.5 text-left
                     transition-colors duration-[120ms] ease-mk hover:bg-[rgba(240,233,224,.06)]
                     focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
        >
          <span
            className="flex h-6 w-6 shrink-0 items-center justify-center rounded-mk-full"
            style={{ background: "rgba(240,233,224,.1)" }}
          >
            <MessageCircle size={15} strokeWidth={1.8} color="#E5DACB" />
          </span>
          <span className={cx(LABEL, "min-w-0")}>
            <span className="block truncate text-mk-body text-[#C0B4A6]">印记</span>
            <span className="block truncate text-[11px] text-[#7C7166]">随时可以问</span>
          </span>
        </button>
      </nav>

      <main className={cx("relative min-w-0 flex-1 overflow-y-auto", dark ? "" : "eco-paper")}>
        <Surface route={route} />
      </main>

      <CoachDrawer />
      {state.toast ? (
        <div className="pointer-events-none fixed bottom-6 left-1/2 z-[60] -translate-x-1/2">
          <div
            className="eco-toast rounded-mk-full px-5 py-3 text-mk-body text-white shadow-mk-lg"
            style={{ background: "#241D18" }}
          >
            {state.toast}
          </div>
        </div>
      ) : null}
    </div>
  );
}

function Surface({ route }: { route: EcoRoute }) {
  switch (route.name) {
    case "home":
      return route.view === "world" ? <WorldView /> : <TreeView />;
    case "projects":
      return <ProjectsHub />;
    case "project-new":
      return <NewProject />;
    case "project":
      return <Workbench id={route.id} cardId={route.cardId} />;
    case "homepage":
      return <MyPage />;
    case "page":
      return <PersonalPage handle={route.handle} />;
  }
}

function surfaceFor(route: EcoRoute) {
  switch (route.name) {
    case "home":
      return route.view === "world" ? ("world" as const) : ("tree" as const);
    case "homepage":
      return "homepage" as const;
    case "project":
      return route.cardId ? ("step" as const) : ("projects" as const);
    default:
      return "projects" as const;
  }
}

/** Labels stay mounted and fade, so the rail's fold is a pure CSS transition
 *  with no JS state to keep in sync — same trick as lite's `Nav`. */
const LABEL =
  "whitespace-nowrap opacity-0 transition-opacity duration-200 ease-mk " +
  "group-hover/rail:opacity-100 group-focus-within/rail:opacity-100 motion-reduce:transition-none";

export { ecoPath };
