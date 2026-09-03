/**
 * sketch.tsx —— 手绘笔触。
 *
 * 产品负责人 2026-09-03 的四张图，除了「能拖」之外还有一件共同的事：标题下面
 * 有一道马克笔划线，区与区之间有铅笔画的圈和箭头。它不是装饰——一块**画出来的**
 * 板和一张 CSS 画的表格给人的许可是不一样的：表格要求填对，板允许挪来挪去。
 * 铁律②要的「趣味」就落在这一层。
 *
 * 🚨 全部走令牌（`currentColor` 由父级给），不写十六进制。暗色模式下整组令牌
 * 会被换掉，写死的颜色不会——见 shared/tone.ts。
 *
 * 🚨 不要用 Tailwind 的透明度语法（`text-mk-peach/40`）。mk-* 是裸 CSS 变量，
 * 所有 alpha 语法都不会生成任何 CSS。要淡就用 opacity 属性。
 */

/** 标题底下那道马克笔。故意画得不平——直线看着像下划线，歪的才像笔。 */
export function MarkerUnderline({ width = 180, color = "var(--mk-accent-500)" }: {
  width?: number;
  color?: string;
}) {
  return (
    <svg
      width={width}
      height={10}
      viewBox="0 0 180 10"
      fill="none"
      aria-hidden
      preserveAspectRatio="none"
      style={{ display: "block", marginTop: 2, opacity: 0.75 }}
    >
      <path
        d="M3 6.5C34 3.2 74 2.4 118 3.6C142 4.2 160 5.4 177 7"
        stroke={color}
        strokeWidth={3.2}
        strokeLinecap="round"
      />
    </svg>
  );
}

/** 从一处指到另一处的手绘箭头。用在「整合起来 ↘」这种把两块连起来的地方。 */
export function SketchArrow({ color = "var(--mk-accent-500)" }: { color?: string }) {
  return (
    <svg width={46} height={30} viewBox="0 0 46 30" fill="none" aria-hidden style={{ opacity: 0.7 }}>
      <path
        d="M3 4C10 18 21 25 40 25"
        stroke={color}
        strokeWidth={2}
        strokeLinecap="round"
        fill="none"
      />
      <path d="M33 19L41 25L32 28" stroke={color} strokeWidth={2} strokeLinecap="round" fill="none" />
    </svg>
  );
}

/**
 * 圈起来的一堆，外加一条贴着名字的胶带。
 *
 * 产品负责人 2026-09-03 图上那两个圈（「减少拿多」「让剩下的有去处」）。为什么
 * 值得画：归堆这件事本来只有一个堆名，藏在每张纸的角上；圈出来之后「我把这四张
 * 看成一回事」变成屏幕上看得见的一块，而这句判断正是头脑风暴真正的产出。
 *
 * 画的是成员的外接矩形，不是凸包。手绘感靠圆角和笔触给，凸包多算的那点精度
 * 在一块可以随手挪纸的板上没有意义——她一挪，圈就跟着变了。
 */
export function GroupLasso({
  box,
  label,
  color,
}: {
  box: { x: number; y: number; w: number; h: number };
  label: string;
  color: string;
}) {
  return (
    <div
      aria-hidden
      style={{
        position: "absolute",
        left: box.x,
        top: box.y,
        width: box.w,
        height: box.h,
        pointerEvents: "none",
      }}
    >
      <svg
        width="100%"
        height="100%"
        viewBox="0 0 100 100"
        preserveAspectRatio="none"
        style={{ position: "absolute", inset: 0 }}
      >
        <rect
          x="1"
          y="1"
          width="98"
          height="98"
          rx="14"
          fill="none"
          stroke={color}
          strokeWidth="1.6"
          strokeLinecap="round"
          vectorEffect="non-scaling-stroke"
          opacity={0.75}
        />
      </svg>
      {/* 胶带：压在圈的上边缘，稍微歪一点。 */}
      <span
        style={{
          position: "absolute",
          left: 14,
          top: -11,
          transform: "rotate(-1.5deg)",
          background: `color-mix(in srgb, ${color} 22%, var(--mk-surface))`,
          color: `color-mix(in srgb, ${color} 70%, var(--mk-ink))`,
          padding: "1px 10px",
          borderRadius: 2,
          fontSize: 12,
          whiteSpace: "nowrap",
        }}
      >
        {label}
      </span>
    </div>
  );
}

/**
 * 空区里那圈虚线。
 *
 * 用 SVG 而不是 `border-dashed`：CSS 的虚线四角是死的直角，一眼是个框；
 * 圆角 + 长短不一的笔画看起来像用铅笔圈出来的一块地方，而「一块地方」正是
 * 我们希望她理解的东西——可以往里放，也可以再拿出来。
 */
export function SketchOutline({ color = "var(--mk-border)" }: { color?: string }) {
  return (
    <svg
      aria-hidden
      style={{ position: "absolute", inset: 0, width: "100%", height: "100%", pointerEvents: "none" }}
      preserveAspectRatio="none"
      viewBox="0 0 100 100"
    >
      <rect
        x="0.6"
        y="0.6"
        width="98.8"
        height="98.8"
        rx="3"
        fill="none"
        stroke={color}
        strokeWidth="0.7"
        strokeDasharray="3 2.2"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
