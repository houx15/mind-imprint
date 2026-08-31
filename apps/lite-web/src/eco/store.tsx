import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import type { CoachMessage, CoachSurface } from "./data/coach";
import { OPENERS, coachReply } from "./data/coach";
import { DEFAULT_SECTIONS } from "./data/homepage";
import {
  SEED_PROJECTS,
  firstProject,
  problemProject,
  trackProject,
} from "./data/projects";
import { activeSteps, branchReply, planById } from "./data/plan";
import { artifactById } from "./data/artifacts";
import { refeed } from "./data/cards";
import { TODAY } from "./data/news";
import type {
  ArtifactState,
  Branch,
  CardEntry,
  CardValue,
  Domain,
  HomepageSection,
  Lang,
  PlanStep,
  Project,
  StyleId,
  ThreadItem,
  TrackId,
} from "./data/types";

/**
 * eco/store — the whole prototype's mutable state, in one place.
 *
 * ## Why one store and not per-screen state
 * The point of the prototype is the LOOP: a planet kept in 世界 grows a
 * keyword on 我的树; a reading picked in the homepage studio shows up on the
 * published page; a project published from the workbench appears on that same
 * page. None of that is visible if each screen owns its own state, so every
 * cross-screen fact lives here.
 *
 * ## Persistence
 * `sessionStorage`, deliberately — not `localStorage`. A prototype should
 * survive a refresh mid-walk (losing a half-built project to a stray reload
 * makes a demo miserable) but should NOT quietly persist across days and make
 * the next viewer wonder why the tree already has their predecessor's
 * additions. Closing the tab resets the world.
 *
 * Every read is wrapped: private windows and storage-blocked contexts throw on
 * access, and the prototype must render correctly with no stored value.
 */

/* ── project helpers ──────────────────────────────────────────────────────
 * Small reducers, extracted because every project action needs them and
 * inlining `projects.map(p => p.id === id ? ... : p)` a dozen times is how the
 * plan, the card list and the thread drift out of sync.
 * ---------------------------------------------------------------------- */

function mapProject(s: EcoState, id: string, fn: (p: Project) => Project): EcoState {
  return { ...s, projects: s.projects.map((p) => (p.id === id ? fn(p) : p)) };
}

/** Several thread items are pushed inside one action, so `Date.now()` alone
 *  collides and React drops the duplicate keys. */
let seq = 0;
function uid(prefix: string): string {
  seq += 1;
  return `${prefix}-${Date.now().toString(36)}-${seq}`;
}

/** Update a card entry, creating it if 印记 never formally summoned it —
 *  which happens when she opens a card herself from the rail. */
function upsertCard(
  cards: CardEntry[],
  cardId: string,
  fn: (c: CardEntry) => CardEntry,
): CardEntry[] {
  return cards.some((c) => c.cardId === cardId)
    ? cards.map((c) => (c.cardId === cardId ? fn(c) : c))
    : [...cards, fn({ cardId, status: "open", values: {} })];
}

/**
 * Update a branch, creating it on first use.
 *
 * 🚨 It used to require a separate `openBranch` call, and `Roads` never made
 * one — so opening a road showed the whole branch surface and then silently
 * swallowed every question asked in it. Creating on write removes the class of
 * bug rather than the instance.
 */
function upsertBranch(
  branches: Branch[],
  approachId: string,
  fn: (b: Branch) => Branch,
): Branch[] {
  return branches.some((b) => b.approachId === approachId)
    ? branches.map((b) => (b.approachId === approachId ? fn(b) : b))
    : [...branches, fn({ approachId, log: [], takeaway: "" })];
}

const FRESH_ARTIFACT: ArtifactState = { status: "idle", round: 0, notes: [] };

/**
 * Walk the project onto plan step `index` and put whatever it opens on the
 * table.
 *
 * 🚨 This is the ONLY place the plan advances. It used to be `submitCard`
 * reaching into `cardsForTrack` for "the next card the track hasn't used",
 * which meant the plan she approved and the sequence she actually walked were
 * two different lists that happened to agree. Now the plan is the sequence.
 */
