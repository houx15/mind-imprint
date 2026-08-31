import type { ReactNode } from "react";
import { useEffect, useState } from "react";

/**
 * eco/ui — the prototype's own small primitive set.
 *
 * Deliberately NOT built on `@/ui` (pro's design-system barrel). Two reasons:
 *  1. Half of these primitives need a DARK variant (the world view's night
 *     ground), which pro's components do not have and should not grow one for
 *     a prototype.
 *  2. Zero coupling to `apps/web` means this prototype cannot break pro — the
 *     standing rule [Lite must never break pro].
 * They still speak in `mk-*` tokens, so the two read as one product.
 *
 * `tone="dark"` = on the night ground; `tone="light"` = on paper.
 */

export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export type Tone = "light" | "dark";

/** A monospace system label — the prototype's main 科技感 device. Used for
 *  things a machine would print: dates, indices, counts, field codes. */
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
      className={cx(
        "eco-mono",
        tone === "dark" ? "text-[#B6A99A]" : "text-mk-muted",
        className,
      )}
    >
      {children}
    </span>
  );
}

export function Chip({
  children,
  active,
  tone = "light",
  onClick,
  hue,
  title,
}: {
  children: ReactNode;
  active?: boolean;
  tone?: Tone;
  onClick?: () => void;
  hue?: string;
  title?: string;
}) {
  const dark = tone === "dark";
  return (
    <button
      type="button"
      title={title}
      onClick={onClick}
      className={cx(
        "inline-flex shrink-0 items-center gap-1.5 rounded-mk-full px-3 py-1.5 text-mk-small",
        "transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2",
        dark ? "focus-visible:ring-[#8A7F72]" : "focus-visible:ring-mk-accent-200",
        !onClick && "cursor-default",
      )}
      style={
        active
          ? {
              background: hue ?? (dark ? "rgba(240,233,224,.92)" : "var(--mk-ink)"),
              color: dark ? "#17130F" : "#FFFFFF",
              border: "1px solid transparent",
            }
          : {
              background: dark ? "rgba(240,233,224,.06)" : "var(--mk-surface)",
              color: dark ? "#CFC3B4" : "var(--mk-secondary)",
              border: dark
                ? "1px solid rgba(240,233,224,.16)"
                : "1px solid var(--mk-border)",
            }
      }
    >
      {hue && !active ? (
        <span className="h-2 w-2 rounded-mk-full" style={{ background: hue }} />
      ) : null}
      {children}
    </button>
  );
}

export function Btn({
  children,
  onClick,
  variant = "primary",
  tone = "light",
  size = "md",
  disabled,
  className,
  iconStart,
  type = "button",
}: {
  children: ReactNode;
  onClick?: () => void;
  variant?: "primary" | "quiet" | "outline";
  tone?: Tone;
  size?: "md" | "sm";
  disabled?: boolean;
  className?: string;
  iconStart?: ReactNode;
  /** `submit` so a Btn can close a form — the project composer submits on
   *  Enter, and a form whose button is `type="button"` silently does nothing
   *  when you press it. */
  type?: "button" | "submit";
}) {
  const dark = tone === "dark";
  // A disabled PRIMARY at 45% opacity still reads as a solid coral button, so
  // students click it and conclude the app froze. Disabled state overrides the
  // fill outright (same treatment as pro's `Button`).
  const base =
    "inline-flex items-center justify-center gap-2 rounded-mk-sm font-medium transition-all " +
    "duration-[140ms] ease-mk focus-visible:outline-none focus-visible:ring-2 " +
    "disabled:cursor-not-allowed";
  const off = dark
    ? "disabled:!bg-[rgba(240,233,224,.1)] disabled:!text-[#7C7166] disabled:!border-transparent disabled:!shadow-none"
    : "disabled:!bg-[#F0E9E1] disabled:!text-[#B8ADA2] disabled:!border-transparent disabled:!shadow-none";
  const sizing = size === "sm" ? "px-3 py-1.5 text-mk-small" : "px-[18px] py-[10px] text-mk-body";
  const styles: Record<string, string> = {
    primary: dark
      ? "bg-[#F0E9E0] text-[#17130F] hover:bg-white focus-visible:ring-[#8A7F72]"
      : "bg-mk-accent text-white hover:bg-mk-accent-600 focus-visible:ring-mk-accent-200",
    outline: dark
      ? "border border-[rgba(240,233,224,.28)] text-[#E8E0D6] hover:bg-[rgba(240,233,224,.08)] focus-visible:ring-[#8A7F72]"
      : "border border-mk-accent text-mk-accent-700 hover:bg-mk-accent-50 focus-visible:ring-mk-accent-200",
    quiet: dark
      ? "text-[#C6B9AA] hover:bg-[rgba(240,233,224,.07)] focus-visible:ring-[#8A7F72]"
      : "text-mk-secondary hover:bg-[color-mix(in_srgb,var(--mk-ink)_5%,transparent)] focus-visible:ring-mk-accent-200",
  };
  return (
    <button
      // eslint-disable-next-line react/button-has-type
      type={type}
      disabled={disabled}
      onClick={onClick}
      className={cx(base, sizing, styles[variant], off, className)}
    >
      {iconStart}
      {children}
    </button>
  );
}

