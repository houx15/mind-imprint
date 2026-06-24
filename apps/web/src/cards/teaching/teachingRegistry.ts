import type { TeachingModule } from "./types";
import { SiftCraapTeaching } from "./modules/SiftCraapTeaching";

// Escape-hatch registry for teaching modules. Task 13 will add emotional-alignment.
export const teachingRegistry: Record<string, TeachingModule> = {
  sift_craap: SiftCraapTeaching,
};

export function pickTeaching(cardId: string): TeachingModule | undefined {
  return teachingRegistry[cardId];
}
