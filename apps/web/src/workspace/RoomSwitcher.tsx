// Room switcher (Task 8): the 5 room tabs are real navigation, not a hint —
// each renders an icon + a ≥14px label with legible inactive contrast. A
// purpose-built sibling to `Segmented` (ui/feedback.tsx) rather than an
// overload of it: `Segmented`'s 12px default / `text-mk-muted` inactive
// styling is correct for its other (hint-weight) callers, so it is left
// untouched and this component owns the tab-specific sizing instead.
import { Icon, BLOCK_META } from "./Icon";

function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export interface RoomSwitcherProps {
  value: string;
  onChange: (value: string) => void;
  className?: string;
}

export function RoomSwitcher({ value, onChange, className }: RoomSwitcherProps) {
  return (
    <div className={cx("inline-flex gap-0.5 rounded-mk-full bg-mk-paper p-1", className)}>
      {BLOCK_META.map((b) => {
        const active = b.key === value;
        return (
          <button
            key={b.key}
            type="button"
            aria-pressed={active}
            data-tour={`room-tab-${b.key}`}
            onClick={() => onChange(b.key)}
            className={cx(
              "flex items-center gap-1.5 rounded-mk-full px-3 py-1.5 transition-colors duration-[120ms] ease-mk",
              "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-mk-accent",
              // NOTE: `mk-*` colors resolve to `var(--mk-*)` (tailwind.config.ts) —
              // Tailwind's JIT cannot apply an opacity modifier to a CSS-variable
              // color, so `text-mk-ink/70` silently emits NO rule (same root
              // cause as the documented `bg-mk-<token>/<opacity>` gotcha). Use the
              // solid `text-mk-secondary` token instead — a real mid-tone color
              // that actually renders, legibly dimmer than active `text-mk-ink`.
              active ? "bg-mk-surface text-mk-ink shadow-mk-xs" : "bg-transparent text-mk-secondary",
            )}
          >
            <span aria-hidden="true" className="flex shrink-0 items-center">
              <Icon name={b.key} size={16} />
            </span>
            <span className="text-mk-body">{b.label}</span>
          </button>
        );
      })}
    </div>
  );
}
