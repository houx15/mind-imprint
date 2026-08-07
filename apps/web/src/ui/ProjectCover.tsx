/**
 * ProjectCover — full-bleed project/directory-card cover (image or gradient).
 *
 * A project's stored `cover` is one of "img:<n>" (a pre-uploaded OSS photo,
 * resolved server-side to a signed `coverUrl`), "grad:<macaron>" (one of the
 * 7 macaron gradients, student-picked at create), or unset. This component
 * is the single place that turns those into pixels so Directory and
 * HomePage never duplicate the branching — same rules, same fallback.
 *
 * Old projects (cover unset/empty, or coverUrl missing for an "img:" cover
 * whose signing failed) fall back to the deterministic title-hash gradient
 * that predates covers entirely — they render exactly as before.
 */
import { coverGradientStyle } from "./cover";

export interface ProjectCoverData {
  cover?: string;
  coverUrl?: string;
  title?: string;
  id: string;
}

function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export function ProjectCover({ project, className }: { project: ProjectCoverData; className?: string }) {
  if (project.cover?.startsWith("img:") && project.coverUrl) {
    return <img src={project.coverUrl} alt="" className={cx("h-[120px] w-full object-cover", className)} />;
  }

  // "grad:<macaron>" seeds with the macaron's own name (matches the create
  // drawer's swatch preview, so the rendered color always matches what the
  // student picked); unset/legacy covers fall back to the title/id hash.
  const gradSeed = project.cover?.startsWith("grad:") ? project.cover.slice("grad:".length) : project.title || project.id;
  return <div className={cx("h-[120px] w-full", className)} style={coverGradientStyle(gradSeed)} />;
}