function enterStep(p: Project, index: number): Project {
  const steps = activeSteps(p.plan);
  const step = steps[index];
  if (!step) {
    return {
      ...p,
      at: steps.length,
      thread: [
        ...p.thread,
        {
          id: uid("done"),
          kind: "say",
          role: "coach",
          text: "计划上的步骤走完了。剩下的部分没有卡片能替你走——去把东西做完，随时回来问我。",
        },
      ],
    };
  }

  const thread: ThreadItem[] = [...p.thread, { id: uid("st"), kind: "step", stepId: step.id }];

  // Hoisted: narrowing on `step.opens` does not survive into the `.some()`
  // callback below, and the optional chain there would silently match a card
  // entry whose id is `undefined`.
  const opens = step.opens;

  if (opens?.kind === "card") {
    const cid = opens.cardId;
    return {
      ...p,
      at: index,
      thread: [...thread, { id: uid("c"), kind: "card", cardId: cid }],
      cards: p.cards.some((c) => c.cardId === cid)
        ? p.cards
        : [...p.cards, { cardId: cid, status: "invited", values: {} }],
    };
  }
  if (opens?.kind === "make") {
    const aid = opens.artifactId;
    return {
      ...p,
      at: index,
      thread: [...thread, { id: uid("m"), kind: "make", artifactId: aid }],
      artifacts: p.artifacts[aid] ? p.artifacts : { ...p.artifacts, [aid]: { ...FRESH_ARTIFACT } },
    };
  }
  return { ...p, at: index, thread };
}

/** 🚨 Bump this whenever a persisted shape changes. `Project` grew `plan`,
 *  `phase`, `approaches`, `branches`, `decision` and `artifacts` on
 *  2026-08-31, and lost `status`; a v2 blob restored into v3 code renders a
 *  project with no plan and no phase. The version guard is the primary
 *  defence and `sane()` below is the belt. */
const STORAGE_KEY = "mk-eco-proto-v3";

/**
 * The personal page.
 *
 * Shrank on 2026-08-31 with the six-step wizard: `step`, `exampleId`, `mode`
 * and `shared` all existed to drive a walkthrough that no longer exists (the
 * teaching moved into the card library, where every track can reach it). What
 * is left is what the published page actually renders.
 */
export interface HomepageState {
  sections: HomepageSection[];
  style: StyleId | null;
  published: boolean;
}

/**
 * A project being started through the 「开新项目」 door.
 *
 * Deliberately thin. An earlier draft carried the whole why-ladder and a
 * generated step list, which meant a student answered three hard questions
 * BEFORE the project existed — and if she bailed, all of it evaporated. Now
 * the door asks for a track and one sentence; everything else happens inside
 * the project, where the answers are kept.
 */
export interface DraftProject {
  track: TrackId | null;
  title: string;
  intent: string;
}

export interface EcoState {
  lang: Lang;
  /** 世界 */
  date: string;
  discovered: string[];
  kept: string[];
  domainFilter: Domain | null;
  /** 我的树 */
  growth: number;
  grownKeywords: { id: string; text: string; field: string; from: string }[];
  /** 印记 */
  coachOpen: boolean;
  coachSurface: CoachSurface;
  coachLog: CoachMessage[];
  /** PBL */
  homepage: HomepageState;
  projects: Project[];
  draft: DraftProject;
  /** How `/eco/projects/new` opens — set by whichever door she came through. */
  newProjectMode: "pick" | "talk";
  /** Toasts — the prototype's only feedback channel for "that worked". */
  toast: string | null;
}

const EMPTY_DRAFT: DraftProject = { track: null, title: "", intent: "" };

/**
 * 🚨 `projects` starts EMPTY, and that is the point.
 *
 * A student's first visit to 项目 is a screen with no projects on it, and the
 * whole design of that screen — one sentence and one button — only exists if
 * the empty state is reachable. Seeding two finished projects made the empty
 * state unreachable and therefore untested. The samples are still one click
 * away on that screen, labelled as prototype furniture.
 */
