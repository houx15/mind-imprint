import type {
  Station, StationCode, StationView, StationState,
  CoachMessage, EquipCard, RubricRow, OnboardingFx,
  CardInstance, TraceEvent, MaterialSource, StructureCard, WritingProjection, WritingReviewItem, Gauge,
  SelfScoreFx, PredictionFx, ReflectionFx, FramingFx, PerspectivesFx, SpotCheckFx,
} from "@mind-imprint/contracts";
import type { AddMaterialBody } from "../api/materials";
import type { ReviewVoice } from "../api/writing";
import type { CreatedSpan } from "../primitives/annotate";

export type { Station, StationCode, StationView, StationState, CoachMessage, EquipCard, RubricRow, OnboardingFx, WritingProjection, WritingReviewItem, Gauge, SelfScoreFx, PredictionFx, ReflectionFx, FramingFx, PerspectivesFx, SpotCheckFx };

// N3f Task 7: the contract id of one of the two stations that have a
// spot-check defined — mirrors agent.SpotCheckSources/SpotCheckArgument
// verbatim (never a third value; an unknown contractId 404s server-side).
export type SpotCheckContractId = "evaluate_sources" | "build_argument";

// The five S4 argument role cards are the wire StructureCard verbatim — one
// shape across the boundary. status is only "done" | "empty"; the live
// inline-edit state the old "active" value modeled is now the full-pane
// StudioToulminCard, not a row in this list.
export type StructureCardFx = StructureCard;

// The 就绪度 gauge is now the contract `Gauge` type verbatim — one shape
// across the boundary (Slice 9 Task 5, mirrors StructureCardFx above).
export type GaugeFx = Gauge;

export type StudioState = {
  project: { title: string; qualLabel: string };
  stations: Station[];
  activeStation: StationCode;
  focusMode: boolean;
  coach: {
    anchor: string;
    messages: CoachMessage[];
    equipment: EquipCard[];
  };
  views: {
    material: MaterialSource[];
    structure: StructureCardFx[];
    writing: WritingProjection;
    review: GaugeFx[];
    onboarding: OnboardingFx;
    // N3d Task 9: S1 立题 / S2 视角与素材 panels — wire-shaped verbatim
    // (FramingFx/PerspectivesFx), same pattern as onboarding above. Views land
    // in Tasks 10/11.
    framing: FramingFx;
    perspectives: PerspectivesFx;
    // N2 Task 8: the 评估 view's self-score/prediction/reflection panels —
    // wire-shaped verbatim (SelfScoreFx/PredictionFx/ReflectionFx), same
    // pattern as GaugeFx/StructureCardFx above.
    selfScore: SelfScoreFx;
    prediction: PredictionFx;
    reflection: ReflectionFx;
    // N3f Task 7: the S3/S4 station spot-check panels — wire-shaped verbatim
    // (SpotCheckFx per station), same pattern as GaugeFx/framing/perspectives
    // above. Pure projected data; the order/disposition actions travel on
    // `spotCheck` below, not here.
    spotChecks: { evaluateSources: SpotCheckFx; buildArgument: SpotCheckFx };
  };
  // A3 Task 9: the project terminal — whether the project has already been
  // finished (archived, growth report generated) and whether it currently
  // qualifies to be. Mirrors `p.finished` / `p.canFinish` off the projection
  // verbatim (Task 3's fields), same pattern as GaugeFx/StructureCardFx above.
  finished: boolean;
  canFinish: boolean;
  // N3f Task 7: the S3/S4 spot-check panels' order-in-flight flags + actions.
  // Deliberately lives on StudioState rather than StudioCallbacks: `state` is
  // the one prop ViewFrame receives verbatim through StudioShell with no
  // manual per-field routing in between (every StudioCallbacks member, by
  // contrast, is explicitly picked into one of ViewFrame's prop groups inside
  // StudioShell.tsx) — folding the two callbacks + two pending flags in here
  // is the smallest change that reaches ViewFrame without touching
  // StudioShell.tsx, which this task's brief does not list. Optional so
  // standalone/story usages of ViewFrame (which never order a spot-check)
  // don't have to supply it.
  spotCheck?: {
    pendingEvaluateSources: boolean;
    pendingBuildArgument: boolean;
    onOrder: (contractId: SpotCheckContractId) => void;
    onDisposition?: (interventionId: string, action: "accept" | "rewrite" | "reject", reason: string) => void;
  };
};

