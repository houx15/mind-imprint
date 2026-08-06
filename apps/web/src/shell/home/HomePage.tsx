import type { MeUser } from "@/api";

/**
 * HomePage — 首页, the new default landing surface (shell Task 4).
 *
 * MINIMAL STUB: Task 5 replaces this body entirely (recent projects, course
 * picks, a "new project" entry, etc). The prop interface is exported here so
 * Task 5 can build against a fixed contract without re-touching StudentApp's
 * wiring.
 */
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

export function HomePage({ user }: HomePageProps) {
  return (
    <div className="flex h-full w-full flex-col gap-2 overflow-y-auto bg-mk-paper p-10">
      <h1 className="text-mk-h1 text-mk-ink">你好，{user?.display_name ?? "同学"} 👋</h1>
      <p className="text-mk-body text-mk-muted">
        首页正在搭建中——你的项目、课程与工具卡很快会汇聚在这里。
      </p>
    </div>
  );
}
