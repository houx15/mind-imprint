import { PROJECT_KIND_LABELS, projectTitle, type Project } from "../api/projects";
import { resolveCover } from "./covers";

/**
 * One project on the kanban.
 *
 * Deliberately quiet. There is no progress bar, no percentage, no step count
 * and no streak — a board that scores her projects against each other turns
 * "what am I working on" into "which am I behind on", and 铁律② rules out
 * exactly that kind of pressure.
 *
 * The title falls back to her own opening sentence rather than 「未命名」
 * (see `projectTitle`): before she has named it, what she wrote is the truest
 * description of the project that exists.
 */
export function ProjectCard({ project, onOpen }: { project: Project; onOpen: (p: Project) => void }) {
  // Via `resolveCover` so the board and the modal's preview can never disagree
  // about a project that has no cover yet — see that function's comment.
  const { ground, glyph } = resolveCover(project.kind, project.coverGround, project.coverGlyph);
  const title = projectTitle(project);
  const named = project.name.trim().length > 0;

  return (
    <button
      type="button"
      onClick={() => onOpen(project)}
      className="group flex w-full gap-3 rounded-mk-lg border border-mk-border bg-mk-surface p-3 text-left shadow-mk-xs transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <span
        aria-hidden
        className="flex h-11 w-11 shrink-0 items-center justify-center rounded-mk-md text-[19px] leading-none"
        style={{
          background: `linear-gradient(145deg, ${ground.from}, ${ground.to})`,
          color: ground.ink,
        }}
      >
        {glyph}
      </span>

      <span className="flex min-w-0 flex-col gap-1">
        <span
          className={
            named
              ? "truncate text-mk-body font-semibold text-mk-ink"
              : // Unnamed: it is a sentence, not a title, so it is set as one —
                // two lines, normal weight, and visibly her words.
                "line-clamp-2 text-mk-small text-mk-secondary"
          }
        >
          {title}
        </span>
        <span className="text-mk-label text-mk-muted">
          {PROJECT_KIND_LABELS[project.kind] ?? project.kind}
        </span>
      </span>
    </button>
  );
}
