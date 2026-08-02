import type { TeachingModule } from "./types";
import { InnerPartsTeaching } from "./modules/InnerPartsTeaching";

// Escape-hatch registry for teaching modules.
export const teachingRegistry: Record<string, TeachingModule> = {
  "emotional-alignment": InnerPartsTeaching,
};

export function pickTeaching(cardId: string): TeachingModule | undefined {
  return teachingRegistry[cardId];
}
