import type { ReactNode } from "react";
import { useEffect, useState } from "react";

/**
 * tree/ui — 这一页要用的三个小原语。
 *
 * 和 `eco/ui.tsx` 里的同名三个是重复的，这是有意的、也是划算的：它们只差在
 * 类名（`tree-mono` / `tree-in` 对 `eco-mono` / `eco-in`），而让一个学生真的
 * 会打开的页面去 import 一个文件头上写着「原型做完就删掉」的模块，是把生产
 * 代码挂在一个计划中的删除上。六十行展示代码换掉这个耦合，值。
 *
 * 同样不建在 `@/ui`（pro 的设计系统 barrel）上：这一页需要**暗色**变体，
 * pro 的组件没有、也不该为这一页长一个；而零耦合意味着这一页不可能弄坏 pro
 * （见 memory: lite-must-not-break-pro）。
 *
 * `tone="dark"` = 在夜色底上；`tone="light"` = 在纸上。
 */

export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export type Tone = "light" | "dark";

/** 等宽的系统标签——这一页主要的科技感手段。用在机器会打印的东西上：日期、
 *  序号、计数、主枝代号。 */
export function Sys({
  children,
  tone = "light",
  className,
}: {
  children: ReactNode;
  tone?: Tone;
  className?: string;
}) {
  return (
    <span
      className={cx("tree-mono", tone === "dark" ? "text-[#B6A99A]" : "text-mk-muted", className)}
    >
      {children}
    </span>
  );
}

/**
 * 一个解释数字的 `?`。
 *
 * 这个产品里每一个读数都在断言关于这个学生的某件事——「关键词 16」是一句关于
 * 她是谁的话。一个旁边没有定义的数字，要么被当作信条接受，要么被忽略，两个都
 * 不是我们想要的。所以任何**推导出来的**（而不是能在屏幕上数出来的）数字，都
 * 带一个这个，说清楚它是由什么算的。
 *
 * 悬停和聚焦都能打开，正文是真的句子，不是标签。
 */
export function Hint({ text, tone = "light" }: { text: string; tone?: Tone }) {
  const [open, setOpen] = useState(false);
  const dark = tone === "dark";
  return (
    <span className="relative inline-flex">
      <button
        type="button"
        aria-label="这个数字是什么"
        onMouseEnter={() => setOpen(true)}
        onMouseLeave={() => setOpen(false)}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onClick={() => setOpen((v) => !v)}
        className="flex h-[15px] w-[15px] items-center justify-center rounded-mk-full text-[10px]
                   font-bold leading-none transition-colors duration-[120ms]
                   focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
        style={{
          border: dark ? "1px solid rgba(240,233,224,.34)" : "1px solid var(--mk-input-border)",
          color: dark ? "#B6A99A" : "var(--mk-muted)",
        }}
      >
        ?
      </button>
      {open ? (
        <span
          role="tooltip"
          className="tree-in absolute right-0 top-[22px] z-50 block w-[268px] rounded-mk-md p-3 text-left
                     text-mk-small font-normal normal-case leading-[1.75]"
          style={{
            letterSpacing: 0,
            background: dark ? "rgba(28,23,19,.97)" : "var(--mk-surface)",
            border: dark ? "1px solid rgba(240,233,224,.18)" : "1px solid var(--mk-border)",
            color: dark ? "#DCD2C6" : "var(--mk-secondary)",
            boxShadow: "0 20px 46px rgba(0,0,0,.34)",
          }}
        >
          {text}
        </span>
      ) : null}
    </span>
  );
}

/**
 * 从右侧滑入的抽屉。Esc 关闭，遮罩点击关闭。
 *
 * 和 `Hint` 一样，是 `eco/ui.tsx` 同名件的一份独立拷贝，理由见文件头。
 */
export function Drawer({
  open,
  onClose,
  children,
  width = 480,
  label,
}: {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  width?: number;
  label: string;
}) {
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex justify-end" role="dialog" aria-label={label}>
      <button
        type="button"
        aria-label="关闭"
        onClick={onClose}
        className="absolute inset-0 cursor-default"
        style={{ background: "rgba(10,8,6,.58)" }}
      />
      <aside
        className="tree-sheet-in relative flex h-full flex-col overflow-hidden"
        style={{
          width: `min(${width}px, 100vw)`,
          background: "#1C1713",
          borderLeft: "1px solid rgba(240,233,224,.14)",
          color: "#F0E9E0",
        }}
      >
        {children}
      </aside>
    </div>
  );
}
