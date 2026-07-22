import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import type { Anchor, StudioProjection, TraceEvent } from "@mind-imprint/contracts";
import { api as defaultApi, ApiError } from "../api";
import type { ProjectListItem } from "../api/projects";
import { StudioShell } from "./StudioShell";
import { Directory } from "./Directory";
import { createStudioConversation, activeCardToState } from "./conversation";
import type { StationCode, StudioState, StudioCallbacks } from "./state";
import type { LocatedSpan } from "./StudioAnnotateCard";

// Narrow structural type, widened (Task 9) to the two ingestion/logging
// calls the 素材 dossier now needs, then (N1 Task 7) to createProject for the
// directory-first create flow — still a Pick off the real ApiClient so test
// fixtures keep injecting plain object literals for just the calls a given
// test actually exercises.
type StudioApi = Pick<
  typeof defaultApi,
  "listProjects" | "getProject" | "createProject" | "addMaterial" | "logSourceOpen" | "putBuffer" | "commitSnapshot" | "orderReview" | "postDisposition" | "attestGate" | "finishProject" | "submitOnboarding" | "submitSelfScore" | "submitReflection"
>;

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

// Map the lean wire projection into the frontend view-model. material (6b),
// structure (7b), writing (Slice 8 Task 9), and review/readiness (Slice 9
// Task 5) are all live — projected straight from the server.
function toStudioState(p: StudioProjection): StudioState {
  return {
    project: p.project,
    stations: p.stations,
    activeStation: p.activeStation,
    focusMode: false,
    coach: p.coach,
    views: {
      material: p.materials,
      structure: p.structure,
      writing: p.writing,
      review: p.readiness ?? [],
      onboarding: p.onboarding,
      // N2 Task 8: same defensive fallback as `readiness` above — older test
      // fixtures/mocks predating this slice may omit these fields entirely.
      selfScore: p.selfScore ?? { dims: [], bands: [] },
      prediction: p.prediction ?? { predicted: [], actual: [], overlap: 0, revealed: false },
      reflection: p.reflection ?? { text: "", prompts: [] },
    },
    finished: p.finished,
    canFinish: p.canFinish,
  };
}

