/**
 * Skeleton loaders (design-system foundation, Part 1 Task 9, spec §15).
 *
 * `.mk-skeleton` is a shimmer box (see `index.css` for the keyframe); this
 * module supplies the shape primitives that mimic real content while it
 * loads: a bare box (`Skeleton`), stacked ragged-edge text bars
 * (`SkeletonText`), and two composite shapes matching common list items
 * (`SkeletonCard`, `SkeletonRow`).
 */

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export type SkeletonRadius = "sm" | "md" | "full";

// Tailwind emits `rounded-mk-*` utilities in alphabetical class order in the
// compiled stylesheet, not theme-declaration order — so stacking two
// `rounded-mk-*` classes on one element does not resolve by which one
// appears later in `className`. Every skeleton shape below applies exactly
// ONE radius utility, ever (see Card.tsx for the same guard).
const RADII: Record<SkeletonRadius, string> = {
  sm: "rounded-mk-sm",
  md: "rounded-mk-md",
  full: "rounded-mk-full",
};

function toDimension(value: number | string | undefined): string | undefined {
  if (value === undefined) return undefined;
  return typeof value === "number" ? `${value}px` : value;
}

export interface SkeletonProps {
  /** Width: number = px, string = passed through verbatim (e.g. "60%"). */
  w?: number | string;
  /** Height: number = px, string = passed through verbatim. Defaults to ~14px. */
  h?: number | string;
  radius?: SkeletonRadius;
  className?: string;
}

/** A single shimmering box; the base primitive every other shape composes. */
export function Skeleton({ w, h, radius = "sm", className }: SkeletonProps) {
  const width = toDimension(w);
  const height = toDimension(h) ?? "14px";
  return (
    <div
      className={cx("mk-skeleton", RADII[radius], className)}
      style={{ width: width ?? "100%", height }}
    />
  );
}

export interface SkeletonTextProps {
  /** Number of stacked text bars. */
  lines?: number;
  className?: string;
}

/** Stacked text-line placeholders; the last bar is shorter for a natural ragged edge. */
export function SkeletonText({ lines = 3, className }: SkeletonTextProps) {
  return (
    <div className={cx("flex flex-col gap-2", className)}>
      {Array.from({ length: lines }, (_, i) => (
        <Skeleton key={i} h={10} w={i === lines - 1 ? "60%" : "100%"} />
      ))}
    </div>
  );
}

export interface SkeletonCardProps {
  className?: string;
}

/** Card-shaped skeleton: cover block + two text lines (mimics a project card). */
export function SkeletonCard({ className }: SkeletonCardProps) {
  return (
    <div className={cx("flex flex-col gap-3", className)}>
      <Skeleton h={120} radius="md" />
      <SkeletonText lines={2} />
    </div>
  );
}

export interface SkeletonRowProps {
  className?: string;
}

/** Compact-row skeleton: 40px square thumb + two short lines (mimics CompactRow). */
export function SkeletonRow({ className }: SkeletonRowProps) {
  return (
    <div className={cx("flex items-center gap-3", className)}>
      <Skeleton w={40} h={40} radius="sm" />
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <Skeleton h={10} w="70%" />
        <Skeleton h={10} w="40%" />
      </div>
    </div>
  );
}
