import { useEffect, useState } from "react";
import type { Anchor, AnnotateState, CompareState, MaterialSource } from "@mind-imprint/contracts";
import type { StudioState } from "./state";
import type { LiveCard } from "./CoachRail";
import type { AddMaterialBody } from "../api/materials";
import { SourceDossier, anchorToSpan } from "./material/SourceDossier";
import { AddSourceForm } from "./material/AddSourceForm";
import { Compare } from "../primitives/compare";
import { StructureView } from "./views/StructureView";
import { WritingView } from "./views/WritingView";
import { ReviewView } from "./views/ReviewView";
import { OnboardingView } from "./views/OnboardingView";

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
  material?: {
    onAdd?: (body: AddMaterialBody) => Promise<void>;
    onOpenLogged?: (materialId: string, timeSpentS: number) => void;
    addError?: string;
  };
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

export function ViewFrame({ state, card, pendingAnchors, lateralMaterialId, material }: ViewFrameProps) {
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

  // S0/S1/S2 render bespoke onboarding screens regardless of their `.view`
  // tag (which is the station's four-view *association* from the design's STA,
  // not what renders during onboarding). The design gates these by station
  // code — `stnS0 || stnS1 || stnS2` — while S3–S6 render their view
  // (viewIsMaterial=S3, viewIsStructure=S4, viewIsWriting=S5, viewIsReview=S6).
  const isOnboarding = active.code === "S0" || active.code === "S1" || active.code === "S2";
  const effectiveView = isOnboarding ? "onboarding" : active.view;

  const isCompareCardActive = card?.status === "active" && card.spec.primitive === "compare";
  const compareState = isCompareCardActive
    ? buildCompareState(card.anchors, card.materialId ?? "", lateralMaterialId ?? "")
    : null;

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
          <SourceDossier
            sources={state.views.material}
            anchors={card?.anchors ?? pendingAnchors ?? []}
            onAddSource={material?.onAdd}
            addSourceError={material?.addError}
            onOpenLogged={material?.onOpenLogged}
          />
        )
      )}
      {effectiveView === "结构" && <StructureView cards={state.views.structure} />}
      {effectiveView === "写作" && <WritingView {...state.views.writing} />}
      {effectiveView === "评估" && <ReviewView gauges={state.views.review} />}
      {effectiveView === "onboarding" && <OnboardingView station={active} data={state.views.onboarding} />}
    </div>
  );
}
