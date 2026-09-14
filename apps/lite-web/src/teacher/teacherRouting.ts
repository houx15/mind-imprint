// The lite teacher shell's URL model. Same approach as ../routing.ts: no
// router library, root-relative paths, History API via `navigate`.
export type TeacherRoute =
  | { view: "classes" }
  | { view: "class"; classId: string }
  | { view: "student"; classId: string; userId: string }
  | { view: "item"; classId: string; userId: string; atomId: string }
  | { view: "settings" }
  | { view: "overview" }
  | { view: "teachers" }
  | { view: "import" };

const dec = (s: string) => {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
};
const enc = encodeURIComponent;

export function parseTeacherRoute(pathname: string): TeacherRoute {
  const seg = pathname.replace(/^\/+|\/+$/g, "").split("/").filter(Boolean).map(dec);
  if (seg[0] === "settings") return { view: "settings" };
  if (seg[0] === "overview") return { view: "overview" };
  if (seg[0] === "teachers") return { view: "teachers" };
  if (seg[0] === "import") return { view: "import" };
  if (seg[0] !== "classes" || !seg[1]) return { view: "classes" };
  const classId = seg[1];
  if (seg[2] !== "students" || !seg[3]) return { view: "class", classId };
  const userId = seg[3];
  if (seg[4] !== "items" || !seg[5]) return { view: "student", classId, userId };
  return { view: "item", classId, userId, atomId: seg[5] };
}

/** Whether `pathname`'s first segment is one this router actually
 * recognises. `parseTeacherRoute` folds every UNrecognised path (`/`,
 * `/readings/abc`, a stray typo) onto `{view:"classes"}` as a safe
 * catch-all — which is right for a mid-session Back/Forward, but wrong for
 * "what should she land on right now": that has to fall through to
 * `landingRoute` instead of pretending she asked for 班级. This is the seam
 * `resolveTeacherRoute` uses to tell "she asked for /classes" apart from
 * "we had nothing better to do with this path". */
export function isTeacherPath(pathname: string): boolean {
  const seg = pathname.replace(/^\/+|\/+$/g, "").split("/").filter(Boolean);
  const first = seg[0];
  return first === "classes" || first === "settings" || first === "overview" || first === "teachers" || first === "import";
}

/** The role-appropriate landing view — same split as pro `ConsoleShell`
 * (`role === "admin" ? "overview" : "classes"`). */
export function landingRoute(role: string): TeacherRoute {
  return role === "admin" ? { view: "overview" } : { view: "classes" };
}

/** The route she should actually be on right now, given the URL and her
 * role. Three cases, in order:
 *
 *  1. `pathname` isn't a recognised teacher path at all (root, a student
 *     path left behind by a same-tab role switch, a stray typo) →
 *     `landingRoute(role)`. Not `parseTeacherRoute`'s fallback: that would
 *     silently put an admin on 班级 instead of her actual landing tab.
 *  2. It parses to an admin-only view (`overview`/`teachers`/`import`) and
 *     she isn't an admin → `{view:"classes"}`. A teacher who typed or
 *     bookmarked one of these has no data to see there.
 *  3. Otherwise, the parsed route stands as-is.
 *
 * Pure — callers (`LiteTeacherShell`) are responsible for reconciling
 * `window.location` with `teacherRoutePath` of whatever this returns, via
 * `history.replaceState` (never `pushState`/`navigate`), so a
 * resolver-driven correction never adds a Back-button trap. */
export function resolveTeacherRoute(pathname: string, role: string): TeacherRoute {
  if (!isTeacherPath(pathname)) return landingRoute(role);
  const parsed = parseTeacherRoute(pathname);
  const adminOnly = parsed.view === "overview" || parsed.view === "teachers" || parsed.view === "import";
  if (adminOnly && role !== "admin") return { view: "classes" };
  return parsed;
}

export function teacherRoutePath(r: TeacherRoute): string {
  switch (r.view) {
    case "classes":
      return "/classes";
    case "settings":
      return "/settings";
    case "overview":
      return "/overview";
    case "teachers":
      return "/teachers";
    case "import":
      return "/import";
    case "class":
      return `/classes/${enc(r.classId)}`;
    case "student":
      return `/classes/${enc(r.classId)}/students/${enc(r.userId)}`;
    case "item":
      return `/classes/${enc(r.classId)}/students/${enc(r.userId)}/items/${enc(r.atomId)}`;
  }
}
