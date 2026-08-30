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
import { SEED_PROJECTS, planFor } from "./data/projects";
import { TODAY } from "./data/news";
import type {
  Domain,
  HomepageSection,
  Lang,
  Project,
  ProjectStep,
  StyleId,
  TrackId,
  WorkMode,
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

const STORAGE_KEY = "mk-eco-proto-v1";

export interface HomepageState {
  /** Index into HOMEPAGE_STEPS. */
  step: number;
  exampleId: string | null;
  sections: HomepageSection[];
  mode: WorkMode | null;
  style: StyleId | null;
  published: boolean;
  /** Reached the share step at least once — unlocks the other tracks. */
  shared: boolean;
}

export interface DraftProject {
  track: TrackId | null;
  title: string;
  why: { who: string; cost: string; mine: string };
  /** Which rung of the why-ladder she is on (0..2), 3 = done. */
  whyStep: number;
  /** Rungs 印记 has already pushed back on once, so it never nags twice. */
  probed: string[];
  steps: ProjectStep[];
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
  /** Toasts — the prototype's only feedback channel for "that worked". */
  toast: string | null;
}

const EMPTY_DRAFT: DraftProject = {
  track: null,
  title: "",
  why: { who: "", cost: "", mine: "" },
  whyStep: 0,
  probed: [],
  steps: [],
};

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
      step: 0,
      exampleId: null,
      sections: DEFAULT_SECTIONS.map((s) => ({ ...s, picked: s.picked ? [...s.picked] : undefined })),
      mode: null,
      style: null,
      published: false,
      shared: false,
    },
    projects: SEED_PROJECTS.map((p) => ({ ...p, steps: p.steps.map((s) => ({ ...s })) })),
    draft: { ...EMPTY_DRAFT, why: { ...EMPTY_DRAFT.why } },
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
      projects: saved.projects?.length ? saved.projects : base.projects,
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

interface EcoApi {
  state: EcoState;
  setLang: (l: Lang) => void;
  setDate: (d: string) => void;
  discover: (id: string) => void;
  keep: (newsId: string, keyword: string, field: string) => void;
  setDomainFilter: (d: Domain | null) => void;
  setGrowth: (n: number) => void;
  openCoach: (surface: CoachSurface, seed?: string) => void;
  closeCoach: () => void;
  say: (text: string) => void;
  /** Homepage studio */
  hpGo: (step: number) => void;
  hpPickExample: (id: string) => void;
  hpSetSections: (s: HomepageSection[]) => void;
  hpSetMode: (m: WorkMode) => void;
  hpSetStyle: (s: StyleId) => void;
  hpWrite: (sectionId: string, value: string) => void;
  hpPick: (sectionId: string, itemId: string) => void;
  hpPublish: () => void;
  hpShare: () => void;
  /** Projects */
  draftTrack: (t: TrackId, title?: string) => void;
  draftWhy: (rung: "who" | "cost" | "mine", value: string) => void;
  draftWhyNext: () => void;
  draftWhyBack: () => void;
  draftProbe: (rung: string) => void;
  draftPlan: () => void;
  draftMoveStep: (id: string, dir: -1 | 1) => void;
  draftRemoveStep: (id: string) => void;
  draftAddStep: (step: ProjectStep) => void;
  createProject: () => string;
  toggleStep: (projectId: string, stepId: string) => void;
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
        toast(`「${keyword}」已经长到你的树上了`);
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

      hpGo: (step) => patch((s) => ({ ...s, homepage: { ...s.homepage, step } })),
      hpPickExample: (exampleId) => patch((s) => ({ ...s, homepage: { ...s.homepage, exampleId } })),
      hpSetSections: (sections) => patch((s) => ({ ...s, homepage: { ...s.homepage, sections } })),
      hpSetMode: (mode) => patch((s) => ({ ...s, homepage: { ...s.homepage, mode } })),
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
        patch((s) => ({ ...s, homepage: { ...s.homepage, published: true, step: 4 } }));
        toast("你的主页已经发布了");
      },
      hpShare: () => patch((s) => ({ ...s, homepage: { ...s.homepage, shared: true } })),

      draftTrack: (track, title) =>
        patch((s) => ({
          ...s,
          draft: { ...EMPTY_DRAFT, why: { who: "", cost: "", mine: "" }, track, title: title ?? "" },
        })),
      draftWhy: (rung, value) =>
        patch((s) => ({ ...s, draft: { ...s.draft, why: { ...s.draft.why, [rung]: value } } })),
      draftWhyNext: () =>
        patch((s) => ({ ...s, draft: { ...s.draft, whyStep: Math.min(3, s.draft.whyStep + 1) } })),
      draftWhyBack: () =>
        patch((s) => ({ ...s, draft: { ...s.draft, whyStep: Math.max(0, s.draft.whyStep - 1) } })),
      draftProbe: (rung) =>
        patch((s) =>
          s.draft.probed.includes(rung)
            ? s
            : { ...s, draft: { ...s.draft, probed: [...s.draft.probed, rung] } },
        ),
      draftPlan: () =>
        patch((s) => ({
          ...s,
          draft: { ...s.draft, steps: s.draft.track ? planFor(s.draft.track) : [] },
        })),
      draftMoveStep: (id, dir) =>
        patch((s) => {
          const steps = [...s.draft.steps];
          const i = steps.findIndex((x) => x.id === id);
          const j = i + dir;
          if (i < 0 || j < 0 || j >= steps.length) return s;
          const a = steps[i]!;
          const b = steps[j]!;
          steps[i] = b;
          steps[j] = a;
          return { ...s, draft: { ...s.draft, steps } };
        }),
      draftRemoveStep: (id) =>
        patch((s) => ({ ...s, draft: { ...s.draft, steps: s.draft.steps.filter((x) => x.id !== id) } })),
      draftAddStep: (step) =>
        patch((s) => ({ ...s, draft: { ...s.draft, steps: [...s.draft.steps, step] } })),
      createProject: () => {
        const id = `p-${Date.now().toString(36)}`;
        setState((s) => {
          if (!s.draft.track) return s;
          const project: Project = {
            id,
            track: s.draft.track,
            title: s.draft.title || "未命名项目",
            status: "running",
            cover: "var(--mk-lake)",
            startedAt: new Date().toISOString().slice(0, 10),
            motivation: { ...s.draft.why },
            steps: s.draft.steps.map((x) => ({ ...x })),
          };
          return { ...s, projects: [project, ...s.projects], draft: { ...EMPTY_DRAFT, why: { who: "", cost: "", mine: "" } } };
        });
        return id;
      },
      toggleStep: (projectId, stepId) =>
        patch((s) => ({
          ...s,
          projects: s.projects.map((p) =>
            p.id !== projectId
              ? p
              : { ...p, steps: p.steps.map((st) => (st.id === stepId ? { ...st, done: !st.done } : st)) },
          ),
        })),
      publishProject: (projectId, summary) => {
        patch((s) => ({
          ...s,
          projects: s.projects.map((p) =>
            p.id === projectId ? { ...p, status: "published" as const, summary } : p,
          ),
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
