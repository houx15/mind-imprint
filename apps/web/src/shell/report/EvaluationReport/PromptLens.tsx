import type { EvaluationReport } from "@mind-imprint/contracts";
import { Card } from "@/ui";

export interface PromptLensProps {
  promptLens: EvaluationReport["promptLens"];
}

/**
 * STUB (Task 10/11 fills in) — real build ports `.plens`/`.prm`: stage +
 * domain chips, blockquote, observation, and an "attention" callout for
 * prompts flagged `attention: true` (no good/bad grade — see AGENTS.md铁律).
 */
export function PromptLens({ promptLens }: PromptLensProps) {
  return (
    <div data-testid="prompt-lens-section">
      <Card className="p-5">
        <p className="text-mk-body text-mk-secondary">{promptLens.summary}</p>
      </Card>
      <div className="mt-4 flex flex-col gap-3.5">
        {promptLens.prompts.map((p, i) => (
          <Card key={i} className="p-4">
            <span className="rounded-mk-full bg-mk-mist-bg px-2.5 py-0.5 text-mk-small font-bold text-mk-mist-fg">{p.stage}</span>
            <blockquote className="mt-2.5 border-l-2 border-mk-accent-100 pl-3.5 text-mk-h3 text-mk-ink">{p.quote}</blockquote>
            <p className="mt-2 text-mk-body text-mk-secondary">{p.observation}</p>
          </Card>
        ))}
      </div>
    </div>
  );
}
