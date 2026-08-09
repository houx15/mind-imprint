import type { StudioStage } from "@mind-imprint/contracts";

// activeDocForStage (Phase B) — which writing document the writing room edits at
// a given studio status. The proposal statuses (写研究提案) operate on the
// proposal doc; everything else (default incl. body_writing / 写正文) operates on
// the essay — the final paper. Mirrors the backend StatusForStage → StatusDef.Doc
// collapse (agent/studioflow.go); kept tiny + tested so the two stay in step.
export function activeDocForStage(stage: StudioStage): "proposal" | "essay" {
  switch (stage) {
    case "proposal_writing":
    case "proposal_review":
      return "proposal";
    default:
      return "essay";
  }
}
