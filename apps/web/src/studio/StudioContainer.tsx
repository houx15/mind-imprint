import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import type { StudioProjection } from "@mind-imprint/contracts";
import { api as defaultApi } from "../api";
import { StudioShell } from "./StudioShell";
import { createStudioConversation } from "./conversation";
import type { StationCode, StudioState, StudioCallbacks } from "./state";

type StudioApi = { listProjects: typeof defaultApi.listProjects; getProject: typeof defaultApi.getProject };
type StudioConversation = ReturnType<typeof createStudioConversation>;
type ConvSnapshot = ReturnType<StudioConversation["getSnapshot"]>;

// Reuse the trial-signin creds/sequence from AppShell.tsx:26-27 so ?studio is
// reachable without a class join-code. Public demo creds — see AppShell's
// comment for the seed-migration source.
const DEMO_EMAIL = "phoebe@demo.mindimprint.local";
const DEMO_PASSWORD = "phoebe-dev-pass";

// Stable fallback used only before the conversation controller exists yet
// (project still loading) — useSyncExternalStore needs a store, and this
// constant reference (defined once at module scope, not per-render) keeps
// getSnapshot referentially stable across renders so it never falsely
// signals a change.
const EMPTY_CONV_SNAPSHOT: ConvSnapshot = { messages: [], sending: false, error: null, disposableInterventionId: null };
const emptyConvSubscribe = () => () => {};
const emptyConvGetSnapshot = () => EMPTY_CONV_SNAPSHOT;

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
  makeConversation = createStudioConversation,
}: {
  api?: StudioApi;
  ensureSession?: () => Promise<void>;
  makeConversation?: typeof createStudioConversation;
}) {
  const [state, setState] = useState<StudioState | null>(null);
  const [projectId, setProjectId] = useState<string | null>(null);
  const [activeStation, setActiveStation] = useState<StationCode | null>(null);
  const [focusMode, setFocusMode] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conv, setConv] = useState<StudioConversation | null>(null);
  // Guards double-dispatch of the same disposition (carry-forward from Task
  // 10's review): the conversation controller itself doesn't clear
  // disposableInterventionId after a dispose call, so a second click before
  // a new intervention arrives would re-POST the same id.
  const disposedIdRef = useRef<string | null>(null);

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
        setProjectId(list[0]!.id);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, [api, ensureSession]);

  // Create the live conversation once the projectId is known — guarded by
  // the projectId dependency so it isn't recreated on every render.
  useEffect(() => {
    if (!projectId) return;
    setConv(makeConversation({ projectId }));
  }, [projectId, makeConversation]);

  // Subscribe to the live conversation's turns via the app's established
  // external-store pattern (matches agent/useConversation.ts). Falls back to
  // a stable empty snapshot before `conv` exists (project still loading).
  const convSnapshot = useSyncExternalStore(
    conv?.subscribe ?? emptyConvSubscribe,
    conv?.getSnapshot ?? emptyConvGetSnapshot,
  );

  if (error) return <div className="mk-studio-error">{error}</div>;
  if (!state || !activeStation) return <div className="mk-studio-loading">正在加载工作室…</div>;

  const callbacks: StudioCallbacks = {
    onSelectStation: setActiveStation,          // client-local view switch
    onToggleFocus: () => setFocusMode((f) => !f),
    onDisposition: (choice, reason) => {
      const id = convSnapshot.disposableInterventionId;
      if (id && disposedIdRef.current === id) return; // already dispatched for this intervention
      disposedIdRef.current = id;
      conv?.dispose(choice, reason);
    },
    onOpenMethodology: () => { /* client-live; StudioShell owns modal state */ },
    onComposerSend: (text) => conv?.send(text),
  };

  // Projection = history on load; controller = this session's live turns.
  const mergedState: StudioState = {
    ...state,
    coach: { ...state.coach, messages: [...state.coach.messages, ...convSnapshot.messages] },
  };

  return (
    <StudioShell
      state={{ ...mergedState, activeStation, focusMode }}
      callbacks={callbacks}
      sending={convSnapshot.sending}
    />
  );
}
