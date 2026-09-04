import { useEffect } from "react";
import { X } from "lucide-react";
import { Icon } from "@/ui";
import { CoursesContainer } from "@/shell/courses/CoursesContainer";

/**
 * CourseModal —— 在项目里上课，上完回到项目。
 *
 * 产品负责人 2026-09-04：「they can open a modal or a new page to view the
 * course then back.」用浮层而不是跳转，是因为跳走会把项目房间卸载：她回来时
 * 对话滚回顶部、右栏回到计划、刚打开的那件工具关掉了。上课是这个项目里的一步，
 * 不是离开这个项目。
 *
 * 里面挂的是 pro 的 `CoursesContainer`（`initialOpen` 直接进播放器），和 课程
 * 页用的是同一个运行时 —— 项目里上的课和课程页上的课必须是同一门课、同一份
 * 进度，否则「她上完了没有」会有两个答案。
 *
 * 关掉的时候调 `onClose`，宿主据此重拉一次进度：她的进度在服务端
 * （course_session），浮层这边不猜。
 */
export function CourseModal({ slug, onClose }: { slug: string; onClose: () => void }) {
  // Esc 关掉。课程运行时本身没有「退出」以外的键盘约定，这一条不会撞上。
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-50 flex flex-col"
      style={{ background: "var(--mk-paper)" }}
      role="dialog"
      aria-modal="true"
      aria-label="课程"
    >
      <div className="flex shrink-0 items-center justify-between border-b border-mk-border px-4 py-2">
        {/* 🚨 按钮不叫「回到项目」：项目房间自己的返回键已经叫这个名字，两个
            同名按钮对读屏的人是两次一样的提示，对测试是一次 strict mode 冲突。 */}
        <span className="text-mk-small text-mk-secondary">上完这一课，回到项目里继续</span>
        <button
          type="button"
          onClick={onClose}
          className="flex items-center gap-1 rounded-mk-full px-2 py-1 text-mk-small text-mk-secondary hover:bg-mk-paper"
        >
          <Icon icon={X} size={15} />
          关闭课程
        </button>
      </div>
      <div className="flex min-h-0 flex-1 flex-col">
        <CoursesContainer
          initialOpen={{ slug, mode: "player" }}
          onGoPortal={onClose}
        />
      </div>
    </div>
  );
}
