import { useEffect, useState } from "react";
import type { Anchor, AnnotateState, CardInstance, CompareState, MaterialSource, TraceEvent } from "@mind-imprint/contracts";
import type { StudioState, SpotCheckContractId } from "./state";
import type { LiveCard } from "./CoachRail";
import type { AddMaterialBody } from "../api/materials";
import type { ReviewVoice } from "../api/writing";
import type { CreatedSpan } from "../primitives/annotate";
import { SourceDossier, anchorToSpan } from "./material/SourceDossier";
import { AddSourceForm } from "./material/AddSourceForm";
import { Compare } from "../primitives/compare";
import { SpotCheckPanel } from "./SpotCheckPanel";
import { StructureView } from "./views/StructureView";
import { WritingView } from "./views/WritingView";
import { ReviewView } from "./views/ReviewView";
import { OnboardingView } from "./views/OnboardingView";
import { FramingView } from "./views/FramingView";
import { PerspectivesView } from "./views/PerspectivesView";

export type ViewFrameProps = {
  state: StudioState;
  // Live tool-card slot (mirrors StudioShell's `card`): its anchors highlight
  // the spans the coach rail is asking about right now.
  card?: LiveCard | null;
  // Fix-wave bug [B]: overlay anchors held across a submit → refetch
  // transition, used ONLY as a fallback when `card` is null (see
  // StudioShell's prop doc) so the highlights never go empty for that round
  // trip.
  pendingAnchors?: Anchor[] | null;
  // The student's in-progress lateral-source pick for the active compare
  // card — LIFTED to StudioContainer (whole-branch review finding [3]) so
  // this pane can show the SAME material StudioCompareCard's picker just
  // selected, instead of only learning about it after a full submit round
  // trip (which never happens while this pane is even visible — see
  // buildCompareState's doc comment).
  lateralMaterialId?: string;
  // N3c task 9 (spec §8): the student's in-progress "go find this sentence
  // in the article" request, lifted to StudioContainer for the same reason
  // `lateralMaterialId` above is — it's visible to BOTH the coach rail
  // (which sets it via the card's onRequestLocate) and this pane (which
  // forces the right source open and puts Annotate in select mode). Non-null
  // only while a locate request targeting THIS card's material is pending;
  // `null`/absent renders 素材 exactly as before this feature existed.
  // `token` (task-9 review IMPORTANT 1 fix) is the container's own
  // monotonically increasing request id — threaded to SourceDossier as
  // `openToken` so a second locate click on the SAME material still forces
  // a fresh reopen even if she navigated back to the list in between.
  locating?: { anchorId: string; dimension: string; materialId: string; token: number } | null;
  onCreateSpan?: (span: CreatedSpan) => void;
  onCancelLocate?: () => void;
  material?: {
    onAdd?: (body: AddMaterialBody) => Promise<void>;
    onOpenLogged?: (materialId: string, timeSpentS: number) => void;
    addError?: string;
  };
  // Card lock/skip for the center-pane interactive card (the S4 Toulmin
  // builder) — the SAME handlers the coach rail uses, threaded here so the
  // 结构 pane can submit its graph. Mirrors the rail's onSubmitCard/onSkipCard.
  onSubmitCard?: (env: CardInstance) => void;
  onSkipCard?: (eventTrace: TraceEvent[]) => void;
  // Slice 8 Task 9: the 写作 view's silent-buffer autosave + snapshot commit
  // — a separate prop object (not folded into StudioCallbacks-only usage)
  // mirroring `material`'s own onAdd/onOpenLogged grouping above. Task 10
  // adds the 整稿体检 work-order trigger, its per-item disposition, and the
  // citations_matched attestation to the same group.
  writing?: {
    onBufferChange?: (text: string) => void;
    onCommit?: (text: string) => void;
    onOrderReview?: (snapshotId: string, voice: ReviewVoice) => void;
    onReviewDisposition?: (interventionId: string, action: "accept" | "rewrite" | "reject", reason: string) => void;
    onAttestCitations?: (confirmed: boolean) => void;
  };
  // A3 Task 9: the 就绪度 view's project terminal — canFinish/finished travel
  // on `state` itself (they're projection fields, same as state.views.review),
  // but finishing/finishError/onFinish are container-local transient handler
  // state, so they travel as their own group, mirroring `writing` above.
  review?: {
    finishing?: boolean;
    finishError?: string | null;
    onFinish?: () => void;
    // N2 Task 8: the 评估 view's self-score + retro submit — canFinish/
    // finished/selfScore/prediction/reflection travel on `state` itself
    // (projection fields), same as finishing/finishError/onFinish above.
    onSelfScore?: (body: { scores: { code: string; band: number }[] }) => void;
    onReflection?: (body: { text: string }) => void;
  };
  // N3f Task 7 (I1 fix): the S3/S4 spot-check panels' order-in-flight flags +
  // actions — `state.views.spotChecks` carries the projection (pure
  // projected data), but the pending flags and the order/disposition
  // callbacks are container-local transient handler state, so (mirroring
  // `review` above, and its own doc comment on the rule) they travel as
  // their own prop group here rather than living on `state`.
  spotCheck?: {
    pendingEvaluateSources: boolean;
    pendingBuildArgument: boolean;
    onOrder: (contractId: SpotCheckContractId) => void;
    onDisposition?: (interventionId: string, action: "accept" | "rewrite" | "reject", reason: string) => void;
  };
  // N1 Task 8: the S0 view's restate + weak-picks submit — mirrors the flat
  // shape OnboardingView's own `onSubmit` takes (no wrapper group, unlike
  // `writing`/`review`, since this is the view's only callback).
  onSubmitOnboarding?: (body: { restate: string; weakPicks: number[] }) => Promise<void>;
  // N3d Task 10: the S1 立题 view's whole-panel submit — mirrors
  // onSubmitOnboarding's flat shape above (FramingView's only callback).
  // Task 12 wires the real handler in from the container; this task only
  // threads the prop through.
  onSubmitFraming?: (body: { terms: { term: string; definition: string }[]; answers: string[]; searchPlan: string[] }) => Promise<void>;
  // N3d Task 11: the S2 视角与素材 view's whole-panel submit + its one
  // explicit attestation — mirrors onSubmitFraming's flat shape above.
  // Task 12 wires the real handlers in from the container.
  onSubmitPerspectives?: (body: { perspectives: { text: string; level: string }[] }) => Promise<void>;
  onAttestSourcesPerPerspective?: (confirmed: boolean) => void;
};

