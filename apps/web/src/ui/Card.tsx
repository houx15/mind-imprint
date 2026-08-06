import type { ComponentPropsWithoutRef, ElementType, ReactNode } from "react";

/**
 * Surface / Card / CompactRow (design-system foundation, Part 1 Task 5).
 *
 * `Surface` is the base elevation primitive — every "object" in the product
 * (card, sheet, row) is a `Surface` at some level (spec §3, §10). `Card` is
 * the object-card flavor (10px radius); `CompactRow` is the hairline list
 * row flavor used for long lists (40px thumb + title/meta + trailing).
 */

export type SurfaceLevel = "hairline" | "sm" | "md" | "lg";

const LEVELS: Record<SurfaceLevel, string> = {
  hairline: "shadow-mk-xs border border-mk-border",
  sm: "shadow-mk-sm",
  md: "shadow-mk-md",
  lg: "shadow-mk-lg",
};

const SURFACE_BASE = "bg-mk-surface rounded-mk-sm";

export type SurfaceProps<T extends ElementType = "div"> = {
  level?: SurfaceLevel;
  as?: T;
  className?: string;
  children?: ReactNode;
} & Omit<ComponentPropsWithoutRef<T>, "as" | "className" | "children">;

export function Surface<T extends ElementType = "div">({
  level = "hairline",
  as,
  className,
  children,
  ...rest
}: SurfaceProps<T>) {
  const Tag = (as ?? "div") as ElementType;
  return (
    <Tag
      className={[SURFACE_BASE, LEVELS[level], className].filter(Boolean).join(" ")}
      {...rest}
    >
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
    <Surface level="md" className={["rounded-mk-md", className].filter(Boolean).join(" ")}>
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
  return (
    <Surface
      level="hairline"
      onClick={onClick}
      className={[
        "flex w-full items-center gap-3 p-3 text-left",
        onClick ? "cursor-pointer" : "",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
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
