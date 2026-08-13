import { Fragment } from "react";
import type { ReactNode } from "react";
import type { EvaluationReport } from "@mind-imprint/contracts";
import { BookOpen, Check, Lightbulb, PenLine, Star } from "lucide-react";
import { Card, Icon, MACARONS, Surface } from "@/ui";

export interface AbstractProps {
  abstract: EvaluationReport["abstract"];
}

/**
 * Splits `text` on `**…**` and wraps the odd (bold) segments in a
 * highlighted `<em>` — the mockup's `em.hl` emphasis convention. Even
 * segments render as plain text.
 */
export function renderEmphasis(text: string): ReactNode {
  const parts = text.split("**");
  return parts.map((part, i) =>
    i % 2 === 1 ? (
      <em key={i} className="rounded-sm bg-mk-accent-50 px-0.5 not-italic font-bold text-mk-accent-600">
        {part}
      </em>
    ) : (
      <Fragment key={i}>{part}</Fragment>
    ),
  );
}

/** `course:source-triage` → `Source Triage`. Report data carries reasons, not display names. */
function courseName(courseId: string): string {
  return courseId
    .replace(/^course:/, "")
    .split("-")
    .filter(Boolean)
    .map((w) => w[0]!.toUpperCase() + w.slice(1))
    .join(" ");
}

const ONE_SENTENCE_CARDS = [
  { key: "materialSentence" as const, label: "材料", icon: BookOpen, macaron: "lake" as const },
  { key: "writingSentence" as const, label: "写作", icon: PenLine, macaron: "matcha" as const },
  { key: "aiSentence" as const, label: "AI 使用", icon: Lightbulb, macaron: "taro" as const },
];

/**
 * Report abstract — ports `.grid3`/`.callout`/`.checks`/`.courses` from
 * `docs/reference/2026-08-13-eval-report-mockup.html`: overview paragraph
 * with `**…**` emphasis, 3 macaron one-sentence cards, accent callout,
 * check-list, recommended-course chips.
 */
export function Abstract({ abstract }: AbstractProps) {
  return (
    <div data-testid="abstract-section" className="flex flex-col gap-4">
      <Card className="p-6">
        <p className="text-mk-body-lg text-mk-ink">{renderEmphasis(abstract.overview)}</p>
      </Card>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {ONE_SENTENCE_CARDS.map((c) => {
          const macaron = MACARONS[c.macaron];
          return (
            <div key={c.key} className="rounded-mk-sm p-4 shadow-mk-xs" style={{ background: macaron.bg }}>
              <div className="mb-2 flex items-center gap-2 text-mk-h3" style={{ color: macaron.fg }}>
                <Icon icon={c.icon} size={17} />
                {c.label}
              </div>
              <p className="text-mk-body text-mk-ink">{abstract[c.key]}</p>
            </div>
          );
        })}
      </div>

      <div className="rounded-mk-sm border border-mk-accent-100 bg-mk-accent-50 p-5">
        <div className="mb-2 flex items-center gap-2 text-mk-label uppercase text-mk-accent-600">
          <Icon icon={Star} size={14} />
          下一步最值得训练的
        </div>
        <p className="text-mk-body text-mk-secondary">{renderEmphasis(abstract.suggestionParagraph)}</p>
      </div>

      <ul className="flex flex-col gap-2.5">
        {abstract.suggestionSentences.map((s, i) => (
          <li key={i} className="flex items-start gap-2.5 text-mk-body text-mk-secondary">
            <Icon icon={Check} size={18} className="mt-0.5 shrink-0 text-mk-success" />
            {s}
          </li>
        ))}
      </ul>

      <div className="flex flex-wrap gap-3.5">
        {abstract.recommendedCourses.map((c) => (
          <Surface key={c.courseId} className="flex max-w-[380px] gap-3 p-4">
            <span
              aria-hidden
              className="h-11 w-11 shrink-0 rounded-mk-sm"
              style={{ background: "linear-gradient(135deg, var(--mk-taro), var(--mk-mist))" }}
            />
            <div>
              <div className="text-mk-body font-semibold text-mk-ink">{courseName(c.courseId)}</div>
              <div className="mt-0.5 text-mk-small text-mk-muted">{c.reason}</div>
            </div>
          </Surface>
        ))}
      </div>
    </div>
  );
}
