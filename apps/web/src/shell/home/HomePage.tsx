import { useEffect, useMemo, useState } from "react";
import type { CourseSummary } from "@mind-imprint/contracts";
import { api, type MeUser, type ProjectListItem } from "@/api";
import { selectHomeCourses } from "./selectHomeCourses";
import { CourseCard, coursePct } from "@/catalog/CourseCard";
import { ProjectCard, NewProjectTile } from "@/catalog/ProjectCard";
import { Button, SkeletonCard, EmptyState, Illustration } from "@/ui";

/**
 * HomePage — 首页 (shell Task 5).
 *
 * Three sections inside a centered ~1200px container on the warm paper bg:
 * a greeting header, a horizontal "最近项目" snapshot (first tile always
 * "新建"), and a "最近课程" grid. Both card kinds are the SHARED catalog cards
 * (`@/catalog/ProjectCard`, `@/catalog/CourseCard`) — the exact same components
 * the 项目 / 课程 list pages render, so the home snapshot never drifts from the
 * full lists. Each data section fetches independently so a failure in one (soft
 * inline message) never blanks the other — see `ProjectsSection`/`CoursesSection`.
 */

function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

const RECENT_LIMIT = 3;
// Width of a tile in the horizontal 最近项目 strip. Height is content-driven —
// the flex row stretches every tile (and the shared card's `h-full`) to the
// tallest, so they stay even without a fixed height.
const TILE_W = "w-[260px]";

function ProjectsSection({
  onOpenProject,
  onCreateProject,
  onGoProjects,
  onViewReport,
}: {
  onOpenProject: (id: string) => void;
  onCreateProject: () => void;
  onGoProjects: () => void;
  onViewReport: (id: string) => void;
}) {
  const [projects, setProjects] = useState<ProjectListItem[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api
      .listProjects()
      .then((ps) => {
        if (!cancelled) setProjects(ps);
      })
      .catch(() => {
        if (!cancelled) {
          setFailed(true);
          setProjects([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Rename in place, mirroring the 项目 directory — the shared ProjectCard's ⋯
  // menu drives this, and the snapshot updates without a refetch.
  async function handleRename(id: string, title: string) {
    const { title: saved } = await api.renameProject(id, title);
    setProjects((prev) => (prev ? prev.map((p) => (p.id === id ? { ...p, title: saved } : p)) : prev));
  }

  const loading = projects === null;
  // The shared, world-readable demo project is appended to every user's list by
  // the API (isDemo). It belongs in the 项目 directory (badged, behind the
  // guide-or-leave guard) — not the home "最近项目" snapshot, whose cards open
  // straight through `onOpenProject` and would bypass that guard (a brand-new
  // user, whose only listed project IS the demo, would otherwise land in the
  // read-only studio unguarded). Keep it out of this strip.
  const recent = (projects ?? []).filter((p) => !p.isDemo).slice(0, RECENT_LIMIT);

  return (
    <section className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="text-mk-h2 text-mk-ink">最近项目</h2>
        {!loading && !failed && recent.length > 0 && (
          <Button variant="link" onClick={onGoProjects}>
            查看全部 →
          </Button>
        )}
      </div>

      {loading ? (
        <div className="flex gap-4 overflow-x-auto pb-2">
          {Array.from({ length: 4 }, (_, i) => (
            <SkeletonCard key={i} className={cx(TILE_W, "shrink-0")} />
          ))}
        </div>
      ) : failed ? (
        <p className="text-mk-body text-mk-muted">最近项目暂时加载不出来，刷新一下再试试。</p>
      ) : recent.length === 0 ? (
        <EmptyState
          illustration="emptyProjects"
          title="快来创建你的第一个写作项目吧！"
          body="带上你真实的写作任务，印记陪你把思路一点点想清楚。"
          action={{ label: "新建项目", onClick: onCreateProject }}
        />
      ) : (
        <div className="flex items-stretch gap-4 overflow-x-auto pb-2">
          <NewProjectTile onClick={onCreateProject} className={cx(TILE_W, "shrink-0")} />
          {recent.map((p) => (
            <div key={p.id} className={cx(TILE_W, "shrink-0")}>
              <ProjectCard
                project={p}
                onOpen={() => onOpenProject(p.id)}
                onRename={handleRename}
                onViewReport={onViewReport}
              />
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function CoursesSection({
  onOpenCourse,
  onGoCourses,
}: {
  onOpenCourse: (slug: string) => void;
  onGoCourses: () => void;
}) {
  const [courses, setCourses] = useState<CourseSummary[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api
      .listCourses()
      .then((cs) => {
        if (!cancelled) setCourses(cs);
      })
      .catch(() => {
        if (!cancelled) {
          setFailed(true);
          setCourses([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const loading = courses === null;
  const homeCourses = useMemo(() => selectHomeCourses(courses ?? [], 6), [courses]);

  return (
    <section className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="text-mk-h2 text-mk-ink">最近课程</h2>
        {!loading && !failed && (courses ?? []).length > 0 && (
          <Button variant="link" onClick={onGoCourses}>
            查看全部 →
          </Button>
        )}
      </div>

      {loading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }, (_, i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
      ) : failed ? (
        <p className="text-mk-body text-mk-muted">最近课程暂时加载不出来，刷新一下再试试。</p>
      ) : (courses ?? []).length === 0 ? (
        <p className="text-mk-body text-mk-muted">课程正在准备中，很快上线。</p>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {homeCourses.map((c) => (
            <CourseCard key={c.slug} course={c} pct={coursePct(c)} onOpen={() => onOpenCourse(c.slug)} />
          ))}
        </div>
      )}
    </section>
  );
}

export interface HomePageProps {
  user: MeUser | null;
  /** Open an existing project (switches to 项目 and focuses it). */
  onOpenProject: (projectId: string) => void;
  /** Open a course by slug (course player, reachable from home). */
  onOpenCourse: (courseSlug: string) => void;
  /** Start a new project. */
  onCreateProject: () => void;
  /** Go to the 项目 tab. */
  onGoProjects: () => void;
  /** Go to the 课程 tab (home 最近课程's "查看全部"). */
  onGoCourses: () => void;
  /** Open a finished project's 评估报告 (project card's ⋯ menu → deep-links into
   * the 项目 tab's 评估报告 sub). */
  onViewReport: (projectId: string) => void;
}

export function HomePage({
  user,
  onOpenProject,
  onOpenCourse,
  onCreateProject,
  onGoProjects,
  onGoCourses,
  onViewReport,
}: HomePageProps) {
  return (
    <div className="h-full w-full overflow-y-auto bg-mk-paper">
      <div className="mx-auto flex max-w-[1200px] flex-col gap-10 px-10 py-10">
        <header className="flex items-center justify-between gap-6">
          <div className="flex flex-col gap-2">
            <h1 className="text-mk-display text-mk-ink">你好，{user?.display_name ?? "同学"} 👋</h1>
            <p className="text-mk-body text-mk-muted">把你手头的写作任务带进来，印记与你一起把它想清楚。</p>
          </div>
          <Illustration name="bookLover" tone="vermilion" className="hidden h-[120px] w-[120px] shrink-0 md:block" alt="" />
        </header>

        <ProjectsSection
          onOpenProject={onOpenProject}
          onCreateProject={onCreateProject}
          onGoProjects={onGoProjects}
          onViewReport={onViewReport}
        />
        <CoursesSection onOpenCourse={onOpenCourse} onGoCourses={onGoCourses} />
      </div>
    </div>
  );
}
