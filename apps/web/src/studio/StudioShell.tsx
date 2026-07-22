import { useState } from "react";
import type { Anchor, TraceEvent } from "@mind-imprint/contracts";
import { StationRail } from "./StationRail";
import { ViewFrame } from "./ViewFrame";
import { CoachRail } from "./CoachRail";
import type { LiveCard } from "./CoachRail";
import type { LocatedSpan } from "./StudioAnnotateCard";
import { MethodologyModal } from "./MethodologyModal";
import type { StudioCallbacks, StudioState, SpotCheckContractId } from "./state";

export type StudioShellProps = {
  state: StudioState;
  callbacks: StudioCallbacks;
  sending?: boolean;
  // Live tool-card slot (Task 10): the conversation's card, threaded down
  // to CoachRail (and, as of Task 9, to ViewFrame so its anchors can
  // highlight the article spans the coach rail is asking about) — defaults
  // to null when there's no conversation yet.
  card?: LiveCard | null;
  // Fix-wave bug [B]: the anchors of the card that was JUST submitted, held
  // by the container from the moment `card` goes null (submit's "done"
  // frame) until the refetch it kicks off lands — passed only to ViewFrame
  // (never to CoachRail's `card`) so the article's highlights don't blink
  // out for that round trip without resurrecting the tool-card sheet.
  pendingAnchors?: Anchor[] | null;
  // Slice 6b Task 9: the 素材 dossier's own transient error state — this
  // doesn't belong on StudioCallbacks (it isn't a callback, it's the
  // student-facing result of the last add attempt) so it travels alongside
  // `card` as its own prop.
  addSourceError?: string;
  // Whole-branch review finding [3]: the student's in-progress lateral-
  // source pick for the active compare card, lifted by StudioContainer so
  // BOTH the coach rail's StudioCompareCard (which sets it) and the center
  // pane's Compare primitive (which reads it) see the same live value —
  // travels alongside `card` for the same reason addSourceError does.
  lateralMaterialId?: string;
  onLateralMaterialChange?: (materialId: string) => void;
  // N3c task 9 (spec §8): the guidance-ladder locate flow's transient
  // state — `locating` travels to ViewFrame (which needs it to force the
  // right source open and put Annotate in select mode); `locatedSpans` +
  // `pendingTrace` travel to CoachRail (which needs them to hand
  // StudioAnnotateCard its located spans and the submittable event_trace).
  // The ACTIONS that drive them (onRequestLocate/onRelocate/onSpanNotFound/
  // onCreateSpan/onCancelLocate) live on `callbacks` instead, matching
  // every other action in this file (onSubmitCard, onAddSource, …).
  // `token` (task-9 review IMPORTANT 1 fix): see ViewFrameProps' own doc
  // comment — forwarded straight through, unchanged, to ViewFrame.
  locating?: { anchorId: string; dimension: string; materialId: string; token: number } | null;
  locatedSpans?: Record<string, LocatedSpan>;
  pendingTrace?: TraceEvent[];
  // A3 Task 9: the 就绪度 view's project terminal — see ViewFrameProps'
  // `review` for why finishing/finishError/onFinish travel here instead of
  // on StudioCallbacks (canFinish/finished are already on `state` itself).
  review?: {
    finishing?: boolean;
    finishError?: string | null;
    onFinish?: () => void;
    // N2 Task 8: mirrors ViewFrameProps' `review` group — forwarded straight
    // through to ViewFrame unchanged (see below).
    onSelfScore?: (body: { scores: { code: string; band: number }[] }) => void;
    onReflection?: (body: { text: string }) => void;
    // N3f Task 9: signs the S6 AI 使用申报单 — mirrors onSelfScore/
    // onReflection above, forwarded straight through to ViewFrame unchanged.
    onSignDeclaration?: () => void;
  };
  // N1 Task 8: the S0 view's restate + weak-picks submit — threaded straight
  // through to ViewFrame (which threads it straight through to
  // OnboardingView's own `onSubmit`), same flat shape at every hop.
  onSubmitOnboarding?: (body: { restate: string; weakPicks: number[] }) => Promise<void>;
  // N3f Task 7 (I1 fix): the S3/S4 spot-check panels' order-in-flight flags +
  // actions — container-local transient handler state (not a projection
  // field), so it travels as its own group here, mirroring `review` above,
  // and is forwarded straight through to ViewFrame unchanged below.
  spotCheck?: {
    pendingEvaluateSources: boolean;
    pendingBuildArgument: boolean;
    onOrder: (contractId: SpotCheckContractId) => void;
    onDisposition?: (interventionId: string, action: "accept" | "rewrite" | "reject", reason: string) => void;
  };
};

// Top bar + 3-column body + focus mode. Design binding:
// docs/design/思维印记_工作区.dc.html ~L757-780 (top bar + focus-exit button),
// body row ~L780 (station rail / center view frame / coach rail).
const FONT = "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif";

function BackIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M15 18l-6-6 6-6" />
    </svg>
  );
}

function FocusIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7" />
    </svg>
  );
}

function ExitFocusIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M9 3H5a2 2 0 00-2 2v4M15 3h4a2 2 0 012 2v4M9 21H5a2 2 0 01-2-2v-4M15 21h4a2 2 0 002-2v-4" />
    </svg>
  );
}

