import type { ReactNode } from "react";

/**
 * awakening/ui —— 这个房间里反复出现的那几块。
 *
 * 它们只用 `awakening.css` 里的 `.awk-*`，**一个 mk token 都不用**：mk 是裸
 * CSS 变量，任何 Tailwind 透明度修饰都不会生成 CSS
 * （memory: tailwind-mk-token-alpha-trap）。这个房间有自己的一套变量。
 *
 * 🚨 这一层的 API 是稳定的，内部换皮不换接口。2026-09-19 把整个房间改成参考
 * 设计那套舱内 HUD 时，就是只改了这里的实现，能量 / 选择印记 / 终端 / 天赋
 * 那几屏一行都没动就跟着换了样子。
 */

export function Eyebrow({ children }: { children: ReactNode }) {
  return <div className="awk-eyebrow">{children}</div>;
}

export function Dim({ children }: { children: ReactNode }) {
  return <span className="awk-dim">{children}</span>;
}

/**
 * 一屏的主体。
 *
 * 它同时做两件事：撑成一整屏（`.awk-screen` 是绝对定位，顶栏以下铺满），
 * 和把内容收进固定宽度。所以每一屏只要 `<Stage>` 包一层就位置正确。
 */
export function Stage({
  children,
  wide = false,
  label,
}: {
  children: ReactNode;
  /** 卡牌那几屏要更宽 —— 工具一打开就铺开（memory: interaction-means-a-board）。 */
  wide?: boolean;
  label?: string;
}) {
  return (
    <section className="awk-screen" aria-label={label}>
      <div className="awk-wrap" style={wide ? { width: "min(1180px, 94%)" } : undefined}>
        {children}
      </div>
    </section>
  );
}

export function Panel({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`awk-panel ${className}`}>{children}</div>;
}

/** 印记说的话。左上角是方的，其余是圆的 —— 一个从左上角发出来的气泡。 */
export function Bubble({ speaker, children }: { speaker?: string; children: ReactNode }) {
  return (
    <div className="awk-bubble">
      {speaker ? <strong>{speaker}：</strong> : null}
      {children}
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
  hwId,
}: {
  index?: string;
  title: string;
  body?: string;
  selected?: boolean;
  wrong?: boolean;
  onClick: () => void;
  disabled?: boolean;
  /** 右上角那个编号，形如 SYS-01。不给就不显示。 */
  hwId?: string;
}) {
  const tone = wrong ? " awk-option--bad" : selected ? " awk-option--selected" : "";
  return (
    <button
      type="button"
      className={`awk-option${tone}`}
      aria-pressed={selected ? "true" : "false"}
      onClick={onClick}
      disabled={disabled}
    >
      {index ? <span className="awk-option-index">{index}</span> : <span />}
      <span className="awk-option-copy">
        <strong>{title}</strong>
        {body ? <span>{body}</span> : null}
      </span>
      <span className="awk-option-arrow">›</span>
      {/* 编号和条码：设计稿里每张可点的卡上都有，是「一台机器上的一格」的记号。 */}
      {hwId ? <span className="awk-hw-id">{hwId}</span> : null}
      <span className="awk-hw-barcode" aria-hidden="true" />
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
    <button
      type={type}
      className="awk-btn awk-btn--primary"
      onClick={onClick}
      disabled={disabled}
    >
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
    <button type="button" className="awk-btn awk-btn--ghost" onClick={onClick} disabled={disabled}>
      {children}
    </button>
  );
}

/** 进度刻度。走到第几屏、终端第几问，都是它。 */
export function Meter({ total, done }: { total: number; done: number }) {
  return (
    <span className="awk-meter" aria-label={`第 ${done} 步，共 ${total} 步`}>
      {Array.from({ length: total }, (_, i) => (
        <span key={i} className={i < done ? "on" : undefined} />
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

/** 等待时那个转着的核心。纯装饰，`prefers-reduced-motion` 下不动。 */
export function Orbit() {
  return (
    <div className="awk-orbit" aria-hidden="true">
      <div className="awk-ring" />
      <div className="awk-ring" />
      <div className="awk-core" />
    </div>
  );
}
