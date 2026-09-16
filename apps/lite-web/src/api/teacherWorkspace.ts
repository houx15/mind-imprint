import { apiFetch } from "./client";
import type { Choice, ChoiceArticle, Turn } from "../teacher/workspace/workspaceLogic";

// api/teacherWorkspace.ts — client for the teacher workspace's one turn.
// Shape read from apps/api/internal/api/lite_teacher_workspace.go
// (liteWorkspaceTurnRequest / liteWorkspaceTurnDTO), not from the design doc:
// that file is the truth for this endpoint (task-5-brief.md's own words).
//
// `Turn`/`Choice` are reused from workspaceLogic.ts rather than redeclared —
// their `role`/`text` and `id`/`label` fields already match the wire shape
// the Go side sends and reads (liteworkspace.Turn / liteworkspace.Choice).

/**
 * "assignment" (布置作业) and "home" (the class conversation, §12.5). apps/api
 * rejects any other value with 「该工作台尚未开放：{surface}」.
 */
export type WorkspaceSurface = "assignment" | "home";

export interface WorkspaceTurnInput {
  surface: WorkspaceSurface;
  classId: string;
  /** The canvas's current draft, exactly as the caller holds it — the model
   *  reads it as "the card's current content" (§ liteWorkspaceCardState). */
  artifact: unknown;
  /** Already windowed: callers must pass `trimTurns(turns)`, not the full
   *  history — the server also bounds this, but sending less is the point. */
  turns: Turn[];
  /** Exactly one of `text`/`choiceId` is set per call — what she typed, or
   *  the id of the choice she tapped. */
  text?: string;
  choiceId?: string;
  /** The `slug` the tapped choice carried, if it carried one. Sent alongside
   *  `choiceId`: the server keeps no state between turns, so an option that
   *  means "use this article" has to carry the article back itself. Without
   *  it the model searches for its own choice id and finds nothing. */
  choiceSlug?: string;
}

/** One tool result the canvas renders directly (§ liteWorkspaceCardDTO).
 *  `kind` is "students" today; `rows` is that tool's own shape, opaque here. */
export interface WorkspaceCard {
  kind: string;
  rows: unknown;
}

export interface WorkspaceTurn {
  reply: string;
  /** 0–4 entries, already clamped server-side. */
  choices: Choice[];
  /** Only the draft fields a tool actually wrote this turn — never null,
   *  an absent key means "untouched", never a stored zero value. */
  patch: Record<string, unknown>;
  /** Never null — an empty turn's tools produced [] on the wire. */
  cards: WorkspaceCard[];
  /** The page `open_page` offered this turn (home surface only), exactly as
   *  sent. Unvalidated here: `navigateRoute` decides whether it becomes a
   *  button. Absent on every other turn. */
  navigate?: WorkspaceNavigate;
}

/** liteWorkspaceNavigateDTO. `label` is already Chinese. */
export interface WorkspaceNavigate {
  view: string;
  classId: string;
  userId?: string;
  assignmentId?: string;
  label: string;
}

/** The wire shape of one choice, before `normalizeChoice` — `article`, when
 *  present, is `liteworkspace.ChoiceArticle` exactly, including its own
 *  `,omitempty` on `coverUrl` (so a signing failure is a missing key, not an
 *  empty string, though `normalizeChoice` treats both the same). */
interface ChoiceDTO {
  id?: string;
  label?: string;
  slug?: string;
  article?: {
    slug?: string;
    zhTitle?: string;
    coverUrl?: string;
    reason?: string;
  };
}

interface WorkspaceTurnDTO {
  reply?: string;
  choices?: ChoiceDTO[];
  patch?: Record<string, unknown>;
  cards?: WorkspaceCard[];
  navigate?: Partial<WorkspaceNavigate> | null;
}

/** Turns one wire choice into `Choice`: a missing `article` stays absent
 *  (never `{}`), and an unsignable cover (`coverUrl` empty or missing) is
 *  normalized to absent too — the one place both callers of `article.coverUrl`
 *  need to check is "is it there", not "is it there and non-empty". */
export function normalizeChoiceArticle(raw: ChoiceDTO["article"]): ChoiceArticle | undefined {
  if (!raw) return undefined;
  return {
    slug: raw.slug ?? "",
    zhTitle: raw.zhTitle ?? "",
    reason: raw.reason ?? "",
    coverUrl: raw.coverUrl || undefined,
  };
}

export function normalizeChoice(raw: ChoiceDTO): Choice {
  return {
    id: raw.id ?? "",
    label: raw.label ?? "",
    slug: raw.slug || undefined,
    article: normalizeChoiceArticle(raw.article),
  };
}

export async function postWorkspaceTurn(input: WorkspaceTurnInput): Promise<WorkspaceTurn> {
  const raw = await apiFetch<WorkspaceTurnDTO>("/api/v1/lite/teacher/workspace/turn", {
    method: "POST",
    body: JSON.stringify(input),
  });
  return {
    reply: raw.reply ?? "",
    choices: (raw.choices ?? []).map(normalizeChoice),
    patch: raw.patch ?? {},
    cards: raw.cards ?? [],
    ...(raw.navigate ? { navigate: normalizeNavigate(raw.navigate) } : {}),
  };
}

/** Missing string fields become "" / absent; `navigateRoute` drops what is
 *  unusable. */
export function normalizeNavigate(raw: Partial<WorkspaceNavigate>): WorkspaceNavigate {
  const str = (v: unknown) => (typeof v === "string" ? v : "");
  const userId = str(raw.userId);
  const assignmentId = str(raw.assignmentId);
  return {
    view: str(raw.view),
    classId: str(raw.classId),
    label: str(raw.label),
    ...(userId ? { userId } : {}),
    ...(assignmentId ? { assignmentId } : {}),
  };
}
