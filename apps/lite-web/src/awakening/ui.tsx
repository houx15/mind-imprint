import type { ReactNode } from "react";

/**
 * awakening/ui —— 这个房间里反复出现的那几块。
 *
 * 它们全部只用 `awakening.css` 里的类（`.awk-*`），**一个 mk token 都不用**。
 * 理由写在那个文件的头部：mk 是裸 CSS 变量，任何 Tailwind 透明度修饰都不会
 * 生成 CSS（memory: tailwind-mk-token-alpha-trap）。这个房间有自己的一套变量。
 */

export function Eyebrow({ children }: { children: ReactNode }) {
  return <div className="awk-eyebrow">{children}</div>;
}

export function Dim({ children }: { children: ReactNode }) {
  return <span className="awk-dim">{children}</span>;
}

/** 一屏的主体。房间里每一屏都是这个宽度，所以换屏时内容不会横向跳动。 */
export function Stage({
  children,
  wide = false,
}: {
  children: ReactNode;
  /** 卡牌那几屏要更宽 —— 工具一打开就铺开（memory: interaction-means-a-board）。 */
  wide?: boolean;
}) {
  return (
    <div
      className="mx-auto w-full px-5 py-10 sm:px-8"
      style={{ maxWidth: wide ? 1180 : 760 }}
    >
      {children}
    </div>
  );
}

export function Panel({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`awk-panel p-6 sm:p-8 ${className}`}>{children}</div>;
}

/** 印记说的话。左边那条竖线是它的标志，整个房间只有它有。 */
export function Bubble({ speaker, children }: { speaker?: string; children: ReactNode }) {
  return (
    <div className="awk-bubble">
      {speaker ? (
        <div className="awk-eyebrow mb-2">{speaker}</div>
      ) : null}
      <div className="text-[15px]">{children}</div>
    </div>
  );
}

/** 一个可选项。选中态由 `aria-pressed` 驱动，所以键盘和读屏都对。 */
export function Choice({
  index,
  title,
  body,
  selected,
  wrong,
  onClick,
  disabled,
}: {
  index?: string;
  title: string;
  body?: string;
  selected?: boolean;
  wrong?: boolean;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      className="awk-card"
      aria-pressed={selected ? "true" : "false"}
      data-wrong={wrong ? "true" : undefined}
      onClick={onClick}
      disabled={disabled}
    >
      <span className="flex items-start gap-3">
        {index ? <span className="awk-index mt-0.5">{index}</span> : null}
        <span className="min-w-0 flex-1">
          <span className="block font-semibold">{title}</span>
          {body ? <span className="awk-dim mt-1 block text-[13px] leading-relaxed">{body}</span> : null}
        </span>
      </span>
    </button>
  );
}

export function Primary({
  children,
  onClick,
  disabled,
  type = "button",
}: {
  children: ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  type?: "button" | "submit";
}) {
  return (
    <button type={type} className="awk-primary" onClick={onClick} disabled={disabled}>
      {children}
    </button>
  );
}

export function Ghost({
  children,
  onClick,
  disabled,
}: {
  children: ReactNode;
  onClick?: () => void;
  disabled?: boolean;
}) {
  return (
    <button type="button" className="awk-ghost" onClick={onClick} disabled={disabled}>
      {children}
    </button>
  );
}

/** 进度刻度。走到第几屏、终端第几问，都是它。 */
export function Meter({ total, done }: { total: number; done: number }) {
  return (
    <span className="awk-meter" aria-label={`第 ${done} 步，共 ${total} 步`}>
      {Array.from({ length: total }, (_, i) => (
        <span key={i} data-on={i < done ? "true" : "false"} />
      ))}
    </span>
  );
}

/** 逐条入场。`--i` 是它的序号，CSS 按它延迟。 */
export function Stagger({ children }: { children: ReactNode[] }) {
  return (
    <>
      {children.map((c, i) => (
        <div key={i} className="awk-in" style={{ ["--i" as string]: i }}>
          {c}
        </div>
      ))}
    </>
  );
}

/** 开场那个转动的核心。纯装饰，`prefers-reduced-motion` 下不动。 */
export function Orbit() {
  return (
    <div className="awk-orbit" aria-hidden="true">
      <div className="awk-ring" />
      <div className="awk-ring" />
      <div className="awk-ring" />
      <div className="awk-core" />
    </div>
  );
}

/** 右上角那颗常亮的点，说明连接还在。 */
export function Pill({ children }: { children: ReactNode }) {
  return (
    <span className="awk-pill">
      <span className="awk-dot" />
      {children}
    </span>
  );
}
