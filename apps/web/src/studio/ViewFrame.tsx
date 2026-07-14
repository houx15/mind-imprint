import { useState } from "react";
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

function StationIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M11 4a7 7 0 100 14 7 7 0 000-14zM21 21l-4-4" />
    </svg>
  );
}

// Projects a live `compare` card's anchors into the primitive's read shape
// (design spec §2.1): every anchor whose dimension is the spec's declared
// `params.lateral_dimension` belongs to the RIGHT (lateral) material, every
// other anchor to the LEFT (checked) one — the same declared rule the server
// uses (card_lifecycle.go's checkedMaterialID/lateralAnchor), never anchor
// array order. `right` stays null (Compare's own empty-assignment state)
// until a lateral anchor with a real material_id exists — which, today,
// only happens once the card has actually been submitted once and its
// anchors persisted back onto the projection; pre-submission this
// legitimately renders an empty right pane rather than inventing one.
function buildCompareState(anchors: Anchor[], lateralDimension: string): CompareState {
  const left: Anchor[] = [];
  const right: Anchor[] = [];
  for (const a of anchors) {
    if (lateralDimension && a.dimension === lateralDimension && a.material_id !== "") right.push(a);
    else left.push(a);
  }
  const toSpans = (list: Anchor[]) => list.map(anchorToSpan).filter((s): s is AnnotateState["spans"][number] => s !== null);
  const leftMaterialId = left.find((a) => a.material_id !== "")?.material_id ?? "";
  const rightMaterialId = right[0]?.material_id ?? "";
  return {
    left: { material_id: leftMaterialId, spans: toSpans(left) },
    right: rightMaterialId ? { material_id: rightMaterialId, spans: toSpans(right) } : null,
    pairs: [],
  };
}

function blocksOf(materials: MaterialSource[], materialId: string) {
  return materials.find((m) => m.id === materialId)?.blocks ?? [];
}

export function ViewFrame({ state, card, pendingAnchors, material }: ViewFrameProps) {
  // The 添加信源 form embedded under Compare's empty right pane — reuses 6b's
  // existing ingestion path (material?.onAdd) exactly like the dossier's own
  // list-view form; Compare itself never ingests (RL-2).
  const [showLateralForm, setShowLateralForm] = useState(false);
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
    ? buildCompareState(card.anchors, ((card.spec.params ?? {}) as { lateral_dimension?: string }).lateral_dimension ?? "")
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
          <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "22px 30px 40px" }}>
            <Compare
              state={compareState}
              onAddLateralSource={() => setShowLateralForm(true)}
              leftBlocks={blocksOf(state.views.material, compareState.left.material_id)}
              rightBlocks={blocksOf(state.views.material, compareState.right?.material_id ?? "")}
            />
            {showLateralForm && material?.onAdd && (
              <div style={{ marginTop: 16 }}>
                <AddSourceForm onSubmit={material.onAdd} error={material.addError} />
              </div>
            )}
          </div>
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
