import type { CSSProperties, ReactNode } from "react";
import { Sparkles, GripVertical } from "lucide-react";
import { Icon } from "@/ui";
import type { Tone } from "../../../shared/tone";

/**
 * Sticky —— 板上的一张纸。
 *
 * 每一块板上被拖来拖去的都是它：观察、线索、点子、障碍。长得一样是有意的——
 * 她学会「这种东西可以拿起来放到别处」一次，十件工具都通了。
 *
 * 🚨 颜色落在整块淡底 + 深字上，不挂左侧色带。产品负责人 2026-09-03：
 * 「I hate left color bar designs, especially when we have a huge list of that」。
 *
 * 🚨 `touchAction: "none"` 不能少。触屏上不关掉浏览器自己的滚动手势，
 * 拖一张纸会变成滚页面，板在手机上直接废掉。
 */
export function Sticky({
  tone,
  label,
  children,
  byYinji,
  dragging,
  selected,
  onPointerDown,
  right,
  style,
}: {
  tone: Tone;
  /** 这张纸是哪一类：观察 / 别人的原话 / 我的推论…… */
  label?: string;
  children: ReactNode;
  /** 印记写的那张。她自己写的和印记写的不是一回事，要看得出来。 */
  byYinji?: boolean;
  /** 正被拖着（原处留一个淡淡的坑）。 */
  dragging?: boolean;
  selected?: boolean;
  onPointerDown?: (e: React.PointerEvent) => void;
  /** 右上角的小动作（拿下来、改一改）。 */
  right?: ReactNode;
  style?: CSSProperties;
}) {
  return (
    <div
      onPointerDown={onPointerDown}
      className="group relative select-none rounded-mk-md px-3 py-2.5 shadow-mk-xs"
      style={{
        background: tone.bg,
        color: tone.fg,
        cursor: onPointerDown ? "grab" : undefined,
        touchAction: "none",
        opacity: dragging ? 0.35 : 1,
        outline: selected ? `2px solid ${tone.solid}` : undefined,
        outlineOffset: 1,
        ...style,
      }}
    >
      <div className="flex items-start justify-between gap-1.5">
        <span className="flex min-w-0 items-center gap-1 text-mk-small" style={{ color: tone.solid }}>
          {onPointerDown && (
            <Icon
              icon={GripVertical}
              size={12}
              // 抓手一直在，不靠 hover 才出现——触屏上没有 hover，而她第一次
              // 打开这块板时最需要知道的就是"这个能拿起来"。
              className="-ml-1 shrink-0 opacity-50"
            />
          )}
          {byYinji && <Icon icon={Sparkles} size={11} className="shrink-0" />}
          <span className="truncate">{byYinji ? `印记${label ? `· ${label}` : ""}` : label}</span>
        </span>
        {right}
      </div>
      {/* 🚨 whitespace-pre-wrap：她粘进来的原话常常带换行，直塞 div 会被折成
          一行——线上已经因为这个把印记分的三条要点挤成一行过一次。 */}
      <p className="mt-1 whitespace-pre-wrap break-words text-mk-body">{children}</p>
    </div>
  );
}