const FRAME: React.CSSProperties = {
  flex: 1,
  minWidth: 520,
  display: "flex",
  flexDirection: "column",
  background: "#F3F4F8",
};

const HEADER: React.CSSProperties = {
  flex: "none",
  display: "flex",
  alignItems: "center",
  gap: 10,
  padding: "0 24px",
  height: 56,
  borderBottom: "1px solid #EAECF2",
  background: "#fff",
};

const ICON_BOX: React.CSSProperties = {
  width: 30,
  height: 30,
  borderRadius: 9,
  background: "#EDEFF9",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
};

const NAME: React.CSSProperties = { fontSize: 15, fontWeight: 800, color: "#1C2333" };

const LINK_BUTTON: React.CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 6,
  background: "none",
  border: "none",
  padding: 0,
  marginBottom: 14,
  color: "#5C4A8A",
  fontSize: 13,
  fontWeight: 700,
  cursor: "pointer",
  fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif",
};

function StationIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M11 4a7 7 0 100 14 7 7 0 000-14zM21 21l-4-4" />
    </svg>
  );
}

function BackIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M15 18l-6-6 6-6" stroke="#5C4A8A" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function FolderIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" stroke="#5C4A8A" strokeWidth="2" strokeLinejoin="round" />
    </svg>
  );
}

