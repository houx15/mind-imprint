import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import type { Anchor, StudioProjection, TraceEvent } from "@mind-imprint/contracts";
import { api as defaultApi, ApiError } from "../api";
import type { ProjectListItem } from "../api/projects";
import { StudioShell } from "./StudioShell";
import { Directory } from "./Directory";
import { ReadingRoom } from "./reading/ReadingRoom";
import { createStudioConversation, activeCardToState } from "./conversation";
import type { StationCode, StudioState, StudioCallbacks, SpotCheckContractId } from "./state";
import type { LocatedSpan } from "./StudioAnnotateCard";

// Narrow structural type, widened (Task 9) to the two ingestion/logging
// calls the 素材 dossier now needs, then (N1 Task 7) to createProject for the
// directory-first create flow — still a Pick off the real ApiClient so test
// fixtures keep injecting plain object literals for just the calls a given
// test actually exercises.
// Task 10: widened again to the five read-together loop calls
// (readTurn/activateProjectCard/evaluateCardSelection/submitProjectCard/
// skipProjectCard) — ReadingRoom's own `useReadingLoop` needs them, and
// ReadingRoom is handed this same `api` object below (structurally, it just
// needs to satisfy `ReadingLoopApi`).
type StudioApi = Pick<
  typeof defaultApi,
  "listProjects" | "getProject" | "createProject" | "addMaterial" | "logSourceOpen" | "prepareSourceAnnotation" | "putBuffer" | "commitSnapshot" | "orderReview" | "orderSpotCheck" | "postDisposition" | "attestGate" | "finishProject" | "submitOnboarding" | "submitSelfScore" | "submitReflection" | "submitFraming" | "submitPerspectives" | "signDeclaration" | "reopenStation" | "readTurn" | "activateProjectCard" | "evaluateCardSelection" | "submitProjectCard" | "skipProjectCard"
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
      // N3d Task 9: mapped straight through, exactly as `onboarding` above —
      // both are top-level StudioProjection fields (siblings of `onboarding`,
      // not nested under a "views" object on the wire).
      framing: p.framing,
      perspectives: p.perspectives,
      // N2 Task 8: same defensive fallback as `readiness` above — older test
      // fixtures/mocks predating this slice may omit these fields entirely.
      selfScore: p.selfScore ?? { dims: [], bands: [] },
      prediction: p.prediction ?? { predicted: [], actual: [], overlap: 0, revealed: false },
      reflection: p.reflection ?? { text: "", prompts: [] },
      // N3f Task 7: same defensive fallback as selfScore/prediction/reflection
      // above — older test fixtures/mocks predating this task may omit
      // spotChecks entirely.
      spotChecks: p.spotChecks ?? {
        evaluateSources: { items: [], orderable: false },
        buildArgument: { items: [], orderable: false },
      },
      // N3f Task 9: same defensive fallback as selfScore/prediction/reflection
      // above — older test fixtures/mocks predating this task may omit
      // declaration entirely.
      declaration: p.declaration ?? {
        asks: 0, dispositions: 0, cardsSpontaneous: 0, cardsPrompted: 0, aiWrittenProse: 0, signed: false,
      },
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
  // Task 8 (read-together redesign): the source currently open in the
  // focused ReadingRoom surface — set by a source-list row click
  // (SourceDossier's `onOpenReading`, threaded via StudioCallbacks), null
  // means the ordinary studio shell renders. Mirrors `openId`'s own
  // directory↔studio swap, just one level in (studio↔reading).
  const [readingMaterialId, setReadingMaterialId] = useState<string | null>(null);
  // Slice 6b Task 9: the server's own honest Chinese message from the last
  // failed 添加信源 attempt — cleared on the next successful add.
  const [addSourceError, setAddSourceError] = useState<string | undefined>(undefined);
  // A3 Task 9: the project terminal's own transient state — not a callback
  // result folded into StudioCallbacks (same reasoning as addSourceError
  // above: this is the student-facing outcome of the last finish attempt,
  // not a callback itself).
  const [finishing, setFinishing] = useState(false);
  const [finishError, setFinishError] = useState<string | null>(null);
  // N3f Task 7: the S3/S4 spot-check panels' order-in-flight flags — mirrors
  // `finishing` above (a transient result of the last button press, not a
  // callback), keyed per station since a student could plausibly have both
  // stations' panels mounted (素材 branch's compare-mode dossier view is one
  // of two 素材 render sites, but 结构's panel is a separate station entirely).
  const [pendingSpotCheck, setPendingSpotCheck] = useState<{ evaluateSources: boolean; buildArgument: boolean }>({
    evaluateSources: false,
    buildArgument: false,
  });
  // Whole-branch review finding [3]: the student's in-progress lateral-
  // source pick for the active compare (SIFT) card. Previously private to
  // StudioCompareCard, so nothing else in the Studio — least of all
  // ViewFrame's center-pane Compare primitive, the whole reason SIFT exists
  // — could ever see it. This is the smallest state location that is
  // visible to BOTH the coach rail (which sets it) and the center pane
  // (which reads it to fill the right pane live): StudioContainer already
  // owns the projection, the live `card`, and every other cross-pane
  // concern. Reset whenever the ACTIVE card instance changes — not on every
  // conversation snapshot, which would wipe the student's own pick mid-fill.
  const [lateralMaterialId, setLateralMaterialId] = useState<string>("");
  // N3c task 9 (spec §8): the student's in-progress "go find this sentence
  // in the article" request from a locate/elicit-mode anchor's 「去文章里选
  // 出这句」 control — the smallest state location visible to BOTH the coach
  // rail (which sets it, via the card's onRequestLocate) and the center pane
  // (which reads it to force the right source open and put Annotate in
  // select mode), same lifting rationale as `lateralMaterialId` above.
  const [locating, setLocating] = useState<{ anchorId: string; dimension: string; materialId: string; token: number } | null>(null);
  // Task-9 review IMPORTANT 1 fix: a monotonically increasing id, bumped on
  // every 「去文章里选出这句」 click (even one naming the same material as a
  // still-pending request) — SourceDossier's forced-open effect keys on this
  // (as `openToken`), not just the material id, so a second locate click
  // after she navigated back to 信源列表 herself is still a distinct COMMAND
  // and reliably reopens the source. A ref, not state: it only needs to be
  // read synchronously inside onRequestLocate below, never rendered.
  const locateTokenRef = useRef(0);
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
  //
  // Task-9 correctness fix: keyed by ANCHOR ID (a map, not an append-only
  // array), holding at most one event per anchor BY CONSTRUCTION. The prior
  // shape reconciled by filtering on `dimension` — but escapes/located spans
  // are tracked per ANCHOR (`locatedSpans` above is keyed by anchor id, and
  // StudioAnnotateCard's own fallback filter is per anchor too), and nothing
  // stops two anchors from sharing the same `dimension` string (the server's
  // anchor validation checks tag coverage/vocabulary, never uniqueness — see
  // apps/api/internal/agent/anchors.go — and legacy untagged cards are
  // unvalidated entirely). Filtering by dimension meant locating on anchor A
  // could silently delete anchor B's still-standing `span_not_found` the
  // instant the two happened to share a dimension — erasing a true record.
  // Keying by anchor id instead makes that structurally impossible: setting
  // one anchor's entry can never touch another anchor's, whatever dimension
  // strings they carry. Flattened via `Object.values(...)` wherever this is
  // handed to the card as `pendingTrace`, so the card's own interface (a
  // flat array) is unchanged.
  const [spanTrace, setSpanTrace] = useState<Record<string, TraceEvent>>({});
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
  // I2 fix (whole-branch review): logs a source-read's elapsed time and
  // refetches (this call also attests recon_logged and runs AdvanceAll
  // server-side — S2's own gate item — so without a refetch here the rail
  // would keep showing the OLD station as `current` until a full page
  // reload). Never throws to the caller: a failed log or a failed refresh
  // must never break the source-close interaction itself. Shared by
  // `callbacks.onOpenLogged` (the ordinary in-shell dossier path, still used
  // by other view swaps) AND the ReadingRoom render below — hoisted above
  // both so it's defined (not just referenced) before either can call it.
  const onOpenLogged = (materialId: string, timeSpentS: number) => {
    if (!projectId) return;
    api
      .logSourceOpen(projectId, materialId, timeSpentS)
      .then(() => refetchProject())
      .catch(() => {
        setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
      });
  };

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
    } catch (err) {
      if (refetchGenRef.current !== myGen) return;
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

  // N3f Task 9: signs the S6 AI 使用申报单 — same shape as submitSelfScore/
  // submitReflection above (no llm_call, refetch to rehydrate `signed` +
  // the frozen counters from what actually persisted).
  const signDeclaration = async () => {
    if (!projectId) return;
    await api.signDeclaration(projectId);
    await refetchProject();
  };

  // N6-E Task 6: re-opens a `waived` station (the journey composer skipped
  // it) — best-effort like the pattern above; a failed reopen just leaves
  // the station waived (no worse a state than before the click), so it's
  // swallowed rather than surfaced as a sync error.
  const onReopenStation = async (code: StationCode) => {
    if (!openId) return;
    try {
      await api.reopenStation(openId, code);
    } catch {
      // best-effort; a failed reopen leaves the station waived (no worse state)
    }
    void refetchProject();
  };

  // N3f Task 7: order a station's spot-check (信源体检 / 论证体检). Unlike
  // `onOrderReview` (Slice 8 Task 10), which drains an SSE generator itself,
  // `api.orderSpotCheck` already does that draining internally and resolves/
  // rejects — so this only needs to track the per-station pending flag and
  // refetch on success. N3d's Important finding I2 was exactly a
  // fire-and-forget write whose gate closed server-side while the rail kept
  // lying until reload — refetchProject here (not a fire-and-forget `.then`)
  // is what closes that gap.
  const orderSpotCheck = async (contractId: SpotCheckContractId) => {
    if (!projectId) return;
    const key = contractId === "evaluate_sources" ? "evaluateSources" : "buildArgument";
    setPendingSpotCheck((prev) => ({ ...prev, [key]: true }));
    try {
      await api.orderSpotCheck(projectId, contractId);
      await refetchProject();
    } catch {
      setSyncError("体检失败，请重试。");
    } finally {
      setPendingSpotCheck((prev) => ({ ...prev, [key]: false }));
    }
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
      setSpanTrace({});
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
    setSpanTrace({});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeCardInstanceId]);

  // Task 8 (read-together redesign): if the ReadingRoom's target material
  // ever vanishes from the loaded projection (e.g. a refetch lands without
  // it), fall back to the studio shell rather than rendering ReadingRoom
  // against nothing — a stale id must not wedge the student on a blank
  // screen. The render below already falls through to the shell whenever the
  // lookup misses; this just clears the stale id so that fallback becomes
  // permanent instead of being re-attempted (harmlessly) on every render.
  useEffect(() => {
    if (readingMaterialId == null || !state) return;
    const stillExists = state.views.material.some((m) => m.id === readingMaterialId);
    if (!stillExists) setReadingMaterialId(null);
  }, [readingMaterialId, state]);

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

  // Task 8 (read-together redesign): a source open for reading replaces the
  // studio shell entirely with the focused ReadingRoom surface — mirrors the
  // `openId == null` directory↔studio swap above, one level in. The lookup
  // can miss (a stale id the effect above hasn't cleared yet, e.g. the very
  // render right after a refetch drops the material) — falling through to
  // the ordinary shell render below rather than crashing.
  if (readingMaterialId != null) {
    const readingSource = state.views.material.find((m) => m.id === readingMaterialId) ?? null;
    if (readingSource) {
      return (
        <ReadingRoom
          projectId={projectId ?? ""}
          source={readingSource}
          api={api}
          onOpenLogged={onOpenLogged}
          onBack={() => {
            setReadingMaterialId(null);
            void refetchProject();
          }}
        />
      );
    }
  }

  // Shared by `onRequestLocate` and `onRelocate` below (whole-branch review
  // IMPORTANT 1 fix) so the material_id lookup / dead-control no-op / token
  // bump / station switch exist in exactly one place.
  //
  // MINOR 3 (task-9 review): an anchor with no material_id can never be
  // satisfied by this control — forcing SourceDossier to "open ''" would
  // switch the station, open nothing, show no hint bar, and give her no way
  // out. A control that cannot work must not be offered (铁律 2's
  // don't-wall-her-in reading), so this simply does nothing rather than
  // putting the UI in an unsatisfiable locate state.
  const requestLocate = (anchorId: string, dimension: string) => {
    const materialId = convSnapshot.card?.anchors.find((a) => a.id === anchorId)?.material_id ?? "";
    if (!materialId) return;
    locateTokenRef.current += 1;
    setLocating({ anchorId, dimension, materialId, token: locateTokenRef.current });
    setActiveStation("S3");
  };

  const callbacks: StudioCallbacks = {
    onSelectStation: setActiveStation,          // client-local view switch
    onReopenStation,                            // N6-E: re-open a waived station
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
      conv
        ?.submitCard(finalEnvelope)
        .then(() => refetchProject())
        .catch(() => {
          // The submit itself already landed server-side — only the refresh
          // that would confirm the lock/anchors failed. Surface that
          // instead of an unhandled rejection or silently rendering as if
          // the lock succeeded (bug [C]).
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
    onRequestLocate: (anchorId, dimension) => requestLocate(anchorId, dimension),
    // Whole-branch review IMPORTANT 1 (N3c): the located branch had no way
    // back — an accidental two-character drag committed via `onCreateSpan`
    // permanently replaced 「去文章里选出这句」 with a bare read-only line,
    // with nothing recorded to undo (unlike the `span_not_found` escape,
    // which already had 「重新找一下」). Clears this anchor's stale
    // `locatedSpans` entry first, then re-issues the SAME locate request
    // `onRequestLocate` would — sharing `requestLocate` rather than
    // duplicating its material_id / token / station-switch logic. The
    // superseded `span_located` trace event is left standing on purpose
    // (铁律 4, same reasoning as the escape's own undo above).
    onRelocate: (anchorId, dimension) => {
      setLocatedSpans((prev) => {
        if (!(anchorId in prev)) return prev;
        const next = { ...prev };
        delete next[anchorId];
        return next;
      });
      requestLocate(anchorId, dimension);
    },
    // 铁律 4 · 过程即数据: a dimension she looked for and could not find is
    // DATA, not an error — recorded as its own trace event, never surfaced
    // as a failure.
    //
    // Task-9 correctness fix: this anchor's own map entry is simply SET,
    // never filtered/appended — undo→retake ("重新找一下" then 「找不到合适
    // 的句子」 again on the same anchor) just overwrites the same key, so an
    // honest single fact ("she couldn't find it, as of now") can never turn
    // into duplicate rows, and — unlike the old dimension-keyed filter —
    // setting THIS anchor's entry can never touch a DIFFERENT anchor's, even
    // one sharing the same dimension string.
    onSpanNotFound: (anchorId, dimension) => {
      setSpanTrace((prev) => ({
        ...prev,
        [anchorId]: { kind: "span_not_found", dimension, at: new Date().toISOString() },
      }));
    },
    // She released the mouse over a real selection in the article pane —
    // write it onto the anchor she was locating, record when it happened
    // (the ONLY honest place to timestamp this — see `spanTrace`'s doc
    // comment), and clear the pending request so the hint bar disappears.
    //
    // Task-9 correctness fix: a located span SUPERSEDES any earlier
    // `span_not_found` recorded for THIS anchor — setting `locating.anchorId`'s
    // map entry naturally replaces whatever was there (an escape or nothing),
    // regardless of what dimension string this anchor happens to share with
    // any other. Without keying by anchor, escape → 「重新找一下」 → locate
    // produced BOTH events for the same anchor (fixed in an earlier pass by
    // filtering on dimension) — but that dimension-filter fix then went on to
    // erase a DIFFERENT anchor's still-standing `span_not_found` whenever the
    // two anchors shared a dimension, because StudioAnnotateCard's own submit
    // uses `pendingTrace` verbatim when the container supplies one (never
    // rebuilding it): a false "couldn't find it" surviving, or a true one
    // vanishing, both count as the process record lying about what happened,
    // which this product treats as worse than recording nothing.
    onCreateSpan: (span) => {
      if (!locating) return;
      setLocatedSpans((prev) => ({
        ...prev,
        [locating.anchorId]: { block_id: span.blockId, start: span.start, end: span.end, quote: span.text },
      }));
      setSpanTrace((prev) => ({
        ...prev,
        [locating.anchorId]: { kind: "span_located", dimension: locating.dimension, block_id: span.blockId, at: new Date().toISOString() },
      }));
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
    // N-fix (2026-07): 印记 reads the article WITH the student. On open, ask
    // the server to surface the source's evaluation card + generate the
    // flagged-sentence anchors, then refetch so the highlights + interactive
    // card appear. Best-effort: a failure never breaks reading (mirrors
    // onOpenLogged's own posture) — it just means no highlights this open.
    onPrepareAnnotation: (materialId) => {
      if (!projectId) return;
      api
        .prepareSourceAnnotation(projectId, materialId)
        .then((surfaced) => {
          // Only reload when 印记 actually surfaced a new card/anchors — an
          // open that found nothing to do (in-flight card, already-evaluated
          // source) costs no needless refetch.
          if (surfaced) refetchProject();
        })
        .catch(() => {
          /* best-effort: reading still works with no highlights */
        });
    },
    // Task 8 (read-together redesign): a source-list row click opens the
    // focused ReadingRoom surface instead of the in-place article view — see
    // the `readingMaterialId` render swap above.
    onOpenReading: (materialId) => setReadingMaterialId(materialId),
    // Task 10: the log-and-refetch body now lives in the hoisted
    // `onOpenLogged` above (shared with the ReadingRoom render below) — this
    // is just the same reference under its StudioCallbacks name.
    onOpenLogged,
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
    // N3d Task 12: the S1 立题 / S2 视角与素材 station views' whole-panel
    // saves — same shape as submitOnboarding/submitSelfScore/submitReflection
    // above (a pure DB write, no llm_call, refetch to rehydrate from what
    // actually persisted). No try/catch here either, matching those three:
    // a rejection propagates to the view's own submit handler, which already
    // resets its local `submitting` flag in a `finally`.
    onSubmitFraming: async (body) => {
      if (!projectId) return;
      await api.submitFraming(projectId, body);
      await refetchProject();
    },
    onSubmitPerspectives: async (body) => {
      if (!projectId) return;
      await api.submitPerspectives(projectId, body);
      await refetchProject();
    },
    // N3d Task 12: the S2 view's one explicit attestation — mirrors
    // onAttestCitations's shape above exactly, just against the
    // evaluate_perspectives contract's sources_per_perspective item. Fires
    // in both directions unchanged: an unchecked confirm clears the item
    // through the same generic gate-attest client just as readily as a
    // checked one sets it — PerspectivesView's own toggle handler is what
    // decides which boolean to pass.
    onAttestSourcesPerPerspective: (confirmed) => {
      if (!projectId) return;
      api
        .attestGate(projectId, "evaluate_perspectives", "sources_per_perspective", confirmed)
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
        state={{
          ...mergedState,
          activeStation,
          focusMode,
        }}
        callbacks={callbacks}
        sending={convSnapshot.sending}
        card={convSnapshot.card}
        addSourceError={addSourceError}
        lateralMaterialId={lateralMaterialId}
        onLateralMaterialChange={setLateralMaterialId}
        locating={locating}
        locatedSpans={locatedSpans}
        pendingTrace={Object.values(spanTrace)}
        review={{ finishing, finishError, onFinish: handleFinish, onSelfScore: submitSelfScore, onReflection: submitReflection, onSignDeclaration: signDeclaration }}
        onSubmitOnboarding={submitOnboarding}
        // N3f Task 7 (I1 fix): the S3/S4 spot-check panels' order-in-flight
        // flags + actions — a container-local transient handler-state group
        // (not a projection field), so it travels as its own StudioShellProps
        // field mirroring `review` above, not folded into `state`.
        // `onDisposition` reuses the SAME generic postDisposition-backed
        // handler `writing.onReviewDisposition` already wires (disposition is
        // generic across intervention types, not review-specific).
        spotCheck={{
          pendingEvaluateSources: pendingSpotCheck.evaluateSources,
          pendingBuildArgument: pendingSpotCheck.buildArgument,
          onOrder: orderSpotCheck,
          onDisposition: callbacks.onReviewDisposition,
        }}
      />
    </>
  );
}