function initialState(): EcoState {
  return {
    lang: "zh",
    date: TODAY,
    discovered: [],
    kept: [],
    domainFilter: null,
    growth: 3,
    grownKeywords: [],
    coachOpen: false,
    coachSurface: "world",
    coachLog: [],
    homepage: {
      sections: DEFAULT_SECTIONS.map((s) => ({ ...s, picked: s.picked ? [...s.picked] : undefined })),
      style: null,
      published: false,
    },
    projects: [],
    draft: { ...EMPTY_DRAFT },
    newProjectMode: "pick",
    toast: null,
  };
}

function load(): EcoState {
  const base = initialState();
  try {
    const raw = window.sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return base;
    const saved = JSON.parse(raw) as Partial<EcoState>;
    return {
      ...base,
      ...saved,
      homepage: { ...base.homepage, ...(saved.homepage ?? {}) },
      draft: { ...base.draft, ...(saved.draft ?? {}) },
      // Only accept stored projects that still match the current shape. An
      // earlier version of this merge claimed old blobs "must never crash the
      // app"; it did not actually check, and a project missing `cards` took
      // the whole hub down.
      projects: saved.projects?.every(sane) ? saved.projects : base.projects,
      // The coach log is intentionally NOT restored: re-reading a stale
      // conversation on load reads as the app talking to itself.
      coachLog: [],
      coachOpen: false,
      toast: null,
    };
  } catch {
    return base;
  }
}

/** Does this look like a `Project` this build can render? */
function sane(p: unknown): p is Project {
  const c = p as Partial<Project> | null;
  return Boolean(
    c &&
      typeof c.id === "string" &&
      Array.isArray(c.cards) &&
      Array.isArray(c.thread) &&
      Array.isArray(c.plan) &&
      typeof c.phase === "string" &&
      c.artifacts !== undefined,
  );
}

interface EcoApi {
  state: EcoState;
  setLang: (l: Lang) => void;
  setDate: (d: string) => void;
  discover: (id: string) => void;
  keep: (newsId: string, keyword: string, field: string) => void;
  /** Take a story back out of 待读. The keyword it seeded STAYS on the tree —
   *  she did collect it, and the model records what happened, not what she
   *  currently wants to be true (铁律④). */
  unkeep: (newsId: string) => void;
  setDomainFilter: (d: Domain | null) => void;
  setGrowth: (n: number) => void;
  openCoach: (surface: CoachSurface, seed?: string) => void;
  closeCoach: () => void;
  say: (text: string) => void;
  /** 我的主页 */
  hpSetSections: (s: HomepageSection[]) => void;
  hpSetStyle: (s: StyleId) => void;
  hpWrite: (sectionId: string, value: string) => void;
  hpPick: (sectionId: string, itemId: string) => void;
  hpPublish: () => void;
  /** Pick an item into a homepage section AND jump to that section's editor —
   *  the 「把它放上我的主页」 action from a reading or a writing. */
  hpPickAndCompose: (sectionId: string, itemId: string) => void;

  /* ── the two doors ──────────────────────────────────────────────── */
  /** 做我的主页 — the one button on an empty hub. */
  startFirstProject: () => string;
  /** 我发现了一个真问题 — the door that goes through 澄清 → 选路 → 计划. */
  startProblemProject: (problem: string) => string;
  /** Put the sample projects on the hub. Prototype furniture, labelled. */
  loadSamples: () => void;
  draftTrack: (t: TrackId, title?: string) => void;
  draftIntent: (v: string) => void;
  draftTitle: (v: string) => void;
  createProject: () => string;
  setNewProjectMode: (m: "pick" | "talk") => void;

  /* ── the planner ────────────────────────────────────────────────── */
  planEdit: (projectId: string, stepId: string, patch: Partial<PlanStep>) => void;
  planToggle: (projectId: string, stepId: string) => void;
  planMove: (projectId: string, stepId: string, dir: -1 | 1) => void;
  planAdd: (projectId: string) => void;
  /** She agreed. Nothing runs before this. */
  approvePlan: (projectId: string) => void;