// Projects a live `compare` card's persisted state into the primitive's read
// shape (design spec §2.1 — CompareState is projected, not stored):
//
//  - left  = the card's OWN material (`cardMaterialId`, FIX-A's
//    card_instance--evaluates-->material edge, whole-branch review finding
//    [5]) — never guessed from anchor contents or array position.
//  - right = `lateralMaterialId`, the student's LIVE pick (finding [3]) —
//    not derived from anchors, because no anchor targets the lateral
//    material until AFTER a full submit (StudioCompareCard.buildAnchors),
//    by which point the card has already gone null and this pane is no
//    longer even rendered. Waiting on anchors would mean the right pane
//    could never fill while a student can actually see it.
//  - pairs = always []. §2.1's own pairing rule (a right-pane anchor joined
//    to the left-pane anchor sharing its `dimension`) can never fire for
//    SIFT: `find` (params.lateral_dimension) is the only field ever routed
//    to the right pane by StudioCompareCard.buildAnchors, and `find`'s own
//    anchor is deliberately never routed left — so `left.find(a =>
//    a.dimension === "find")` is always undefined and no pair can ever
//    form. Two independent FIX-B reviews proved this pairing computation
//    (plus the "对照笔记" rendering, RELATION_LABEL, and findSpanTag it fed)
//    was structurally dead code with zero coverage of its populated path,
//    so it was deleted rather than shipped unverified — "written but never
//    read" has already bitten this project twice. A future compare card
//    must design its OWN pairing deliberately against its OWN field layout;
//    do not resurrect this loop on the assumption it already fits.
function buildCompareState(anchors: Anchor[], cardMaterialId: string, lateralMaterialId: string): CompareState {
  const left: Anchor[] = [];
  const right: Anchor[] = [];
  for (const a of anchors) {
    if (cardMaterialId && a.material_id !== "" && a.material_id !== cardMaterialId) right.push(a);
    else left.push(a);
  }
  const toSpans = (list: Anchor[]) => list.map(anchorToSpan).filter((s): s is AnnotateState["spans"][number] => s !== null);

  return {
    left: { material_id: cardMaterialId, spans: toSpans(left) },
    right: lateralMaterialId ? { material_id: lateralMaterialId, spans: toSpans(right) } : null,
    pairs: [],
  };
}

function blocksOf(materials: MaterialSource[], materialId: string) {
  return materials.find((m) => m.id === materialId)?.blocks ?? [];
}

