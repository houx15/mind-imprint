import type { EvaluationReport } from "@mind-imprint/contracts";
import { Card } from "@/ui";

export interface AbstractProps {
  abstract: EvaluationReport["abstract"];
}

/**
 * STUB (Task 8) — real build is Task 9: `renderEmphasis()` for `**…**`
 * spans, 3 macaron one-sentence cards, accent callout (suggestionParagraph),
 * check-list (suggestionSentences), recommended-course chips.
 */
export function Abstract({ abstract }: AbstractProps) {
  return (
    <Card className="p-6" data-testid="abstract-section">
      <p className="text-mk-body-lg text-mk-ink">{abstract.overview.replace(/\*\*/g, "")}</p>
      <p className="mt-3 text-mk-body text-mk-secondary">{abstract.materialSentence}</p>
    </Card>
  );
}
