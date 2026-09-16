// The lite teacher shell's URL model. Same approach as ../routing.ts: no
// router library, root-relative paths, History API via `navigate`.
export type TeacherRoute =
  | { view: "classes" }
  | { view: "class"; classId: string }
  // `/classes/:classId/weekly`. `weekly` is a reserved segment after the class
  // id; the week itself is page state, not part of the URL.
  | { view: "classWeekly"; classId: string }
  // `/classes/:classId/chat` — the class conversation opened from the summary
  // on a class card. `chat` is reserved after the class id, like `weekly`.
  | { view: "classChat"; classId: string }
  | { view: "student"; classId: string; userId: string }
  | { view: "item"; classId: string; userId: string; atomId: string }
  | { view: "settings" }
  | { view: "overview" }
  | { view: "teachers" }
  | { view: "import" }
  | { view: "assignments" }
  // `classId` is never read from the URL (there is no `?class=` query): the
  // form picks the class itself. It exists only for a same-tab navigation
  // (e.g. "+ 布置作业" from inside a class) to pre-select one, so it stays
  // optional and `teacherRoutePath` ignores it.
  | { view: "assignmentNew"; classId?: string }
  // `?tab=grading` carries which tab she was on back through a round trip to
  // `/gradings/:gid` and back (返回 from the grading view lands her on 批改,
  // not the default 学生) — a query string, not a path segment, since it is
  // presentation state on top of the same underlying page, the same reason
  // the class weekly page's selected week is NOT in the URL either.
  | { view: "assignment"; assignmentId: string; tab?: "grading" }
  // `/parent-reports` (one class's reports at a time) and
  // `/parent-reports/:reportId` (the editor). Top-level rather than under a
  // class: a report stays readable and revocable after its student leaves.
  | { view: "parentReports" }
  | { view: "parentReport"; reportId: string }
  // /gradings/:gid — one AI 批改, from a homework's 批改 tab or a student's item page.
  | { view: "grading"; gradingId: string };

const dec = (s: string) => {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
};
const enc = encodeURIComponent;

export function parseTeacherRoute(pathname: string): TeacherRoute {
  // Split off a query string before segmenting — only the assignment route
  // reads one (`?tab=grading`); every other branch works on `rawPath` alone
  // exactly as before, so a stray/unknown query elsewhere is silently
  // ignored rather than corrupting the last path segment.
  const qIndex = pathname.indexOf("?");
  const rawPath = qIndex < 0 ? pathname : pathname.slice(0, qIndex);
  const rawSearch = qIndex < 0 ? "" : pathname.slice(qIndex + 1);
  const seg = rawPath.replace(/^\/+|\/+$/g, "").split("/").filter(Boolean).map(dec);
  if (seg[0] === "settings") return { view: "settings" };
  if (seg[0] === "overview") return { view: "overview" };
  if (seg[0] === "teachers") return { view: "teachers" };
  if (seg[0] === "import") return { view: "import" };
  if (seg[0] === "assignments") {
    if (!seg[1]) return { view: "assignments" };
    if (seg[1] === "new") return { view: "assignmentNew" };
    const tab = new URLSearchParams(rawSearch).get("tab");
    return tab === "grading" ? { view: "assignment", assignmentId: seg[1], tab: "grading" } : { view: "assignment", assignmentId: seg[1] };
  }
  if (seg[0] === "parent-reports") {
    if (!seg[1]) return { view: "parentReports" };
    return { view: "parentReport", reportId: seg[1] };
  }
  if (seg[0] === "gradings" && seg[1]) return { view: "grading", gradingId: seg[1] };
  if (seg[0] !== "classes" || !seg[1]) return { view: "classes" };
  const classId = seg[1];
  if (seg[2] === "weekly") return { view: "classWeekly", classId };
  if (seg[2] === "chat") return { view: "classChat", classId };
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
  return (
    first === "classes" ||
    first === "settings" ||
    first === "overview" ||
    first === "teachers" ||
    first === "import" ||
    first === "assignments" ||
    first === "parent-reports" ||
    first === "gradings"
  );
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
    case "assignments":
      return "/assignments";
    case "assignmentNew":
      return "/assignments/new";
    case "assignment":
      return r.tab === "grading" ? `/assignments/${enc(r.assignmentId)}?tab=grading` : `/assignments/${enc(r.assignmentId)}`;
    case "parentReports":
      return "/parent-reports";
    case "parentReport":
      return `/parent-reports/${enc(r.reportId)}`;
    case "grading":
      return `/gradings/${enc(r.gradingId)}`;
    case "class":
      return `/classes/${enc(r.classId)}`;
    case "classWeekly":
      return `/classes/${enc(r.classId)}/weekly`;
    case "classChat":
      return `/classes/${enc(r.classId)}/chat`;
    case "student":
      return `/classes/${enc(r.classId)}/students/${enc(r.userId)}`;
    case "item":
      return `/classes/${enc(r.classId)}/students/${enc(r.userId)}/items/${enc(r.atomId)}`;
  }
}
