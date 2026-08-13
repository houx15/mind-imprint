import type { EvaluationReport } from "@mind-imprint/contracts";
import { Card } from "@/ui";

export interface PromptLensProps {
  promptLens: EvaluationReport["promptLens"];
}

/**
 * PromptLens — ports `.plens`/`.prm` from
 * `docs/reference/2026-08-13-eval-report-mockup.html`: an intro summary
 * card, then per `PromptItem` a stage chip, an "attention" chip ONLY when
 * `attention === true` (`需要留意`), the `relatedDomains` chip, the `quote`
 * as a blockquote, and the `observation`. Deliberately no good/bad grade of
 * any kind — a prompt is not scored, only whether it produced downstream
 * process evidence (see AGENTS.md 铁律①: AI 不替学生定论).
 */
export function PromptLens({ promptLens }: PromptLensProps) {
  return (
    <div data-testid="prompt-lens-section" className="flex flex-col gap-4">
      <Card className="p-5">
        <p className="text-mk-body text-mk-secondary">{promptLens.summary}</p>
      </Card>
      <div className="flex flex-col gap-3.5">
        {promptLens.prompts.map((p, i) => (
          <Card key={i} className="p-4" data-testid="prompt-item">
            <div className="mb-2.5 flex flex-wrap items-center gap-2">
              <span className="rounded-mk-full bg-mk-mist-bg px-2.5 py-0.5 text-mk-small font-bold text-mk-mist-fg">
                {p.stage}
              </span>
              {p.attention && (
                <span className="rounded-mk-full bg-mk-warning-bg px-2.5 py-0.5 text-mk-small font-bold text-mk-warning">
                  需要留意
                </span>
              )}
              <span className="rounded-mk-full bg-mk-taro-bg px-2.5 py-0.5 text-mk-small font-semibold text-mk-taro-fg">
                {p.relatedDomains.join(" · ")}
              </span>
            </div>
            <blockquote className="mb-2.5 border-l-2 border-mk-accent-100 pl-3.5 text-mk-h3 text-mk-ink">
              {p.quote}
            </blockquote>
            <p className="text-mk-body text-mk-secondary">
              <span className="font-semibold text-mk-ink">观察意义：</span>
              {p.observation}
            </p>
          </Card>
        ))}
      </div>
    </div>
  );
}
