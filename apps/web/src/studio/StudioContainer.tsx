import { useEffect, useState } from "react";
import type { StudioProjection } from "@mind-imprint/contracts";
import { api as defaultApi } from "../api";
import { StudioShell } from "./StudioShell";
import type { StationCode, StudioState, StudioCallbacks } from "./state";

type StudioApi = { listProjects: typeof defaultApi.listProjects; getProject: typeof defaultApi.getProject };

// Reuse the trial-signin creds/sequence from AppShell.tsx:26-27 so ?studio is
// reachable without a class join-code. Public demo creds — see AppShell's
// comment for the seed-migration source.
const DEMO_EMAIL = "phoebe@demo.mindimprint.local";
const DEMO_PASSWORD = "phoebe-dev-pass";

// Map the lean wire projection into the frontend view-model, stubbing the
// deferred center-pane views (material → Slice 6, structure/writing/review → 7/8/9).
function toStudioState(p: StudioProjection): StudioState {
  return {
    project: p.project,
    stations: p.stations,
    activeStation: p.activeStation,
    focusMode: false,
    coach: p.coach,
    views: {
      material: [],
      structure: [],
      writing: { draft: "", mode: "edit" },
      review: [],
      onboarding: p.onboarding,
    },
  };
}

async function defaultEnsureSession(): Promise<void> {
  // Reuse the trial-signin path (AppShell): if unauthenticated under ?studio,
  // sign in as the seeded sample student. Unlike AppShell's ?trial=1 (a
  // one-shot deep link that gets stripped), ?studio is the routing switch
  // Root.tsx checks on every load, so we keep the search string on replace.
  try {
    await defaultApi.getMe();
  } catch {
    await defaultApi.signin({ email: DEMO_EMAIL, password: DEMO_PASSWORD });
    window.history.replaceState({}, "", window.location.pathname + window.location.search);
  }
}

export function StudioContainer({
  api = defaultApi,
  ensureSession = defaultEnsureSession,
}: {
  api?: StudioApi;
  ensureSession?: () => Promise<void>;
}) {
  const [state, setState] = useState<StudioState | null>(null);
  const [activeStation, setActiveStation] = useState<StationCode | null>(null);
  const [focusMode, setFocusMode] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        await ensureSession();
        const list = await api.listProjects();
        if (list.length === 0) throw new Error("no projects");
        const proj = await api.getProject(list[0]!.id);
        if (cancelled) return;
        const s = toStudioState(proj);
        setState(s);
        setActiveStation(s.activeStation);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, [api, ensureSession]);

  if (error) return <div className="mk-studio-error">{error}</div>;
  if (!state || !activeStation) return <div className="mk-studio-loading">正在加载工作室…</div>;

  const callbacks: StudioCallbacks = {
    onSelectStation: setActiveStation,          // client-local view switch
    onToggleFocus: () => setFocusMode((f) => !f),
    onDisposition: () => { /* wired in 5c */ },
    onOpenMethodology: () => { /* client-live; StudioShell owns modal state */ },
    onComposerSend: () => { /* wired in 5c */ },
  };

  return <StudioShell state={{ ...state, activeStation, focusMode }} callbacks={callbacks} />;
}
