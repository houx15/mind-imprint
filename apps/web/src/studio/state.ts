import type {
  Station, StationCode, StationView, StationState,
  CoachMessage, EquipCard, RubricRow, OnboardingFx,
  CardInstance, TraceEvent, MaterialSource, StructureCard,
} from "@mind-imprint/contracts";
import type { AddMaterialBody } from "../api/materials";

export type { Station, StationCode, StationView, StationState, CoachMessage, EquipCard, RubricRow, OnboardingFx };

// The five S4 argument role cards are the wire StructureCard verbatim — one
// shape across the boundary. status is only "done" | "empty"; the live
// inline-edit state the old "active" value modeled is now the full-pane
// StudioToulminCard, not a row in this list.
export type StructureCardFx = StructureCard;

export type GaugeFx = {
  table: string;        // 表A .. 表H
  total: number;
  lit: number;
  note: string;
  level: "full" | "partial" | "empty";
};

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
    writing: { draft: string; mode: "edit" | "preview" };
    review: GaugeFx[];
    onboarding: OnboardingFx;
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
};
