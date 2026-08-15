import type { RuntimeSceneResult } from "@mind-imprint/course-contract";

/** The fixed opening start-action label (§6.1). */
export const OPENING_START_LABEL = "一起开始吧";

export interface OpeningSceneProps {
  scene: RuntimeSceneResult;
  title: string;
  estimatedMinutes: number;
  objectives: string[];
  learningPreview: string[];
  onStart: () => void;
}

/**
 * §6.1 / §17.3 — presentational opening: greeting (the prepared/generated scene
 * text), the course title + estimate + learning preview, and the single fixed
 * start action. Carries no generation logic; it renders a {@link RuntimeSceneResult}
 * the CoursePlayer obtained from the opening generator (fallback in this slice).
 */
export function OpeningScene({ scene, title, estimatedMinutes, learningPreview, onStart }: OpeningSceneProps) {
  return (
    <section className="course-opening" aria-label="课程开场" data-fallback={scene.fallbackUsed ? "true" : undefined}>
      <h1 className="course-opening__title">{title}</h1>
      <p className="course-opening__greeting">{scene.text}</p>
      <p className="course-opening__estimate">预计 {estimatedMinutes} 分钟</p>
      {learningPreview.length > 0 ? (
        <ul className="course-opening__preview">
          {learningPreview.map((item, i) => (
            <li key={i}>{item}</li>
          ))}
        </ul>
      ) : null}
      <button type="button" className="course-opening__start" onClick={onStart}>
        {OPENING_START_LABEL}
      </button>
    </section>
  );
}
