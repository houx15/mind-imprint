import { Library } from "lucide-react";
import { Icon } from "@/ui";
import { navigate, liteRoutePath, readingLibraryPath } from "../routing";

/**
 * ReadingsTabs —— 阅读页顶部的两档切换。
 *
 * 阅读这一格下面有两个入口，它们是并列的：她自己带来的文章（粘贴、贴链接、传
 * 文件），和我们排好的分级阅读库。上一版把后者放在落地页最下面一个
 * 「查看全部 20 篇」的小按钮里 —— 那是一整座书架挂在一个次要控件上，报回来的
 * 原话是「not obvious enough」。
 *
 * 所以它上升成一个页签：两页都在顶部渲染这一行，当前这一页高亮。它不是一个
 * 跳转按钮，是一个位置指示器 —— 她任何时候都看得见另一边还有什么。
 *
 * 两页共用它，也就取代了分级阅读那一页原来的「返回阅读」：一个页签行里的
 * 「粘贴文章」就是那条返回路径，再摆一个返回箭头是同一件事说两遍。
 */

export type ReadingsTab = "own" | "library";

export function ReadingsTabs({ active, libraryCount }: { active: ReadingsTab; libraryCount?: number }) {
  return (
    <div
      role="tablist"
      aria-label="阅读入口"
      className="inline-flex items-center gap-1 rounded-mk-full border border-mk-border bg-mk-surface p-1 shadow-mk-xs"
    >
      <Tab
        active={active === "own"}
        label="粘贴文章"
        onPick={() => navigate(liteRoutePath({ tab: "readings" }))}
      />
      <Tab
        active={active === "library"}
        label="分级阅读"
        count={libraryCount}
        icon
        onPick={() => navigate(readingLibraryPath())}
      />
    </div>
  );
}

function Tab({
  active,
  label,
  count,
  icon,
  onPick,
}: {
  active: boolean;
  label: string;
  count?: number;
  icon?: boolean;
  onPick: () => void;
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onPick}
      className={`flex items-center gap-1.5 rounded-mk-full px-3.5 py-1.5 text-mk-small transition-colors duration-[120ms] ease-mk ${
        active
          ? "font-medium text-white"
          : "text-mk-secondary hover:bg-mk-accent-50 hover:text-mk-accent-700"
      }`}
      // mk 令牌是裸 CSS 变量，Tailwind 的 alpha 语法在它上面不出任何 CSS，
      // 所以选中态的底色走 inline style。
      style={active ? { background: "var(--mk-accent-500)" } : undefined}
    >
      {icon && <Icon icon={Library} size={14} />}
      {label}
      {typeof count === "number" && count > 0 && (
        <span className={active ? "text-white/80" : "text-mk-muted"}>{count}</span>
      )}
    </button>
  );
}
