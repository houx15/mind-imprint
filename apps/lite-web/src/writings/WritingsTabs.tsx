import { Library } from "lucide-react";
import { Icon } from "@/ui";
import { navigate, liteRoutePath, writingLibraryPath } from "../routing";

/**
 * WritingsTabs —— 写作页顶部的两档切换。
 *
 * 和阅读那边（ReadingsTabs）同一个形状，理由也一样：写作这一格下面有两个
 * 并列的入口 —— 她自己想写的，和我们排好的题库。上一版题库根本不存在，
 * 落地页上只有四条写死的示范题；把题库塞进一个「查看更多」的小链接里，
 * 就会重蹈阅读那边「not obvious enough」的覆辙。
 *
 * 它是位置指示器，不是跳转按钮：两页都渲染这一行，当前页高亮，
 * 所以她任何时候都看得见另一边还有什么。
 */

export type WritingsTab = "own" | "library";

export function WritingsTabs({
  active,
  libraryCount,
}: {
  active: WritingsTab;
  libraryCount?: number;
}) {
  return (
    <div
      role="tablist"
      aria-label="写作入口"
      className="inline-flex items-center gap-1 rounded-mk-full border border-mk-border bg-mk-surface p-1 shadow-mk-xs"
    >
      <Tab
        active={active === "own"}
        label="自己写"
        onPick={() => navigate(liteRoutePath({ tab: "writings" }))}
      />
      <Tab
        active={active === "library"}
        label="写作题库"
        count={libraryCount}
        icon
        onPick={() => navigate(writingLibraryPath())}
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
      // mk 令牌是裸 CSS 变量，Tailwind 的 alpha 语法在它上面不出任何 CSS。
      style={active ? { background: "var(--mk-accent-500)" } : undefined}
    >
      {icon && <Icon icon={Library} size={14} />}
      {label}
      {typeof count === "number" && count > 0 && (
        <span className={active ? "text-white/80" : "text-mk-muted"}>
          {count}
        </span>
      )}
    </button>
  );
}
