import type { CSSProperties } from "react";
import type { SelectionEval } from "@mind-imprint/contracts";
import type { ReadingOutcome } from "./readingLoop";

// The 阅读成果 (reading outcomes) view — the demo's `.trace-view`. Every
// finding the student CONFIRMS accumulates here as an integrated note card:
// her own selected sentence + the finding/judgment she reached + the full AI
// review, with a 回到原文 button that re-focuses the source sentence. This is
// the fix for the crux complaint — confirmed findings used to vanish.

// Tones read off the shared design-system semantic tokens (--mk-success/
// --mk-warning/--mk-danger, apps/web/src/index.css) rather than a local hex
// table — same mapping HangingCard.tsx uses for the identical verdict/check
// shape.
const VERDICT_TONE: Record<SelectionEval["verdict"], { fg: string; bg: string }> = {
  strong: { fg: "var(--mk-success)", bg: "var(--mk-success-bg)" },
  partial: { fg: "var(--mk-warning)", bg: "var(--mk-warning-bg)" },
  rethink: { fg: "var(--mk-danger)", bg: "var(--mk-danger-bg)" },
};

const CHECK_TONE: Record<SelectionEval["checks"][number]["status"], { fg: string; bg: string; mark: string; label: string }> = {
  pass: { fg: "var(--mk-success)", bg: "var(--mk-success-bg)", mark: "✓", label: "通过" },
  partial: { fg: "var(--mk-warning)", bg: "var(--mk-warning-bg)", mark: "~", label: "待补足" },
  miss: { fg: "var(--mk-danger)", bg: "var(--mk-danger-bg)", mark: "✕", label: "未通过" },
};

function OutcomeCard({ outcome, index, onLocate }: { outcome: ReadingOutcome; index: number; onLocate: () => void }) {
  const review = outcome.eval;
  const tone = VERDICT_TONE[review.verdict];
  return (
    <article className="mk-outcome-card">
      <header className="mk-outcome-card__head">
        <span className="mk-outcome-card__status">已确认</span>
        <strong className="mk-outcome-card__label">
          成果 {index + 1} · {outcome.cardName}
        </strong>
      </header>

      <section className="mk-outcome-card__finding">
        <span>我的阅读发现</span>
        <h4>{outcome.finding}</h4>
      </section>

      <blockquote className="mk-outcome-card__quote">
        <span>你的选句</span>
        <p>{outcome.quote}</p>
      </blockquote>

      <section className="mk-outcome-card__review" style={{ "--tone-fg": tone.fg, "--tone-bg": tone.bg } as CSSProperties}>
        <header>
          <span>AI 复核</span>
          <strong>{review.verdictLabel}</strong>
        </header>
        <p>{review.verdictReason}</p>
        <div className="mk-outcome-card__checks">
          {review.checks.map((check) => {
            const ct = CHECK_TONE[check.status];
            return (
              <div key={check.key} className="mk-outcome-card__check">
                <span style={{ color: ct.fg, background: ct.bg }}>{ct.mark}</span>
                <div>
                  <strong>{check.label}</strong>
                  <p>{check.explanation}</p>
                </div>
                <small style={{ color: ct.fg }}>{ct.label}</small>
              </div>
            );
          })}
        </div>
      </section>

      <dl className="mk-outcome-card__argument">
        <div>
          <dt>当前判断</dt>
          <dd>{outcome.judgment || "—"}</dd>
        </div>
        <div>
          <dt>复核依据</dt>
          <dd>{outcome.support || "—"}</dd>
        </div>
        <div>
          <dt>仍需确认</dt>
          <dd>{outcome.caveat || "—"}</dd>
        </div>
      </dl>

      {review.nextStep && (
        <div className="mk-outcome-card__next">
          <span>下一步</span>
          <p>{review.nextStep}</p>
        </div>
      )}

      <button type="button" className="mk-outcome-card__locate" onClick={onLocate}>
        回到原文继续思考
      </button>
    </article>
  );
}

export function ReadingOutcomes({
  outcomes,
  onLocate,
}: {
  outcomes: ReadingOutcome[];
  onLocate: (blockId: string) => void;
}) {
  return (
    <section className="mk-reading-room__trace">
      <div className="mk-reading-room__trace-intro">
        <span className="mk-reading-room__kicker">YOUR NOTES</span>
        <h2>我的阅读成果</h2>
        <p>每条成果都整合你的选句、当前判断和 AI 复核，并且能回到原文继续完善。</p>
      </div>
      <div className="mk-reading-room__trace-list">
        {outcomes.length === 0 ? (
          <div className="mk-outcome-placeholder">完成并保存一次透镜练习后，这里会形成第一条完整的阅读成果。</div>
        ) : (
          outcomes.map((outcome, i) => (
            <OutcomeCard key={outcome.id} outcome={outcome} index={i} onLocate={() => onLocate(outcome.blockId)} />
          ))
        )}
      </div>
    </section>
  );
}
