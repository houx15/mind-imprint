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
import { SEED_PROJECTS, openingThread, trackById } from "./data/projects";
import { cardsForTrack, refeed } from "./data/cards";
import { TODAY } from "./data/news";
import type {
  CardEntry,
  CardValue,
  Domain,
  HomepageSection,
  Lang,
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
 * published page; a project published from a step workspace appears on that
 * same page. None of that is visible if each screen owns its own state, so
 * every cross-screen fact lives here.
 *
 * ## Persistence
 * `sessionStorage`, deliberately — not `localStorage`. A prototype should
 * survive a refresh mid-walk (losing a half-built homepage to a stray reload
 * makes a demo miserable) but should NOT quietly persist across days and make
 * the next viewer wonder why the tree already has their predecessor's
 * additions. Closing the tab resets the world.
 *
 * Every read is wrapped: private windows and storage-blocked contexts throw on
 * access, and the prototype must render correctly with no stored value.
 */


/* ── project helpers ──────────────────────────────────────────────────────
 * Two tiny reducers, extracted because every project action needs them and
 * inlining `projects.map(p => p.id === id ? ... : p)` five times is how the
 * card list and the thread drift out of sync.
 * ---------------------------------------------------------------------- */

function mapProject(s: EcoState, id: string, fn: (p: Project) => Project): EcoState {
  return { ...s, projects: s.projects.map((p) => (p.id === id ? fn(p) : p)) };
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

/** 🚨 Bump this whenever a persisted shape changes. `Project` grew `cards` and
 *  `thread` and lost `steps` on 2026-08-31; a v1 blob restored into v2 code
 *  crashed the hub on first render. The version guard is the primary defence
 *  and `sane()` below is the belt. */
const STORAGE_KEY = "mk-eco-proto-v2";

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
 * A project being started.
 *
 * Deliberately thin. v1's draft carried the whole why-ladder and a generated
 * step list, which meant a student answered three hard questions BEFORE the
 * project existed — and if she bailed, all of it evaporated. Now the door
 * asks for a track and one sentence; the why is the first card INSIDE the
 * project, where the answers are kept.
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
    projects: SEED_PROJECTS.map((p) => ({
      ...p,
      thread: p.thread.map((t) => ({ ...t })),
      cards: p.cards.map((c) => ({ ...c, values: { ...c.values } })),
    })),
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
    // Shallow-merge only: a stored shape from an older build must never crash
    // the app, so anything missing falls back to the fresh value.
    return {
      ...base,
      ...saved,
      homepage: { ...base.homepage, ...(saved.homepage ?? {}) },
      draft: { ...base.draft, ...(saved.draft ?? {}) },
      // Only accept stored projects that still match the current shape. The
      // merge below claimed old blobs "must never crash the app"; it did not
      // actually check, and a v1 project (no `cards`) took the whole hub down.
      projects: saved.projects?.length && saved.projects.every(sane)
        ? saved.projects
        : base.projects,
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
  return Boolean(c && Array.isArray(c.cards) && Array.isArray(c.thread) && typeof c.id === "string");
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
  /** Projects — the workbench */
  draftTrack: (t: TrackId, title?: string) => void;
  draftIntent: (v: string) => void;
  draftTitle: (v: string) => void;
  createProject: () => string;
  setNewProjectMode: (m: "pick" | "talk") => void;
  /** She accepted the invitation and opened the card (铁律②: HER call). */
  openCardEntry: (projectId: string, cardId: string) => void;
  /** Autosave while she works. Does not change status. */
  saveCard: (projectId: string, cardId: string, values: Record<string, CardValue>) => void;
  /** 提交 — the card comes back into the conversation and 印记 summons the
   *  next one. This is the whole loop, in one function. */
  submitCard: (projectId: string, cardId: string, values: Record<string, CardValue>) => void;
  /** 现在不做. The invitation stays in the rail; 印记 does not nag. */
  skipCard: (projectId: string, cardId: string) => void;
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

      draftTrack: (track, title) =>
        patch((s) => ({ ...s, draft: { ...EMPTY_DRAFT, track, title: title ?? "" } })),
      draftTitle: (title) => patch((s) => ({ ...s, draft: { ...s.draft, title } })),
      draftIntent: (intent) => patch((s) => ({ ...s, draft: { ...s.draft, intent } })),
      createProject: () => {
        const id = `p-${Date.now().toString(36)}`;
        setState((s) => {
          if (!s.draft.track) return s;
          const seq = cardsForTrack(s.draft.track);
          // Every track's sequence is a non-empty literal, so index 0 exists.
          const first = seq[0]!;
          const project: Project = {
            id,
            track: s.draft.track,
            title: s.draft.title.trim() || "未命名项目",
            intent: s.draft.intent.trim(),
            status: "running",
            cover: trackById(s.draft.track).hue,
            startedAt: new Date().toISOString().slice(0, 10),
            // A project is born WITH its first card already on the table. An
            // empty room with a "开始" button is where v1 lost people.
            thread: openingThread(s.draft.track, first),
            cards: [{ cardId: first, status: "invited", values: {} }],
          };
          return { ...s, projects: [project, ...s.projects], draft: { ...EMPTY_DRAFT } };
        });
        return id;
      },
      setNewProjectMode: (newProjectMode) => patch((s) => ({ ...s, newProjectMode })),

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
            const thread: ThreadItem[] = [
              ...p.thread,
              { id: `r-${Date.now()}`, kind: "say", role: "coach", text: refeed(cardId, values) },
            ];
            // 印记 summons the next card in the track's sequence. This is
            // orchestration, not authorship — no confirmation gate (AGENTS.md:
            // 铁律 govern her prose, not the system's own steps). What she
            // confirms is OPENING it.
            const next = cardsForTrack(p.track).find((cid) => !cards.some((c) => c.cardId === cid));
            if (!next) {
              return {
                ...p,
                cards,
                thread: [
                  ...thread,
                  {
                    id: `d-${Date.now() + 1}`,
                    kind: "say",
                    role: "coach",
                    text: "这条赛道的卡你都过完了。剩下的部分没有卡片能替你走——去把东西做完，随时回来问我。",
                  },
                ],
              };
            }
            return {
              ...p,
              cards: [...cards, { cardId: next, status: "invited" as const, values: {} }],
              thread: [...thread, { id: `c-${Date.now() + 1}`, kind: "card", cardId: next }],
            };
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
                id: `s-${Date.now()}`,
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

      sayInProject: (projectId, text) =>
        patch((s) =>
          mapProject(s, projectId, (p) => ({
            ...p,
            thread: [
              ...p.thread,
              { id: `u-${Date.now()}`, kind: "say", role: "student", text },
              { id: `a-${Date.now() + 1}`, kind: "say", role: "coach", text: coachReply(text).text },
            ],
          })),
        ),

      publishProject: (projectId, summary) => {
        // 🚨 Publishing must ALSO place the project on her page, because the
        // toast and the step copy both promise exactly that. The page renders
        // only what she picked (principle 03 — she curates), so publishing
        // picks it FOR her; she can still unpick it in the studio. Saying
        // "它现在在你的主页上" while doing nothing was the one place this
        // prototype told a student something untrue.
        patch((s) => ({
          ...s,
          projects: s.projects.map((p) =>
            p.id === projectId ? { ...p, status: "published" as const, summary } : p,
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
        toast(
          "项目已发布，它现在在你的主页上",
        );
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
