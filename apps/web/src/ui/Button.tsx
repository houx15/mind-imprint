import type { ButtonHTMLAttributes, ReactNode } from "react";
import { Icon } from "@/ui/Icon";
import type { LucideIcon } from "@/ui/Icon";
import { PebbleInlineSpinner } from "@/ui/loaders";

/**
 * Button (design-system foundation, Part 1 Task 4).
 *
 * Variant/size classes are built entirely from Tailwind `mk-*` tokens (spec
 * §7) — no inline hex values. `loading` disables the button and swaps in an
 * inline spinner.
 */

export type ButtonVariant = "primary" | "secondary" | "ghost" | "link" | "danger";
export type ButtonSize = "md" | "sm";

const VARIANTS: Record<ButtonVariant, string> = {
  primary: "bg-mk-accent text-white hover:bg-mk-accent-600 active:bg-mk-accent-700",
  secondary: "bg-mk-surface text-mk-accent-700 border border-mk-accent hover:bg-mk-accent-50",
  ghost: "bg-transparent text-mk-ink hover:bg-mk-accent-50",
  link: "bg-transparent text-mk-accent-700 hover:underline px-0",
  danger: "bg-mk-danger text-white hover:brightness-95",
};

const SIZES: Record<ButtonSize, string> = {
  md: "text-mk-body px-[18px] py-[10px]",
  sm: "text-mk-small px-3 py-1.5",
};

const BASE =
  "inline-flex items-center justify-center gap-2 font-medium rounded-mk-sm " +
  "transition-colors duration-[120ms] ease-mk focus-visible:outline-none " +
  "focus-visible:ring-[3px] focus-visible:ring-mk-accent/15 " +
  "disabled:bg-[#F0E9E1] disabled:text-[#B8ADA2] disabled:cursor-not-allowed";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  iconStart?: ReactNode;
  iconEnd?: ReactNode;
  loading?: boolean;
}

export function Button({
  variant = "primary",
  size = "md",
  iconStart,
  iconEnd,
  loading = false,
  disabled,
  className,
  children,
  ...rest
}: ButtonProps) {
  const isDisabled = disabled || loading;
  return (
    <button
      type="button"
      className={[BASE, VARIANTS[variant], SIZES[size], className].filter(Boolean).join(" ")}
      disabled={isDisabled}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <PebbleInlineSpinner size={16} /> : iconStart}
      {children}
      {!loading && iconEnd}
    </button>
  );
}

export interface IconButtonProps {
  icon: LucideIcon;
  label: string;
  size?: ButtonSize;
  variant?: ButtonVariant;
  onClick?: ButtonHTMLAttributes<HTMLButtonElement>["onClick"];
  disabled?: boolean;
  className?: string;
}

const ICON_BUTTON_SIZES: Record<ButtonSize, string> = {
  md: "p-[10px]",
  sm: "p-1.5",
};

export function IconButton({
  icon,
  label,
  size = "md",
  variant = "ghost",
  onClick,
  disabled,
  className,
}: IconButtonProps) {
  return (
    <button
      type="button"
      aria-label={label}
      onClick={onClick}
      disabled={disabled}
      className={[
        BASE,
        VARIANTS[variant],
        ICON_BUTTON_SIZES[size],
        "aspect-square",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
    >
      <Icon icon={icon} size={size === "sm" ? 16 : 20} />
    </button>
  );
}
