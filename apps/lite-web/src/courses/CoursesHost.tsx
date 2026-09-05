import { useCallback, useEffect, useRef, useState } from "react";
import { Segmented } from "@/ui";
import { CoursesContainer, type CourseOpenTarget } from "@/shell/courses/CoursesContainer";
import { LearningHistory } from "@/shell/courses/LearningHistory";

/**
 * CoursesHost —— 轻量版的 课程 页。
 *
 * 它挂的是 pro 的 `CoursesContainer`（目录 → 详情 → 播放器 → 报告）和
 * `LearningHistory`，不重写一份：课程运行时（`@mind-imprint/course-renderer`）
 * 和这些界面本来就与版本无关，一门课在两个版本里就是同一门课。轻量版和完整版
 * 的差别只有一条 —— **看得见哪些课**，而那一条在服务端按学校的 edition 过滤
 * （course.audience，migration 0133），不在这里。
 *
 * 没有 pro 的 图鉴 那一档：图鉴是完整版的卡片收集，轻量版没有那套东西。
 *
 * 播放器打开时整页沉浸（`onImmersiveChange`），左边那条导航轨由 `LiteApp`
 * 收起来。
 */

type Sub = "courses" | "history";

const SUBS: { value: Sub; label: string }[] = [
  { value: "courses", label: "课程" },
  { value: "history", label: "学习记录" },
];

export function CoursesHost({
  slug,
  onOpenCourse,
  onBackToList,
  onImmersiveChange,
}: {
  /** 当前 URL 里的课程 slug，`/courses` 时是 undefined。 */
  slug?: string;
  /** 打开了某一门课：把 slug 写进 URL。 */
  onOpenCourse: (slug: string) => void;
  /** 回到目录：URL 回到 `/courses`。 */
  onBackToList: () => void;
  /** 播放器/报告开着的时候为 true，宿主据此收起导航轨。 */
  onImmersiveChange: (immersive: boolean) => void;
}) {
  const [sub, setSub] = useState<Sub>("courses");
  const [inCourse, setInCourse] = useState(false);

  // URL 里的 slug → 容器。容器自己维护 grid/detail/player/report 的视图状态，
  // 所以这里只在 slug 真的变了的时候递一次 target，之后由容器接管。
  const [openTarget, setOpenTarget] = useState<CourseOpenTarget | null>(
    slug ? { slug, mode: "detail" } : null,
  );
  const lastSlug = useRef<string | undefined>(slug);

  // 一个 bump 过的计数：URL 从 `/courses/:slug` 回到 `/courses`（浏览器后退），
  // 容器据此回到目录。和 pro 的 shell 用的是同一个约定。
  const [closeSignal, setCloseSignal] = useState<number | null>(null);

  useEffect(() => {
    if (slug === lastSlug.current) return;
    lastSlug.current = slug;
    if (slug) {
      setSub("courses");
      setOpenTarget({ slug, mode: "detail" });
    } else {
      setCloseSignal((n) => (n ?? 0) + 1);
    }
  }, [slug]);

  const handleImmersive = useCallback(
    (immersive: boolean) => {
      setInCourse(immersive);
      onImmersiveChange(immersive);
    },
    [onImmersiveChange],
  );

  // 离开这一页时把沉浸态收掉，否则导航轨会一直藏着。
  useEffect(() => () => onImmersiveChange(false), [onImmersiveChange]);

  const handleActiveCourse = useCallback(
    (active: string | null) => {
      if (active) onOpenCourse(active);
      else onBackToList();
    },
    [onOpenCourse, onBackToList],
  );

  return (
    <div className="flex h-full min-h-0 flex-col bg-mk-paper">
      {!inCourse && (
        <div className="shrink-0 px-6 pt-5">
          <Segmented
            options={SUBS}
            value={sub}
            onChange={(v) => setSub(v as Sub)}
            variant="island"
          />
        </div>
      )}
      <div className="flex min-h-0 flex-1 flex-col">
        {sub === "history" ? (
          <LearningHistory
            onOpen={(target) => {
              setSub("courses");
              setOpenTarget(target);
              onOpenCourse(target.slug);
            }}
          />
        ) : (
          <CoursesContainer
            initialOpen={openTarget}
            onCourseConsumed={() => setOpenTarget(null)}
            onImmersiveChange={handleImmersive}
            onActiveCourseChange={handleActiveCourse}
            closeSignal={closeSignal}
            onCloseSignalConsumed={() => setCloseSignal(null)}
          />
        )}
      </div>
    </div>
  );
}
