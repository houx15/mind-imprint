import curious from "../home/assets/curious-learner-v2.webp";
import together from "../home/assets/learning-together-v3.webp";
import makers from "../home/assets/project-makers-v2.webp";
import "./landing.css";

const ART = { reading: curious, writing: together, project: makers };

/** Shared presentation for Lite entry pages; owns no data or workflow state. */
export function LandingHeader({
  title,
  description,
  kind,
}: {
  title: string;
  description: string;
  kind: keyof typeof ART;
}) {
  return (
    <header className="learning-landing-header">
      <div>
        <span className="learning-landing-kicker">学习空间</span>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      <img src={ART[kind]} alt="" />
    </header>
  );
}
