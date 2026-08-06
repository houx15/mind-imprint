import type { ComponentPropsWithoutRef, ElementType, KeyboardEvent, ReactNode } from "react";

/**
 * Surface / Card / CompactRow (design-system foundation, Part 1 Task 5).
 *
 * `Surface` is the base elevation primitive — every "object" in the product
 * (card, sheet, row) is a `Surface` at some level (spec §3, §10). `Card` is
 * the object-card flavor (10px radius); `CompactRow` is the hairline list
 * row flavor used for long lists (40px thumb + title/meta + trailing).
 */

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export type SurfaceLevel = "hairline" | "sm" | "md" | "lg";
export type SurfaceRadius = "sm" | "md";

const LEVELS: Record<SurfaceLevel, string> = {
  hairline: "shadow-mk-xs border border-mk-border",
  sm: "shadow-mk-sm",
  md: "shadow-mk-md",
  lg: "shadow-mk-lg",
};

// Tailwind emits `rounded-mk-*` utilities in alphabetical class order in the
// compiled stylesheet (lg, md, sm) — NOT theme-declaration order — so two
// `rounded-mk-*` classes on the same element are NOT resolved by which one
// appears later in `className`; `rounded-mk-sm` always wins the cascade tie.
// Surface must therefore apply exactly ONE radius utility, ever.
const RADII: Record<SurfaceRadius, string> = {
  sm: "rounded-mk-sm",
  md: "rounded-mk-md",
};

const SURFACE_BASE = "bg-mk-surface";

export type SurfaceProps<T extends ElementType = "div"> = {
  level?: SurfaceLevel;
  radius?: SurfaceRadius;
  as?: T;
  className?: string;
  children?: ReactNode;
} & Omit<ComponentPropsWithoutRef<T>, "as" | "className" | "children">;

export function Surface<T extends ElementType = "div">({
  level = "hairline",
  radius = "sm",
  as,
  className,
  children,
  ...rest
}: SurfaceProps<T>) {
  const Tag = (as ?? "div") as ElementType;
  return (
    <Tag className={cx(SURFACE_BASE, RADII[radius], LEVELS[level], className)} {...rest}>
      {children}
    </Tag>
  );
}

export interface CardProps {
  className?: string;
  children?: ReactNode;
}

/** Object card: `Surface level="md"` with the 10px card radius. */
export function Card({ className, children }: CardProps) {
  return (
    <Surface level="md" radius="md" className={className}>
      {children}
    </Surface>
  );
}

export interface CompactRowProps {
  thumb?: ReactNode;
  title: ReactNode;
  meta?: ReactNode;
  trailing?: ReactNode;
  onClick?: () => void;
  className?: string;
}

/** Hairline row for long lists: optional 40px thumb + title/meta + trailing. */
export function CompactRow({ thumb, title, meta, trailing, onClick, className }: CompactRowProps) {
  const interactive = Boolean(onClick);
  return (
    <Surface
      level="hairline"
      onClick={onClick}
      role={interactive ? "button" : undefined}
      tabIndex={interactive ? 0 : undefined}
      onKeyDown={
        interactive
          ? (e: KeyboardEvent) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onClick?.();
              }
            }
          : undefined
      }
      className={cx("flex w-full items-center gap-3 p-3 text-left", interactive && "cursor-pointer", className)}
    >
      {thumb && (
        <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-mk-sm bg-mk-paper">
          {thumb}
        </div>
      )}
      <div className="min-w-0 flex-1">
        <div className="truncate text-mk-h3">{title}</div>
        {meta && <div className="truncate text-mk-small text-mk-muted">{meta}</div>}
      </div>
      {trailing && <div className="shrink-0">{trailing}</div>}
    </Surface>
  );
}
