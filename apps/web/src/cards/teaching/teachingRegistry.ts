import type { TeachingModule } from "./types";

// Escape-hatch registry for teaching modules. Initially empty; Tasks 12 and 13
// will populate this with concrete modules.
export const teachingRegistry: Record<string, TeachingModule> = {};

export function pickTeaching(cardId: string): TeachingModule | undefined {
  return teachingRegistry[cardId];
}
