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
  title: Bi;
  summary: Bi;
  /** The hook. A question, never a summary — it is what the planet whispers. */
  hook: Bi;
  source: string;
  /** 0..1, shown as a readout. Editorial weight, not engagement. */
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

export type StepKind = "learn" | "research" | "design" | "make" | "document" | "test" | "publish";

export interface ProjectStep {
  id: string;
  kind: StepKind;
  /** Which instrument the step's workspace opens. Defaults to `kind`, but a
   *  step's TYPE and its TOOL are not the same thing: 设计问卷 is a `design`
   *  step whose tool is the questionnaire builder, while 去发出去收回 30 份 is a
   *  `research` step whose tool is a working log. */
  tool?: StepKind | "survey";
  title: string;
  /** Why this step exists — the plan explains itself, it is not a checklist. */
  why: string;
  minutes: number;
  done: boolean;
}

export interface Project {
  id: string;
  track: TrackId;
  title: string;
  /** The 动机卡 — pinned to the header, resurfaced later. */
  motivation?: { who: string; cost: string; mine: string };
  steps: ProjectStep[];
  status: "draft" | "running" | "published";
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

export type WorkMode = "ask" | "propose" | "tidy";

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
