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
  credibility: boolean;
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
  credibility: true,
};

// Demo is read-only pro, not a third lifecycle — same surfaces.
export const DEMO_CAPABILITIES: RoomCapabilities = { ...PRO_CAPABILITIES, mode: "demo" };

// comprehensionCheck is forward-declared true here — a genuine reading
// capability landing in P2; nothing consumes it yet.
//
// exemplars stays false here: it denotes the English-writing 示范 paragraphs,
// a WRITING-room feature landing in P3. A reading preset has no business
// enabling it — its correct home is a future LITE_WRITING_CAPABILITIES. It
// stays on the shared RoomCapabilities type (forward-declared) so the writing
// preset can turn it on when it lands; it just isn't this preset's to enable.
// credibility stays false here: it names the 可信度 verdict a CRAAP-style
// producer writes onto the record (verdict + why). Lite has no such
// producer, so the field could only ever read 尚未评估 — a permanent lie
// dressed as a status. Off until lite grows something that actually
// evaluates credibility.
export const LITE_READING_CAPABILITIES: RoomCapabilities = {
  mode: "lite",
  plan: false,
  evidenceMap: false,
  explorationLeads: false,
  proposalImpact: false,
  essayTrack: false,
  comprehensionCheck: true,
  exemplars: false,
  credibility: false,
};
