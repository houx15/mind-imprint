import { useEffect, useRef, useState } from "react";

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export interface RulerSection {
  id: string;
  idx: string;
  label: string;
}

export interface RulerProps {
  sections: RulerSection[];
}

/**
 * Sticky 目录 ruler — ports `.ruler`/`.ruler ol`/`.ruler li` from
 * `docs/reference/2026-08-13-eval-report-mockup.html`. Scroll-sync mirrors
 * the mockup's `<script>` IntersectionObserver exactly (same
 * `rootMargin`/`threshold`); click scrolls the target section into view.
 *
 * Guards `typeof IntersectionObserver === "undefined"` (not polyfilled in
 * jsdom) so this renders safely in tests without crashing — it just skips
 * live scroll-sync there.
 */
export function Ruler({ sections }: RulerProps) {
  const [activeId, setActiveId] = useState<string>(sections[0]?.id ?? "");
  const sectionsRef = useRef(sections);
  sectionsRef.current = sections;

  useEffect(() => {
    if (typeof IntersectionObserver === "undefined") return;
    const els = sectionsRef.current
      .map((s) => document.getElementById(s.id))
      .filter((el): el is HTMLElement => el !== null);
    if (els.length === 0) return;

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) setActiveId(entry.target.id);
        }
      },
      { rootMargin: "-15% 0px -75% 0px", threshold: 0 },
    );
    els.forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, [sections]);

  function handleClick(id: string) {
    document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  return (
    <nav
      className="sticky top-[59px] h-[calc(100vh-59px)] w-56 shrink-0 overflow-auto py-8 pl-8 pr-3 max-[1040px]:hidden"
      aria-label="报告目录"
    >
      <ol className="relative list-none pl-4">
        <li aria-hidden className="pointer-events-none absolute bottom-2 left-[3px] top-2 w-0.5 rounded-full bg-mk-border" />
        {sections.map((s) => {
          const active = s.id === activeId;
          return (
            <li
              key={s.id}
              data-testid={`ruler-item-${s.id}`}
              onClick={() => handleClick(s.id)}
              className={cx(
                "relative cursor-pointer py-2.5 pl-3.5 text-mk-body transition-transform",
                active ? "origin-left scale-105 font-bold text-mk-ink" : "text-mk-faint",
              )}
            >
              <span
                aria-hidden
                className="absolute -left-3.5 top-1/2 -translate-y-1/2 rounded-full transition-all"
                style={{
                  width: active ? 11 : 7,
                  height: active ? 11 : 7,
                  background: active ? "var(--mk-accent)" : "var(--mk-input-border)",
                  boxShadow: active ? "0 0 0 4px var(--mk-accent-50)" : "none",
                }}
              />
              <span className={cx("mr-2 text-mk-small tabular-nums", active ? "text-mk-accent-600" : "text-mk-faint")}>
                {s.idx}
              </span>
              <span>{s.label}</span>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
