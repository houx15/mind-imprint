import type { SourceFixture } from "../workspace/material/fixtures";

export type StationCode = "S0" | "S1" | "S2" | "S3" | "S4" | "S5" | "S6";
export type StationView = "结构" | "素材" | "写作" | "评估" | "onboarding";
export type StationState = "done" | "current" | "locked";

export type Station = {
  code: StationCode;
  name: string;
  view: StationView;
  state: StationState;
  gate?: { total: number; passed: number };
  backflow?: boolean;
};

export type CoachMessage =
  | { kind: "student"; body: string }
  | { kind: "ai"; body: string; tag?: string }
  | { kind: "flag"; label: string; body: string };

export type EquipCard = { id: string; name: string; spont: "自发" | "提示后" };

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

export type RubricRow = { official: string; plain: string; weak: boolean };
export type OnboardingFx = {
  restatePrompt: string;
  rubricRows: RubricRow[];
  planSteps: string[];  // 立题 / 找素材 / 评估来源 / 搭论证 / 成稿 / 反思归档
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
  onDisposition: (choice: "accept" | "revise" | "reject", reason: string) => void;
  onOpenMethodology: (cardId: string) => void;
  onComposerSend: (text: string) => void;
};
