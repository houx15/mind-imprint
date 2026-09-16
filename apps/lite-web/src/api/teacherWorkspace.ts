import { apiFetch } from "./client";
import type { Choice, Turn } from "../teacher/workspace/workspaceLogic";

// api/teacherWorkspace.ts — client for the teacher workspace's one turn.
// Shape read from apps/api/internal/api/lite_teacher_workspace.go
// (liteWorkspaceTurnRequest / liteWorkspaceTurnDTO), not from the design doc:
// that file is the truth for this endpoint (task-5-brief.md's own words).
//
// `Turn`/`Choice` are reused from workspaceLogic.ts rather than redeclared —
// their `role`/`text` and `id`/`label` fields already match the wire shape
// the Go side sends and reads (liteworkspace.Turn / liteworkspace.Choice).

/**
 * Only "assignment" exists today (apps/api rejects any other value with
 * 「该工作台尚未开放：{surface}」). Typed as its own union — not a bare string —
 * so the two surfaces D2/D3 add later are a one-line addition here, not a
 * call-site rewrite.
 */
export type WorkspaceSurface = "assignment";

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
}

interface WorkspaceTurnDTO {
  reply?: string;
  choices?: Choice[];
  patch?: Record<string, unknown>;
  cards?: WorkspaceCard[];
}

export async function postWorkspaceTurn(input: WorkspaceTurnInput): Promise<WorkspaceTurn> {
  const raw = await apiFetch<WorkspaceTurnDTO>("/api/v1/lite/teacher/workspace/turn", {
    method: "POST",
    body: JSON.stringify(input),
  });
  return {
    reply: raw.reply ?? "",
    choices: raw.choices ?? [],
    patch: raw.patch ?? {},
    cards: raw.cards ?? [],
  };
}
