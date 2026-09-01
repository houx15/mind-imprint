import type { ProjectKind } from "../api/projects";

/**
 * Project covers — a ground and a glyph, two taps.
 *
 * Lifted from the prototype (`eco/data/projects.ts`) with one deliberate
 * change: the prototype stored the ground as an INDEX into its array, and an
 * index in a database column is a promise never to reorder the array. These
 * carry stable string ids instead, so `cover_ground` means the same thing next
 * year as it does today.
 *
 * Why a gradient and a glyph rather than an image upload: every project gets a
 * cover that looks deliberate, with no student stuck at "find a picture" and no
 * board where one card is a blurry photo taken at an angle.
 */
export interface CoverGround {
  id: string;
  from: string;
  to: string;
  ink: string;
}

export const COVER_GROUNDS: CoverGround[] = [
  { id: "coral", from: "#E8695E", to: "#B8412F", ink: "#FFF6F2" },
  { id: "matcha", from: "#5FA97E", to: "#2F6B4A", ink: "#F2FBF5" },
  { id: "lake", from: "#4E7EA6", to: "#26496B", ink: "#F0F7FD" },
  { id: "amber", from: "#E0A63A", to: "#A9701A", ink: "#FFFAF0" },
  { id: "taro", from: "#9B7BC4", to: "#5E4189", ink: "#F9F5FF" },
  { id: "teal", from: "#3F8E8A", to: "#1D5754", ink: "#EFFBFA" },
  { id: "berry", from: "#C4657F", to: "#87334A", ink: "#FFF3F6" },
  { id: "slate", from: "#5A5F73", to: "#2B2F3D", ink: "#F4F5F8" },
];

export const COVER_GLYPHS = [
  "◈", "◉", "✳", "◇", "❋", "⌘", "☺", "▲",
  "✧", "◐", "⬡", "✦", "❖", "◎", "✿", "⌂",
];

/**
 * Resolve a stored ground id to something renderable.
 *
 * An unknown id falls back to the first ground rather than rendering nothing.
 * Same reasoning as `groupByStatus` dropping an unknown status: the server does
 * not police this vocabulary, so the client must never be blanked by a value it
 * has not heard of.
 */
export function groundById(id: string): CoverGround {
  return COVER_GROUNDS.find((g) => g.id === id) ?? COVER_GROUNDS[0]!;
}

/**
 * The one place a stored cover becomes a rendered cover.
 *
 * 🚨 Both the kanban card and the modal's preview go through here. They used to
 * fall back separately — the card to `COVER_GROUNDS[0]`, the modal to
 * `defaultCover(kind)` — so a project with no cover yet showed as coral on the
 * board and blue in the modal, and appeared to change colour the moment she
 * saved. Neither fallback was wrong; having two was.
 */
export function resolveCover(
  kind: ProjectKind,
  ground: string,
  glyph: string,
): { ground: CoverGround; glyph: string } {
  const fallback = defaultCover(kind);
  return {
    ground: groundById(ground.trim() || fallback.ground),
    glyph: glyph.trim() || fallback.glyph,
  };
}

/** Each kind opens on a different ground, so a board of several projects does
 *  not arrive as eight identical squares. She overrides it in one tap; the
 *  point is that choosing is easier against something than against nothing. */
const KIND_GROUND: Record<ProjectKind, string> = {
  website: "lake",
  research: "taro",
  design: "coral",
  making: "amber",
  investigation: "matcha",
};

const KIND_GLYPH: Record<ProjectKind, string> = {
  website: "⌂",
  research: "◉",
  design: "❋",
  making: "▲",
  investigation: "◐",
};

export function defaultCover(kind: ProjectKind): { ground: string; glyph: string } {
  return { ground: KIND_GROUND[kind], glyph: KIND_GLYPH[kind] };
}
