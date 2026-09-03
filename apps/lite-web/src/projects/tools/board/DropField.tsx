import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { Plus } from "lucide-react";
import { Icon } from "@/ui";
import type { Tone } from "../../../shared/tone";
import { SketchOutline } from "./sketch";

/**
 * DropField —— 一块可以往里放东西的地方。
 *
 * 四象限里的「谁 / 需要什么 / 为什么 / 证据」、线索碰撞的三列、想法板上圈出来
 * 的一堆，都是它。
 *
 * 🚨 空的时候必须自己说清楚"往这儿放"。四张图里每一个空区都写着「拖拽便签到
 * 这里」——一块什么都没写的空框在她眼里是一个还没加载完的组件，不是一个可以
 * 放东西的地方。
 *
 * 🚨 悬停时整块变色，不是加一道边。要让她在松手之前就确定东西会落在哪儿；
 * 一条 2px 的边在拖着一张纸的时候看不见。
 */
export function DropField({
  zoneRef,
  testId,
  icon,
  title,
  hint,
  tone,
  active,
  count,
  isEmpty,
  empty = "拖拽便签到这里",
  onAdd,
  children,
  minHeight = 148,
}: {
  /** 来自 useZoneDrag：`zoneRef={drag.zoneRef("who")}`。 */
  zoneRef: (el: HTMLElement | null) => void;
  /** 给 e2e 抓这一格用。拖放没法靠文本定位——落点是一块**地方**，不是一行字。 */
  testId?: string;
  icon?: LucideIcon;
  title: string;
  /** 这一格要的是什么。「受影响或涉及的人」。 */
  hint?: string;
  tone: Tone;
  /** 指针正悬在这块上。 */
  active?: boolean;
  count?: number;
  /** 这一格是空的。🚨 显式给，不去嗅探 children——「还在写的那一行」也是
   *  children，嗅探会让她一边打字一边看见「拖拽便签到这里」压在下面。 */
  isEmpty?: boolean;
  empty?: string;
  /** 直接在这一格里写一张新的。拖是主路，写是退路——她手上不一定正好有那张纸。 */
  onAdd?: () => void;
  children: ReactNode;
  minHeight?: number;
}) {
  return (
    <section
      ref={zoneRef}
      data-testid={testId}
      className="relative rounded-mk-lg p-3 transition-colors"
      style={{
        minHeight,
        background: active ? tone.bg : "var(--mk-surface)",
        // 🚨 不用 border-dashed：CSS 虚线的直角一眼是个输入框。手绘的圈才像
        // "一块地方"——可以往里放，也可以再拿出来。
        boxShadow: active ? `inset 0 0 0 2px ${tone.solid}` : undefined,
      }}
    >
      {!active && <SketchOutline />}

      <header className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <h3 className="flex items-center gap-1.5 text-mk-body font-semibold" style={{ color: tone.fg }}>
            {icon && <Icon icon={icon} size={15} style={{ color: tone.solid }} />}
            {title}
            {typeof count === "number" && count > 0 && (
              <span
                className="rounded-mk-full px-1.5 text-mk-small tabular-nums"
                style={{ background: tone.bg, color: tone.solid }}
              >
                {count}
              </span>
            )}
          </h3>
          {hint && <p className="mt-0.5 text-mk-small text-mk-faint">{hint}</p>}
        </div>
        {onAdd && (
          <button
            type="button"
            onClick={onAdd}
            aria-label={`往「${title}」里写一条`}
            title={`往「${title}」里写一条`}
            className="shrink-0 rounded-mk-full p-1"
            style={{ background: tone.bg, color: tone.solid }}
          >
            <Icon icon={Plus} size={14} />
          </button>
        )}
      </header>

      <div className="relative mt-2.5 flex flex-col gap-2">
        {isEmpty ? (
          <p className="py-5 text-center text-mk-small text-mk-faint">{empty}</p>
        ) : (
          children
        )}
      </div>
    </section>
  );
}
