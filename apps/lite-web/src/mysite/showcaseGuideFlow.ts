import type { ShowcaseGuideDestination, ShowcaseGuideStage, ShowcaseState } from "../api/showcase";

export type ShowcaseGuidePhase = "welcome" | "build" | "revise";
export type ShowcaseBuildStep = Exclude<ShowcaseGuideStage, "revise">;

const buildSteps: ShowcaseBuildStep[] = ["design", "hero", "profile", "works", "components", "finish"];

export function initialShowcaseGuide(state: Pick<ShowcaseState, "draft" | "revision" | "published" | "hasLegacySite">): {phase: ShowcaseGuidePhase; stage: ShowcaseGuideStage} {
  if (state.published || state.hasLegacySite || state.draft.guideCompleted) return {phase: "revise", stage: "revise"};
  if (state.draft.guideStage && buildSteps.includes(state.draft.guideStage)) return {phase: "build", stage: state.draft.guideStage};
  if (state.revision > 0) return {phase: "revise", stage: "revise"};
  return {phase: "welcome", stage: "design"};
}

export function destinationEditor(destination: ShowcaseGuideDestination): {stage: ShowcaseBuildStep; heroTab?: "text" | "art" | "image"; profileTab?: "content" | "tree" | "avatar"} {
  switch (destination) {
    case "hero-text": return {stage: "hero", heroTab: "text"};
    case "hero-art": return {stage: "hero", heroTab: "art"};
    case "hero-image": return {stage: "hero", heroTab: "image"};
    case "profile-content": return {stage: "profile", profileTab: "content"};
    case "profile-tree": return {stage: "profile", profileTab: "tree"};
    case "profile-avatar": return {stage: "profile", profileTab: "avatar"};
    default: return {stage: destination};
  }
}