export type StudioCallbacks = {
  onSelectStation: (code: StationCode) => void;
  onToggleFocus: () => void;
  onDisposition: (choice: "accept" | "rewrite" | "reject", reason: string) => void;
  onOpenMethodology: (cardId: string) => void;
  onComposerSend: (text: string) => void;
  // Live tool-card slot (Task 10): optional because the disposition/
  // placeholder path (no active card) never needs them.
  onOpenCard?: (cardInstanceId: string) => void;
  onSubmitCard?: (finalEnvelope: CardInstance) => void;
  onSkipCard?: (eventTrace: TraceEvent[]) => void;
  // Slice 6b Task 9: the 素材 dossier's ingestion form + reading-time ledger.
  // Optional for the same reason as the card slot above — standalone/story
  // usages of StudioShell never need them.
  onAddSource?: (body: AddMaterialBody) => Promise<void>;
  onOpenLogged?: (materialId: string, timeSpentS: number) => void;
  // Slice 8 Task 9: the 写作 view's silent buffer autosave + snapshot commit.
  // Optional for the same reason as the pair above — standalone/story usages
  // of StudioShell never need them.
  onBufferChange?: (text: string) => void;
  onCommit?: (text: string) => void;
  // Slice 8 Task 10: 整稿体检 work-order — order a review over the given
  // committed snapshot, record a three-key disposition on one review item,
  // and attest the student-written citations_matched gate item. Optional for
  // the same reason as the rest of this group.
  onOrderReview?: (snapshotId: string, voice: ReviewVoice) => void;
  onReviewDisposition?: (interventionId: string, action: "accept" | "rewrite" | "reject", reason: string) => void;
  onAttestCitations?: (confirmed: boolean) => void;
  // N3c task 9 (spec §8): the guidance-ladder locate flow — 「去文章里选出这
  // 句」 (coach rail) sets the pending locate request; the article pane
  // (center) reports back either a created span or a "couldn't find it"
  // escape. Optional for the same reason as the rest of this group —
  // standalone/story usages of StudioShell never need them.
  onRequestLocate?: (anchorId: string, dimension: string) => void;
  onSpanNotFound?: (anchorId: string, dimension: string) => void;
  onCreateSpan?: (span: CreatedSpan) => void;
  onCancelLocate?: () => void;
  // Whole-branch review IMPORTANT 1 (N3c): a located span is otherwise
  // permanent — unlike the `span_not_found` escape, which already has
  // 「重新找一下」 to undo it, a located span had no way back at all. Clears
  // this anchor's `locatedSpans` entry AND re-issues the locate request in
  // one action, mirroring the escape's undo-then-retake pattern. Optional
  // for the same reason as the rest of this group.
  onRelocate?: (anchorId: string, dimension: string) => void;
  // N3d Task 9: S1/S2 station views — each saves its whole panel set explicitly
  // (mirrors OnboardingView's submit shape, not autosave). Optional for the
  // same reason as the rest of this group; Tasks 10/11 wire the real handlers.
  onSubmitFraming?: (body: { terms: { term: string; definition: string }[]; answers: string[]; searchPlan: string[] }) => Promise<void>;
  onSubmitPerspectives?: (body: { perspectives: { text: string; level: string }[] }) => Promise<void>;
  // N3d Task 11: the S2 视角与素材 view's one explicit attestation
  // (sources_per_perspective) — mirrors onAttestCitations's shape above.
  // Optional for the same reason as the rest of this group; Task 12 wires
  // the real handler in from the container.
  onAttestSourcesPerPerspective?: (confirmed: boolean) => void;
};
