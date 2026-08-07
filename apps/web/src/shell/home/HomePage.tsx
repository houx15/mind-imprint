import { useEffect, useState } from "react";
import type { CourseSummary } from "@mind-imprint/contracts";
import { api, type MeUser, type ProjectListItem } from "@/api";
import {
  Icon,
  Plus,
  Card,
  Badge,
  type BadgeTone,
  Button,
  SkeletonCard,
  EmptyState,
  Illustration,
  coverGradientStyle,
} from "@/ui";

/**
 * HomePage — 首页 (shell Task 5, replaces the Task 4 stub).
 *
 * Three sections inside a centered ~1200px container on the warm paper bg:
 * a greeting header, a horizontal "最近项目" snapshot (first tile always
 * "新建"), and a "推荐课程" grid. Each data section fetches independently so
 * a failure in one (soft inline message) never blanks the other — see
 * `ProjectsSection`/`CoursesSection` below.
 */

function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

const RECENT_LIMIT = 3;
const TILE_W = "w-[260px]";
const TILE_H = "h-[264px]";

// Mirrors workspace/Directory.tsx's STATUS_META labels (the one other place
// project lifecycle status is shown to a student) so the wording never drifts
// between the projects list and this home snapshot.
const STATUS_LABEL: Record<ProjectListItem["status"], string> = {
  forming: "立题中",
  working: "进行中",
  evaluating: "评估中",
  done: "已完成",
};

const STATUS_TONE: Record<ProjectListItem["status"], BadgeTone> = {
  forming: "draft",
  working: "progress",
  evaluating: "pending",
  done: "done",
};

function NewProjectTile({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cx(
        TILE_W,
        TILE_H,
        "flex shrink-0 flex-col items-center justify-center gap-2 rounded-mk-md border border-dashed border-mk-border text-mk-muted",
        "transition-colors duration-[120ms] ease-mk hover:border-mk-accent hover:text-mk-accent-600",
        "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-mk-accent/15",
      )}
    >
      <span className="flex h-9 w-9 items-center justify-center rounded-mk-full bg-mk-accent-50 text-mk-accent-600">
        <Icon icon={Plus} size={18} />
      </span>
      <span className="text-mk-body font-medium">新建项目</span>
    </button>
  );
}

function ProjectTile({ project, onClick }: { project: ProjectListItem; onClick: () => void }) {
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick();
        }
      }}
      className={cx(TILE_W, TILE_H, "shrink-0 cursor-pointer")}
    >
      <Card className="flex h-full w-full flex-col gap-2 p-4">
        <div className="h-[120px] w-full shrink-0 rounded-mk-sm" style={coverGradientStyle(project.title || project.id)} />
        <div className="line-clamp-2 flex-1 text-mk-h3 text-mk-ink">{project.title || "未命名项目"}</div>
        <Badge tone={STATUS_TONE[project.status]} className="self-start">
          {STATUS_LABEL[project.status]}
        </Badge>
        <div className="truncate text-mk-small text-mk-muted">{project.qualLabel}</div>
      </Card>
    </div>
  );
}

function CourseCard({ course, onClick }: { course: CourseSummary; onClick: () => void }) {
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick();
        }
      }}
      className="cursor-pointer"
    >
      <Card className="flex h-full flex-col gap-3 p-4">
        <div className="h-[100px] w-full shrink-0 rounded-mk-sm" style={coverGradientStyle(course.slug)} />
        <div className="text-mk-h3 text-mk-ink">{course.title}</div>
        <p className="line-clamp-2 flex-1 text-mk-body text-mk-muted">{course.blurb}</p>
        <div className="flex items-center gap-2">
          <Badge tone="draft">{course.branch}</Badge>
          <span className="truncate text-mk-small text-mk-muted">
            {course.time_label} · {course.step_count} 步
          </span>
        </div>
      </Card>
    </div>
  );
}

function ProjectsSection({
  onOpenProject,
  onCreateProject,
  onGoProjects,
}: {
  onOpenProject: (id: string) => void;
  onCreateProject: () => void;
  onGoProjects: () => void;
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

  const loading = projects === null;
  const recent = (projects ?? []).slice(0, RECENT_LIMIT);

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
        <div className="flex gap-4 overflow-x-auto pb-2">
          <NewProjectTile onClick={onCreateProject} />
          {recent.map((p) => (
            <ProjectTile key={p.id} project={p} onClick={() => onOpenProject(p.id)} />
          ))}
        </div>
      )}
    </section>
  );
}

function CoursesSection({ onOpenCourse }: { onOpenCourse: (slug: string) => void }) {
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

  return (
    <section className="flex flex-col gap-4">
      <h2 className="text-mk-h2 text-mk-ink">好的写作，建立在阅读之上哦 · 推荐课程</h2>

      {loading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }, (_, i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
      ) : failed ? (
        <p className="text-mk-body text-mk-muted">推荐课程暂时加载不出来，刷新一下再试试。</p>
      ) : (courses ?? []).length === 0 ? (
        <p className="text-mk-body text-mk-muted">课程正在准备中，很快上线。</p>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {(courses ?? []).map((c) => (
            <CourseCard key={c.slug} course={c} onClick={() => onOpenCourse(c.slug)} />
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
  /** Go to the 图鉴 tab. */
  onGoGallery: () => void;
}

export function HomePage({ user, onOpenProject, onOpenCourse, onCreateProject, onGoProjects }: HomePageProps) {
  return (
    <div className="h-full w-full overflow-y-auto bg-mk-paper">
      <div className="mx-auto flex max-w-[1200px] flex-col gap-10 px-10 py-10">
        <header className="flex items-center justify-between gap-6">
          <div className="flex flex-col gap-2">
            <h1 className="text-mk-display text-mk-ink">你好，{user?.display_name ?? "同学"} 👋</h1>
            <p className="text-mk-body text-mk-muted">把你手头的写作任务带来，印记陪你一点一点想清楚。</p>
          </div>
          <Illustration name="bookLover" tone="vermilion" className="hidden h-[120px] w-[120px] shrink-0 md:block" alt="" />
        </header>

        <ProjectsSection onOpenProject={onOpenProject} onCreateProject={onCreateProject} onGoProjects={onGoProjects} />
        <CoursesSection onOpenCourse={onOpenCourse} />
      </div>
    </div>
  );
}