export function Panel({
  children,
  className,
  tone = "light",
  style,
}: {
  children: ReactNode;
  className?: string;
  tone?: Tone;
  style?: React.CSSProperties;
}) {
  const dark = tone === "dark";
  return (
    <div
      className={cx("rounded-mk-lg", className)}
      style={{
        background: dark ? "rgba(28,23,19,.72)" : "var(--mk-surface)",
        border: dark ? "1px solid rgba(240,233,224,.13)" : "1px solid var(--mk-border)",
        backdropFilter: dark ? "blur(14px)" : undefined,
        boxShadow: dark ? "0 24px 60px rgba(0,0,0,.42)" : "var(--mk-shadow-sm)",
        ...style,
      }}
    >
      {children}
    </div>
  );
}

/** A right-hand sheet. Used by the news detail, the keyword drawer and 印记.
 *  Escape closes; the overlay closes; focus is not trapped (prototype). */
export function Drawer({
  open,
  onClose,
  children,
  tone = "light",
  width = 480,
  label,
}: {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  tone?: Tone;
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
  const dark = tone === "dark";
  return (
    <div className="fixed inset-0 z-50 flex justify-end" role="dialog" aria-label={label}>
      <button
        type="button"
        aria-label="关闭"
        onClick={onClose}
        className="absolute inset-0 cursor-default"
        style={{ background: dark ? "rgba(10,8,6,.58)" : "rgba(51,48,46,.28)" }}
      />
      <aside
        className="eco-sheet-in relative flex h-full flex-col overflow-hidden"
        style={{
          width: `min(${width}px, 100vw)`,
          background: dark ? "#1C1713" : "var(--mk-surface)",
          borderLeft: dark ? "1px solid rgba(240,233,224,.14)" : "1px solid var(--mk-border)",
          color: dark ? "#F0E9E0" : "var(--mk-ink)",
        }}
      >
        {children}
      </aside>
    </div>
  );
}

/** Section heading with an index — the numbering is part of the tech register
 *  and makes long screens navigable by eye. */
export function SectionHead({
  index,
  title,
  sub,
  tone = "light",
  right,
}: {
  index?: string;
  title: string;
  sub?: string;
  tone?: Tone;
  right?: ReactNode;
}) {
  const dark = tone === "dark";
  return (
    <div className="mb-4 flex items-end justify-between gap-4">
      <div className="min-w-0">
        {index ? <Sys tone={tone}>{index}</Sys> : null}
        <h2
          className={cx("text-mk-h1 mt-1", dark ? "text-[#F5EFE7]" : "text-mk-ink")}
        >
          {title}
        </h2>
        {sub ? (
          <p className={cx("mt-1.5 text-mk-body", dark ? "text-[#B6A99A]" : "text-mk-secondary")}>
            {sub}
          </p>
        ) : null}
      </div>
      {right}
    </div>
  );
}

/** A labelled textarea that keeps the hint visible while she types — the hint
 *  IS the teaching, so it must not vanish on focus the way a placeholder does. */
export function Field({
  label,
  hint,
  value,
  onChange,
  rows = 4,
  placeholder,
  right,
}: {
  label: string;
  hint?: string;
  value: string;
  onChange: (v: string) => void;
  rows?: number;
  placeholder?: string;
  right?: ReactNode;
}) {
  return (
    <label className="block">
      <span className="flex items-center justify-between gap-3">
        <span className="text-mk-h3 text-mk-ink">{label}</span>
        {right}
      </span>
      {hint ? <span className="mt-1 block text-mk-small text-mk-muted">{hint}</span> : null}
      <textarea
        rows={rows}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className={
          "mt-2 w-full resize-y rounded-mk-md border border-mk-input-border bg-mk-surface p-3 " +
          "text-mk-prose text-mk-ink outline-none transition-colors duration-[120ms] ease-mk " +
          "placeholder:text-mk-faint focus:border-mk-accent-300 focus:ring-2 focus:ring-mk-accent-100"
        }
      />
    </label>
  );
}

/**
 * `**bold**` only.
 *
 * Copy in this prototype is written by hand with one piece of markup: bold, to
 * name the thing that matters in a sentence (a real method name, the move to
 * steal). A full markdown renderer would be more machinery than that earns —
 * but printing the asterisks, which is what happened before this existed, is
 * worse than either.
 */
export function Bold({ text, className }: { text: string; className?: string }) {
  return (
    <>
      {text.split(/(\*\*[^*]+\*\*)/g).map((part, i) =>
        part.startsWith("**") && part.endsWith("**") ? (
          <strong key={i} className={className ?? "font-semibold text-mk-ink"}>
            {part.slice(2, -2)}
          </strong>
        ) : (
          <span key={i}>{part}</span>
        ),
      )}
    </>
  );
}

/**
 * A `?` that explains a number.
 *
 * Every readout in this product claims something about the student — 「关键词
 * 16」 is a statement about who she is. A number with no definition next to it
 * is either taken on faith or ignored, and neither is what we want, so any
 * figure that is DERIVED (rather than counted off the screen) carries one of
 * these saying exactly what went into it.
 *
 * Hover and focus both open it, and the body is real prose, not a label.
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
          className="eco-in absolute right-0 top-[22px] z-50 block w-[268px] rounded-mk-md p-3 text-left
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

/** Empty state that says what to do, not just that there is nothing. */
export function Empty({ title, body, action }: { title: string; body: string; action?: ReactNode }) {
  return (
    <div className="rounded-mk-lg border border-dashed border-mk-border px-6 py-10 text-center">
      <p className="text-mk-h3 text-mk-ink">{title}</p>
      <p className="mx-auto mt-2 max-w-[46ch] text-mk-body text-mk-secondary">{body}</p>
      {action ? <div className="mt-4 flex justify-center">{action}</div> : null}
    </div>
  );
}
