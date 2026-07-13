import type { SourceFixture } from "./material/fixtures";
import type {
  Station, StationCode, StationView, StationState,
  CoachMessage, EquipCard, RubricRow, OnboardingFx,
  CardInstance, TraceEvent,
} from "@mind-imprint/contracts";

export type { Station, StationCode, StationView, StationState, CoachMessage, EquipCard, RubricRow, OnboardingFx };

export type StructureCardFx = {
  id: string;
  role: string;         // 核心主张 / 理据·推理 / 支撑证据 / 反方·钢人 / 让步·转折
  status: "done" | "active" | "empty";
  preview?: string;     // collapsed text when done
  question?: string;    // AI question when active
};

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
    material: SourceFixture[];
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
};