export function ViewFrame({ state, card, pendingAnchors, lateralMaterialId, locating, onCreateSpan, onCancelLocate, material, onSubmitCard, onSkipCard, writing, review, spotCheck, onSubmitOnboarding, onSubmitFraming, onSubmitPerspectives, onAttestSourcesPerPerspective }: ViewFrameProps) {
  // The 添加信源 form embedded under Compare's empty right pane — reuses 6b's
  // existing ingestion path (material?.onAdd) exactly like the dossier's own
  // list-view form; Compare itself never ingests (RL-2).
  const [showLateralForm, setShowLateralForm] = useState(false);
  // Whole-branch review finding [3]: Compare used to REPLACE the dossier
  // outright — the source list, 检索日志 ledger, and chip all vanished, so
  // she couldn't read her other sources while checking one. This toggle
  // keeps both reachable: default to Compare (that's why a compare card
  // surfaced), one click away from the full dossier, one click back.
  const [showDossierDuringCompare, setShowDossierDuringCompare] = useState(false);
  // A fresh compare card instance always opens on Compare itself, never
  // wherever a PREVIOUS card happened to leave the toggle — SIFT can
  // surface repeatedly across a project's materials. showLateralForm resets
  // for the same reason (minor, whole-branch review): without this, the
  // inline 添加信源 form stayed mounted under a brand-new card's still-empty
  // right pane, left over from whatever the PREVIOUS card's student did.
  useEffect(() => {
    setShowDossierDuringCompare(false);
    setShowLateralForm(false);
  }, [card?.cardInstanceId]);
  const active = state.stations.find((s) => s.code === state.activeStation);

  if (!active) {
    return <div style={FRAME} />;
  }

  // S0/S1/S2 each render their OWN screen (regardless of their `.view` tag,
  // which is the station's four-view *association* from the design's STA,
  // not what renders during onboarding) — while S3–S6 render their view
  // (viewIsMaterial=S3, viewIsStructure=S4, viewIsWriting=S5, viewIsReview=S6).
  //
  // N3d: S0/S1/S2 each render their OWN screen. Before this slice all three
  // collapsed onto OnboardingView, which meant S1 and S2 showed a
  // "coming in a later slice" placeholder — while their gates had no
  // producer at all, so neither station could ever complete.
  const stationScreen = active.code === "S0" || active.code === "S1" || active.code === "S2" ? active.code : null;
  const effectiveView = stationScreen ? "station" : active.view;

  const isCompareCardActive = card?.status === "active" && card.spec.primitive === "compare";
  const compareState = isCompareCardActive
    ? buildCompareState(card.anchors, card.materialId ?? "", lateralMaterialId ?? "")
    : null;

  // The active S4 Toulmin card renders its interactive builder in the 结构
  // center pane (detected off the primitive, never the id — same pattern as
  // compare above). Its needSrc slots cite from the project's CRAAP-locked
  // materials (design copy: 信源评估里已锁定的), NOT from the card's own
  // material — a toulmin card is project-scoped (material_id "").
  const isGraphCardActive = card?.status === "active" && card.spec.primitive === "graph";
  const lockedSources = (state.views.material ?? [])
    .filter((m) => m.locked)
    .map((m) => ({ id: m.id, name: m.title }));

  return (
    <div style={FRAME}>
      <div style={HEADER}>
        <div style={ICON_BOX}>
          <StationIcon />
        </div>
        <span style={NAME}>{active.name}</span>
      </div>
      {effectiveView === "素材" && (
        compareState ? (
          showDossierDuringCompare ? (
            <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "22px 30px 40px" }}>
              <button type="button" onClick={() => setShowDossierDuringCompare(false)} style={LINK_BUTTON}>
                <BackIcon />
                返回横向核查
              </button>
              <SourceDossier
                sources={state.views.material}
                anchors={card?.anchors ?? pendingAnchors ?? []}
                onAddSource={material?.onAdd}
                addSourceError={material?.addError}
                onOpenLogged={material?.onOpenLogged}
              />
              {/* N3f Task 7: a student in cross-check mode has not left S3 —
                  信源体检 must render here too, not only the non-compare
                  branch below. */}
              <SpotCheckPanel
                title="信源体检"
                data={state.views.spotChecks.evaluateSources}
                onOrder={() => spotCheck?.onOrder("evaluate_sources")}
                onDisposition={spotCheck?.onDisposition}
                pending={spotCheck?.pendingEvaluateSources ?? false}
              />
            </div>
          ) : (
            <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "22px 30px 40px" }}>
              <button type="button" onClick={() => setShowDossierDuringCompare(true)} style={LINK_BUTTON}>
                <FolderIcon />
                查看信源档案（共 {state.views.material.length} 篇）
              </button>
              <Compare
                state={compareState}
                onAddLateralSource={() => setShowLateralForm(true)}
                leftBlocks={blocksOf(state.views.material, compareState.left.material_id)}
                rightBlocks={blocksOf(state.views.material, compareState.right?.material_id ?? "")}
              />
              {showLateralForm && material?.onAdd && (
                <div style={{ marginTop: 16 }}>
                  <AddSourceForm
                    onSubmit={async (body) => {
                      await material.onAdd!(body);
                      // Minor (whole-branch review): a successful add must
                      // collapse this wrapper too, not just AddSourceForm's
                      // own internal expanded/collapsed state — otherwise its
                      // spent, re-collapsed toggle keeps sitting here,
                      // redundant with Compare's own "添加信源" CTA, under
                      // whatever the right pane now shows.
                      setShowLateralForm(false);
                    }}
                    error={material.addError}
                  />
                </div>
              )}
            </div>
          )
        ) : (
          <>
            <SourceDossier
              sources={state.views.material}
              anchors={card?.anchors ?? pendingAnchors ?? []}
              onAddSource={material?.onAdd}
              addSourceError={material?.addError}
              onOpenLogged={material?.onOpenLogged}
              openSourceId={locating?.materialId ?? null}
              openToken={locating?.token}
              selectMode={locating ? { dimension: locating.dimension, onCancel: onCancelLocate ?? (() => {}) } : null}
              onCreateSpan={onCreateSpan}
            />
            <SpotCheckPanel
              title="信源体检"
              data={state.views.spotChecks.evaluateSources}
              onOrder={() => spotCheck?.onOrder("evaluate_sources")}
              onDisposition={spotCheck?.onDisposition}
              pending={spotCheck?.pendingEvaluateSources ?? false}
            />
          </>
        )
      )}
      {effectiveView === "结构" && (
        // StructureView owns its own scroll/padding (WRAP/COL, unmodified by
        // this task — the brief scopes this addition to ViewFrame's 结构
        // branch, not StructureView.tsx). This outer div is the ONE scroll
        // region for the branch as a whole, so the panel scrolls together
        // with the role cards above it instead of fighting them for a
        // second flex:1 share of the column.
        <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
          <StructureView
            cards={state.views.structure}
            toulminCard={isGraphCardActive ? card : null}
            lockedSources={lockedSources}
            onSubmitCard={onSubmitCard}
            onSkipCard={onSkipCard}
          />
          {/* M3 fix: mirrors StructureView's own WRAP/COL split (its `WRAP`
              constant carries the padding, its `COL` constant carries the
              maxWidth) instead of one div doing both — with no box-sizing:
              border-box reset anywhere in this app, a single div combining
              `maxWidth: 760` and horizontal padding renders 60px WIDER than
              a 760px column that gets its padding from an ancestor, which is
              exactly how StructureView's own role-card column is built. */}
          <div style={{ padding: "0 30px 40px" }}>
            <div style={{ maxWidth: 760, margin: "0 auto" }}>
              <SpotCheckPanel
                title="论证体检"
                data={state.views.spotChecks.buildArgument}
                onOrder={() => spotCheck?.onOrder("build_argument")}
                onDisposition={spotCheck?.onDisposition}
                pending={spotCheck?.pendingBuildArgument ?? false}
              />
            </div>
          </div>
        </div>
      )}
      {effectiveView === "写作" && (
        <WritingView
          {...state.views.writing}
          onBufferChange={writing?.onBufferChange}
          onCommit={writing?.onCommit}
          onOrderReview={writing?.onOrderReview}
          onReviewDisposition={writing?.onReviewDisposition}
          onAttestCitations={writing?.onAttestCitations}
        />
      )}
      {effectiveView === "评估" && (
        <ReviewView
          gauges={state.views.review}
          canFinish={state.canFinish}
          finished={state.finished}
          finishing={review?.finishing ?? false}
          finishError={review?.finishError ?? null}
          onFinish={review?.onFinish ?? (() => {})}
          selfScore={state.views.selfScore}
          prediction={state.views.prediction}
          reflection={state.views.reflection}
          onSelfScore={review?.onSelfScore ?? (() => {})}
          onReflection={review?.onReflection ?? (() => {})}
        />
      )}
      {stationScreen === "S0" && (
        <OnboardingView station={active} data={state.views.onboarding} onSubmit={onSubmitOnboarding} />
      )}
      {stationScreen === "S1" && <FramingView data={state.views.framing} onSubmit={onSubmitFraming} />}
      {stationScreen === "S2" && (
        <PerspectivesView
          data={state.views.perspectives}
          material={state.views.material}
          onSubmit={onSubmitPerspectives}
          onAddSource={material?.onAdd}
          addSourceError={material?.addError}
          onOpenLogged={material?.onOpenLogged}
          onAttestSourcesPerPerspective={onAttestSourcesPerPerspective}
        />
      )}
    </div>
  );
}
