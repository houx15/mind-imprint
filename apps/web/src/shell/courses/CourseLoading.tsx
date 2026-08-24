import { PebbleProgress, useRotatingCaption } from "@/ui";

/**
 * The shared "loading a course" affordance. Every full-panel course load —
 * the player router probing for a 2.0 definition, RuntimeCoursePlayer /
 * CoursePlayer waiting on their payload, CourseDetail fetching its summary,
 * CourseReport assembling — used to fall back to a bare "正在加载课程…" line (and
 * CourseReport even reinvented its own CSS spinner). This gives them ONE
 * consistent, on-brand treatment: the 豆豆 rides the accent fill bar
 * (`PebbleProgress`, indeterminate), a caption below.
 *
 * Pass `caption` for a single line, or `captions` for honest rotating lines on
 * the slower "assembling your report" moments. The global reduced-motion block
 * (index.css) already stills the bar's animation for those users.
 */
export function CourseLoading({
  caption,
  captions,
  slim = false,
}: {
  caption?: string;
  /** Honest rotating lines — cycles ~every 2.2s. Takes precedence over `caption`. */
  captions?: string[];
  slim?: boolean;
}) {
  const rotating = useRotatingCaption(captions ?? []);
  const line = captions && captions.length > 0 ? rotating : caption ?? "正在加载课程…";
  return (
    <div
      role="status"
      aria-live="polite"
      aria-busy="true"
      style={{
        height: "100%",
        minHeight: 0,
        flex: 1,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: 6,
        padding: 40,
        background: "var(--mk-paper)",
      }}
    >
      <div style={{ width: 232 }}>
        <PebbleProgress slim={slim} />
      </div>
      <div style={{ fontSize: 14, color: "var(--mk-muted)", fontWeight: 600 }}>{line}</div>
    </div>
  );
}
