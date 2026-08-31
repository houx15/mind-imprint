/**
 * eco/route — the prototype's URL model.
 *
 * Same shape as lite's own `routing.ts` (root-relative paths, History API, no
 * router library) but rooted at `/eco` so the prototype cannot collide with
 * the real `/readings` and `/writings` routes. Everything the prototype owns
 * lives under that prefix, which is also why mounting it is a one-line change
 * in `rootElementFor.tsx`.
 *
 * `/eco/p/:handle` is the odd one out: a published personal page is meant to
 * be opened by someone with no account (a friend, a parent), so it renders
 * WITHOUT the app shell — see `EcoRoot`.
 */

export const ECO_PREFIX = "/eco";

export type EcoRoute =
  | { name: "home"; view: "world" | "tree" }
  | { name: "readings"; id?: string }
  | { name: "writings"; id?: string }
  | { name: "projects" }
  | { name: "project-new" }
  | { name: "project"; id: string; cardId?: string }
  | { name: "homepage" }
  | { name: "page"; handle: string };

export function isEcoPath(pathname: string): boolean {
  return pathname === ECO_PREFIX || pathname.startsWith(`${ECO_PREFIX}/`);
}

export function parseEcoRoute(pathname: string): EcoRoute {
  const rest = pathname
    .replace(/\/index\.html$/i, "/")
    .slice(ECO_PREFIX.length)
    .replace(/^\/+/, "")
    .replace(/\/+$/, "");
  const seg = rest.length === 0 ? [] : rest.split("/").map(decode);
  const [a, b, c, d] = seg;

  switch (a) {
    case undefined:
    case "":
    case "world":
      return { name: "home", view: "world" };
    case "tree":
    case "me":
      return { name: "home", view: "tree" };
    case "readings":
      return b ? { name: "readings", id: b } : { name: "readings" };
    case "writings":
      return b ? { name: "writings", id: b } : { name: "writings" };
    case "projects":
      if (!b) return { name: "projects" };
      if (b === "new") return { name: "project-new" };
      // `/eco/projects/:id/card/:cardId`
      return c === "card" && d ? { name: "project", id: b, cardId: d } : { name: "project", id: b };
    case "homepage":
      return { name: "homepage" };
    case "p":
      return b ? { name: "page", handle: b } : { name: "home", view: "world" };
    default:
      return { name: "home", view: "world" };
  }
}

export function ecoPath(route: EcoRoute): string {
  switch (route.name) {
    case "home":
      return route.view === "tree" ? `${ECO_PREFIX}/tree` : `${ECO_PREFIX}/world`;
    case "readings":
      return route.id ? `${ECO_PREFIX}/readings/${enc(route.id)}` : `${ECO_PREFIX}/readings`;
    case "writings":
      return route.id ? `${ECO_PREFIX}/writings/${enc(route.id)}` : `${ECO_PREFIX}/writings`;
    case "projects":
      return `${ECO_PREFIX}/projects`;
    case "project-new":
      return `${ECO_PREFIX}/projects/new`;
    case "project":
      return route.cardId
        ? `${ECO_PREFIX}/projects/${enc(route.id)}/card/${enc(route.cardId)}`
        : `${ECO_PREFIX}/projects/${enc(route.id)}`;
    case "homepage":
      return `${ECO_PREFIX}/homepage`;
    case "page":
      return `${ECO_PREFIX}/p/${enc(route.handle)}`;
  }
}

/** Push a path and tell listeners. Mirrors lite's `navigate`. */
export function go(route: EcoRoute): void {
  const path = ecoPath(route);
  if (window.location.pathname === path) return;
  window.history.pushState(null, "", path);
  window.dispatchEvent(new PopStateEvent("popstate"));
}

function enc(v: string) {
  return encodeURIComponent(v);
}
function decode(v: string) {
  try {
    return decodeURIComponent(v);
  } catch {
    return v;
  }
}
