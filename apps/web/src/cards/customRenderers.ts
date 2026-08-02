import type { ComponentType } from "react";
import { CardRenderer, type CardBodyProps } from "./CardRenderer";
import { BeliefSpectrumRenderer } from "./renderers/BeliefSpectrumRenderer";
import { InnerPartsRenderer } from "./renderers/InnerPartsRenderer";

// Escape-hatch: a card_id may map to a bespoke renderer that overrides the
// schema CardRenderer. The hard guardrail is the shared CardBodyProps contract
// — a custom renderer writes ONLY field_values via onField, so CardSheetHost
// builds the standard envelope identically. Custom look, standard output.
export const customRenderers: Record<string, ComponentType<CardBodyProps>> = {
  "belief-spectrum": BeliefSpectrumRenderer,
  "emotional-alignment": InnerPartsRenderer,
};

export function pickCardBody(cardId: string): ComponentType<CardBodyProps> {
  return customRenderers[cardId] ?? CardRenderer;
}
