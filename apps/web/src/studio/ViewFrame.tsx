import type { StudioState } from "./state";
import type { LiveCard } from "./CoachRail";
import type { AddMaterialBody } from "../api/materials";
import { SourceDossier } from "./material/SourceDossier";
import { StructureView } from "./views/StructureView";
import { WritingView } from "./views/WritingView";
import { ReviewView } from "./views/ReviewView";
import { OnboardingView } from "./views/OnboardingView";

export type ViewFrameProps = {
  state: StudioState;
  // Live tool-card slot (mirrors StudioShell's `card`): its anchors highlight
  // the spans the coach rail is asking about right now.
  card?: LiveCard | null;
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

export function ViewFrame({ state, card, material }: ViewFrameProps) {
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

  return (
    <div style={FRAME}>
      <div style={HEADER}>
        <div style={ICON_BOX}>
          <StationIcon />
        </div>
        <span style={NAME}>{active.name}</span>
      </div>
      {effectiveView === "素材" && (
        <SourceDossier
          sources={state.views.material}
          anchors={card?.anchors ?? []}
          onAddSource={material?.onAdd}
          addSourceError={material?.addError}
          onOpenLogged={material?.onOpenLogged}
        />
      )}
      {effectiveView === "结构" && <StructureView cards={state.views.structure} />}
      {effectiveView === "写作" && <WritingView {...state.views.writing} />}
      {effectiveView === "评估" && <ReviewView gauges={state.views.review} />}
      {effectiveView === "onboarding" && <OnboardingView station={active} data={state.views.onboarding} />}
    </div>
  );
}
