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
  /** Does not block 提交. For the fields she can only fill AFTER she has been
   *  somewhere — the result of a test, what the property manager actually
   *  said, whether her three words survived looking at ten pages. Blocking
   *  submission on those would mean the card cannot come back until the whole
   *  errand is done, which defeats the point of feeding it back. */
  optional?: boolean;
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
  | { id: string; kind: "card"; cardId: string }
  /** 印记 putting something it BUILT on the table for judgement. */
  | { id: string; kind: "make"; artifactId: string }
  /** A plan step opening. Renders as a divider, not a bubble — the thread has
   *  to show where in the agreed plan the conversation currently is, or the
   *  plan stops being the thing that is actually running. */
  | { id: string; kind: "step"; stepId: string };

/* ─── the planner ─────────────────────────────────────────────────────────
 *
 * A project does not begin at the first card. It begins at a PLAN — the thing
 * Codex and Cowork put in front of you before they touch anything, and the
 * thing v2 of this prototype was missing. 印记 proposes it; she reads it,
 * changes it, times it, and approves it. Nothing runs until she has.
 *
 * The plan is also where the division of labour becomes concrete. Every step
 * names three things: what she brings, what 印记 does, and — the one that
 * matters — **what she has to decide**. A step where she decides nothing is a
 * step she should not be sitting through.
 * ------------------------------------------------------------------------ */

/** What a plan step opens on the stage when she reaches it. */
export type StageRef =
  | { kind: "card"; cardId: string }
  | { kind: "make"; artifactId: string };

export interface PlanStep {
  id: string;
  title: string;
  /** One line: what actually happens here. Editable — it is her plan. */
  blurb: string;
  /** What she has to bring. Concrete, not "参与". */
  youBring: string;
  /** What 印记 does. In a project it may genuinely build things. */
  iBring: string;
  /** The judgement that stays hers at this step. Every step has one. */
  decide: string;
  /** Her own timing, written when she reviews the plan. Empty until then —
   *  印记 deliberately does NOT pre-fill it: a schedule you did not write is
   *  a schedule you will not keep. */
  when: string;
  opens: StageRef | null;
  /** She switched it off. Kept rather than deleted — 过程即数据: declining a
   *  step is a record, not an absence. */
  off?: boolean;
  /** She wrote this step herself. */
  mine?: boolean;
}

/* ─── approaches, branches, decisions ─────────────────────────────────
 *
 * When a student arrives with a real problem, the first thing 印记 owes her is
 * not a plan — it is the news that there is more than one way to solve it.
 *
 * So it proposes two or three APPROACHES and stops. Each one is a hook: she
 * can open a BRANCH — a side conversation about that option alone — dig as far
 * as she wants, and bring back a sentence. Only then does she DECIDE, and a
 * decision here is not a click: it is a choice plus the reason plus what she
 * knowingly gave up.
 *
 * This is the mechanism that makes the whole product's claim true. 印记 can
 * build anything in this project. It cannot pick the road.
 * ------------------------------------------------------------------------ */

export interface Approach {
  id: string;
  name: string;
  /** One line naming its SHAPE, so the two options are visibly different
   *  kinds of thing rather than two flavours of one. */
  shape: string;
  how: string[];
  /** What it costs — named by 印记, unprompted. An option presented without
   *  its price is an option that has already been chosen for her. */
  costs: string[];
  /** What it needs from her specifically. */
  needs: string;
  hue: string;
  /** The plan 印记 will propose if she picks this one. Key into `PLANS`. */
  planId: string;
}

export interface BranchTurn {
  id: string;
  role: "coach" | "student";
  text: string;
}

/** A side conversation about ONE approach. */
export interface Branch {
  approachId: string;
  log: BranchTurn[];
  /** What she brings back. This is the point of the branch — without it the
   *  digging was just reading. */
  takeaway: string;
}

export interface Decision {
  approachId: string;
  /** Why. Required — the button does not enable without it. */
  why: string;
  /** What she is knowingly giving up by not taking the other road. */
  gaveUp: string;
  at: string;
}

/* ─── artifacts ────────────────────────────────────────────────────
 *
 * The other half of the loop. A 工具卡 is work SHE does; an artifact is work
 * 印记 does and hands over for judgement — three style options, a content
 * draft assembled from her own library, a built page.
 *
 * Every artifact costs her the same three things: read it, decide about it,
 * say why. `verdict` without `why` is not accepted anywhere in this file's
 * consumers, and that is the whole design.
 * ------------------------------------------------------------------------ */

export type ArtifactKind = "options" | "draft" | "build";

export interface ArtifactOption {
  id: string;
  name: string;
  tag: string;
  bullets: string[];
  /** Miniature paint — an option you cannot see is an option you cannot judge. */
  paper: string;
  ink: string;
  accent: string;
  font?: string;
}

export interface BuildRound {
  /** What changed since the last round, in 印记's words. */
  changed: string;
  headline: string;
  lines: string[];
  /** Problems 印记 names about its own work, unprompted. */
  admits: string[];
}

export interface ArtifactSpec {
  id: string;
  kind: ArtifactKind;
  title: string;
  /** 印记's framing on handover: what it did, and what it GUESSED. Naming
   *  the guesses is what makes review possible instead of polite. */
  note: string;
  /** What it needs back. Never "看看喜欢吗". */
  ask: string;
  /** The lines it prints while it works. The wait is real work, shown. */
  steps: string[];
  options?: ArtifactOption[];
  blocks?: { id: string; label: string; hint: string; text: string }[];
  rounds?: BuildRound[];
}

export interface ArtifactState {
  status: "idle" | "working" | "ready" | "settled";
  /** `options` — which one she took. */
  choice?: string;
  /** The reason. Nothing settles without it. */
  why?: string;
  /** `draft` — her edits, keyed by block id. */
  blocks?: Record<string, string>;
  /** `build` — which round is on screen, and every note she has filed. */
  round: number;
  notes: { round: number; text: string }[];
}

/** Where a project is in its own life. Explicit rather than derived: the
 *  sub-states of "not started yet" (clarify → choose a road → agree a plan)
 *  are exactly the ones a derivation gets wrong. */
export type ProjectPhase = "frame" | "choose" | "plan" | "run" | "published";

export interface Project {
  id: string;
  track: TrackId;
  title: string;
  /** The one line she wrote when she started. Kept verbatim, forever. */
  intent: string;
  /** The conversation. Cards live IN it, not beside it. */
  thread: ThreadItem[];
  cards: CardEntry[];
  phase: ProjectPhase;
  /** 印记's proposed roads, and hers. Empty for a project that arrived with
   *  its road already chosen (the personal page). */
  approaches: Approach[];
  branches: Branch[];
  decision: Decision | null;
  plan: PlanStep[];
  /** Index into the ACTIVE (non-`off`) steps. */
  at: number;
  artifacts: Record<string, ArtifactState>;
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