export function StudioContainer({
  api = defaultApi,
  makeConversation = createStudioConversation,
  onFinished,
}: {
  api?: StudioApi;
  makeConversation?: typeof createStudioConversation;
  // A3 Task 9: fired once `api.finishProject` resolves — the shell routes to
  // the 成长报告 tab. Optional so standalone/story usages of StudioContainer
  // (which never need tab routing) don't have to supply it.
  onFinished?: () => void;
}) {
  const [state, setState] = useState<StudioState | null>(null);
  const [projectId, setProjectId] = useState<string | null>(null);
  const [activeStation, setActiveStation] = useState<StationCode | null>(null);
  const [focusMode, setFocusMode] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conv, setConv] = useState<StudioConversation | null>(null);
  // N1 Task 7: directory-first. `projects` holds the student's list (fetched
  // on mount, refreshed after a create); `openId` is the project currently
  // opened into the studio — null means the <Directory> is showing, not the
  // studio. `creating` is the transient in-flight state of the 新建论文 form.
  const [projects, setProjects] = useState<ProjectListItem[]>([]);
  const [openId, setOpenId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  // Slice 6b Task 9: the server's own honest Chinese message from the last
  // failed 添加信源 attempt — cleared on the next successful add.
  const [addSourceError, setAddSourceError] = useState<string | undefined>(undefined);
  // A3 Task 9: the project terminal's own transient state — not a callback
  // result folded into StudioCallbacks (same reasoning as addSourceError
  // above: this is the student-facing outcome of the last finish attempt,
  // not a callback itself).
  const [finishing, setFinishing] = useState(false);
  const [finishError, setFinishError] = useState<string | null>(null);
  // Fix-wave bug [B]: submitCard nulls `card` on its SSE "done" frame, well
  // before refetchProject's GET lands with the persisted anchors — without
  // this, ViewFrame's `card?.anchors ?? []` goes empty for that whole round
  // trip and the article's highlights visibly blink out. Held as a
  // transient overlay from the moment of submit until the refetch resolves
  // (success OR failure — see onSubmitCard below), so there is never a
  // frame with neither the live card's anchors nor the refreshed persisted
  // ones.
  const [pendingAnchors, setPendingAnchors] = useState<Anchor[] | null>(null);
  // Whole-branch review finding [3]: the student's in-progress lateral-
  // source pick for the active compare (SIFT) card. Previously private to
  // StudioCompareCard, so nothing else in the Studio — least of all
  // ViewFrame's center-pane Compare primitive, the whole reason SIFT exists
  // — could ever see it. This is the smallest state location that is
  // visible to BOTH the coach rail (which sets it, via StudioCompareCard's
  // lateral-material picker) and the center pane (which reads it to fill
  // the right pane live): StudioContainer already owns the projection, the
  // live `card`, and every other cross-pane concern (pendingAnchors above
  // is the same pattern for a different fix-wave finding). Reset whenever
  // the ACTIVE card instance changes — not on every conversation snapshot,
  // which would wipe the student's own pick mid-fill.
  const [lateralMaterialId, setLateralMaterialId] = useState<string>("");
  // N3c task 9 (spec §8): the student's in-progress "go find this sentence
  // in the article" request from a locate/elicit-mode anchor's 「去文章里选
  // 出这句」 control — the smallest state location visible to BOTH the coach
  // rail (which sets it, via the card's onRequestLocate) and the center pane
  // (which reads it to force the right source open and put Annotate in
  // select mode), same lifting rationale as `lateralMaterialId` above.
  const [locating, setLocating] = useState<{ anchorId: string; dimension: string; materialId: string } | null>(null);
  // The spans she has located herself, keyed by anchor id — handed to
  // StudioAnnotateCard as `locatedSpans` so its OWN handleLock merges them
  // onto the matching anchor; this container never reaches into the card's
  // envelope directly (brief: "the card stays the single builder of its own
  // submit envelope").
  const [locatedSpans, setLocatedSpans] = useState<Record<string, LocatedSpan>>({});
  // The span_located/span_not_found trace accumulated for the CURRENT card
  // instance (spec §6), threaded into the card as `pendingTrace` so IT still
  // assembles the final event_trace rather than this container mutating the
  // envelope after the fact. Only this container can honestly timestamp a
  // `span_located` event — it happens on the article pane, at selection
  // time, not whenever the card later gets around to locking.
  const [spanTrace, setSpanTrace] = useState<TraceEvent[]>([]);
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
  // Fix-wave-3 findings [1]+[2]: request-generation guard for overlapping
  // refetchProject calls (e.g. an add-source refetch still in flight when a
  // card lock issues its own). Without it, whichever GET happens to RESOLVE
  // last wins regardless of which was ISSUED last, and each one's
  // dropFirst(n) applies its own n — captured against the buffer at ITS
  // issue time — to whatever buffer exists when IT lands, which may since
  // have been shifted by the other. Incremented synchronously the moment a
  // refetch is issued; a response is applied only if it is still the
  // latest-issued one by the time it settles, so the last-ISSUED refetch
  // always wins and every dropFirst(n) is checked against the buffer it was
  // actually measured against.
  const refetchGenRef = useRef(0);
  // CRITICAL (whole-branch review): the server's own activeCard (an open
  // card_instance, status proposed/active) captured from the FIRST project
  // fetch, for the conversation-creation effect below to seed as its initial
  // card. Without this, a page reload while a card is open shows NO card
  // (the client never projected one — it only ever sourced `card` from the
  // SSE conversation snapshot), while the row stays proposed/active
  // server-side — and FIX-D's own suppression then blocks EVERY future card
  // from surfacing, ever, project-wide. A ref (not state) because it only
  // needs to be read once, synchronously, by the second effect below, which
  // fires in the same commit as `setProjectId` — no re-render round trip to
  // race.
  const initialActiveCardRef = useRef<ReturnType<typeof activeCardToState>>(null);
  // Slice 8 Task 9: debounce timer for the 写作 view's silent-edit buffer
  // autosave — the textarea itself is a controlled input updated
  // synchronously on every keystroke (below), but the PUT /buffer network
  // call is debounced so typing doesn't fire a request per character.
  const bufferSaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Clears any pending debounced save on unmount — a fired setTimeout after
  // unmount would call setState-less `api.putBuffer` fine (it doesn't touch
  // React state), but leaving it dangling is needless work for a screen
  // nobody is looking at anymore, and it would race a fresh mount's own
  // timer if the Studio ever remounted with the same projectId.
  useEffect(() => () => { if (bufferSaveTimerRef.current) clearTimeout(bufferSaveTimerRef.current); }, []);

  // N1 Task 7: on mount, fetch the student's project list into state — do NOT
  // auto-open. The <Directory> renders from this; opening a row (or creating)
  // is what sets `openId`, which the projection-load effect below reacts to.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await api.listProjects();
        if (cancelled) return;
        setProjects(list);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, [api]);

  // N1 Task 7: load the opened project's projection whenever `openId` changes
  // (student clicked a directory row, or a create just resolved). Keyed on
  // `openId` rather than `list[0]` — this is what makes the studio show the
  // project the student actually chose. Seeds the same initialActiveCardRef /
  // projectId the conversation-creation effect below still consumes.
  useEffect(() => {
    if (!openId) return;
    let cancelled = false;
    (async () => {
      try {
        const proj = await api.getProject(openId);
        if (cancelled) return;
        const s = toStudioState(proj);
        setState(s);
        setActiveStation(s.activeStation);
        initialActiveCardRef.current = activeCardToState(proj.activeCard);
        setProjectId(openId);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, [openId, api]);

  // Create the live conversation once the projectId is known — guarded by
  // the projectId dependency so it isn't recreated on every render. Seeded
  // with the server's own open card (if any) so a fresh mount rehydrates it
  // instead of showing nothing while FIX-D's suppression waits forever for a
  // "done" that will never come from a client that never knew the card
  // existed.
  useEffect(() => {
    if (!projectId) return;
    setConv(makeConversation({ projectId, initialCard: initialActiveCardRef.current }));
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
    // This refetch's generation — claimed synchronously, before the await,
    // so an overlapping refetch issued afterward always claims a strictly
    // higher number.
    const myGen = ++refetchGenRef.current;
    const priorMessageCount = conv?.getSnapshot().messages.length ?? 0;
    try {
      const proj = await api.getProject(projectId);
      // A newer refetch has since been issued — this response is stale.
      // Discard it entirely: no setState, no dropFirst, no error surfaced.
      // The newer refetch (still in flight or already landed) owns both the
      // projection and the reporting from here.
      if (refetchGenRef.current !== myGen) return;
      setState(toStudioState(proj));
      conv?.dropFirst(priorMessageCount);
      setSyncError(null);
      // Bug [4]: the overlay has served its purpose the moment the
      // projection is refreshed — clear it on the winning response.
      setPendingAnchors(null);
    } catch (err) {
      if (refetchGenRef.current !== myGen) return;
      // Bug [4]: also clear on the winning response's FAILURE path — a
      // failed refetch never delivers the persisted anchors this overlay
      // was standing in for, so there is nothing left for it to guard.
      setPendingAnchors(null);
      throw err;
    }
  };

  // N1 Task 8: the S0 view's restate + weak-picks submit — a pure DB write
  // (no llm_call, no migration), so unlike onSubmitCard/onCommit above there
  // is nothing to reconcile beyond re-pulling the projection so the
  // textarea/rubric-picks re-hydrate from what actually persisted.
  const submitOnboarding = async (body: { restate: string; weakPicks: number[] }) => {
    if (!projectId) return;
    await api.submitOnboarding(projectId, body);
    await refetchProject();
  };

  // N2 Task 8: the 评估 view's self-score + retro submits — same shape as
  // submitOnboarding above (pure DB writes, no llm_call, refetch to
  // rehydrate from what actually persisted).
  const submitSelfScore = async (body: { scores: { code: string; band: number }[] }) => {
    if (!projectId) return;
    await api.submitSelfScore(projectId, body);
    await refetchProject();
  };

  const submitReflection = async (body: { text: string }) => {
    if (!projectId) return;
    await api.submitReflection(projectId, body);
    await refetchProject();
  };

  // A3 Task 9: the project terminal — POSTs the finish, then routes to the
  // 成长报告 tab via `onFinished`. Deliberately does NOT refetchProject on
  // success: the project is closed out and StudentApp is about to unmount
  // this whole surface for the growth tab, so there is nothing left here to
  // keep in sync.
  async function handleFinish() {
    if (!projectId || finishing) return;
    setFinishing(true);
    setFinishError(null);
    try {
      await api.finishProject(projectId);
      onFinished?.();
    } catch {
      setFinishError("归档失败，请重试");
    } finally {
      setFinishing(false);
    }
  }

  // N1 Task 7: the 新建论文 create flow — POST the new project, refresh the
  // list so the directory is current, then open straight into it. `creating`
  // gates the form's submit; the finally guarantees it clears even if the
  // create or the refresh throws.
  const handleCreate = async (body: { title: string; prompt: string }) => {
    setCreating(true);
    try {
      const { id } = await api.createProject(body);
      setProjects(await api.listProjects());
      setOpenId(id);
    } finally {
      setCreating(false);
    }
  };

  // N1 Task 7: ← 返回 — drop back to the directory. Tears down the opened
  // project's studio state (projection, live conversation, view-local station/
  // focus) so a later re-open loads cleanly rather than flashing the previous
  // project's projection.
  const handleBack = () => {
    setOpenId(null);
    setProjectId(null);
    setState(null);
    setActiveStation(null);
    setConv(null);
    setFocusMode(false);
    setError(null);
  };

  // Subscribe to the live conversation's turns via the app's established
  // external-store pattern (matches agent/useConversation.ts). Falls back to
  // a stable empty snapshot before `conv` exists (project still loading).
  const convSnapshot = useSyncExternalStore(
    conv?.subscribe ?? emptyConvSubscribe,
    conv?.getSnapshot ?? emptyConvGetSnapshot,
  );

  // Resets `lateralMaterialId` whenever the ACTIVE card instance changes —
  // a new compare card starts with no pick, and a card going null (submit
  // resolved, or she skipped) clears any stale pick before the next one
  // surfaces. Keyed on cardInstanceId alone (not the whole `card` object,
  // which gets a new reference on every conversation snapshot) so it does
  // NOT re-fire — and clobber the student's own in-progress pick — merely
  // because `sending` flipped or a new AI message arrived while she's still
  // filling the same card. Re-seeds from any lateral-dimension anchor the
  // card already carries (parity with the pre-lift behavior this replaces:
  // higher guidance levels may one day pre-seed this via `anchors`).
  const activeCardInstanceId = convSnapshot.card?.cardInstanceId ?? null;
  useEffect(() => {
    const c = convSnapshot.card;
    if (!c) {
      setLateralMaterialId("");
      setLocating(null);
      setLocatedSpans({});
      setSpanTrace([]);
      return;
    }
    const dim = ((c.spec.params ?? {}) as { lateral_dimension?: string }).lateral_dimension ?? "";
    const seeded = dim ? c.anchors.find((a) => a.dimension === dim && a.material_id !== "")?.material_id ?? "" : "";
    setLateralMaterialId(seeded);
    // N3c task 9: a NEW card instance must not inherit the PREVIOUS one's
    // in-progress locate request/located spans/pending trace — mirrors the
    // lateralMaterialId reset above and its own FIX-C precedent (a stale
    // pick silently bleeding into the next instance). Without this, a fresh
    // CRAAP card would show anchor A's located sentence attached to a
    // different anchor, or submit with a stale span_located event from a
    // card she already locked.
    setLocating(null);
    setLocatedSpans({});
    setSpanTrace([]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeCardInstanceId]);

  // N1 Task 7: directory-first. A load error (listProjects, or a failed
  // open) is surfaced first; otherwise, nothing open ⇒ the <Directory>
  // (its own create form carries the honest empty affordance, replacing the
  // old static "还没有项目" placeholder). Only an actually-opened project
  // falls through to the studio render below.
  if (error) return <div className="mk-studio-error">{error}</div>;
  if (openId == null) {
    return <Directory projects={projects} onOpen={setOpenId} onCreate={handleCreate} creating={creating} />;
  }
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
        .catch(() => {
          // The submit itself already landed server-side — only the refresh
          // that would confirm the lock/anchors failed. Surface that
          // instead of an unhandled rejection or silently rendering as if
          // the lock succeeded (bug [C]). refetchProject's own catch clause
          // has already cleared the anchor overlay (bug [4]) before this
          // rethrows.
          setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
        });
    },
    onSkipCard: (eventTrace) => {
      // Bug [3]: skipCard (unlike submitCard) has no internal try/catch —
      // it rejects when the skip API call itself fails, which is a
      // genuinely different failure from a post-skip refetch failing. The
      // two-argument .then() keeps them apart: the reject handler only ever
      // sees "the skip itself didn't go through" (the card correctly stays
      // open), never a refetch problem.
      conv?.skipCard(eventTrace).then(
        () =>
          refetchProject().catch(() => {
            setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
          }),
        () => {
          setSyncError("跳过失败，请重试");
        },
      );
    },
    // N3c task 9 (spec §8): 「去文章里选出这句」 — switches the station to
    // 素材 and forces the card's OWN material open (never guessed: read off
    // the anchor's own material_id, the same "never guess, read the edge"
    // discipline StudioCompareCard's materialId prop already follows —
    // whole-branch review finding [5]).
    onRequestLocate: (anchorId, dimension) => {
      const materialId = convSnapshot.card?.anchors.find((a) => a.id === anchorId)?.material_id ?? "";
      setLocating({ anchorId, dimension, materialId });
      setActiveStation("S3");
    },
    // 铁律 4 · 过程即数据: a dimension she looked for and could not find is
    // DATA, not an error — recorded as its own trace event, never surfaced
    // as a failure.
    onSpanNotFound: (anchorId, dimension) => {
      setSpanTrace((prev) => [...prev, { kind: "span_not_found", dimension, at: new Date().toISOString() }]);
    },
    // She released the mouse over a real selection in the article pane —
    // write it onto the anchor she was locating, record when it happened
    // (the ONLY honest place to timestamp this — see `spanTrace`'s doc
    // comment), and clear the pending request so the hint bar disappears.
    onCreateSpan: (span) => {
      if (!locating) return;
      setLocatedSpans((prev) => ({
        ...prev,
        [locating.anchorId]: { block_id: span.blockId, start: span.start, end: span.end, quote: span.text },
      }));
      setSpanTrace((prev) => [...prev, { kind: "span_located", dimension: locating.dimension, block_id: span.blockId, at: new Date().toISOString() }]);
      setLocating(null);
    },
    // The article pane's own inline "取消" — she's stepping back from THIS
    // attempt without having decided anything, unlike the card's own 「找不
    // 到合适的句子」 escape (onSpanNotFound above), which records data. No
    // trace event: nothing happened yet to record.
    onCancelLocate: () => setLocating(null),
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
    onBufferChange: (text) => {
      // Optimistic local update FIRST — the textarea is controlled by
      // state.views.writing.buffer, so without this the keystroke would
      // never appear (it'd just get overwritten back to the last-fetched
      // value on the next render). The network PUT is debounced separately
      // below; the student's own view of what she typed is never delayed.
      setState((prev) => (prev ? { ...prev, views: { ...prev.views, writing: { ...prev.views.writing, buffer: text } } } : prev));
      if (!projectId) return;
      if (bufferSaveTimerRef.current) clearTimeout(bufferSaveTimerRef.current);
      bufferSaveTimerRef.current = setTimeout(() => {
        api.putBuffer(projectId, text).catch(() => {
          setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
        });
      }, 600);
    },
    onCommit: async (text) => {
      if (!projectId) return;
      // WritingView awaits this to know whether to switch to preview (spec
      // §7) — must return (not fire-and-forget) the promise, and rethrow
      // after surfacing the error so a failed commit does NOT flip the view
      // to a preview of a draft that was never actually captured.
      try {
        await api.commitSnapshot(projectId, text);
      } catch (err) {
        setSyncError("提交失败，请重试。");
        throw err;
      }
      // The commit itself succeeded — a refetch hiccup from here is display
      // staleness, not a failed commit (same split as onAddSource above): it
      // gets the generic sync-error banner and does NOT block the view from
      // switching to preview.
      try {
        await refetchProject();
      } catch {
        setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
      }
    },
    // Slice 8 Task 10: 整稿体检 — orderReview is a plain SSE generator (no
    // conversation controller involved, unlike submitCard/skipCard above).
    // The stream's own "review" items are raw model output with no
    // interventionId/disposition (studioTurn.ts's doc comment); the
    // work-order's actual render shape only exists once the projection is
    // refetched, so this drains the generator purely to know when the
    // review is done (and to surface a review_rejected error, if any)
    // before refetching.
    onOrderReview: (snapshotId, voice) => {
      if (!projectId) return;
      (async () => {
        for await (const ev of api.orderReview(projectId, snapshotId, voice)) {
          if (ev.type === "error") setSyncError(ev.message || "体检失败，请重试。");
        }
        await refetchProject();
      })().catch(() => {
        setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
      });
    },
    // The review work-order's own three-key disposition — same disposition
    // endpoint Slice 5c already wired for the coach rail's own intervention
    // disposal (api.postDisposition), just keyed off a review_item
    // intervention instead of the live conversation's disposableInterventionId.
    onReviewDisposition: (interventionId, action, reason) => {
      if (!projectId) return;
      api
        .postDisposition(projectId, interventionId, action, reason)
        .then(() => refetchProject())
        .catch(() => {
          setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
        });
    },
    // The S5 student-written citations_matched gate item — draft_polish is
    // the only contract this attestation is ever recorded against.
    onAttestCitations: (confirmed) => {
      if (!projectId) return;
      api
        .attestGate(projectId, "draft_polish", "citations_matched", confirmed)
        .then(() => refetchProject())
        .catch(() => {
          setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
        });
    },
  };

  // Projection = history on load; controller = this session's live turns.
  const mergedState: StudioState = {
    ...state,
    coach: { ...state.coach, messages: [...state.coach.messages, ...convSnapshot.messages] },
  };

  return (
    <>
      {/* N1 Task 7: ← 返回 to the directory. */}
      <button
        type="button"
        aria-label="返回"
        onClick={handleBack}
        style={{
          position: "fixed",
          top: 10,
          left: 12,
          zIndex: 60,
          display: "inline-flex",
          alignItems: "center",
          gap: 6,
          background: "#fff",
          border: "1px solid #EAECF2",
          color: "#3A4256",
          borderRadius: 10,
          padding: "7px 12px",
          fontSize: 12.5,
          fontWeight: 700,
          cursor: "pointer",
          fontFamily: "inherit",
          boxShadow: "0 2px 8px rgba(20,30,60,.08)",
        }}
      >
        ← 返回
      </button>
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
        lateralMaterialId={lateralMaterialId}
        onLateralMaterialChange={setLateralMaterialId}
        locating={locating}
        locatedSpans={locatedSpans}
        pendingTrace={spanTrace}
        review={{ finishing, finishError, onFinish: handleFinish, onSelfScore: submitSelfScore, onReflection: submitReflection }}
        onSubmitOnboarding={submitOnboarding}
      />
    </>
  );
}
