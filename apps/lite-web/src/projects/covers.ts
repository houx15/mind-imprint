
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
  kind: string,
  ground: string,
  glyph: string,
): { ground: CoverGround; glyph: string } {
  const fallback = defaultCover(kind);
  return {
    ground: groundById(ground.trim() || fallback.ground),
    glyph: glyph.trim() || fallback.glyph,
  };
}

/**
 * 不同类别开在不同底色上，一板项目才不会是八个一模一样的方块。她一下就能改；
 * 有个起点，只是让"选一个"比对着空白容易。
 *
 * 类别现在是自由字符串（迁移 0112），所以这里认识的就给它的底色，不认识的
 * 按名字稳定地散开——同一个名字每次都得到同一个底色，不会刷新一次换一种。
 */
const KIND_COVER: Record<string, { ground: string; glyph: string }> = {
  网站搭建: { ground: "lake", glyph: "⌂" },
  内容设计: { ground: "coral", glyph: "❋" },
  田野调查: { ground: "matcha", glyph: "◐" },
  数据分析: { ground: "taro", glyph: "◉" },
  产品原型: { ground: "amber", glyph: "▲" },
  活动策划: { ground: "coral", glyph: "✦" },
  研究写作: { ground: "taro", glyph: "◉" },
  // 分类器时代的英文值，照旧认得。
  website: { ground: "lake", glyph: "⌂" },
  research: { ground: "taro", glyph: "◉" },
  design: { ground: "coral", glyph: "❋" },
  making: { ground: "amber", glyph: "▲" },
  investigation: { ground: "matcha", glyph: "◐" },
};

export function defaultCover(kind: string): { ground: string; glyph: string } {
  const known = KIND_COVER[kind.trim()];
  if (known) return known;
  // 还没定类别，或者是一个我们没见过的名字：按名字取一个稳定的底色。
  let h = 0;
  for (const ch of kind) h = (h * 31 + ch.codePointAt(0)!) % 100000;
  const ground = COVER_GROUNDS[h % COVER_GROUNDS.length]!;
  return { ground: ground.id, glyph: "◇" };
}