export function StudioShell({
  state,
  callbacks,
  sending = false,
  card = null,
  pendingAnchors = null,
  addSourceError,
  lateralMaterialId,
  onLateralMaterialChange,
  locating = null,
  locatedSpans,
  pendingTrace,
  review,
  onSubmitOnboarding,
  spotCheck,
}: StudioShellProps) {
  const activeView = state.stations.find((s) => s.code === state.activeStation)?.view ?? "结构";
  // MethodologyModal is owned HERE (not by CoachRail) so its full-bleed scrim
  // covers the whole workspace instead of being clipped to the 388px coach
  // rail. Design: docs/design/思维印记_工作区.dc.html ~L1409.
  const [methId, setMethId] = useState<string | null>(null);

  function handleOpenMethodology(id: string) {
    setMethId(id);
    callbacks.onOpenMethodology(id);
  }

  return (
    <div style={{ height: "100%", minHeight: "100vh", display: "flex", flexDirection: "column", position: "relative", fontFamily: FONT }}>
      {!state.focusMode && (
        <div style={{ height: 56, flex: "none", background: "#fff", borderBottom: "1px solid #EAECF2", display: "flex", alignItems: "center", padding: "0 22px", gap: 14 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 7, color: "#6B7384", fontSize: 13, fontWeight: 600, cursor: "pointer", padding: "7px 11px", borderRadius: 9 }}>
            <BackIcon />
            写作工作室
          </div>
          <div style={{ width: 1, height: 22, background: "#EAECF2" }} />
          <span style={{ fontSize: 15, fontWeight: 700, color: "#1C2333", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
            {state.project.title}
          </span>
          <span style={{ flex: "none", fontSize: 11.5, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "3px 10px", borderRadius: 999 }}>
            {state.project.qualLabel}
          </span>
          <button
            type="button"
            onClick={callbacks.onToggleFocus}
            title="专注模式：收起顶栏与侧栏，只留写作区与 AI 陪练"
            style={{
              marginLeft: "auto",
              flex: "none",
              display: "inline-flex",
              alignItems: "center",
              gap: 7,
              fontSize: 13,
              fontWeight: 700,
              color: state.focusMode ? "#2A3B7A" : "#6B7384",
              background: state.focusMode ? "#EDEFF9" : "transparent",
              border: `1px solid ${state.focusMode ? "#DfE3F4" : "#E7E9F0"}`,
              padding: "7px 13px",
              borderRadius: 10,
              cursor: "pointer",
              fontFamily: "inherit",
            }}
          >
            <FocusIcon />
            {state.focusMode ? "退出专注" : "专注模式"}
          </button>
        </div>
      )}

      {state.focusMode && (
        <button
          type="button"
          onClick={callbacks.onToggleFocus}
          title="退出专注"
          style={{
            position: "absolute",
            top: 12,
            right: 18,
            zIndex: 40,
            display: "inline-flex",
            alignItems: "center",
            gap: 7,
            fontSize: 12.5,
            fontWeight: 700,
            color: "#2A3B7A",
            background: "#fff",
            border: "1px solid #DfE3F4",
            boxShadow: "0 4px 14px rgba(20,30,60,.14)",
            padding: "8px 13px",
            borderRadius: 10,
            cursor: "pointer",
            fontFamily: "inherit",
          }}
        >
          <ExitFocusIcon />
          退出专注
        </button>
      )}

      <div style={{ flex: 1, minHeight: 0, display: "flex", overflowX: "auto" }}>
        <StationRail stations={state.stations} active={state.activeStation} focus={state.focusMode} onSelect={callbacks.onSelectStation} />
        <ViewFrame
          state={state}
          card={card}
          pendingAnchors={pendingAnchors}
          lateralMaterialId={lateralMaterialId}
          locating={locating}
          onCreateSpan={callbacks.onCreateSpan}
          onCancelLocate={callbacks.onCancelLocate}
          material={{ onAdd: callbacks.onAddSource, onOpenLogged: callbacks.onOpenLogged, addError: addSourceError }}
          onSubmitCard={callbacks.onSubmitCard}
          onSkipCard={callbacks.onSkipCard}
          writing={{
            onBufferChange: callbacks.onBufferChange,
            onCommit: callbacks.onCommit,
            onOrderReview: callbacks.onOrderReview,
            onReviewDisposition: callbacks.onReviewDisposition,
            onAttestCitations: callbacks.onAttestCitations,
          }}
          review={review}
          spotCheck={spotCheck}
          onSubmitOnboarding={onSubmitOnboarding}
          // N3d Task 12: S1/S2's whole-panel saves + S2's one attestation —
          // these three live on `callbacks` (StudioCallbacks), unlike
          // onSubmitOnboarding above, so they're pulled off it here rather
          // than threaded as their own StudioShellProps fields (mirrors how
          // `writing.onAttestCitations` below is sourced from `callbacks`
          // too, just flattened onto ViewFrame's own top-level props instead
          // of a group, matching onSubmitFraming's own flat shape).
          onSubmitFraming={callbacks.onSubmitFraming}
          onSubmitPerspectives={callbacks.onSubmitPerspectives}
          onAttestSourcesPerPerspective={callbacks.onAttestSourcesPerPerspective}
        />
        <CoachRail
          anchor={state.coach.anchor}
          messages={state.coach.messages}
          equipment={state.coach.equipment}
          activeView={activeView}
          onDisposition={callbacks.onDisposition}
          onOpenMethodology={handleOpenMethodology}
          onSend={callbacks.onComposerSend}
          sending={sending}
          card={card}
          onOpenCard={callbacks.onOpenCard}
          onSubmitCard={callbacks.onSubmitCard}
          onSkipCard={callbacks.onSkipCard}
          materials={state.views.material}
          lateralMaterialId={lateralMaterialId}
          onLateralMaterialChange={onLateralMaterialChange}
          locatedSpans={locatedSpans}
          onRequestLocate={callbacks.onRequestLocate}
          onRelocate={callbacks.onRelocate}
          onSpanNotFound={callbacks.onSpanNotFound}
          pendingTrace={pendingTrace}
        />
      </div>

      <MethodologyModal cardId={methId} onClose={() => setMethId(null)} />
    </div>
  );
}
