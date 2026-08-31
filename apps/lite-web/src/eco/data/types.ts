/**
 * eco/data — the prototype's whole world, as plain data.
 *
 * Nothing here talks to the API. The point of this prototype is to settle the
 * SHAPE of the ecosystem (world → tree → project → page) before any of it is
 * built for real, so every screen reads from these modules and every mutation
 * lands in `eco/store.tsx`'s in-memory state.
 *
 * One honesty rule this file exists to hold: the news items are INVENTED for
 * the prototype. They are plausible, they are dated, they carry sources — and
 * they are not real reporting. The world view therefore renders a permanent
 * 「原型数据」chip; see `world/WorldView.tsx`. Never remove it while the data
 * is fake.
 */

export type Lang = "zh" | "en";

/** A bilingual string. Every visible sentence in the world view has both. */
export interface Bi {
  zh: string;
  en: string;
}

/** News domains. 政治/冲突 is deliberately absent — it is filtered upstream,
 *  and the world view says so out loud rather than hiding the omission. */
export type Domain =
  | "tech"
  | "science"
  | "environment"
  | "space"
  | "health"
  | "culture"
  | "economy"
  | "education";

export interface NewsItem {
  id: string;
  /** ISO date, `YYYY-MM-DD`. Five items exist per date. */
  date: string;
  /** 1..5 — 1 is the most important of that day. Drives planet size + orbit. */
  rank: number;
  domain: Domain;
  /** 导读 — our own one-line orientation, in the second person: what to watch
   *  for while reading. It is the FIRST thing on the card, above the news's
   *  own headline, because a 13-year-old needs a reason to look before she
   *  needs a fact. */
  lead: Bi;
  title: Bi;
  summary: Bi;
  /** The hook. A question, never a summary — it is what the planet whispers. */
  hook: Bi;
  /** Two more questions, opened after the reading. `hook` is the one that
   *  pulls her in; these two are the ones that survive the article. */
  hooks: Bi[];
  source: string;
  /** 0..1. Editorial rank weight — used for ordering, never shown: a raw
   *  0.85 on the card told a student nothing she could act on. */
  weight: number;
  /** Keywords this item would grow on her tree if she keeps it. */
  keywords: string[];
}

/** The six main fields = the tree's main branches. */
export type FieldId =
  | "humanities"
  | "science"
  | "society"
  | "making"
  | "arts"
  | "self";

export interface Field {
  id: FieldId;
  label: string;
  en: string;
  /** A macaron token name (`peach` | `matcha` | …) — see index.css `:root`. */
  hue: string;
  /** Where the branch leaves the trunk, in the tree SVG's own coordinates. */
  angle: number;
}

export type SourceKind = "reading" | "writing" | "project" | "news" | "course";

export interface KeywordSource {
  kind: SourceKind;
  id: string;
  label: string;
  /** Her own words, or the sentence that made this keyword appear. */
  evidence?: string;
  date: string;
}

export interface Keyword {
  id: string;
  text: string;
  en: string;
  field: FieldId;
  /** 1..5. Node size + the 强度 readout. */
  strength: number;
  /** Which growth stop it first appeared at (index into GROWTH_STOPS). */
  bornAt: number;
  /** 印记's one-line read of what this keyword is for her. */
  note: string;
  sources: KeywordSource[];
  /** A 高光时刻 — she did something notably well here. Gold star on the tree. */
  shining?: { title: string; body: string; date: string };
  /** Position along its branch, 0 (near trunk) .. 1 (tip), and a lateral
   *  offset so the leaves don't stack. Hand-placed; a real build would lay
   *  these out, but hand-placing is what makes the mock look designed. */
  at: { t: number; spread: number };
}

export interface Reading {
  id: string;
  title: string;
  source: string;
  date: string;
  minutes: number;
  /** Her own takeaway sentence, if she finished it. */
  takeaway?: string;
  quote?: string;
  keywords: string[];
  field: FieldId;
  /** Excerpt shown in the (static) reading view. */
  excerpt: string[];
}

