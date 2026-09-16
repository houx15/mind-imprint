import type { WorkspaceCard, WorkspaceNavigate } from "../../api/teacherWorkspace";
import { STATUS_LABEL, type AssignmentStatus } from "../../shared/deadline";
import { STATUS_ORDER } from "../assignmentLogic";
import { KIND_OPTIONS } from "../labels";
import type { TeacherRoute } from "../teacherRouting";

// teacher/workspace/homeLogic.ts — pure logic for the class conversation
// (surface "home", §12.5): the page `open_page` offered, and the tool result
// cards narrowed from their opaque wire rows. No React, no network.

/** A page offer that passed `navigateRoute`: where the button goes, and what
 *  it reads (「前往：{label}」). */
export interface NavigateTarget {
  route: TeacherRoute;
  label: string;
  classId: string;
}

/**
 * `navigate` → the route its button opens, or null (no button).
 *
 * Dropped: an unknown view, an empty label, a class other than the page's
 * own, and `student`/`assignment` without the id they need. The server
 * already validates all of this; this is the client not trusting a shape it
 * did not build.
 */
export function navigateRoute(nav: WorkspaceNavigate | undefined, pageClassId: string): NavigateTarget | null {
  if (!nav || !nav.label.trim() || nav.classId !== pageClassId || !pageClassId) return null;
  const classId = pageClassId;
  let route: TeacherRoute | null = null;
  switch (nav.view) {
    case "classWeekly":
      route = { view: "classWeekly", classId };
      break;
    case "student":
      route = nav.userId ? { view: "student", classId, userId: nav.userId } : null;
      break;
    case "assignmentNew":
      route = { view: "assignmentNew", classId };
      break;
    case "assignment":
      route = nav.assignmentId ? { view: "assignment", assignmentId: nav.assignmentId } : null;
      break;
    case "parentReports":
      route = { view: "parentReports" };
      break;
  }
  return route ? { route, label: nav.label, classId } : null;
}

/**
 * The thread keeps a turn's `cards` and drops everything else it does not
 * know about, so the page offer travels as one more card of a kind the
 * server never sends. It is replaced on the next successful turn and cleared
 * on reset, exactly like the tool cards.
 */
export const NAVIGATE_CARD_KIND = "client:navigate";

export function withNavigateCard(cards: WorkspaceCard[], nav: WorkspaceNavigate | undefined): WorkspaceCard[] {
  return nav ? [...cards, { kind: NAVIGATE_CARD_KIND, rows: nav }] : cards;
}

/** The page offer carried by `withNavigateCard`, if any. */
export function navigateOf(cards: WorkspaceCard[]): WorkspaceNavigate | undefined {
  const card = cards.find((c) => c.kind === NAVIGATE_CARD_KIND);
  return card ? (card.rows as WorkspaceNavigate) : undefined;
}

type Raw = Record<string, unknown>;
const isObj = (v: unknown): v is Raw => !!v && typeof v === "object" && !Array.isArray(v);
const str = (v: unknown) => (typeof v === "string" ? v : "");

export interface StudentCardRow {
  id: string;
  name: string;
}

/** A "students" card's rows (`liteworkspace.Student`), malformed rows skipped. */
export function studentRows(raw: unknown): StudentCardRow[] {
  if (!Array.isArray(raw)) return [];
  const out: StudentCardRow[] = [];
  for (const r of raw) {
    if (!isObj(r)) continue;
    if (typeof r.id === "string" && typeof r.name === "string") out.push({ id: r.id, name: r.name });
  }
  return out;
}

export interface SnapshotStudent {
  userId: string;
  name: string;
  kind: string;
  code: string;
  label: string;
  evidence: string;
}

export interface ClassSnapshotView {
  classSize: number;
  activeStudents: number;
  finished: number;
  /** Integer percent; -1 when nothing was due. */
  assignmentRate: number;
  praise: SnapshotStudent[];
  watch: SnapshotStudent[];
}

function snapshotStudents(raw: unknown): SnapshotStudent[] {
  if (!Array.isArray(raw)) return [];
  return raw.filter(isObj).flatMap((r) => {
    const name = str(r.name);
    if (!name) return [];
    return [{ userId: str(r.userId), name, kind: str(r.kind), code: str(r.code), label: str(r.label), evidence: str(r.evidence) }];
  });
}

/** A "classSnapshot" card's rows — a single object, not an array. null when
 *  it is not an object or a count is missing: a card of zeros would be a
 *  statement the server never made. */
export function classSnapshotView(raw: unknown): ClassSnapshotView | null {
  if (!isObj(raw)) return null;
  const nums = [raw.classSize, raw.activeStudents, raw.finished, raw.assignmentRate];
  if (!nums.every((v) => typeof v === "number" && Number.isFinite(v))) return null;
  return {
    classSize: raw.classSize as number,
    activeStudents: raw.activeStudents as number,
    finished: raw.finished as number,
    assignmentRate: raw.assignmentRate as number,
    praise: snapshotStudents(raw.praise),
    watch: snapshotStudents(raw.watch),
  };
}

export interface AssignmentCardRow {
  id: string;
  title: string;
  /** "" when the kind is not one the teacher UI knows — never the raw key. */
  kindLabel: string;
  dueAt: string;
  /** Non-zero counts of known statuses, in `STATUS_ORDER`. */
  counts: { status: AssignmentStatus; label: string; n: number }[];
}

/** An "assignments" card's rows (raw `AssignmentSummaryDTO`s). Kind and
 *  status keys are mapped to the labels the rest of the teacher UI uses; an
 *  unknown key is left out rather than shown raw. */
export function assignmentCardRows(raw: unknown): AssignmentCardRow[] {
  if (!Array.isArray(raw)) return [];
  const out: AssignmentCardRow[] = [];
  for (const r of raw) {
    if (!isObj(r)) continue;
    const id = str(r.id);
    const title = str(r.title);
    if (!id || !title) continue;
    const counts = isObj(r.counts) ? r.counts : {};
    out.push({
      id,
      title,
      kindLabel: KIND_OPTIONS.find((o) => o.value === r.kind)?.label ?? "",
      dueAt: str(r.dueAt),
      counts: STATUS_ORDER.flatMap((status) => {
        const n = counts[status];
        return typeof n === "number" && n > 0 ? [{ status, label: STATUS_LABEL[status], n }] : [];
      }),
    });
  }
  return out;
}
