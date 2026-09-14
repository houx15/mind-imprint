// The lite teacher shell's URL model. Same approach as ../routing.ts: no
// router library, root-relative paths, History API via `navigate`.
export type TeacherRoute =
  | { view: "classes" }
  | { view: "class"; classId: string }
  | { view: "student"; classId: string; userId: string }
  | { view: "item"; classId: string; userId: string; atomId: string }
  | { view: "settings" };

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
  if (seg[0] !== "classes" || !seg[1]) return { view: "classes" };
  const classId = seg[1];
  if (seg[2] !== "students" || !seg[3]) return { view: "class", classId };
  const userId = seg[3];
  if (seg[4] !== "items" || !seg[5]) return { view: "student", classId, userId };
  return { view: "item", classId, userId, atomId: seg[5] };
}

export function teacherRoutePath(r: TeacherRoute): string {
  switch (r.view) {
    case "classes":
      return "/classes";
    case "settings":
      return "/settings";
    case "class":
      return `/classes/${enc(r.classId)}`;
    case "student":
      return `/classes/${enc(r.classId)}/students/${enc(r.userId)}`;
    case "item":
      return `/classes/${enc(r.classId)}/students/${enc(r.userId)}/items/${enc(r.atomId)}`;
  }
}
