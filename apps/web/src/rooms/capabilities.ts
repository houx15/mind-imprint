// capabilities.ts — what a room is allowed to show.
//
// The rooms used to reach for project-lifecycle facts directly (and carried a
// separate `demoMode` boolean). The lite edition runs the SAME room components
// with no project around them, so a room now reads one object instead of
// asking what stage a project is in. Adding an edition means adding a preset
// here, not threading another boolean through the tree.
export type RoomCapabilities = {
  mode: "pro" | "lite" | "demo";
  plan: boolean;
  evidenceMap: boolean;
  explorationLeads: boolean;
  proposalImpact: boolean;
  essayTrack: boolean;
  comprehensionCheck: boolean;
  exemplars: boolean;
};

export const PRO_CAPABILITIES: RoomCapabilities = {
  mode: "pro",
  plan: true,
  evidenceMap: true,
  explorationLeads: true,
  proposalImpact: true,
  essayTrack: true,
  comprehensionCheck: false,
  exemplars: false,
};

// Demo is read-only pro, not a third lifecycle — same surfaces.
export const DEMO_CAPABILITIES: RoomCapabilities = { ...PRO_CAPABILITIES, mode: "demo" };

// comprehensionCheck and exemplars are forward-declared true here for lite
// reading (they land in P2/P3); nothing consumes them yet.
export const LITE_READING_CAPABILITIES: RoomCapabilities = {
  mode: "lite",
  plan: false,
  evidenceMap: false,
  explorationLeads: false,
  proposalImpact: false,
  essayTrack: false,
  comprehensionCheck: true,
  exemplars: true,
};
