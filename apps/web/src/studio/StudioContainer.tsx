import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import type { StudioProjection } from "@mind-imprint/contracts";
import { api as defaultApi, ApiError } from "../api";
import { StudioShell } from "./StudioShell";
import { createStudioConversation } from "./conversation";
import type { StationCode, StudioState, StudioCallbacks } from "./state";

// Narrow structural type, widened (Task 9) to the two ingestion/logging
// calls the 素材 dossier now needs — still a Pick off the real ApiClient so
// test fixtures keep injecting plain object literals for just the calls a
// given test actually exercises.
type StudioApi = Pick<typeof defaultApi, "listProjects" | "getProject" | "addMaterial" | "logSourceOpen">;

type StudioConversation = ReturnType<typeof createStudioConversation>;
type ConvSnapshot = ReturnType<StudioConversation["getSnapshot"]>;

// Stable fallback used only before the conversation controller exists yet
// (project still loading) — useSyncExternalStore needs a store, and this
// constant reference (defined once at module scope, not per-render) keeps
// getSnapshot referentially stable across renders so it never falsely
// signals a change.
const EMPTY_CONV_SNAPSHOT: ConvSnapshot = { messages: [], sending: false, error: null, disposableInterventionId: null, card: null };
const emptyConvSubscribe = () => () => {};
const emptyConvGetSnapshot = () => EMPTY_CONV_SNAPSHOT;

// Map the lean wire projection into the frontend view-model, stubbing the
// deferred center-pane views (structure/writing/review → Slices 7/8/9).
// material is live as of Slice 6b — projected straight from the server.
function toStudioState(p: StudioProjection): StudioState {
  return {
    project: p.project,
    stations: p.stations,
    activeStation: p.activeStation,
    focusMode: false,
    coach: p.coach,
    views: {
      material: p.materials,
      structure: [],
      writing: { draft: "", mode: "edit" },
      review: [],
      onboarding: p.onboarding,
    },
  };
}

export function StudioContainer({
  api = defaultApi,
  makeConversation = createStudioConversation,
}: {
  api?: StudioApi;
  makeConversation?: typeof createStudioConversation;
}) {
  const [state, setState] = useState<StudioState | null>(null);
  const [projectId, setProjectId] = useState<string | null>(null);
  const [activeStation, setActiveStation] = useState<StationCode | null>(null);
  const [focusMode, setFocusMode] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [empty, setEmpty] = useState(false);
  const [conv, setConv] = useState<StudioConversation | null>(null);
  // Slice 6b Task 9: the server's own honest Chinese message from the last
  // failed 添加信源 attempt — cleared on the next successful add.
  const [addSourceError, setAddSourceError] = useState<string | undefined>(undefined);
  // Guards double-dispatch of the same disposition (carry-forward from Task
  // 10's review): the conversation controller itself doesn't clear
  // disposableInterventionId after a dispose call, so a second click before
  // a new intervention arrives would re-POST the same id.
  const disposedIdRef = useRef<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await api.listProjects();
        if (cancelled) return;
        if (list.length === 0) { setEmpty(true); return; }
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
  }, [api]);

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

  if (empty) {
    return (
      <div className="mk-studio-empty" style={{ height: "100%", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 10, color: "#6B7384" }}>
        <div style={{ fontSize: 16, fontWeight: 700, color: "#3A4256" }}>还没有项目</div>
        <div style={{ fontSize: 13.5, lineHeight: 1.7, maxWidth: 420, textAlign: "center" }}>
          工作室从一个真实的写作任务开始。创建入口马上就来——在那之前，这里会保持空着。
        </div>
      </div>
    );
  }
  if (error) return <div className="mk-studio-error">{error}</div>;
  if (!state || !activeStation) return <div className="mk-studio-loading">正在加载工作室…</div>;

  const callbacks: StudioCallbacks = {
    onSelectStation: setActiveStation,          // client-local view switch
    onToggleFocus: () => setFocusMode((f) => !f),
    onDisposition: (choice, reason) => {
      const id = convSnapshot.disposableInterventionId;
      if (id && disposedIdRef.current === id) return; // already dispatched for this intervention
      disposedIdRef.current = id;
      conv?.dispose(choice, reason).catch(() => { disposedIdRef.current = null; });
    },
    onOpenMethodology: () => { /* client-live; StudioShell owns modal state */ },
    onComposerSend: (text) => conv?.send(text),
    onOpenCard: () => conv?.openCard(),
    onSubmitCard: (finalEnvelope) => conv?.submitCard(finalEnvelope),
    onSkipCard: (eventTrace) => conv?.skipCard(eventTrace),
    onAddSource: async (body) => {
      if (!projectId) return;
      try {
        await api.addMaterial(projectId, body);
        setAddSourceError(undefined);
        // The projection is the single source of truth for 素材 — refetch
        // rather than hand-patch local state so the new source (and any
        // server-side derivations of it) render exactly as stored.
        const proj = await api.getProject(projectId);
        setState(toStudioState(proj));
      } catch (err) {
        setAddSourceError(err instanceof ApiError ? err.message : "添加信源失败，请重试");
        throw err; // AddSourceForm relies on the rejection to skip its own reset()
      }
    },
    onOpenLogged: (materialId, timeSpentS) => {
      if (!projectId) return;
      // Fire-and-forget: a lost reading-time sample must never surface as
      // an error to the student.
      api.logSourceOpen(projectId, materialId, timeSpentS).catch(() => {});
    },
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
      card={convSnapshot.card}
      addSourceError={addSourceError}
    />
  );
}