  /* ── roads ──────────────────────────────────────────────────────── */
  /** Ask something inside the side conversation about ONE approach. The
   *  branch is created by the first thing written in it. */
  askInBranch: (projectId: string, approachId: string, text: string) => void;
  /** What she brings back out of the branch. */
  setTakeaway: (projectId: string, approachId: string, text: string) => void;
  /** 🚨 A decision is a choice PLUS a reason PLUS what she gave up. */
  decideRoad: (projectId: string, approachId: string, why: string, gaveUp: string) => void;

  /* ── cards ──────────────────────────────────────────────────────── */
  /** She accepted the invitation and opened the card (铁律②: HER call). */
  openCardEntry: (projectId: string, cardId: string) => void;
  /** Autosave while she works. Does not change status. */
  saveCard: (projectId: string, cardId: string, values: Record<string, CardValue>) => void;
  /** 提交 — the card comes back into the conversation and the plan advances. */
  submitCard: (projectId: string, cardId: string, values: Record<string, CardValue>) => void;
  /** 现在不做. The invitation stays in the rail; 印记 does not nag. */
  skipCard: (projectId: string, cardId: string) => void;

  /* ── artifacts (印记 builds, she judges) ─────────────────────────── */
  runArtifact: (projectId: string, artifactId: string) => void;
  artifactReady: (projectId: string, artifactId: string) => void;
  setArtifactChoice: (projectId: string, artifactId: string, choice: string) => void;
  setArtifactWhy: (projectId: string, artifactId: string, why: string) => void;
  setArtifactBlock: (projectId: string, artifactId: string, blockId: string, text: string) => void;
  /** A round of concrete feedback on something 印记 built. */
  noteBuild: (projectId: string, artifactId: string, text: string) => void;
  /** She is done judging it. Feeds back into the thread and advances the plan. */
  settleArtifact: (projectId: string, artifactId: string, summary: string) => void;

  /** Free talk inside a project. */
  sayInProject: (projectId: string, text: string) => void;
  publishProject: (projectId: string, summary: string) => void;
  toast: (msg: string) => void;
}

const EcoContext = createContext<EcoApi | null>(null);

