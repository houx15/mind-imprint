import type { RuntimeSceneResult } from "@mind-imprint/course-contract";

export interface ClosingSceneProps {
  scene: RuntimeSceneResult;
  summary: string;
  takeaways: string[];
  transferApplications: string[];
}

/**
 * §6.2 / §17.3 — presentational closing: the prepared/generated narration plus
 * the prepared summary, takeaways, and transfer applications. Purely
 * presentational — the {@link RuntimeSceneResult} comes from the closing
 * generator (fallback in this slice).
 */
export function ClosingScene({ scene, summary, takeaways, transferApplications }: ClosingSceneProps) {
  return (
    <section className="course-closing" aria-label="课程收尾" data-fallback={scene.fallbackUsed ? "true" : undefined}>
      <p className="course-closing__narration">{scene.text}</p>
      <h2 className="course-closing__summary-heading">小结</h2>
      <p className="course-closing__summary">{summary}</p>
      <h3 className="course-closing__takeaways-heading">要点</h3>
      <ul className="course-closing__takeaways">
        {takeaways.map((item, i) => (
          <li key={i}>{item}</li>
        ))}
      </ul>
      <h3 className="course-closing__transfer-heading">迁移应用</h3>
      <ul className="course-closing__transfer">
        {transferApplications.map((item, i) => (
          <li key={i}>{item}</li>
        ))}
      </ul>
    </section>
  );
}
