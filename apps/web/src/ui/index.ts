/**
 * ui/ barrel (design-system foundation, Part 1 Task 13).
 *
 * Single import surface for the whole design-system library: tokens, accent
 * theming, icons, and every primitive component. Every module keeps its own
 * *unexported* local `cx` class-merge helper, so there is no `cx` export here
 * and no name collision to resolve across modules.
 *
 * ONE real collision: `Icon.tsx` re-exports lucide-react's `Menu` (hamburger
 * icon) while `overlays.tsx` exports the `Menu` popover component. The
 * popover is the primitive consumers reach for by that name, so the lucide
 * icon is re-exported here under the explicit alias `MenuIcon`.
 */

export * from "./tokens";
export * from "./accent";
export * from "./cover";
export * from "./ProjectCover";
export type { LucideIcon } from "./Icon";
export {
  Icon,
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
  Loader2,
  MoreHorizontal,
  Menu as MenuIcon,
} from "./Icon";
export * from "./Button";
export * from "./Card";
export * from "./forms";
export * from "./feedback";
export * from "./overlays";
export * from "./Skeleton";
export * from "./Pebble";
export * from "./loaders";
export * from "./Illustration";
export * from "./SplitPane";