export interface Writing {
  id: string;
  title: string;
  date: string;
  words: number;
  /** The structure she used — lite teaches spine names. */
  spine: string;
  keywords: string[];
  field: FieldId;
  body: string[];
}

export type TrackId = "design" | "website" | "game" | "survey" | "other";

/* ─── the card layer ──────────────────────────────────────────────────────
 *
 * A 工具卡 is "a tool in the tool-use loop that a HUMAN executes" — the core
 * mental model in AGENTS.md. 印记 decides a card is needed and summons it with
 * a stated reason; she opens it (or declines); she fills it in; the filled
 * card is fed back into the conversation. The value lives in the STRUCTURE of
 * the card, not in prose instructions, which is why a new card is a new spec
 * object and never new renderer code.
 * ------------------------------------------------------------------------ */

/** One row of a repeatable group. Keys are the group's column ids. */
export type CardRow = Record<string, string>;

export type CardValue = string | string[] | CardRow[];

export type CardFieldKind = "text" | "textarea" | "choice" | "multi" | "rows";

export interface CardColumn {
  id: string;
  label: string;
  placeholder?: string;
  /** Renders as a textarea and takes the full row width. */
  wide?: boolean;
}

export interface CardField {
  id: string;
  kind: CardFieldKind;
  label: string;
  hint?: string;
  placeholder?: string;
  /** `choice` / `multi` */
  options?: { id: string; label: string; blurb?: string }[];
  /** `rows` */
  columns?: CardColumn[];
  /** `rows`: how many filled rows before the card counts as answered. */
  min?: number;
  rowsLabel?: string;
}

export interface CardSpec {
  id: string;
  title: string;
  /** One line, in 印记's voice, saying WHY it is offering this card right now.
   *  Never summon a card without showing the reason — an unexplained card is
   *  an ambush, and this product's whole claim is that the student can see
   *  what the AI is doing. */
  reason: string;
  /** What the card teaches, in the second person. */
  teaches: string;
  /** Key into `METHODS` — the named practice this card comes from. */
  method?: string;
  glyph: string;
  hue: string;
  minutes: number;
  fields: CardField[];
}

export interface CardEntry {
  cardId: string;
  /** `invited` — 印记 offered it, she has not opened it (opening is HER call,
   *  铁律②). `open` — started. `done` — submitted and fed back. */
  status: "invited" | "open" | "done";
  values: Record<string, CardValue>;
  doneAt?: string;
}

export type ThreadItem =
  | { id: string; kind: "say"; role: "coach" | "student"; text: string }
  /** 印记 summoning a card into the conversation. */
  | { id: string; kind: "card"; cardId: string };

export interface Project {
  id: string;
  track: TrackId;
  title: string;
  /** The one line she wrote when she started. Kept verbatim, forever. */
  intent: string;
  /** The conversation. Cards live IN it, not beside it. */
  thread: ThreadItem[];
  cards: CardEntry[];
  status: "running" | "published";
  cover: string;
  summary?: string;
  startedAt: string;
}

export interface PageExample {
  id: string;
  name: string;
  who: string;
  url: string;
  /** What is good about it, concretely. */
  good: string[];
  /** The one structural move to steal. */
  steal: string;
  /** Palette for the thumbnail mock. */
  swatch: [string, string];
  shape: "essay" | "grid" | "terminal" | "playful" | "minimal" | "notebook";
}

export type StyleId = "morning" | "terminal" | "magazine" | "garden";

export interface HomepageSection {
  id: string;
  label: string;
  hint: string;
  /** What she wrote. Empty = not written yet. */
  value: string;
  /** Sections that carry picked items instead of prose. */
  picker?: "readings" | "writings" | "projects";
  picked?: string[];
  enabled: boolean;
}
