import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import type { Anchor, StudioProjection } from "@mind-imprint/contracts";
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
  // Fix-wave bug [B]: submitCard nulls `card` on its SSE "done" frame, well
  // before refetchProject's GET lands with the persisted anchors — without
  // this, ViewFrame's `card?.anchors ?? []` goes empty for that whole round
  // trip and the article's highlights visibly blink out. Held as a
  // transient overlay from the moment of submit until the refetch resolves
  // (success OR failure — see onSubmitCard below), so there is never a
  // frame with neither the live card's anchors nor the refreshed persisted
  // ones.
  const [pendingAnchors, setPendingAnchors] = useState<Anchor[] | null>(null);
  // Fix-wave bugs [C]/[D]: a refetch that fails after a submit/skip/add
  // already succeeded server-side must not be an unhandled rejection and
  // must not be misreported as that mutation having failed — this is the
  // one honest, generic "the screen may be behind the server" notice for
  // that whole class of failure.
  const [syncError, setSyncError] = useState<string | null>(null);
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

  // The projection is the single source of truth for everything the server
  // derives (materials/locked/role/anchors, the coach thread's persisted
  // history, …) — refetch it any time a turn changes server state instead of
  // hand-patching local state. Reused by add-source, card submit, and card
  // skip; must not touch activeStation/focusMode (client-local view state)
  // or blow away an in-flight conv beyond its own message buffer.
  //
  // conv.dropFirst(n) empties the conversation controller's local turn
  // buffer up to the point the GET was issued, once the refetched projection
  // lands: projectCoach rebuilds the thread from persisted interventions +
  // chat_messages, which by then includes this session's own turns — so
  // without dropping them, CoachRail (keyed by array index, no dedupe) would
  // render every one of them twice. `n` is captured BEFORE the GET fires —
  // fix-wave bug [A] — so a turn that enters the buffer during the round
  // trip (the student keeps chatting while the request is in flight) is
  // never in that prefix and always survives, instead of being silently
  // deleted by an unconditional clear.
  const refetchProject = async () => {
    if (!projectId) return;
    const priorMessageCount = conv?.getSnapshot().messages.length ?? 0;
    const proj = await api.getProject(projectId);
    setState(toStudioState(proj));
    conv?.dropFirst(priorMessageCount);
    setSyncError(null);
  };

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
    onSubmitCard: (finalEnvelope) => {
      // Capture the live card's anchors BEFORE submitCard's SSE "done" frame
      // clears `card` — held as an overlay so ViewFrame never sees a frame
      // with no anchors at all (bug [B]).
      setPendingAnchors(convSnapshot.card?.anchors ?? null);
      conv
        ?.submitCard(finalEnvelope)
        .then(() => refetchProject())
        .then(() => setPendingAnchors(null))
        .catch(() => {
          // The submit itself already landed server-side — only the refresh
          // that would confirm the lock/anchors failed. Surface that
          // instead of an unhandled rejection or silently rendering as if
          // the lock succeeded (bug [C]). Keep the anchor overlay: since the
          // refetch didn't land, the persisted anchors it would have
          // supplied never arrived either.
          setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
        });
    },
    onSkipCard: (eventTrace) => {
      conv
        ?.skipCard(eventTrace)
        .then(() => refetchProject())
        .catch(() => {
          setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
        });
    },
    onAddSource: async (body) => {
      if (!projectId) return;
      try {
        await api.addMaterial(projectId, body);
      } catch (err) {
        setAddSourceError(err instanceof ApiError ? err.message : "添加信源失败，请重试");
        throw err; // AddSourceForm relies on the rejection to skip its own reset()
      }
      // The add itself succeeded — clear any previous add-error. From here
      // on a failure belongs to the refresh, not the add (bug [D]): it must
      // not be reported as "添加信源失败" (which would leave the form primed
      // to re-submit and create a duplicate source) and it must not
      // silently pretend the dossier is already current.
      setAddSourceError(undefined);
      try {
        // The projection is the single source of truth for 素材 — refetch
        // rather than hand-patch local state so the new source (and any
        // server-side derivations of it) render exactly as stored.
        await refetchProject();
      } catch {
        setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
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
    <>
      {syncError && (
        <div
          role="status"
          className="mk-studio-sync-warning"
          style={{
            position: "fixed",
            top: 10,
            left: "50%",
            transform: "translateX(-50%)",
            zIndex: 60,
            background: "#FBEEE7",
            border: "1px solid #F1D6C8",
            color: "#C96F4F",
            borderRadius: 10,
            padding: "9px 15px",
            fontSize: 12.5,
            fontWeight: 600,
            boxShadow: "0 4px 14px rgba(20,30,60,.14)",
          }}
        >
          {syncError}
        </div>
      )}
      <StudioShell
        state={{ ...mergedState, activeStation, focusMode }}
        callbacks={callbacks}
        sending={convSnapshot.sending}
        card={convSnapshot.card}
        pendingAnchors={pendingAnchors}
        addSourceError={addSourceError}
      />
    </>
  );
}
