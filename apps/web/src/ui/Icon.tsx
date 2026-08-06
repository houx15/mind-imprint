import type { LucideIcon } from "lucide-react";
import type { SVGProps } from "react";

/**
 * Lucide icon wrapper (design-system foundation, Part 1 Task 3).
 *
 * Locks stroke weight to 1.75 across the product and gives a consistent
 * default size, while forwarding any other SVG props through to the
 * underlying lucide-react icon component.
 */
export function Icon({
  icon: LucideIconComponent,
  size = 20,
  ...rest
}: { icon: LucideIcon; size?: number } & SVGProps<SVGSVGElement>) {
  return <LucideIconComponent size={size} strokeWidth={1.75} {...rest} />;
}

// Re-export commonly used icons so consumers can import them alongside `Icon`
// from a single module.
export {
  Search,
  X,
  Check,
  ChevronDown,
  ChevronUp,
  ChevronLeft,
  ChevronRight,
  Plus,
  Minus,
  Settings,
  ArrowLeft,
  ArrowRight,
  Menu,
  Loader2,
} from "lucide-react";
export type { LucideIcon } from "lucide-react";
