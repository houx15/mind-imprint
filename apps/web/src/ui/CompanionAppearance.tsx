import { createContext, useContext, type ReactNode } from "react";

/** Optional character artwork. Legacy consumers keep the original SVG. */
const CompanionAppearance = createContext<string | null>(null);
export function CompanionAppearanceProvider({ image, children }: { image: string; children: ReactNode }) {
  return <CompanionAppearance.Provider value={image}>{children}</CompanionAppearance.Provider>;
}
export const useCompanionImage = () => useContext(CompanionAppearance);