export function EcoProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<EcoState>(load);
  const toastTimer = useRef<number | null>(null);

  useEffect(() => {
    try {
      window.sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } catch {
      // Storage blocked (private window, site-data off). The prototype keeps
      // working from memory; nothing here is worth surfacing to the student.
    }
  }, [state]);

  const patch = useCallback((fn: (s: EcoState) => EcoState) => setState(fn), []);

  const toast = useCallback((msg: string) => {
    setState((s) => ({ ...s, toast: msg }));
    if (toastTimer.current) window.clearTimeout(toastTimer.current);
    toastTimer.current = window.setTimeout(() => setState((s) => ({ ...s, toast: null })), 3200);
  }, []);

  useEffect(() => () => {
    if (toastTimer.current) window.clearTimeout(toastTimer.current);
  }, []);

  const api = useMemo<EcoApi>(() => {
    const openCoach = (surface: CoachSurface, seed?: string) =>
      patch((s) => {
        const opener = OPENERS[surface];
        const log: CoachMessage[] =
          s.coachLog.length > 0 && s.coachSurface === surface
            ? s.coachLog
            : [{ id: `c-${Date.now()}`, role: "coach", text: opener.text, choices: opener.choices }];
        const withSeed: CoachMessage[] = seed
          ? [
              ...log,
              { id: `s-${Date.now()}`, role: "student", text: seed },
              { id: `r-${Date.now() + 1}`, role: "coach", ...coachReply(seed) },
            ]
          : log;
        return { ...s, coachOpen: true, coachSurface: surface, coachLog: withSeed };
      });

    /** Add a project and hand back its id. */
    const add = (make: (id: string) => Project): string => {
      const id = `p-${Date.now().toString(36)}`;
      setState((s) => ({ ...s, projects: [make(id), ...s.projects] }));
      return id;
    };

    return {
      state,
      setLang: (lang) => patch((s) => ({ ...s, lang })),
      setDate: (date) => patch((s) => ({ ...s, date })),
      discover: (id) =>
        patch((s) => (s.discovered.includes(id) ? s : { ...s, discovered: [...s.discovered, id] })),
      keep: (newsId, keyword, field) => {
        patch((s) =>
          s.kept.includes(newsId)
            ? s
            : {
                ...s,
                kept: [...s.kept, newsId],
                grownKeywords: [
                  ...s.grownKeywords,
                  { id: `grown-${newsId}`, text: keyword, field, from: newsId },
                ],
              },
        );
        // Two things happen, and the toast names the one she asked for. The
        // keyword growing is a consequence of collecting, not a separate
        // action she chose — 收藏 is the verb on the button.
        toast(`已收藏到阅读室的「待读」·「${keyword}」也长到了你的兴趣树上`);
      },
      unkeep: (newsId) => {
        patch((s) => ({ ...s, kept: s.kept.filter((k) => k !== newsId) }));
        toast("已经从待读里移走了");
      },
      setDomainFilter: (domainFilter) => patch((s) => ({ ...s, domainFilter })),
      setGrowth: (growth) => patch((s) => ({ ...s, growth })),
      openCoach,
      closeCoach: () => patch((s) => ({ ...s, coachOpen: false })),
      say: (text) =>
        patch((s) => ({
          ...s,
          coachLog: [
            ...s.coachLog,
            { id: `s-${Date.now()}`, role: "student", text },
            { id: `r-${Date.now() + 1}`, role: "coach", ...coachReply(text) },
          ],
        })),

      hpSetSections: (sections) => patch((s) => ({ ...s, homepage: { ...s.homepage, sections } })),
      hpSetStyle: (style) => patch((s) => ({ ...s, homepage: { ...s.homepage, style } })),
      hpWrite: (sectionId, value) =>
        patch((s) => ({
          ...s,
          homepage: {
            ...s.homepage,
            sections: s.homepage.sections.map((sec) =>
              sec.id === sectionId ? { ...sec, value } : sec,
            ),
          },
        })),
      hpPick: (sectionId, itemId) =>
        patch((s) => ({
          ...s,
          homepage: {
            ...s.homepage,
            sections: s.homepage.sections.map((sec) => {
              if (sec.id !== sectionId) return sec;
              const picked = sec.picked ?? [];
              return {
                ...sec,
                picked: picked.includes(itemId)
                  ? picked.filter((p) => p !== itemId)
                  : [...picked, itemId],
              };
            }),
          },
        })),
      hpPublish: () => {
        patch((s) => ({ ...s, homepage: { ...s.homepage, published: true } }));
        toast("你的主页已经发布了");
      },
      hpPickAndCompose: (sectionId, itemId) => {
        patch((s) => ({
          ...s,
          homepage: {
            ...s.homepage,
            sections: s.homepage.sections.map((sec) =>
              sec.id === sectionId
                ? {
                    ...sec,
                    enabled: true,
                    picked: sec.picked?.includes(itemId)
                      ? sec.picked
                      : [...(sec.picked ?? []), itemId],
                  }
                : sec,
            ),
          },
        }));
        toast("已经挑上你的主页了，可以再调整");
      },

      /* ── doors ─────────────────────────────────────────────────── */
      startFirstProject: () => add(firstProject),
      startProblemProject: (problem) => add((id) => problemProject(id, problem)),
      loadSamples: () => {
        patch((s) => ({
          ...s,
          projects: [
            ...s.projects,
            ...SEED_PROJECTS.filter((seed) => !s.projects.some((p) => p.id === seed.id)).map((p) => ({
              ...p,
              thread: p.thread.map((t) => ({ ...t })),
              cards: p.cards.map((c) => ({ ...c, values: { ...c.values } })),
              plan: p.plan.map((st) => ({ ...st })),
            })),
          ],
        }));
        toast("示例项目已经放上来了");
      },

      draftTrack: (track, title) =>
        patch((s) => ({ ...s, draft: { ...EMPTY_DRAFT, track, title: title ?? "" } })),
      draftTitle: (title) => patch((s) => ({ ...s, draft: { ...s.draft, title } })),
      draftIntent: (intent) => patch((s) => ({ ...s, draft: { ...s.draft, intent } })),
      createProject: () => {
        const id = `p-${Date.now().toString(36)}`;
        setState((s) => {
          if (!s.draft.track) return s;
          return {
            ...s,
            projects: [
              trackProject(id, s.draft.track, s.draft.title, s.draft.intent),
              ...s.projects,
            ],
            draft: { ...EMPTY_DRAFT },
          };
        });
        return id;
      },
      setNewProjectMode: (newProjectMode) => patch((s) => ({ ...s, newProjectMode })),

      /* ── the planner ───────────────────────────────────────────── */
      planEdit: (projectId, stepId, p) =>
        patch((s) =>
          mapProject(s, projectId, (proj) => ({
            ...proj,
            plan: proj.plan.map((st) => (st.id === stepId ? { ...st, ...p } : st)),
          })),
        ),
      planToggle: (projectId, stepId) =>
        patch((s) =>
          mapProject(s, projectId, (proj) => ({
            ...proj,
            plan: proj.plan.map((st) => (st.id === stepId ? { ...st, off: !st.off } : st)),
          })),
        ),
      planMove: (projectId, stepId, dir) =>
        patch((s) =>
          mapProject(s, projectId, (proj) => {
            const i = proj.plan.findIndex((st) => st.id === stepId);
            const j = i + dir;
            if (i < 0 || j < 0 || j >= proj.plan.length) return proj;
            const plan = [...proj.plan];
            const a = plan[i]!;
            const b = plan[j]!;
            plan[i] = b;
            plan[j] = a;
            return { ...proj, plan };
          }),
        ),
      planAdd: (projectId) =>
        patch((s) =>
          mapProject(s, projectId, (proj) => ({
            ...proj,
            plan: [
              ...proj.plan,
              {
                id: uid("mine"),
                title: "我自己加的一步",
                blurb: "写清楚这一步要发生什么。",
                youBring: "",
                iBring: "",
                decide: "",
                when: "",
                opens: null,
                mine: true,
              },
            ],
          })),
        ),
      approvePlan: (projectId) => {
        patch((s) =>
          mapProject(s, projectId, (proj) => {
            const started: Project = {
              ...proj,
              phase: "run",
              at: -1,
              thread: [
                ...proj.thread,
                {
                  id: uid("go"),
                  kind: "say",
                  role: "coach",
                  text: `计划算数了。${activeSteps(proj.plan).length} 步，按你排的顺序走。\n\n中间任何一步你都可以说「这条不对」——改计划比硬走完一个错的计划便宜得多。`,
                },
              ],
            };
            return enterStep(started, 0);
          }),
        );
        toast("计划已确认，开工");
      },

      /* ── roads ─────────────────────────────────────────────────── */
      askInBranch: (projectId, approachId, text) =>
        patch((s) =>
          mapProject(s, projectId, (proj) => ({
            ...proj,
            branches: upsertBranch(proj.branches, approachId, (b) => ({
              ...b,
              log: [
                ...b.log,
                { id: uid("bs"), role: "student" as const, text },
                { id: uid("bc"), role: "coach" as const, text: branchReply(approachId, text) },
              ],
            })),
          })),
        ),
      setTakeaway: (projectId, approachId, takeaway) =>
        patch((s) =>
          mapProject(s, projectId, (proj) => ({
            ...proj,
            branches: upsertBranch(proj.branches, approachId, (b) => ({ ...b, takeaway })),
          })),
        ),
      decideRoad: (projectId, approachId, why, gaveUp) => {
        patch((s) =>
          mapProject(s, projectId, (proj) => {
            const road = proj.approaches.find((a) => a.id === approachId);
            if (!road) return proj;
            return {
              ...proj,
              phase: "plan",
              decision: { approachId, why, gaveUp, at: new Date().toISOString().slice(0, 10) },
              plan: planById(road.planId),
              title: road.id === "signs" ? "花园里的指路标牌" : proj.title,
              thread: [
                ...proj.thread,
                {
                  id: uid("dec"),
                  kind: "say",
                  role: "student",
                  text: `我选「${road.name}」。理由：${why}`,
                },
                {
                  id: uid("plan"),
                  kind: "say",
                  role: "coach",
                  text: `记下了。你的理由是「${why.trim().slice(0, 40)}」——这句我会一直留着，做到一半你怀疑自己的时候可以回来看。\n\n那按这条路，我排了一份计划。**先看一遍再说开不开工**：不同意的步骤直接改掉、关掉，或者加一步我没想到的。\n\n还有一件事只有你能做：**给每一步写上时间**。我不替你排——你自己排的时间，你才会当真。`,
                },
              ],
            };
          }),
        );
        toast("决定已经记下来了，连同你的理由");
      },

      /* ── cards ─────────────────────────────────────────────────── */
      openCardEntry: (projectId, cardId) =>
        patch((s) =>
          mapProject(s, projectId, (p) => ({
            ...p,
            cards: upsertCard(p.cards, cardId, (c) =>
              // Reopening a finished card must not un-finish it — she is
              // allowed to go back and read what she wrote.
              c.status === "done" ? c : { ...c, status: "open" },
            ),
          })),
        ),

      saveCard: (projectId, cardId, values) =>
        patch((s) =>
          mapProject(s, projectId, (p) => ({
            ...p,
            cards: upsertCard(p.cards, cardId, (c) => ({ ...c, values })),
          })),
        ),

      submitCard: (projectId, cardId, values) => {
        patch((s) =>
          mapProject(s, projectId, (p) => {
            const cards = upsertCard(p.cards, cardId, (c) => ({
              ...c,
              status: "done" as const,
              values,
              doneAt: new Date().toISOString().slice(0, 10),
            }));
            const fed: Project = {
              ...p,
              cards,
              thread: [
                ...p.thread,
                { id: uid("r"), kind: "say", role: "coach", text: refeed(cardId, values) },
              ],
            };
            // 问题澄清 is the one card that does not advance a plan — there is
            // no plan yet. It hands over to 选路 instead.
            if (p.phase === "frame" && cardId === "frame") {
              return {
                ...fed,
                phase: "choose",
                thread: [
                  ...fed.thread,
                  {
                    id: uid("roads"),
                    kind: "say",
                    role: "coach",
                    text: `我想到 ${p.approaches.length} 条路。它们不是同一件事的两种做法——一条做在屏幕上，随时能改；一条做在现实里，装上去就很难动。\n\n**我不替你选。** 右边那两张卡你都可以点开单独问，问完把你想清楚的那一句写下来，再决定。`,
                  },
                ],
              };
            }
            return enterStep(fed, fed.at + 1);
          }),
        );
        toast("这张卡已经收进项目材料里了");
      },

      skipCard: (projectId, cardId) =>
        patch((s) =>
          mapProject(s, projectId, (p) => ({
            ...p,
            thread: [
              ...p.thread,
              {
                id: uid("sk"),
                kind: "say",
                role: "coach",
                // 印记 does not nag and does not sulk. The card stays in the
                // rail; that is the whole consequence of saying no (铁律②).
                text: "行，先不做。它留在右边的材料栏里，你什么时候想做都可以点开。",
              },
            ],
            cards: p.cards.map((c) => (c.cardId === cardId ? { ...c, status: "invited" as const } : c)),
          })),
        ),

      /* ── artifacts ─────────────────────────────────────────────── */
      runArtifact: (projectId, artifactId) =>
        patch((s) =>
          mapProject(s, projectId, (p) => ({
            ...p,
            artifacts: {
              ...p.artifacts,
              [artifactId]: { ...(p.artifacts[artifactId] ?? FRESH_ARTIFACT), status: "working" },
            },
          })),
        ),
      artifactReady: (projectId, artifactId) =>
        patch((s) =>
          mapProject(s, projectId, (p) => ({
            ...p,
            artifacts: {
              ...p.artifacts,
              [artifactId]: { ...(p.artifacts[artifactId] ?? FRESH_ARTIFACT), status: "ready" },
            },
          })),
        ),
      setArtifactChoice: (projectId, artifactId, choice) =>
        patch((s) =>
          mapProject(s, projectId, (p) => ({
            ...p,
            artifacts: {
              ...p.artifacts,
              [artifactId]: { ...(p.artifacts[artifactId] ?? FRESH_ARTIFACT), choice },
            },
          })),
        ),
      setArtifactWhy: (projectId, artifactId, why) =>
        patch((s) =>
          mapProject(s, projectId, (p) => ({
            ...p,
            artifacts: {
              ...p.artifacts,
              [artifactId]: { ...(p.artifacts[artifactId] ?? FRESH_ARTIFACT), why },
            },
          })),
        ),
      setArtifactBlock: (projectId, artifactId, blockId, text) =>
        patch((s) =>
          mapProject(s, projectId, (p) => {
            const cur = p.artifacts[artifactId] ?? FRESH_ARTIFACT;
            return {
              ...p,
              artifacts: {
                ...p.artifacts,
                [artifactId]: { ...cur, blocks: { ...(cur.blocks ?? {}), [blockId]: text } },
              },
            };
          }),
        ),
      noteBuild: (projectId, artifactId, text) =>
        patch((s) =>
          mapProject(s, projectId, (p) => {
            const cur = p.artifacts[artifactId] ?? FRESH_ARTIFACT;
            const spec = artifactById(artifactId);
            const last = (spec?.rounds?.length ?? 1) - 1;
            const round = Math.min(cur.round + 1, last);
            return {
              ...p,
              artifacts: {
                ...p.artifacts,
                [artifactId]: {
                  ...cur,
                  round,
                  status: "working",
                  notes: [...cur.notes, { round: cur.round, text }],
                },
              },
              thread: [
                ...p.thread,
                { id: uid("fb"), kind: "say", role: "student", text },
                {
                  id: uid("fbr"),
                  kind: "say",
                  role: "coach",
                  text: `收到。「${text.trim().slice(0, 34)}」——这条我能直接动手改，我去改一版。`,
                },
              ],
            };
          }),
        ),
      settleArtifact: (projectId, artifactId, summary) => {
        patch((s) =>
          mapProject(s, projectId, (p) => {
            const cur = p.artifacts[artifactId] ?? FRESH_ARTIFACT;
            const settled: Project = {
              ...p,
              artifacts: { ...p.artifacts, [artifactId]: { ...cur, status: "settled" } },
              thread: [
                ...p.thread,
                { id: uid("as"), kind: "say", role: "coach", text: summary },
              ],
            };
            return enterStep(settled, settled.at + 1);
          }),
        );
        toast("这一步收下了");
      },

      sayInProject: (projectId, text) =>
        patch((s) =>
          mapProject(s, projectId, (p) => ({
            ...p,
            thread: [
              ...p.thread,
              { id: uid("u"), kind: "say", role: "student", text },
              { id: uid("a"), kind: "say", role: "coach", text: coachReply(text).text },
            ],
          })),
        ),

      publishProject: (projectId, summary) => {
        // 🚨 Publishing must ALSO place the project on her page, because the
        // toast and the copy both promise exactly that. The page renders only
        // what she picked (she curates), so publishing picks it FOR her; she
        // can still unpick it in the studio. Saying "它现在在你的主页上" while
        // doing nothing was the one place this prototype told a student
        // something untrue.
        patch((s) => ({
          ...s,
          projects: s.projects.map((p) =>
            p.id === projectId ? { ...p, phase: "published" as const, summary } : p,
          ),
          homepage: {
            ...s.homepage,
            sections: s.homepage.sections.map((sec) =>
              sec.id === "projects"
                ? {
                    ...sec,
                    enabled: true,
                    picked: sec.picked?.includes(projectId)
                      ? sec.picked
                      : [...(sec.picked ?? []), projectId],
                  }
                : sec,
            ),
          },
        }));
        toast("项目已发布，它现在在你的主页上");
      },
      toast,
    };
  }, [state, patch, toast]);

  return <EcoContext.Provider value={api}>{children}</EcoContext.Provider>;
}

export function useEco(): EcoApi {
  const ctx = useContext(EcoContext);
  if (!ctx) throw new Error("useEco must be used inside <EcoProvider>");
  return ctx;
}
