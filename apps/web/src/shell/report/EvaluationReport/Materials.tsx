import type { EvaluationReport } from "@mind-imprint/contracts";
import { Icon, Surface, X } from "@/ui";

export interface MaterialsProps {
  materials: EvaluationReport["materials"];
}

type MaterialTone = "success" | "warning" | "danger" | "neutral";

const TONE_STYLE: Record<MaterialTone, { bar: string; chip: string }> = {
  success: { bar: "bg-mk-success", chip: "bg-mk-success-bg text-mk-success" },
  warning: { bar: "bg-mk-warning", chip: "bg-mk-warning-bg text-mk-warning" },
  danger: { bar: "bg-mk-danger", chip: "bg-mk-danger-bg text-mk-danger" },
  neutral: { bar: "bg-mk-faint", chip: "bg-mk-paper text-mk-muted" },
};

/**
 * Maps a material's free-text `finalStatus` to a credibility tone. Keyword
 * substring match, not an enum — `finalStatus` is model-authored prose (see
 * mock fixture: "bridge source", "限制性证据", "candidate / removed", …), so
 * this stays a best-effort classifier with a neutral fallback rather than a
 * closed switch.
 */
function materialTone(finalStatus: string): MaterialTone {
  const s = finalStatus.toLowerCase();
  if (s.includes("rabbit-hole")) return "neutral";
  if (s.includes("counterclaim") || s.includes("candidate") || s.includes("removed")) return "danger";
  if (s.includes("限制") || s.includes("第二媒介") || s.includes("origin")) return "warning";
  if (s.includes("核心") || s.includes("bridge")) return "success";
  return "neutral";
}

/**
 * Materials — ports `.mats`/`.mat` from
 * `docs/reference/2026-08-13-eval-report-mockup.html`: one hairline card per
 * material with a credibility color left-bar (derived from `finalStatus`),
 * the source, a `finalStatus` chip, a `usedIn` chip when non-null, the
 * `comment` prose, and a red `cannotSupport` boundary line.
 */
export function Materials({ materials }: MaterialsProps) {
  return (
    <div className="flex flex-col gap-3.5" data-testid="materials-section">
      {materials.map((m) => {
        const tone = TONE_STYLE[materialTone(m.finalStatus)];
        const usedInLabel = m.usedIn ? (m.usedIn.label ?? m.usedIn.id) : null;
        return (
          <Surface key={m.materialId} level="md" radius="md" className="overflow-hidden p-0" data-testid="material-card">
            <div className="flex items-stretch">
              <span aria-hidden className={`w-1.5 shrink-0 ${tone.bar}`} />
              <div className="min-w-0 flex-1 p-4">
                <div className="flex flex-wrap items-center gap-2.5">
                  <span className="text-mk-h3 text-mk-ink">{m.source}</span>
                  <span className={`rounded-mk-full px-2.5 py-0.5 text-mk-small font-bold ${tone.chip}`}>
                    {m.finalStatus}
                  </span>
                  {usedInLabel ? (
                    <span className="rounded-mk-full bg-mk-lake-bg px-2.5 py-0.5 text-mk-small font-semibold text-mk-lake-fg">
                      {usedInLabel}
                    </span>
                  ) : null}
                </div>
                <p className="mt-2.5 text-mk-body text-mk-secondary">{m.comment}</p>
                <p className="mt-2 flex items-start gap-1.5 text-mk-small text-mk-danger">
                  <Icon icon={X} size={14} className="mt-0.5 shrink-0" />
                  <span>{m.cannotSupport}</span>
                </p>
              </div>
            </div>
          </Surface>
        );
      })}
    </div>
  );
}
