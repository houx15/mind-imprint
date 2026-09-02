import { useEffect, useMemo, useState } from "react";
import { ArrowRight } from "lucide-react";
import { Icon } from "@/ui";
import { ApiError } from "../api/client";
import {
  createProject,
  groupByStatus,
  listProjects,
  PROJECT_STATUSES,
  PROJECT_STATUS_LABELS,
  type Project,
} from "../api/projects";
import { navigate, projectPath } from "../routing";
import { ProjectCard } from "./ProjectCard";
import { apiErrorText } from "../api/errorText";

/**
 * ProjectsLanding — the 项目 tab's front door.
 *
 * TWO PARTS, IN THIS ORDER.
 *
 *  1. **A big input box.** The same shape as the 阅读 and 写作 front doors: she
 *     writes what she wants to do, in her own words, and 印记 works out what
 *     kind of project that is. She never picks a category off a menu — a
 *     13-year-old with an idea does not yet know whether it is "research" or
 *     "design", and making her choose first is asking her to classify a thing
 *     she has not built yet.
 *  2. **A kanban of everything she has going.** Reading and writing each
 *     concern one thing at a time, so their history lives in a shelf. Projects
 *     do not: she accumulates questions, some still being talked about, some
 *     underway, some finished, some being kept alive. The board is the honest
 *     shape of that, and it is why 项目 is one page rather than a room with a
 *     sidebar.
 *
 * The 圈点 ink-stroke flourish from `ReadingsLanding` is deliberately NOT
 * repeated here. That gesture is a reader's red pen; borrowing it for a second
 * page would turn a specific idea into product decoration.
 */
export function ProjectsLanding() {
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [idea, setIdea] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  /** Set once a project exists on the server and she has yet to name it. */

  useEffect(() => {
    let cancelled = false;
    listProjects()
      .then((rows) => {
        if (!cancelled) setProjects(rows);
      })
      .catch((err) => {
        if (!cancelled) {
          setProjects([]);
          setLoadError(apiErrorText(err));
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const columns = useMemo(() => groupByStatus(projects ?? []), [projects]);
  const isFirstEver = projects !== null && projects.length === 0;

  async function handleStart() {
    const text = idea.trim();
    if (!text || creating) return;
    setCreating(true);
    setError(null);
    try {
      const created = await createProject(text);
      setProjects((prev) => [created, ...(prev ?? [])]);
      setIdea("");
      // 直接进房间。产品负责人 2026-09-02：名字等问题想清楚了再改，别在她刚
      // 写下一句话、还什么都没想明白的时候拦住她要一个名字。她写的那句话进去
      // 就是她对印记说的第一句话。
      navigate(projectPath(created.id));
    } catch (err) {
      // 印记 failing to read her idea is surfaced, never smoothed over into a
      // project with a guessed type.
      setError(apiErrorText(err));
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="relative min-h-full">
      <div className="relative mx-auto flex w-full max-w-[1100px] flex-col px-4 pb-20 pt-16 sm:px-6">
        {/* ── 1 · the big box ─────────────────────────────────────────── */}
        <div className="mx-auto w-full max-w-[720px]">
          <h1 className="text-center text-mk-display text-mk-ink">最近想做点什么</h1>
          <p className="mt-3 text-center text-mk-body text-mk-secondary">
            一句话就行。想清楚要做什么，是我们一起的第一件事。
          </p>

          <div className="mt-7 rounded-mk-lg border border-mk-border bg-mk-surface p-3 shadow-mk-sm focus-within:border-mk-accent-200">
            <textarea
              value={idea}
              onChange={(e) => setIdea(e.target.value)}
              rows={4}
              disabled={creating}
              placeholder="比如：我们学校每天剩好多饭，我想弄明白这些饭最后去哪了，能不能少一点。"
              className="w-full resize-none bg-transparent text-mk-prose text-mk-ink outline-none placeholder:text-mk-faint"
            />
            {/* Stacked on a phone. Side by side, the hint wrapped to two lines
                and squeezed the button until 开始 broke across two lines as
                「开 / 始」 — a two-character button cannot be allowed to wrap,
                so the button is nowrap and the row stops competing for width
                below `sm`. */}
            <div className="flex flex-col items-stretch gap-2 pt-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
              <span className="order-2 text-mk-small text-mk-muted sm:order-1">
                {creating ? "印记在读你写的这段话…" : "写完按开始，印记会先跟你聊清楚要做什么。"}
              </span>
              <button
                type="button"
                onClick={handleStart}
                disabled={!idea.trim() || creating}
                className="order-1 flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white transition-opacity duration-[120ms] ease-mk disabled:opacity-40 sm:order-2"
                style={{ background: "var(--mk-accent-500)" }}
              >
                开始
                <Icon icon={ArrowRight} size={15} />
              </button>
            </div>
          </div>

          {error && (
            <p className="mt-3 text-center text-mk-small" style={{ color: "var(--mk-danger)" }}>
              {error}
            </p>
          )}

          {isFirstEver && (
            <p className="mt-6 text-center text-mk-small text-mk-muted">
              你的第一个项目是做一个属于你自己的主页。往后读过的、写过的、做过的，都能放上去。
            </p>
          )}
        </div>

        {/* ── 2 · the board ───────────────────────────────────────────── */}
        {loadError && (
          <p className="mt-10 text-center text-mk-small" style={{ color: "var(--mk-danger)" }}>
            {loadError}
          </p>
        )}

        {projects !== null && projects.length > 0 && (
          // Columns scroll on the x axis together; the PAGE never does.
          <div className="mt-16 -mx-4 overflow-x-auto px-4 sm:-mx-6 sm:px-6">
            <div className="flex min-w-[880px] gap-4">
              {PROJECT_STATUSES.map((status) => (
                <section key={status} className="flex w-full min-w-[168px] flex-col gap-3">
                  <header className="flex items-baseline justify-between border-b border-mk-border pb-2">
                    <h2 className="text-mk-label uppercase text-mk-secondary">
                      {PROJECT_STATUS_LABELS[status]}
                    </h2>
                    <span className="text-mk-small text-mk-faint">{columns[status].length}</span>
                  </header>
                  <div className="flex flex-col gap-2">
                    {columns[status].map((p) => (
                      // Tapping a card goes INTO the project. The naming modal
                      // opens once, right after creation — reopening it on
                      // every visit would make renaming the main thing a card
                      // does, which it is not.
                      <ProjectCard key={p.id} project={p} onOpen={(x) => navigate(projectPath(x.id))} />
                    ))}
                  </div>
                </section>
              ))}
            </div>
          </div>
        )}
      </div>

    </div>
  );
}
